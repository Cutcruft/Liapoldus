package unit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/materializer"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
)

func mustRawMap(raw string) map[string]any {
	m := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		panic("bad fixture: " + raw)
	}
	return m
}

func seedDef(t *testing.T, mem *storage.Memory, siteID, id string, source string, sha string) {
	t.Helper()
	def := &domain.ComponentDefinition{
		SiteID: siteID, ID: id, Name: id, Kind: "component",
		Source: source, Schema: mustRawMap(`{"type":"object"}`), CurrentSHA: sha,
		CreatedAt: testNow(), UpdatedAt: testNow(),
	}
	if err := mem.Save(context.Background(), def); err != nil {
		t.Fatal(err)
	}
}

func seedMaterializedPage(t *testing.T, mem *storage.Memory, siteID, pageID string) {
	t.Helper()
	list := []domain.Element{
		{ID: "root", ComponentID: "container", Props: map[string]domain.ElementProp{}},
		{ID: "kids", ComponentID: "text", Props: map[string]domain.ElementProp{}},
		{ID: "hidden", ComponentID: "hero", Props: map[string]domain.ElementProp{}},
	}
	page := domain.Page{ID: pageID, SiteID: siteID, Name: "Home", Slug: "index", List: list, Version: 1,
		CreatedAt: testNow(), UpdatedAt: testNow()}
	version := domain.PageVersion{ID: "pagever_1", PageID: pageID, Number: 1, List: list, CreatedAt: testNow()}
	if err := mem.CreatePage(context.Background(), page, version); err != nil {
		t.Fatal(err)
	}
}

func materializerHarness(t *testing.T) (*storage.Memory, *materializer.Materializer) {
	t.Helper()
	mem := storage.NewMemory()
	mat := materializer.New(mem, mem, mem, mem, nil, nil, mem)
	return mem, mat
}

