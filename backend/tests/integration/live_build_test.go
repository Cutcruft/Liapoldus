package integrationtest

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/api/client"
	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/artifactstore"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/builder"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/shared"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
)

// liveBuildFixture materializes + builds a workspace, publishes it to the
// artifact root for both environments, installs the shared bundles next to it
// and returns a client router serving that root.
func liveBuildFixture(t *testing.T) (http.Handler, string) {
	t.Helper()
	mem := storage.NewMemory()
	site := seedBuildSiteInMemory(t, mem, "site_lb")
	ws := materializeTestWorkspace(t, mem, site.ID, t.TempDir()+"/ws")

	bl, err := builder.New().Build(context.Background(), ws)
	if err != nil {
		t.Fatalf("esbuild: %v", err)
	}
	root := t.TempDir()
	store := artifactstore.New(root)
	pub := build.Workspace{Dir: ws.Dir, Manifest: ws.Manifest}
	for _, env := range []string{domain.EnvironmentDevelopment, domain.EnvironmentProduction} {
		if _, err := store.Publish(context.Background(), site.ID, env, "snap-"+env, pub, bl); err != nil {
			t.Fatalf("publish %s: %v", env, err)
		}
	}
	if err := shared.Install(root); err != nil {
		t.Fatalf("install shared: %v", err)
	}

	app := &client.App{
		BuildDir: root,
		Logger:   slog.New(slog.DiscardHandler),
	}
	return client.NewRouter(app), site.ID
}

func TestLiveBuildServesPublishablePage(t *testing.T) {
	handler, siteID := liveBuildFixture(t)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	get := func(path string) (*http.Response, string) {
		resp, err := srv.Client().Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return resp, string(data)
	}

	for _, env := range []string{domain.EnvironmentDevelopment, domain.EnvironmentProduction} {
		shell := "/build/" + siteID + "/" + env + "/snap-" + env + "/dist/index.html"
		resp, data := get(shell)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("shell %s: status %d", env, resp.StatusCode)
		}
		for _, want := range []string{
			`<div id="root"></div>`,
			`<script type="module" src="./entry.js"></script>`,
			`<link rel="modulepreload" href="./pages/page_e2e.js">`,
			`"@liapoldus/ui-runtime":"/build/_shared/@liapoldus/ui-runtime/0.1.0.js"`,
			`"react":"/build/_shared/react/18.3.1.js"`,
		} {
			if !strings.Contains(data, want) {
				t.Fatalf("shell %s missing %q; body:\n%s", env, want, data)
			}
		}
		wantCache := "no-cache"
		if env == domain.EnvironmentProduction {
			wantCache = "public, max-age=31536000, immutable"
		}
		if got := resp.Header.Get("Cache-Control"); got != wantCache {
			t.Errorf("shell %s cache-control = %q, want %q", env, got, wantCache)
		}

		if _, entry := get(shell[:len(shell)-len("index.html")] + "entry.js"); entry == "" {
			t.Fatalf("entry.js for %s served empty", env)
		}
		if _, chunk := get(shell[:len(shell)-len("index.html")] + "pages/page_e2e.js"); chunk == "" {
			t.Fatalf("code-split page chunk for %s served empty", env)
		}
	}

	if resp, shared := get("/build/_shared/react/18.3.1.js"); shared == "" {
		t.Fatalf("shared react not served (status %d)", resp.StatusCode)
	} else if got := resp.Header.Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("shared cache-control = %q", got)
	}
}

func TestLiveBuildManifestServed(t *testing.T) {
	handler, siteID := liveBuildFixture(t)
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/build/" + siteID + "/" + domain.EnvironmentProduction + "/snap-production/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var manifest build.Manifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest.Shared["react"] != "/build/_shared/react/18.3.1.js" {
		t.Errorf("manifest shared = %#v", manifest.Shared)
	}
	if len(manifest.Pages) != 1 || manifest.Pages[0].PageID != "page_e2e" || manifest.Pages[0].Chunk != "pages/page_e2e.js" {
		t.Errorf("manifest pages = %#v", manifest.Pages)
	}
	if manifest.HomePage != "page_e2e" {
		t.Errorf("manifest home = %q, want page_e2e", manifest.HomePage)
	}
}
