package integrationtest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/builder"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/materializer"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/shared"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
)

// seedBuildSiteInMemory mirrors the unit helpers but for this package.
func seedBuildSiteInMemory(t *testing.T, mem *storage.Memory, id string) domain.Site {
	t.Helper()
	site := domain.Site{ID: id, Slug: "slug-" + id, Name: "E2E site", DefaultLocale: "ru", CreatedAt: time.Now().UTC()}
	if err := mem.CreateSite(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	return site
}

func seedDefinitionSource(t *testing.T, mem *storage.Memory, siteID, id, source string) {
	t.Helper()
	def := &domain.ComponentDefinition{
		SiteID: siteID, ID: id, Name: id, Kind: "component", Source: source,
		Schema: mustMap(`{"type":"object"}`), CurrentSHA: "sha_" + id,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := mem.Save(context.Background(), def); err != nil {
		t.Fatal(err)
	}
}

// materializeTestWorkspace seeds a site with two component definitions and one
// page, creates a snapshot and runs the real materializer into dir.
func materializeTestWorkspace(t *testing.T, mem *storage.Memory, siteID string, dir string) build.Workspace {
	t.Helper()
	ctx := context.Background()
	seedDefinitionSource(t, mem, siteID, "text",
		"import React from \"react\";\nexport default (props) => props?.title ?? null;\n")
	seedDefinitionSource(t, mem, siteID, "container",
		"import React from \"react\";\nexport default (props) => React.createElement(\"div\", null, props?.children ?? null);\n")

	root := domain.ComponentNode{
		InstanceID: "root", DefinitionID: "container",
		Children: []domain.ComponentNode{{InstanceID: "kids", DefinitionID: "text"}},
	}
	page := domain.Page{ID: "page_e2e", SiteID: siteID, Name: "Home", Slug: "index", Root: root, Version: 1,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	version := domain.PageVersion{ID: "pagever_e2e", PageID: "page_e2e", Number: 1, Root: root, CreatedAt: time.Now().UTC()}
	if err := mem.CreatePage(ctx, page, version); err != nil {
		t.Fatal(err)
	}
	snapshot := domain.Snapshot{ID: "snapshot_e2e", SiteID: siteID,
		Pages: []domain.SnapshotPage{{PageID: "page_e2e", VersionID: "pagever_e2e", Version: 1}}, CreatedAt: time.Now().UTC()}
	if err := mem.CreateSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}

	mat := materializer.New(mem, mem, mem, mem, shared.NewResolver(), nil, mem)
	ws, err := mat.Materialize(ctx, build.WorkspaceRequest{SiteID: siteID, SnapshotID: snapshot.ID, Environment: domain.EnvironmentDevelopment, Dir: dir})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	return ws
}

func readBundle(t *testing.T, distDir, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(distDir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return data
}

func TestEsbuildBuilderProducesBundleWithExternals(t *testing.T) {
	mem := storage.NewMemory()
	site := seedBuildSiteInMemory(t, mem, "site_bb")
	ws := materializeTestWorkspace(t, mem, site.ID, filepath.Join(t.TempDir(), "ws"))

	bl, err := builder.New().Build(context.Background(), ws)
	if err != nil {
		t.Fatalf("esbuild: %v", err)
	}
	pageChunk := string(readBundle(t, bl.DistDir, "pages/page_e2e.js"))
	entry := string(readBundle(t, bl.DistDir, "entry.js"))

	// Externals stay as runtime imports instead of being inlined: the root
	// entry only boots (mount), the code-split page chunk carries the
	// registered definitions + page tree.
	for _, external := range []string{"react", "@liapoldus/ui-runtime"} {
		if !strings.Contains(pageChunk, `"`+external+`"`) {
			t.Fatalf("page chunk must keep external import for %s; got:\n%s", external, pageChunk[:min(len(pageChunk), 400)])
		}
	}
	if !strings.Contains(entry, `"@liapoldus/ui-runtime"`) {
		t.Fatalf("root entry must import ui-runtime; got:\n%s", entry)
	}
	if strings.Contains(entry, "ComponentRegistry") || strings.Contains(entry, "registerPage") {
		t.Fatalf("root entry must be boot-only (splitting): %s", entry)
	}
	// The site source made it into the page chunk: registration + page tree.
	if !strings.Contains(pageChunk, "registerDefinition") || !strings.Contains(pageChunk, "registerPage") {
		t.Fatalf("page chunk must register its content; got:\n%s", pageChunk[:min(len(pageChunk), 400)])
	}
	if !strings.Contains(entry, "site_bb") {
		t.Fatalf("root entry must call mount with the site id; got:\n%s", entry)
	}
	// The manifest carries an import map for the shared externals.
	if len(ws.Manifest.Externals) != len(build.SharedExternals) {
		t.Fatalf("externals = %#v", ws.Manifest.Externals)
	}
	if ws.Manifest.Shared["react"] != "/build/_shared/react/18.3.1.js" {
		t.Fatalf("manifest import map = %#v", ws.Manifest.Shared)
	}
	if len(ws.Manifest.Pages) != 1 || ws.Manifest.Pages[0].Chunk != "pages/page_e2e.js" {
		t.Fatalf("manifest pages = %#v", ws.Manifest.Pages)
	}
	// The runtime shell (dist/index.html) is written for the published artifact
	// and modulepreloads the home page chunk (first screen, no round-trip).
	shell, err := os.ReadFile(filepath.Join(bl.DistDir, "index.html"))
	if err != nil {
		t.Fatalf("read dist/index.html: %v", err)
	}
	for _, want := range []string{`<div id="root"></div>`, `src="./entry.js"`,
		`"react":"/build/_shared/react/18.3.1.js"`,
		`<link rel="modulepreload" href="./pages/page_e2e.js">`} {
		if !strings.Contains(string(shell), want) {
			t.Fatalf("shell missing %q; got:\n%s", want, shell)
		}
	}
}

func TestEsbuildBuilderFailsOnSyntaxError(t *testing.T) {
	mem := storage.NewMemory()
	site := seedBuildSiteInMemory(t, mem, "site_be")
	// Override the container source with a syntax error after materialization.
	ws := materializeTestWorkspace(t, mem, site.ID, filepath.Join(t.TempDir(), "ws"))
	bad := "export default (props => { return props..title; };\n"
	if err := os.WriteFile(filepath.Join(ws.Dir, "src", "definitions", "container.tsx"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := builder.New().Build(context.Background(), ws)
	if err == nil || !strings.Contains(err.Error(), "esbuild failed") {
		t.Fatalf("error = %v, want esbuild failure", err)
	}
	if _, statErr := os.Stat(filepath.Join(ws.Dir, "dist")); !os.IsNotExist(statErr) {
		t.Fatalf("dist must not be created on failure, stat error = %v", statErr)
	}
}

func TestEsbuildIncrementalRebuildSmoke(t *testing.T) {
	mem := storage.NewMemory()
	site := seedBuildSiteInMemory(t, mem, "site_inc")
	ws := materializeTestWorkspace(t, mem, site.ID, filepath.Join(t.TempDir(), "ws"))

	options := api.BuildOptions{
		EntryPoints: []string{
			filepath.Join(ws.Dir, "src", "entry.tsx"),
			filepath.Join(ws.Dir, "src", "pages", "page_e2e.tsx"),
		},
		Outdir:    filepath.Join(ws.Dir, "dist"),
		Bundle:    true,
		Write:     true,
		Splitting: true,
		Format:    api.FormatESModule,
		Platform:  api.PlatformBrowser,
		External:  build.SharedExternals,
		LogLevel:  api.LogLevelSilent,
	}
	ctx, ctxErr := api.Context(options)
	if ctxErr != nil {
		t.Fatalf("esbuild context: %v", ctxErr.Errors)
	}
	defer ctx.Dispose()

	first := ctx.Rebuild()
	if len(first.Errors) > 0 {
		t.Fatalf("initial build errors: %v", first.Errors)
	}

	// Touch a definition source and rebuild without re-creating the workspace.
	textFile := filepath.Join(ws.Dir, "src", "definitions", "text.tsx")
	newSource := "import React from \"react\";\nexport default (props) => props?.title?.toUpperCase() ?? null;\n"
	if err := os.WriteFile(textFile, []byte(newSource), 0o644); err != nil {
		t.Fatal(err)
	}
	second := ctx.Rebuild()
	if len(second.Errors) > 0 {
		t.Fatalf("rebuild errors: %v", second.Errors)
	}
	bundle := readBundle(t, filepath.Join(ws.Dir, "dist"), "pages/page_e2e.js")
	if !strings.Contains(string(bundle), "toUpperCase") {
		t.Fatalf("rebuild must pick up the edited source")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
