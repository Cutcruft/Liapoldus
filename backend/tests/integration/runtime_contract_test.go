package integrationtest

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/api/client"
	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/application/form"
	"github.com/liapoldus/liapoldus/backend/internal/application/route"
	appruntime "github.com/liapoldus/liapoldus/backend/internal/application/runtime"
	siteapp "github.com/liapoldus/liapoldus/backend/internal/application/site"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/artifactstore"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/builder"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/materializer"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
)

// runtimeContractFixture seeds a site with one page + snapshot and a published
// production build, plus a renderPage route for "/" pointing at the page.
func runtimeContractFixture(t *testing.T, siteID string) (*storage.Memory, domain.Site, domain.Snapshot, *build.Service) {
	t.Helper()
	ctx := context.Background()
	mem := storage.NewMemory()
	site := seedBuildSiteInMemory(t, mem, siteID)

	seedDefinitionSource(t, mem, site.ID, "container",
		"import React from \"react\";\nexport default (props) => React.createElement(\"div\", null, props?.children ?? null);\n")
	seedDefinitionSource(t, mem, site.ID, "text",
		"import React from \"react\";\nexport default (props) => props?.title ?? null;\n")

	root := domain.ComponentNode{
		InstanceID: "root", DefinitionID: "container", Props: map[string]any{"gap": 12},
		Children: []domain.ComponentNode{{InstanceID: "title", DefinitionID: "text", Props: map[string]any{"title": "Hello"}}},
	}
	page := domain.Page{ID: "page_boot", SiteID: site.ID, Name: "Home", Slug: "index", Root: root, Version: 1,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	version := domain.PageVersion{ID: "pagever_boot", PageID: page.ID, Number: 1, Root: root, CreatedAt: time.Now().UTC()}
	if err := mem.CreatePage(ctx, page, version); err != nil {
		t.Fatal(err)
	}
	snapshot := domain.Snapshot{ID: "snapshot_boot", SiteID: site.ID,
		Pages: []domain.SnapshotPage{{PageID: page.ID, VersionID: version.ID, Version: 1}}, CreatedAt: time.Now().UTC()}
	if err := mem.CreateSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}

	routes := route.NewService(mem, route.Settings{DefaultStatus: 301, Allowed: map[int]bool{301: true}})
	if _, err := routes.Create(ctx, site.ID, "^/$", 10, domain.RouteAction{Type: "renderPage", PageID: page.ID}); err != nil {
		t.Fatal(err)
	}

	artifacts := artifactstore.New(filepath.Join(t.TempDir(), "build"))
	builds := build.NewService(mem, mem, mem, materializer.New(mem, mem, mem, mem, nil, nil, mem), builder.New(), artifacts)
	if _, err := builds.Create(ctx, site.ID, snapshot.ID, domain.EnvironmentProduction); err != nil {
		t.Fatalf("publish production build: %v", err)
	}
	return mem, site, snapshot, builds
}

