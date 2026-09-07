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
	componentapp "github.com/liapoldus/liapoldus/backend/internal/application/component"
	routeapp "github.com/liapoldus/liapoldus/backend/internal/application/route"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Materializer turns a site snapshot into an esbuild-able workspace:
// src/entry.tsx (boot only) + src/definitions/<id>.tsx + one code-split
// src/pages/<pageID>.tsx per snapshot page + manifest.json (plus, when the
// snapshot has a frozen dependency lock and a DepLayouter is configured,
// node_modules/ + dist/_deps/ bundles). Definition sources come from the
// component registry mirror of the git HEAD (R5), so no git checkout is needed
// at build time. Page chunks self-register their definitions and page tree via
// ComponentRegistry/registerPage; mount() dynamic-imports the current page's
// chunk at navigation (spec §16).
type Materializer struct {
	snapshots domain.SnapshotRepository
	pages     domain.PageRepository
	defs      domain.ComponentDefinitionRepository
	depsRepo  domain.DependencyRepository
	shared    build.SharedResolver
	deps      build.DepLayouter
	routes    domain.RouteRepository
}

func New(snapshots domain.SnapshotRepository, pages domain.PageRepository,
	defs domain.ComponentDefinitionRepository, depsRepo domain.DependencyRepository,
	shared build.SharedResolver, deps build.DepLayouter, routes domain.RouteRepository) *Materializer {
	return &Materializer{snapshots: snapshots, pages: pages, defs: defs, depsRepo: depsRepo, shared: shared, deps: deps, routes: routes}
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

	// Load every page version pinned by the snapshot and collect the
	// definition ids each page's tree references. Each page becomes a
	// code-split chunk that registers exactly its own definitions.
	var pages []pageSlice
	seenDefs := make(map[string]bool)
	for _, p := range snapshot.Pages {
		version, err := m.pages.GetPageVersion(ctx, p.PageID, p.VersionID)
		if err != nil {
			return fail("page "+p.PageID, err)
		}
		pageDefs := map[string]bool{}
		collectElementDefs(version.List, pageDefs)
		for id := range pageDefs {
			seenDefs[id] = true
		}
		pages = append(pages, pageSlice{Page: p, Version: version, Definitions: sortedKeys(pageDefs)})
	}
	definitionIds := make([]string, 0, len(seenDefs))
	for id := range seenDefs {
		definitionIds = append(definitionIds, id)
	}
	sort.Strings(definitionIds)

	// Resolve the site's declared dependency names up-front: the definitions
	// loop below scans their sources for bare subpath imports and must only
	// keep subpaths that belong to a declared top-level package.
	var declaredDeps []string
	if len(snapshot.DepsLock.Deps) > 0 {
		if m.deps == nil || m.depsRepo == nil {
			return fail("deps", fmt.Errorf("%w: snapshot has dependencies but no dependency layout is configured", domain.ErrInvalidRequest))
		}
		declared, err := m.depsRepo.ListDependenciesBySite(ctx, req.SiteID)
		if err != nil {
			return fail("deps", err)
		}
		for _, dep := range declared {
			declaredDeps = append(declaredDeps, dep.Name)
		}
	}
	declaredSet := make(map[string]bool, len(declaredDeps))
	for _, name := range declaredDeps {
		declaredSet[name] = true
	}

	// Resolve sources from the registry and write definition files. Under the
	// R10 hierarchy page elements reference sections only, so the set is
	// expanded transitively through @site/components/<id> imports (sections →
	// their allowlisted primitives) before writing. The whole import graph is
	// validated: an inconsistent snapshot fails the build instead of emitting
	// a broken page chunk.
	definitions, err := expandDefinitions(ctx, m.defs, req.SiteID, definitionIds)
	if err != nil {
		return fail("definitions", err)
	}
	if err := componentapp.ValidateGraph(definitions); err != nil {
		return fail("definitions", err)
	}
	refs := make(map[string]build.DefinitionRef, len(definitions))
	srcDir := filepath.Join(req.Dir, "src", "definitions")
	var subpathImports []string
	for _, def := range definitions {
		id := def.ID
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
		subpathImports = append(subpathImports, bareSubpathImports(def.Source, declaredSet)...)
	}

	manifest := build.Manifest{
		SiteID:      req.SiteID,
		SnapshotID:  req.SnapshotID,
		Environment: req.Environment,
		Definitions: refs,
		Pages:       pageRefs(pages),
		HomePage:    homePage(req.SiteID, snapshot.Pages, m.routes),
		Externals:   build.SharedExternals,
	}
	if m.shared != nil {
		manifest.Shared = m.shared.SharedURLs()
	}

	// Dependency layout: frozen lock of the snapshot → node_modules/ +
	// dist/_deps/ bundles (top-level + bare subpaths) + the manifest
	// import-map and site-bundle externals.
	if len(snapshot.DepsLock.Deps) > 0 {
		layout, err := m.deps.MaterializeDeps(ctx, build.DepLayoutRequest{
			Dir:      req.Dir,
			Lock:     snapshot.DepsLock,
			TopLevel: declaredDeps,
			Subpaths: uniqueSorted(subpathImports),
		})
		if err != nil {
			return fail("deps", err)
		}
		manifest.Deps = layout.Deps
		manifest.Externals = append(append([]string{}, build.SharedExternals...), layout.Externals...)
		manifest.Externals = uniqueSorted(manifest.Externals)
		manifest.Styles = layout.Styles
	}

	// Write one code-split entry per page: the chunk statically imports and
	// registers the page's definitions, then hands the page tree to the
	// runtime PageRegistry via registerPage (spec §16).
	pagesDir := filepath.Join(req.Dir, "src", "pages")
	if len(pages) > 0 {
		if err := os.MkdirAll(pagesDir, 0o755); err != nil {
			return fail("mkdir pages", err)
		}
	}
	for _, p := range pages {
		elementsJSON, err := json.Marshal(pageElements{
			SnapshotID: req.SnapshotID,
			VersionID:  p.Version.ID,
			PageID:     p.Page.PageID,
			Elements:   wireElements(p.Version.List),
		})
		if err != nil {
			return fail("elements "+p.Page.PageID, err)
		}
		chunk := generatePageChunk(p.Page.PageID, string(elementsJSON), p.Definitions)
		if err := os.WriteFile(filepath.Join(pagesDir, p.Page.PageID+".tsx"), []byte(chunk), 0o644); err != nil {
			return fail("write page "+p.Page.PageID, err)
		}
	}

	entry := generateEntry(req.SiteID, req.Environment)
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

// pageSlice bundles a snapshot page with its pinned version and the sorted
// definition ids its tree references.
type pageSlice struct {
	Page        domain.SnapshotPage
	Version     domain.PageVersion
	Definitions []string
}

// collectElementDefs records every component id referenced by a page's element
// list.
func collectElementDefs(list []domain.Element, seen map[string]bool) {
	for _, el := range list {
		if el.ComponentID != "" {
			seen[el.ComponentID] = true
		}
	}
}

// expandDefinitions returns every definition reachable from roots by
// transitively following @site/components/<id> imports (sections → their
// allowlisted primitives). Roots are page-level section ids; the closure adds
// each section's imports until nothing new is found. Unknown imported ids are
// allowed through here — ValidateGraph reports them as a specific error.
func expandDefinitions(ctx context.Context, defs domain.ComponentDefinitionRepository, siteID string, roots []string) ([]domain.ComponentDefinition, error) {
	byID := make(map[string]domain.ComponentDefinition)
	var order []string
	var visit func(string) error
	visited := make(map[string]bool)
	visit = func(id string) error {
		if visited[id] {
			return nil
		}
		visited[id] = true
		def, err := defs.Get(ctx, siteID, id)
		if err != nil {
			return err
		}
		byID[id] = *def
		order = append(order, id)
		for _, child := range componentapp.SiteComponentImports(def.Source) {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, id := range roots {
		if err := visit(id); err != nil {
			return nil, err
		}
	}
	out := make([]domain.ComponentDefinition, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out, nil
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// pageRefs maps each page to its split chunk artifact (dist/pages/<PageID>.js.
// esbuild mirrors the entry tree under an outbase of src/, so an entry at
// src/pages/<id>.tsx emits dist/pages/<id>.js). Sorted for deterministic
// manifest output; HomePage is computed independently.
func pageRefs(pages []pageSlice) []build.PageRef {
	out := make([]build.PageRef, 0, len(pages))
	for _, p := range pages {
		out = append(out, build.PageRef{
			PageID:      p.Page.PageID,
			Chunk:       "pages/" + p.Page.PageID + ".js",
			Definitions: p.Definitions,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PageID < out[j].PageID })
	return out
}

// homePage picks the snapshot page the boot contract will render first, using
// the same heuristic as the runtime Service.initialTree: the most specific
// renderPage route referencing a snapshot page, else the first page in
// snapshot order. Returns "" for a snapshot without pages. When no route
// repository is configured the fallback (first page) applies.
func homePage(siteID string, pages []domain.SnapshotPage, routes domain.RouteRepository) string {
	if len(pages) == 0 {
		return ""
	}
	inSnapshot := make(map[string]bool, len(pages))
	for _, p := range pages {
		inSnapshot[p.PageID] = true
	}
	if routes != nil {
		all, err := routes.ListRoutesBySite(context.Background(), siteID)
		if err == nil {
			sorted := append([]domain.Route(nil), all...)
			sort.SliceStable(sorted, func(i, j int) bool {
				if sorted[i].Priority != sorted[j].Priority {
					return sorted[i].Priority > sorted[j].Priority
				}
				return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
			})
			for _, r := range sorted {
				if r.Action.Type == routeapp.RenderPage && inSnapshot[r.Action.PageID] {
					return r.Action.PageID
				}
			}
		}
	}
	return pages[0].PageID
}

// pageElements is the wire shape registerPage stores for a page (matches the
// ui-runtime page descriptor; Elements is the flat element list).
type pageElements struct {
	SnapshotID string        `json:"snapshotId"`
	VersionID  string        `json:"versionId"`
	PageID     string        `json:"pageId"`
	Elements   []wireElement `json:"elements"`
}

// wireElement mirrors the runtime ElementDescriptor: props/bindings are always
// objects/arrays (never null) because the ui-runtime crawler iterates them.
type wireElement struct {
	ID          string                        `json:"id"`
	ComponentID string                        `json:"componentId"`
	Props       map[string]domain.ElementProp `json:"props"`
	Bindings    []domain.BindingSource        `json:"bindings"`
}

func wireElements(list []domain.Element) []wireElement {
	out := make([]wireElement, 0, len(list))
	for _, el := range list {
		out = append(out, wireElement{
			ID:          el.ID,
			ComponentID: el.ComponentID,
			Props:       wireElementProps(el.Props),
			Bindings:    wireElementBindings(el.Bindings),
		})
	}
	return out
}

func wireElementBindings(bindings []domain.BindingSource) []domain.BindingSource {
	if bindings == nil {
		return []domain.BindingSource{}
	}
	return bindings
}

func wireElementProps(props map[string]domain.ElementProp) map[string]domain.ElementProp {
	if len(props) == 0 {
		return map[string]domain.ElementProp{}
	}
	out := make(map[string]domain.ElementProp, len(props))
	for k, v := range props {
		out[k] = v
	}
	return out
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

// importSpecifierRe is gone — scanning lives in build.ScanImportSpecifiers.

// bareSubpathImports returns the bare subpath specifiers (e.g. "lodash/map")
// found in source that belong to a declared top-level dependency. Top-level
// imports, relative/absolute paths, URLs and node built-ins are handled
// elsewhere (top-level _deps bundles / the site bundle resolver) and are
// intentionally left out.
func bareSubpathImports(source string, declared map[string]bool) []string {
	var out []string
	for _, spec := range build.ScanImportSpecifiers(source) {
		top, _, isSub := build.SplitBareSpecifier(spec)
		if isSub && declared[top] {
			out = append(out, spec)
		}
	}
	return out
}

// generateEntry builds src/entry.tsx: only the boot/mount — every definition
// and page tree lives in code-split page chunks (spec §16) that mount()
// dynamic-imports on navigation.
func generateEntry(siteID, environment string) string {
	var builder strings.Builder
	builder.WriteString("import { mount } from \"@liapoldus/ui-runtime\";\n\n")
	fmt.Fprintf(&builder, "void mount(%q, %q);\n", siteID, environment)
	return builder.String()
}

// generatePageChunk builds src/pages/<pageID>.tsx: the chunk registers the
// page's definitions in the ComponentRegistry and hands the page element list
// to the runtime PageRegistry. elements is a DSL JSON object literal (already
// marshalled).
func generatePageChunk(pageID, elements string, definitionIds []string) string {
	var builder strings.Builder
	builder.WriteString("import { ComponentRegistry, registerPage } from \"@liapoldus/ui-runtime\";\n")
	for _, id := range definitionIds {
		fmt.Fprintf(&builder, "import %s from \"../definitions/%s\";\n", definitionName(id), id)
	}
	builder.WriteString("\n")
	for _, id := range definitionIds {
		fmt.Fprintf(&builder, "ComponentRegistry.registerDefinition(%q, %s);\n", id, definitionName(id))
	}
	fmt.Fprintf(&builder, "\nregisterPage(%q, %s);\n", pageID, elements)
	return builder.String()
}
