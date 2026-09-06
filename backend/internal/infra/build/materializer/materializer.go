package materializer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Materializer turns a site snapshot into an esbuild-able workspace:
// src/entry.tsx + src/definitions/<id>.tsx + manifest.json (plus, when the
// snapshot has a frozen dependency lock and a DepLayouter is configured,
// node_modules/ + dist/_deps/ bundles). Definition sources come from the
// component registry mirror of the git HEAD (R5), so no git checkout is needed
// at build time.
type Materializer struct {
	snapshots domain.SnapshotRepository
	pages     domain.PageRepository
	defs      domain.ComponentDefinitionRepository
	depsRepo  domain.DependencyRepository
	shared    build.SharedResolver
	deps      build.DepLayouter
}

func New(snapshots domain.SnapshotRepository, pages domain.PageRepository,
	defs domain.ComponentDefinitionRepository, depsRepo domain.DependencyRepository,
	shared build.SharedResolver, deps build.DepLayouter) *Materializer {
	return &Materializer{snapshots: snapshots, pages: pages, defs: defs, depsRepo: depsRepo, shared: shared, deps: deps}
}

var _ build.WorkspaceBuilder = (*Materializer)(nil)

// definitionName sanitizes a definition id into a safe ES import identifier.
var definitionNameRegexp = regexp.MustCompile(`[^A-Za-z0-9_]`)

func definitionName(id string) string {
	return "def_" + definitionNameRegexp.ReplaceAllString(id, "_")
}

func (m *Materializer) Materialize(ctx context.Context, req build.WorkspaceRequest) (build.Workspace, error) {
	cleanup := func() {
		_ = os.RemoveAll(req.Dir)
	}

	fail := func(step string, err error) (build.Workspace, error) {
		cleanup()
		return build.Workspace{}, fmt.Errorf("materialize %s: %w", step, err)
	}

	snapshot, err := m.snapshots.GetSnapshot(ctx, req.SnapshotID)
	if err != nil {
		return fail("snapshot", err)
	}
	if snapshot.SiteID != req.SiteID {
		return fail("snapshot", fmt.Errorf("%w: snapshot does not belong to site", domain.ErrNotFound))
	}

	// Collect every definition referenced by the snapshot's page trees.
	definitionIds, err := m.collectDefinitions(ctx, snapshot)
	if err != nil {
		return fail("tree", err)
	}

	// Resolve sources from the registry and write definition files.
	refs := make(map[string]build.DefinitionRef, len(definitionIds))
	srcDir := filepath.Join(req.Dir, "src", "definitions")
	for _, id := range definitionIds {
		def, err := m.defs.Get(ctx, req.SiteID, id)
		if err != nil {
			return fail("definition "+id, err)
		}
		if strings.TrimSpace(def.Source) == "" {
			return fail("definition "+id, fmt.Errorf("%w: definition has no source", domain.ErrInvalidRequest))
		}
		fileName := filepath.ToSlash(filepath.Join("src", "definitions", id+".tsx"))
		if err := os.MkdirAll(srcDir, 0o755); err != nil {
			return fail("mkdir", err)
		}
		if err := os.WriteFile(filepath.Join(req.Dir, filepath.FromSlash(fileName)), []byte(def.Source), 0o644); err != nil {
			return fail("write "+id, err)
		}
		refs[id] = build.DefinitionRef{File: fileName, SHA: def.CurrentSHA}
	}

	manifest := build.Manifest{
		SiteID:      req.SiteID,
		SnapshotID:  req.SnapshotID,
		Environment: req.Environment,
		Definitions: refs,
		Pages:       pageIDs(snapshot),
		Externals:   build.SharedExternals,
	}
	if m.shared != nil {
		manifest.Shared = m.shared.SharedURLs()
	}

	// Dependency layout: frozen lock of the snapshot → node_modules/ +
	// dist/_deps/ bundles + the manifest import-map and site-bundle externals.
	if len(snapshot.DepsLock.Deps) > 0 {
		if m.deps == nil || m.depsRepo == nil {
			return fail("deps", fmt.Errorf("%w: snapshot has dependencies but no dependency layout is configured", domain.ErrInvalidRequest))
		}
		topLevel, err := m.depsRepo.ListDependenciesBySite(ctx, req.SiteID)
		if err != nil {
			return fail("deps", err)
		}
		names := make([]string, 0, len(topLevel))
		for _, dep := range topLevel {
			names = append(names, dep.Name)
		}
		layout, err := m.deps.MaterializeDeps(ctx, build.DepLayoutRequest{
			Dir:      req.Dir,
			Lock:     snapshot.DepsLock,
			TopLevel: names,
		})
		if err != nil {
			return fail("deps", err)
		}
		manifest.Deps = layout.Deps
		manifest.Externals = append(append([]string{}, build.SharedExternals...), layout.Externals...)
		manifest.Externals = uniqueSorted(manifest.Externals)
	}

	entry := generateEntry(req.SiteID, req.Environment, definitionIds)
	if err := os.MkdirAll(filepath.Join(req.Dir, "src"), 0o755); err != nil {
		return fail("mkdir src", err)
	}
	if err := os.WriteFile(filepath.Join(req.Dir, "src", "entry.tsx"), []byte(entry), 0o644); err != nil {
		return fail("write entry", err)
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fail("manifest", err)
	}
	if err := os.WriteFile(filepath.Join(req.Dir, "manifest.json"), manifestData, 0o644); err != nil {
		return fail("write manifest", err)
	}

	return build.Workspace{Dir: req.Dir, Manifest: manifest}, nil
}

// collectDefinitions walks every page version pinned by the snapshot and
// returns the unique, sorted definition ids referenced by their trees.
func (m *Materializer) collectDefinitions(ctx context.Context, snapshot domain.Snapshot) ([]string, error) {
	seen := make(map[string]bool)
	for _, page := range snapshot.Pages {
		version, err := m.pages.GetPageVersion(ctx, page.PageID, page.VersionID)
		if err != nil {
			return nil, err
		}
		collectNode(version.Root, seen)
	}
	result := make([]string, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	sort.Strings(result)
	return result, nil
}

func collectNode(node domain.ComponentNode, seen map[string]bool) {
	if node.DefinitionID != "" {
		seen[node.DefinitionID] = true
	}
	for _, child := range node.Children {
		collectNode(child, seen)
	}
}

func pageIDs(snapshot domain.Snapshot) []string {
	result := make([]string, 0, len(snapshot.Pages))
	for _, page := range snapshot.Pages {
		result = append(result, page.PageID)
	}
	sort.Strings(result)
	return result
}

// uniqueSorted dedupes + sorts a slice in place.
func uniqueSorted(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}

// generateEntry builds src/entry.tsx: every definition is statically imported
// and registered in the ComponentRegistry before boot().
func generateEntry(siteID, environment string, definitionIds []string) string {
	var builder strings.Builder
	builder.WriteString("import { ComponentRegistry, boot } from \"@liapoldus/ui-runtime\";\n")
	for _, id := range definitionIds {
		fmt.Fprintf(&builder, "import %s from \"./definitions/%s\";\n", definitionName(id), id)
	}
	builder.WriteString("\n")
	for _, id := range definitionIds {
		fmt.Fprintf(&builder, "ComponentRegistry.register(%q, %s);\n", id, definitionName(id))
	}
	fmt.Fprintf(&builder, "\nvoid boot(%q, %q);\n", siteID, environment)
	return builder.String()
}