func runtimeClientServer(t *testing.T, mem *storage.Memory, routes *route.Service, builds *build.Service) *httptest.Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	rt := appruntime.NewService(mem, mem, mem, routes, builds)
	handler := client.NewRouter(&client.App{
		Sites:   siteapp.NewService(mem, siteapp.Settings{DefaultLocale: "ru"}),
		Routes:  routes,
		Forms:   form.NewService(mem, mem, form.Settings{}),
		Runtime: rt,
		Logger:  logger,
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func getContractJSON(t *testing.T, url, siteID, environment, versionID string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest("GET", url+"/runtime/contract", nil)
	if err != nil {
		t.Fatal(err)
	}
	q := req.URL.Query()
	q.Set("siteId", siteID)
	if environment != "" {
		q.Set("environment", environment)
	}
	if versionID != "" {
		q.Set("versionId", versionID)
	}
	req.URL.RawQuery = q.Encode()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("contract is not JSON: %v\n%s", err, body)
	}
	return res.StatusCode, out
}

func TestRuntimeContractPinsBootRelease(t *testing.T) {
	mem, site, snapshot, builds := runtimeContractFixture(t, "site_rc1")
	routes := route.NewService(mem, route.Settings{DefaultStatus: 301, Allowed: map[int]bool{301: true}})
	srv := runtimeClientServer(t, mem, routes, builds)

	status, body := getContractJSON(t, srv.URL, site.ID, "production", "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, body %#v", status, body)
	}
	if body["siteId"] != site.ID || body["environment"] != "production" ||
		body["version"] != snapshot.ID || body["locale"] != site.DefaultLocale {
		t.Fatalf("contract meta = %#v", body)
	}
	if providers, ok := body["providers"].([]any); !ok || len(providers) != 0 {
		t.Fatalf("providers = %#v", body["providers"])
	}
	routesArr, _ := body["routes"].([]any)
	if len(routesArr) != 1 {
		t.Fatalf("routes = %#v", body["routes"])
	}
	home := routesArr[0].(map[string]any)
	if home["matcher"] != "^/$" {
		t.Fatalf("home route = %#v", home)
	}
	tree, _ := body["tree"].(map[string]any)
	if tree["snapshotId"] != snapshot.ID {
		t.Fatalf("tree = %#v", tree)
	}
	homeAction, _ := home["action"].(map[string]any)
	if tree["pageId"] != homeAction["pageId"] {
		t.Fatalf("tree pageId = %#v, want %v", tree["pageId"], homeAction["pageId"])
	}
	root, _ := tree["root"].(map[string]any)
	if root["instanceId"] != "root" || root["definitionId"] != "container" {
		t.Fatalf("tree root = %#v", root)
	}
	caps, _ := body["capabilities"].(map[string]any)
	if caps["dev"] != false || caps["formSubmissions"] != true {
		t.Fatalf("capabilities = %#v", caps)
	}
}