func TestMaterializeCreatesWorkspace(t *testing.T) {
	ctx := context.Background()
	mem, mat := materializerHarness(t)
	site := seedBuildSite(t, mem)
	seedDef(t, mem, site.ID, "text", "export default (props) => props.title ?? null;\n", "sha_text")
	seedDef(t, mem, site.ID, "container", "export default (props) => props.children;\n", "sha_cont")
	seedDef(t, mem, site.ID, "hero", "export default (props) => props.title;\n", "sha_hero")
	seedMaterializedPage(t, mem, site.ID, "page_1")
	snapshot := domain.Snapshot{ID: "snapshot_m1", SiteID: site.ID,
		Pages: []domain.SnapshotPage{{PageID: "page_1", VersionID: "pagever_1", Version: 1}}, CreatedAt: testNow()}
	if err := mem.CreateSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}

	wsDir := filepath.Join(t.TempDir(), "ws")
	ws, err := mat.Materialize(ctx, build.WorkspaceRequest{
		SiteID: site.ID, SnapshotID: snapshot.ID, Environment: domain.EnvironmentDevelopment, Dir: wsDir,
	})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}

	// Files on disk.
	entry, err := os.ReadFile(filepath.Join(ws.Dir, "src", "entry.tsx"))
	if err != nil {
		t.Fatalf("read entry: %v", err)
	}
	for _, id := range []string{"container", "text", "hero"} {
		path := filepath.Join(ws.Dir, "src", "definitions", id+".tsx")
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("definition %s missing: %v", id, err)
		}
	}
	text, err := os.ReadFile(filepath.Join(ws.Dir, "src", "definitions", "text.tsx"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "props.title") {
		t.Fatalf("definition content = %q", text)
	}

	// entry.tsx is the split boot stub: mount() only — definitions and page
	// trees live in code-split page chunks.
	content := string(entry)
	if !strings.Contains(content, `import { mount } from "@liapoldus/ui-runtime";`) {
		t.Fatalf("entry must import ui-runtime mount, got: %s", content)
	}
	if strings.Contains(content, "ComponentRegistry") || strings.Contains(content, "registerDefinition") {
		t.Fatalf("root entry must not register definitions (they live in page chunks): %s", content)
	}
	if strings.Contains(content, "registerPage") {
		t.Fatalf("root entry must not carry page trees: %s", content)
	}
	if !strings.Contains(content, `void mount("`+site.ID+`", "`+domain.EnvironmentDevelopment+`");`) {
		t.Fatalf("entry must call mount: %s", content)
	}

	// The page chunk registers the page's own definitions and its tree.
	pageChunk, err := os.ReadFile(filepath.Join(ws.Dir, "src", "pages", "page_1.tsx"))
	if err != nil {
		t.Fatalf("read page chunk: %v", err)
	}
	chunk := string(pageChunk)
	if !strings.Contains(chunk, `ComponentRegistry.registerDefinition("container", def_container);`) ||
		!strings.Contains(chunk, `ComponentRegistry.registerDefinition("text", def_text);`) ||
		!strings.Contains(chunk, `import def_container from "../definitions/container";`) {
		t.Fatalf("page chunk must register its definitions: %s", chunk)
	}
	if !strings.Contains(chunk, `registerPage("page_1"`) {
		t.Fatalf("page chunk must hand the tree to registerPage: %s", chunk)
	}
	if !strings.Contains(chunk, `"snapshotId":"snapshot_m1"`) || !strings.Contains(chunk, `"elements":[`) {
		t.Fatalf("page chunk elements JSON missing: %s", chunk)
	}

	// manifest.json on disk matches the returned manifest.
	manifestData, err := os.ReadFile(filepath.Join(ws.Dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifestData), `"sha_text"`) || !strings.Contains(string(manifestData), `"container"`) {
		t.Fatalf("manifest.json missing refs: %s", manifestData)
	}
	if len(ws.Manifest.Definitions) != 3 {
		t.Fatalf("manifest definitions = %d, want 3 (hero/text/container)", len(ws.Manifest.Definitions))
	}
	if ws.Manifest.Definitions["text"].SHA != "sha_text" {
		t.Fatalf("text sha = %q", ws.Manifest.Definitions["text"].SHA)
	}
	if len(ws.Manifest.Pages) != 1 {
		t.Fatalf("manifest pages = %#v", ws.Manifest.Pages)
	}
	if p := ws.Manifest.Pages[0]; p.PageID != "page_1" || p.Chunk != "pages/page_1.js" || len(p.Definitions) != 3 {
		t.Fatalf("manifest page ref = %#v", p)
	}
	if ws.Manifest.HomePage != "page_1" {
		t.Fatalf("manifest home = %q, want page_1 (fallback, no routes)", ws.Manifest.HomePage)
	}
	if len(ws.Manifest.Externals) != 4 {
		t.Fatalf("externals = %#v", ws.Manifest.Externals)
	}
	if ws.Manifest.Shared != nil {
		t.Fatalf("manifest must not carry a shared import map without a resolver: %#v", ws.Manifest.Shared)
	}
}

type resolverStub struct{ urls map[string]string }

func (r resolverStub) SharedURLs() map[string]string { return r.urls }

func TestMaterializeManifestSharedImportMap(t *testing.T) {
	ctx := context.Background()
	mem := storage.NewMemory()
	mat := materializer.New(mem, mem, mem, mem, resolverStub{urls: map[string]string{
		"react":                 "/build/_shared/react/18.3.1.js",
		"@liapoldus/ui-runtime": "/build/_shared/@liapoldus/ui-runtime/0.1.0.js",
	}}, nil, mem)
	site := seedBuildSite(t, mem)
	seedDef(t, mem, site.ID, "text", "export default (props) => props.title ?? null;\n", "sha_text")
	seedDef(t, mem, site.ID, "container", "export default (props) => props.children;\n", "sha_cont")
	seedDef(t, mem, site.ID, "hero", "export default (props) => props.title;\n", "sha_hero")
	seedMaterializedPage(t, mem, site.ID, "page_1")
	snapshot := domain.Snapshot{ID: "snapshot_shared", SiteID: site.ID,
		Pages: []domain.SnapshotPage{{PageID: "page_1", VersionID: "pagever_1", Version: 1}}, CreatedAt: testNow()}
	if err := mem.CreateSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}

	ws, err := mat.Materialize(ctx, build.WorkspaceRequest{
		SiteID: site.ID, SnapshotID: snapshot.ID, Environment: domain.EnvironmentDevelopment, Dir: filepath.Join(t.TempDir(), "ws"),
	})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if ws.Manifest.Shared["react"] != "/build/_shared/react/18.3.1.js" {
		t.Fatalf("shared import map = %#v", ws.Manifest.Shared)
	}
	if ws.Manifest.Shared["@liapoldus/ui-runtime"] != "/build/_shared/@liapoldus/ui-runtime/0.1.0.js" {
		t.Fatalf("shared import map = %#v", ws.Manifest.Shared)
	}
	manifestData, err := os.ReadFile(filepath.Join(ws.Dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifestData), `"shared"`) {
		t.Fatalf("manifest.json must persist the import map: %s", manifestData)
	}
}

