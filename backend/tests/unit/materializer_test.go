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
	root := domain.ComponentNode{
		InstanceID:   "root",
		DefinitionID: "container",
		Props:        map[string]any{},
		Children: []domain.ComponentNode{
			{InstanceID: "kids", DefinitionID: "text"},
			{InstanceID: "hidden", DefinitionID: "hero"},
		},
	}
	page := domain.Page{ID: pageID, SiteID: siteID, Name: "Home", Slug: "index", Root: root, Version: 1,
		CreatedAt: testNow(), UpdatedAt: testNow()}
	version := domain.PageVersion{ID: "pagever_1", PageID: pageID, Number: 1, Root: root, CreatedAt: testNow()}
	if err := mem.CreatePage(context.Background(), page, version); err != nil {
		t.Fatal(err)
	}
}

func materializerHarness(t *testing.T) (*storage.Memory, *materializer.Materializer) {
	t.Helper()
	mem := storage.NewMemory()
	mat := materializer.New(mem, mem, mem, mem, nil, nil)
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

	// entry.tsx imports every definition and registers them before boot.
	content := string(entry)
	if !strings.Contains(content, `import { ComponentRegistry, boot } from "@liapoldus/ui-runtime";`) {
		t.Fatalf("entry must import ui-runtime, got: %s", content)
	}
	if !strings.Contains(content, `import def_container from "./definitions/container";`) ||
		!strings.Contains(content, `import def_text from "./definitions/text";`) ||
		!strings.Contains(content, `import def_hero from "./definitions/hero";`) {
		t.Fatalf("entry must import all definitions: %s", content)
	}
	for _, reg := range []string{
		`ComponentRegistry.registerDefinition("container", def_container);`,
		`ComponentRegistry.registerDefinition("text", def_text);`,
	} {
		if !strings.Contains(content, reg) {
			t.Fatalf("entry missing %q: %s", reg, content)
		}
	}
	if !strings.Contains(content, `void boot("`+site.ID+`", "`+domain.EnvironmentDevelopment+`");`) {
		t.Fatalf("entry must call boot: %s", content)
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
	if len(ws.Manifest.Pages) != 1 || ws.Manifest.Pages[0] != "page_1" {
		t.Fatalf("manifest pages = %#v", ws.Manifest.Pages)
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
	}}, nil)
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

	// Unknown definition referenced by the tree.
	snapshot2 := domain.Snapshot{ID: "snapshot_m3b", SiteID: site.ID, CreatedAt: testNow()}
	_ = snapshot2
	root := domain.ComponentNode{InstanceID: "root", DefinitionID: "missing"}
	page := domain.Page{ID: "page_2", SiteID: site.ID, Name: "Miss", Slug: "miss", Root: root, Version: 1, CreatedAt: testNow(), UpdatedAt: testNow()}
	version := domain.PageVersion{ID: "pagever_2", PageID: "page_2", Number: 1, Root: root, CreatedAt: testNow()}
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
	heroRoot := domain.ComponentNode{InstanceID: "r", DefinitionID: "text2"}
	page3 := domain.Page{ID: "page_3", SiteID: site.ID, Name: "P", Slug: "p", Root: heroRoot, Version: 1, CreatedAt: testNow(), UpdatedAt: testNow()}
	version3 := domain.PageVersion{ID: "pagever_3", PageID: "page_3", Number: 1, Root: heroRoot, CreatedAt: testNow()}
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
	mat := materializer.New(mem, mem, mem, mem, nil, layouter)
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