func TestRuntimeContractVersionIdOverridesEnvironment(t *testing.T) {
	mem := storage.NewMemory()
	ctx := context.Background()

	seedDefinitionSource(t, mem, "site_rc2", "container",
		"import React from \"react\";\nexport default (props) => React.createElement(\"div\", null, props?.children ?? null);\n")
	if err := mem.CreateSite(ctx, domain.Site{ID: "site_rc2", Slug: "site-rc2", Name: "S2", DefaultLocale: "ru", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	rootA := domain.ComponentNode{InstanceID: "rootA", DefinitionID: "container"}
	pageA := domain.Page{ID: "page_a", SiteID: "site_rc2", Slug: "a", Root: rootA, Version: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := mem.CreatePage(ctx, pageA, domain.PageVersion{ID: "pagever_a", PageID: "page_a", Number: 1, Root: rootA}); err != nil {
		t.Fatal(err)
	}
	snapA := domain.Snapshot{ID: "snapshot_a", SiteID: "site_rc2", Pages: []domain.SnapshotPage{{PageID: "page_a", VersionID: "pagever_a", Version: 1}}, CreatedAt: time.Now().UTC()}
	if err := mem.CreateSnapshot(ctx, snapA); err != nil {
		t.Fatal(err)
	}
	rootB := domain.ComponentNode{InstanceID: "rootB", DefinitionID: "container"}
	pageB := domain.Page{ID: "page_b", SiteID: "site_rc2", Slug: "b", Root: rootB, Version: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := mem.CreatePage(ctx, pageB, domain.PageVersion{ID: "pagever_b", PageID: "page_b", Number: 1, Root: rootB}); err != nil {
		t.Fatal(err)
	}
	snapB := domain.Snapshot{ID: "snapshot_b", SiteID: "site_rc2", Pages: []domain.SnapshotPage{{PageID: "page_b", VersionID: "pagever_b", Version: 1}}, CreatedAt: time.Now().UTC()}
	if err := mem.CreateSnapshot(ctx, snapB); err != nil {
		t.Fatal(err)
	}

	// Only snapshot_a is published (production). snapshot_b must still be
	// reachable by explicit versionId.
	artifacts := artifactstore.New(filepath.Join(t.TempDir(), "build"))
	builds := build.NewService(mem, mem, mem, materializer.New(mem, mem, mem, mem, nil, nil, mem), builder.New(), artifacts)
	if _, err := builds.Create(ctx, "site_rc2", snapA.ID, domain.EnvironmentProduction); err != nil {
		t.Fatalf("publish: %v", err)
	}

	routes := route.NewService(mem, route.Settings{DefaultStatus: 301, Allowed: map[int]bool{301: true}})
	siteSvc := siteapp.NewService(mem, siteapp.Settings{DefaultLocale: "ru"})
	rt := appruntime.NewService(mem, mem, mem, routes, builds)
	srv := httptest.NewServer(client.NewRouter(&client.App{
		Sites: siteSvc, Routes: routes, Forms: form.NewService(mem, mem, form.Settings{}),
		Runtime: rt, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}))
	defer srv.Close()

	status, body := getContractJSON(t, srv.URL, "site_rc2", "", snapB.ID)
	if status != http.StatusOK || body["version"] != snapB.ID {
		t.Fatalf("versionId override failed: status %d body %#v", status, body)
	}
	tree := body["tree"].(map[string]any)
	if tree["root"].(map[string]any)["instanceId"] != "rootB" {
		t.Fatalf("tree root = %#v", tree["root"])
	}

	// A versionId from another site is not found.
	if status, _ := getContractJSON(t, srv.URL, "site_rc2", "", "snapshot_foreign"); status != http.StatusNotFound {
		t.Fatalf("foreign versionId status = %d", status)
	}
}

func TestRuntimeContractErrors(t *testing.T) {
	mem, site, _, builds := runtimeContractFixture(t, "site_rc3")
	routes := route.NewService(mem, route.Settings{DefaultStatus: 301, Allowed: map[int]bool{301: true}})
	srv := runtimeClientServer(t, mem, routes, builds)

	if status, _ := getContractJSON(t, srv.URL, site.ID, "staging", ""); status != http.StatusBadRequest {
		t.Fatalf("bad environment status = %d", status)
	}

	// A bare client server with no published build → 404.
	empty := storage.NewMemory()
	rt := appruntime.NewService(empty, empty, empty, routes,
		build.NewService(empty, empty, empty, nil, nil, artifactstore.New(filepath.Join(t.TempDir(), "build"))))
	other := httptest.NewServer(client.NewRouter(&client.App{
		Sites: siteapp.NewService(empty, siteapp.Settings{}), Routes: routes,
		Forms:   form.NewService(empty, empty, form.Settings{}),
		Runtime: rt, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}))
	defer other.Close()
	if status, _ := getContractJSON(t, other.URL, site.ID, "production", ""); status != http.StatusNotFound {
		t.Fatalf("unpublished status = %d", status)
	}
}

type bootProbe struct {
	SiteID               string `json:"siteId"`
	Environment          string `json:"environment"`
	Ready                bool   `json:"ready"`
	Locale               string `json:"locale"`
	HasContentOp         bool   `json:"hasContentOp"`
	HomePageID           string `json:"homePageId"`
	TreeRootInstanceID   string `json:"treeRootInstanceId"`
	TreeRootDefinitionID string `json:"treeRootDefinitionId"`
}

// TestBootStandaloneScript runs the real ui-runtime boot() (core bundle, no
// react) against a live contract served by an httptest client server — the
// автономный boot-скрипт from the Этап 4 spec.
func TestBootStandaloneScript(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not available")
	}
	mem, site, _, builds := runtimeContractFixture(t, "site_boot")
	routes := route.NewService(mem, route.Settings{DefaultStatus: 301, Allowed: map[int]bool{301: true}})
	srv := runtimeClientServer(t, mem, routes, builds)

	script := filepath.Join("fixtures", "boot.mjs")
	cmd := exec.Command("node", script, srv.URL, site.ID, domain.EnvironmentProduction)
	out, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = exitErr.Stderr
		}
		t.Fatalf("boot script failed: %v\n%s", err, strings.TrimSpace(string(stderr)))
	}
	var probe bootProbe
	if err := json.Unmarshal(out, &probe); err != nil {
		t.Fatalf("probe is not JSON: %v\n%s", err, out)
	}
	if !probe.Ready || probe.SiteID != site.ID || probe.Environment != "production" {
		t.Fatalf("boot probe = %#v", probe)
	}
	if !probe.HasContentOp {
		t.Fatalf("builtin operations not registered: %#v", probe)
	}
	if probe.HomePageID != "page_boot" {
		t.Fatalf("home page = %q, want page_boot", probe.HomePageID)
	}
	if probe.TreeRootInstanceID != "root" || probe.TreeRootDefinitionID != "container" {
		t.Fatalf("tree root = %#v", probe)
	}
	if probe.Locale == "" {
		t.Fatalf("locale missing: %#v", probe)
	}
}