func TestMaterializeDeterministic(t *testing.T) {
	ctx := context.Background()
	mem, mat := materializerHarness(t)
	site := seedBuildSite(t, mem)
	seedDef(t, mem, site.ID, "text", "export default (p) => p.title;\n", "sha_text")
	seedDef(t, mem, site.ID, "container", "export default (p) => p.children;\n", "sha_cont")
	seedDef(t, mem, site.ID, "hero", "export default (p) => p.title;\n", "sha_hero")
	seedMaterializedPage(t, mem, site.ID, "page_1")
	snapshot := domain.Snapshot{ID: "snapshot_m2", SiteID: site.ID,
		Pages: []domain.SnapshotPage{{PageID: "page_1", VersionID: "pagever_1", Version: 1}}, CreatedAt: testNow()}
	if err := mem.CreateSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}

	first, err := mat.Materialize(ctx, build.WorkspaceRequest{
		SiteID: site.ID, SnapshotID: snapshot.ID, Environment: domain.EnvironmentDevelopment, Dir: filepath.Join(t.TempDir(), "ws1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := mat.Materialize(ctx, build.WorkspaceRequest{
		SiteID: site.ID, SnapshotID: snapshot.ID, Environment: domain.EnvironmentDevelopment, Dir: filepath.Join(t.TempDir(), "ws2"),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"src/entry.tsx", "manifest.json", "src/definitions/text.tsx"} {
		a, _ := os.ReadFile(filepath.Join(first.Dir, filepath.FromSlash(name)))
		b, _ := os.ReadFile(filepath.Join(second.Dir, filepath.FromSlash(name)))
		if string(a) != string(b) {
			t.Fatalf("%s differs between materializations", name)
		}
	}
}

func TestMaterializeErrors(t *testing.T) {
	ctx := context.Background()
	mem, mat := materializerHarness(t)
	site := seedBuildSite(t, mem)
	seedMaterializedPage(t, mem, site.ID, "page_1")
	snapshot := domain.Snapshot{ID: "snapshot_m3", SiteID: site.ID,
		Pages: []domain.SnapshotPage{{PageID: "page_1", VersionID: "pagever_1", Version: 1}}, CreatedAt: testNow()}
	if err := mem.CreateSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	// definitionId "hero" has no registry entry nor source file entry for
	// container → tree references an unknown definition (container registered
	// but has empty source).

	seedDef(t, mem, site.ID, "container", "export default (p) => p.children;\n", "sha_c")

	// Unknown definition referenced by the element list.
	snapshot2 := domain.Snapshot{ID: "snapshot_m3b", SiteID: site.ID, CreatedAt: testNow()}
	_ = snapshot2
	root := []domain.Element{{ID: "root", ComponentID: "missing", Props: map[string]domain.ElementProp{}}}
	page := domain.Page{ID: "page_2", SiteID: site.ID, Name: "Miss", Slug: "miss", List: root, Version: 1, CreatedAt: testNow(), UpdatedAt: testNow()}
	version := domain.PageVersion{ID: "pagever_2", PageID: "page_2", Number: 1, List: root, CreatedAt: testNow()}
	if err := mem.CreatePage(ctx, page, version); err != nil {
		t.Fatal(err)
	}
	snap := domain.Snapshot{ID: "snapshot_missing", SiteID: site.ID,
		Pages: []domain.SnapshotPage{{PageID: "page_2", VersionID: "pagever_2", Version: 1}}, CreatedAt: testNow()}
	if err := mem.CreateSnapshot(ctx, snap); err != nil {
		t.Fatal(err)
	}
	wsDir := filepath.Join(t.TempDir(), "ws-missing")
	if _, err := mat.Materialize(ctx, build.WorkspaceRequest{SiteID: site.ID, SnapshotID: snap.ID, Environment: domain.EnvironmentDevelopment, Dir: wsDir}); err == nil {
		t.Fatalf("unknown definition must fail materialization")
	}
	if _, err := os.Stat(wsDir); !os.IsNotExist(err) {
		t.Fatalf("workspace dir must be cleaned up on failure, stat error = %v", err)
	}

	// Foreign snapshot.
	if _, err := mat.Materialize(ctx, build.WorkspaceRequest{SiteID: site.ID, SnapshotID: "snapshot_nope", Environment: domain.EnvironmentDevelopment, Dir: filepath.Join(t.TempDir(), "ws-nope")}); err == nil {
		t.Fatalf("missing snapshot must fail")
	}

	// Empty source is invalid.
	seedDef(t, mem, site.ID, "text2", "", "sha_x")
	snapEmpty := domain.Snapshot{ID: "snapshot_m4", SiteID: site.ID,
		Pages: []domain.SnapshotPage{{PageID: "page_1", VersionID: "pagever_1", Version: 1}}, CreatedAt: testNow()}
	if err := mem.CreateSnapshot(ctx, snapEmpty); err != nil {
		t.Fatal(err)
	}
	heroRoot := []domain.Element{{ID: "r", ComponentID: "text2", Props: map[string]domain.ElementProp{}}}
	page3 := domain.Page{ID: "page_3", SiteID: site.ID, Name: "P", Slug: "p", List: heroRoot, Version: 1, CreatedAt: testNow(), UpdatedAt: testNow()}
	version3 := domain.PageVersion{ID: "pagever_3", PageID: "page_3", Number: 1, List: heroRoot, CreatedAt: testNow()}
	if err := mem.CreatePage(ctx, page3, version3); err != nil {
		t.Fatal(err)
	}
	if err := mem.CreateSnapshot(ctx, domain.Snapshot{ID: "snapshot_m5", SiteID: site.ID,
		Pages: []domain.SnapshotPage{{PageID: "page_3", VersionID: "pagever_3", Version: 1}}, CreatedAt: testNow()}); err != nil {
		t.Fatal(err)
	}
	if _, err := mat.Materialize(ctx, build.WorkspaceRequest{SiteID: site.ID, SnapshotID: "snapshot_m5", Environment: domain.EnvironmentDevelopment, Dir: filepath.Join(t.TempDir(), "ws-empty")}); err == nil {
		t.Fatalf("empty source must fail materialization")
	}
}

// recordingLayouter captures the DepLayoutRequest and returns a canned
// import-map, replacing the real esbuild layout in materializer tests.
type recordingLayouter struct{ req build.DepLayoutRequest }

func (r *recordingLayouter) MaterializeDeps(_ context.Context, req build.DepLayoutRequest) (build.DepLayout, error) {
	r.req = req
	return build.DepLayout{
		Deps: map[string]build.DepRef{
			"agent":          {Name: "agent", Version: "1.0.0", PublicArtifact: "dist/_deps/agent@1.0.0.js", Integrity: "sha512-agent"},
			"agent/lib/util": {Name: "agent", Version: "1.0.0", PublicArtifact: "dist/_deps/agent@1.0.0/lib/util.js", Integrity: "sha512-agent"},
		},
		Externals: []string{"agent", "agent/lib/util"},
	}, nil
}

func TestMaterializeDetectsDeclaredSubpathImports(t *testing.T) {
	ctx := context.Background()
	mem := storage.NewMemory()
	layouter := &recordingLayouter{}
	mat := materializer.New(mem, mem, mem, mem, nil, layouter, mem)
	site := seedBuildSite(t, mem)
	seedDef(t, mem, site.ID, "text",
		"import { agentName } from \"agent\";\n"+
			"import { utilMarker } from \"agent/lib/util\";\n"+
			"import { doubling } from \"helper/lib/math\";\n"+
			"export default (props) => props?.title ?? agentName + \":\" + utilMarker;\n", "sha_sub")
	seedDef(t, mem, site.ID, "container", "export default (props) => props.children;\n", "sha_cont")
	seedDef(t, mem, site.ID, "hero", "export default (props) => props.title;\n", "sha_hero")
	seedMaterializedPage(t, mem, site.ID, "page_1")
	if err := mem.CreateDependency(ctx, domain.Dependency{
		SiteID: site.ID, Name: "agent", Spec: "^1.0.0",
		CreatedAt: testNow(), UpdatedAt: testNow(),
	}); err != nil {
		t.Fatal(err)
	}
	snapshot := domain.Snapshot{ID: "snapshot_sub", SiteID: site.ID,
		Pages: []domain.SnapshotPage{{PageID: "page_1", VersionID: "pagever_1", Version: 1}},
		DepsLock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: "sha512-agent"},
		}},
		CreatedAt: testNow()}
	if err := mem.CreateSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}

	ws, err := mat.Materialize(ctx, build.WorkspaceRequest{
		SiteID: site.ID, SnapshotID: snapshot.ID, Environment: domain.EnvironmentDevelopment, Dir: filepath.Join(t.TempDir(), "ws"),
	})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if got, want := layouter.req.Subpaths, []string{"agent/lib/util"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("request.Subpaths = %#v, want %#v", got, want)
	}
	if got := layouter.req.TopLevel; len(got) != 1 || got[0] != "agent" {
		t.Fatalf("request.TopLevel = %#v, want [agent]", got)
	}
	// The subpath import naming an undeclared package (helper) must be left
	// out, and the import map must carry the declared subpath artifact.
	if _, ok := ws.Manifest.Deps["helper/lib/math"]; ok {
		t.Fatalf("undeclared subpath must not be materialized: %#v", ws.Manifest.Deps)
	}
	ref, ok := ws.Manifest.Deps["agent/lib/util"]
	if !ok || ref.PublicArtifact != "dist/_deps/agent@1.0.0/lib/util.js" {
		t.Fatalf("manifest deps = %#v", ws.Manifest.Deps)
	}
	found := false
	for _, ext := range ws.Manifest.Externals {
		if ext == "agent/lib/util" {
			found = true
		}
	}
	if !found {
		t.Fatalf("externals must include agent/lib/util: %#v", ws.Manifest.Externals)
	}
}

// TestMaterializePageChunksPerPageAndHomeByRoute covers the code-splitting
// contract (spec §16): each page gets its own chunk with only its definitions,
// and HomePage follows the route heuristic (most specific renderPage).
func TestMaterializePageChunksPerPageAndHomeByRoute(t *testing.T) {
	ctx := context.Background()
	mem, mat := materializerHarness(t)
	site := seedBuildSite(t, mem)
	seedDef(t, mem, site.ID, "text", "export default (props) => props.title;\n", "sha_text")
	seedDef(t, mem, site.ID, "hero", "export default (props) => props.title;\n", "sha_hero")

	rootA := []domain.Element{{ID: "root_a", ComponentID: "text", Props: map[string]domain.ElementProp{}}}
	pageA := domain.Page{ID: "page_a", SiteID: site.ID, Name: "A", Slug: "a", List: rootA, Version: 1,
		CreatedAt: testNow(), UpdatedAt: testNow()}
	if err := mem.CreatePage(ctx, pageA, domain.PageVersion{ID: "pagever_a", PageID: "page_a", Number: 1, List: rootA, CreatedAt: testNow()}); err != nil {
		t.Fatal(err)
	}
	rootB := []domain.Element{{ID: "root_b", ComponentID: "hero", Props: map[string]domain.ElementProp{}}}
	pageB := domain.Page{ID: "page_b", SiteID: site.ID, Name: "B", Slug: "b", List: rootB, Version: 1,
		CreatedAt: testNow(), UpdatedAt: testNow()}
	if err := mem.CreatePage(ctx, pageB, domain.PageVersion{ID: "pagever_b", PageID: "page_b", Number: 1, List: rootB, CreatedAt: testNow()}); err != nil {
		t.Fatal(err)
	}
	snapshot := domain.Snapshot{ID: "snapshot_pages", SiteID: site.ID,
		Pages: []domain.SnapshotPage{
			{PageID: "page_a", VersionID: "pagever_a", Version: 1},
			{PageID: "page_b", VersionID: "pagever_b", Version: 1},
		}, CreatedAt: testNow()}
	if err := mem.CreateSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	// A low-priority route on page_a and a high-priority one on page_b: page_b
	// is the home page (priority desc wins), regardless of snapshot order.
	if err := mem.CreateRoute(ctx, domain.Route{ID: "route_a", SiteID: site.ID, Matcher: "^/a$", Priority: 10,
		Action: domain.RouteAction{Type: "renderPage", PageID: "page_a"}, CreatedAt: testNow(), UpdatedAt: testNow()}); err != nil {
		t.Fatal(err)
	}
	if err := mem.CreateRoute(ctx, domain.Route{ID: "route_b", SiteID: site.ID, Matcher: "^/$", Priority: 100,
		Action: domain.RouteAction{Type: "renderPage", PageID: "page_b"}, CreatedAt: testNow(), UpdatedAt: testNow()}); err != nil {
		t.Fatal(err)
	}

	ws, err := mat.Materialize(ctx, build.WorkspaceRequest{
		SiteID: site.ID, SnapshotID: snapshot.ID, Environment: domain.EnvironmentProduction, Dir: filepath.Join(t.TempDir(), "ws"),
	})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if ws.Manifest.HomePage != "page_b" {
		t.Fatalf("home = %q, want page_b (highest-priority renderPage route)", ws.Manifest.HomePage)
	}
	if len(ws.Manifest.Pages) != 2 {
		t.Fatalf("pages = %#v", ws.Manifest.Pages)
	}
	byID := map[string]build.PageRef{}
	for _, p := range ws.Manifest.Pages {
		byID[p.PageID] = p
	}
	if byID["page_a"].Chunk != "pages/page_a.js" || len(byID["page_a"].Definitions) != 1 || byID["page_a"].Definitions[0] != "text" {
		t.Fatalf("page_a ref = %#v", byID["page_a"])
	}
	if byID["page_b"].Chunk != "pages/page_b.js" || len(byID["page_b"].Definitions) != 1 || byID["page_b"].Definitions[0] != "hero" {
		t.Fatalf("page_b ref = %#v", byID["page_b"])
	}

	chunkA, err := os.ReadFile(filepath.Join(ws.Dir, "src", "pages", "page_a.tsx"))
	if err != nil {
		t.Fatal(err)
	}
	chunkB, err := os.ReadFile(filepath.Join(ws.Dir, "src", "pages", "page_b.tsx"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(chunkA), "hero") || strings.Contains(string(chunkB), "text") {
		t.Fatalf("page chunks must not leak each other's definitions: A=%q B=%q", chunkA, chunkB)
	}
	for name, want := range map[string]string{
		"page_a.tsx": `registerPage("page_a", {"snapshotId":"snapshot_pages","versionId":"pagever_a","pageId":"page_a"`,
		"page_b.tsx": `registerPage("page_b", {"snapshotId":"snapshot_pages","versionId":"pagever_b","pageId":"page_b"`,
	} {
		data, err := os.ReadFile(filepath.Join(ws.Dir, "src", "pages", name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), want) {
			t.Fatalf("chunk %s must register its own tree, want %q in:\n%s", name, want, data)
		}
	}
}

type fontsStub struct{ faces []build.FontFace }

func (f fontsStub) FontFaces(context.Context, string) ([]build.FontFace, error) { return f.faces, nil }

// TestMaterializeFontFaces covers the P2 contract: declared web fonts are
// resolved through build.FontResolver into Manifest.FontFaces (and manifest.json).
func TestMaterializeFontFaces(t *testing.T) {
	ctx := context.Background()
	mem := storage.NewMemory()
	mat := materializer.New(mem, mem, mem, mem, nil, nil, mem).WithFonts(fontsStub{faces: []build.FontFace{
		{Family: "Inter", Weight: "400", Style: "italic", URL: "/build/_assets/inter-italic.woff2"},
	}})
	site := seedBuildSite(t, mem)
	seedDef(t, mem, site.ID, "text", "export default (props) => props.title;\n", "sha_ftext")
	seedDef(t, mem, site.ID, "container", "export default (props) => props.children;\n", "sha_fcont")
	seedDef(t, mem, site.ID, "hero", "export default (props) => props.title;\n", "sha_fhero")
	seedMaterializedPage(t, mem, site.ID, "page_1")
	snapshot := domain.Snapshot{ID: "snapshot_fonts", SiteID: site.ID,
		Pages: []domain.SnapshotPage{{PageID: "page_1", VersionID: "pagever_1", Version: 1}}, CreatedAt: testNow()}
	if err := mem.CreateSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}

	ws, err := mat.Materialize(ctx, build.WorkspaceRequest{
		SiteID: site.ID, SnapshotID: snapshot.ID, Environment: domain.EnvironmentDevelopment, Dir: filepath.Join(t.TempDir(), "ws"),
	})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if len(ws.Manifest.FontFaces) != 1 {
		t.Fatalf("manifest fontfaces = %#v", ws.Manifest.FontFaces)
	}
	ff := ws.Manifest.FontFaces[0]
	if ff.Family != "Inter" || ff.URL != "/build/_assets/inter-italic.woff2" || ff.Weight != "400" || ff.Style != "italic" {
		t.Fatalf("fontface = %#v", ff)
	}
	manifestData, err := os.ReadFile(filepath.Join(ws.Dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifestData), `"fontFaces"`) || !strings.Contains(string(manifestData), `"family": "Inter"`) {
		t.Fatalf("manifest.json must persist fontFaces: %s", manifestData)
	}
}

// TestMaterializeNoFontResolverKeepsManifestLean: a nil FontResolver must not
// leave a stray fontFaces entry in manifest.json.
func TestMaterializeNoFontResolverKeepsManifestLean(t *testing.T) {
	ctx := context.Background()
	mem, mat := materializerHarness(t)
	site := seedBuildSite(t, mem)
	seedDef(t, mem, site.ID, "text", "export default (props) => props.title;\n", "sha_nf")
	seedDef(t, mem, site.ID, "container", "export default (props) => props.children;\n", "sha_nfc")
	seedDef(t, mem, site.ID, "hero", "export default (props) => props.title;\n", "sha_nfh")
	seedMaterializedPage(t, mem, site.ID, "page_1")
	snapshot := domain.Snapshot{ID: "snapshot_nof", SiteID: site.ID,
		Pages: []domain.SnapshotPage{{PageID: "page_1", VersionID: "pagever_1", Version: 1}}, CreatedAt: testNow()}
	if err := mem.CreateSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	ws, err := mat.Materialize(ctx, build.WorkspaceRequest{
		SiteID: site.ID, SnapshotID: snapshot.ID, Environment: domain.EnvironmentDevelopment, Dir: filepath.Join(t.TempDir(), "ws"),
	})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if ws.Manifest.FontFaces != nil {
		t.Fatalf("manifest fontfaces must be nil without a resolver: %#v", ws.Manifest.FontFaces)
	}
	manifestData, err := os.ReadFile(filepath.Join(ws.Dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(manifestData), `"fontFaces"`) {
		t.Fatalf("manifest.json must not carry fontFaces without a resolver: %s", manifestData)
	}
}
