// Package layout materializes a snapshot's frozen dependency lock into a build
// workspace (dependency-service spec §11 шаг 4): every locked package (top-level
// + transitive) is fetched, verified against its sha512 integrity and unpacked
// into node_modules/ (flat, one version per name); the site's top-level deps get
// their own esbuild bundles under dist/_deps/ and come back as the manifest
// import-map.
package layout

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/deps/store"
)

// TarballFetcher downloads and verifies (sha512) a package tarball. The
// layout hands it the tarball URL + integrity from the immutable
// dep_packages cache, so the lock's pinned version + integrity are what gets
// verified, never a re-resolve at build time.
type TarballFetcher interface {
	Tarball(ctx context.Context, name, version, tarballURL, integrity string) ([]byte, error)
}

// Layout is the infra implementation of build.DepLayouter.
type Layout struct {
	pkgs     domain.DepPackageRepository
	blobs    *store.Store
	fetch    TarballFetcher
	accessor domain.DependencyRepository
}

type LayoutOptions struct {
	Packages domain.DepPackageRepository
	Store    *store.Store
	Fetch    TarballFetcher
	// Accessor, when set, records per-tarball last-access stamps so the
	// dependency cache can LRU-evict the least recently used tarballs
	// (cache-limits/eviction, spec §11). Optional; nil disables tracking.
	Accessor domain.DependencyRepository
}

func New(opts LayoutOptions) *Layout {
	return &Layout{
		pkgs:     opts.Packages,
		blobs:    opts.Store,
		fetch:    opts.Fetch,
		accessor: opts.Accessor,
	}
}

var _ build.DepLayouter = (*Layout)(nil)

// MaterializeDeps implements build.DepLayouter. An empty lock is a no-op.
func (l *Layout) MaterializeDeps(ctx context.Context, req build.DepLayoutRequest) (build.DepLayout, error) {
	if len(req.Lock.Deps) == 0 {
		return build.DepLayout{Deps: map[string]build.DepRef{}, Externals: []string{}}, nil
	}

	// 1. Compute the physical placement of every locked instance and lay it
	// out: the hoisted instance of a package lives at node_modules/<name>,
	// every conflicting second version is nested under each parent instance
	// that requires it (node_modules/<parent>/node_modules/<name>, spec §5).
	// Paths are computed in lock order — the resolver emits parents before
	// children — so unwrapping is deterministic and esbuild's node resolution
	// walks up from every importing file to the correct version.
	instances, perName, err := indexInstances(req.Lock.Deps)
	if err != nil {
		return build.DepLayout{}, err
	}
	// A legacy flat lock carries no hoist markers at all; its sole instance
	// per package goes to the root. In marker mode the Hoisted flag is
	// authoritative (a package may resolve only a nested instance).
	legacy := true
	for _, dep := range req.Lock.Deps {
		if dep.Hoisted || len(dep.RequestedBy) > 0 {
			legacy = false
			break
		}
	}
	hoisted := func(name string) (string, bool) {
		keys := perName[name]
		if len(keys) == 0 {
			return "", false
		}
		for _, key := range keys {
			if instances[key].Hoisted {
				return key, true
			}
		}
		if legacy {
			return keys[0], true
		}
		return "", false
	}
	if err := l.layOut(ctx, req.Dir, req.Lock.Deps, instances, hoisted, legacy); err != nil {
		return build.DepLayout{}, err
	}

	topSet := make(map[string]bool, len(req.TopLevel))
	for _, name := range req.TopLevel {
		topSet[name] = true
	}

	// 2. Scan each top-level dependency's own source for bare subpath imports
	// of other declared top-level deps (and of the shared runtime libraries).
	// Those specifiers stay external in the dep bundle and get their own
	// _deps/<name>@<ver>/<subpath>.js artifact (spec §6), so every consumer
	// shares one import-mapped copy. Plain top-level imports, subpaths of
	// transitive-only packages, self-subpaths and relative paths stay inlined
	// (status quo), and non-JS subpaths of declared deps fail with the same
	// hint as site-source subpaths.
	perDepExternals := make(map[string][]string, len(req.TopLevel))
	var discovered []string
	for _, name := range req.TopLevel {
		ext, subs, err := l.scanDepSubpaths(req.Dir, name, topSet)
		if err != nil {
			return build.DepLayout{}, err
		}
		perDepExternals[name] = ext
		discovered = append(discovered, subs...)
	}

	// 3. Bundle the top-level deps (the site's declared bare imports) one by
	// one. Only the shared libraries and the dep-internal subpath specifiers
	// stay external; transitive dependencies are inlined into the top-level
	// bundle, per spec §6. A stylesheet reached from a bundle's own files is
	// extracted by esbuild into a combined .css artifact (spec §12).
	result := build.DepLayout{Deps: map[string]build.DepRef{}}
	var styles []string
	externals := make([]string, 0, len(req.TopLevel)+len(req.Subpaths)+len(discovered))
	for _, name := range req.TopLevel {
		key, ok := hoisted(name)
		if !ok {
			return build.DepLayout{}, &build.DepBuildError{
				Pkg: name, Hint: "declared dependency is absent from the snapshot lock (lock was frozen before the declaration)",
			}
		}
		dep := instances[key]
		ref, err := l.bundle(ctx, req.Dir, dep, "", perDepExternals[name]...)
		if err != nil {
			return build.DepLayout{}, err
		}
		result.Deps[dep.Name] = ref
		externals = append(externals, dep.Name)
		if ref.CSSArtifact != "" {
			styles = append(styles, ref.CSSArtifact)
		}
	}

	// 4. Bundle each bare subpath of a declared top-level dep that the site
	// source or another top-level dep pulls in directly (spec §6): one
	// _deps/<name>@<ver>/<subpath>.js artifact + import-map entry. Shared
	// externals stay external; everything else is inlined into the subpath
	// bundle exactly like the top-level one. Subpaths of undeclared
	// (transitive) packages are not materialized — they resolve from
	// node_modules and inline where imported.
	subpaths := uniqueSorted(append(append([]string{}, req.Subpaths...), discovered...))
	for _, spec := range subpaths {
		top, rest, ok := build.SplitBareSpecifier(spec)
		if !ok || !topSet[top] {
			continue
		}
		if !validSubpath(rest) {
			return build.DepLayout{}, &build.DepBuildError{
				Pkg: top, Hint: fmt.Sprintf("subpath %q is not a filesystem-safe JS module path", spec),
			}
		}
		key, ok := hoisted(top)
		if !ok {
			return build.DepLayout{}, &build.DepBuildError{
				Pkg: top, Hint: fmt.Sprintf("subpath import %q names a package absent from the snapshot lock", spec),
			}
		}
		// A CSS subpath becomes a .css artifact (+ import-map stub) instead of
		// a JS re-export bundle (spec §12): the consuming bundle keeps the bare
		// import external, the shell injects a <link> and maps the specifier to
		// the stub so the route-effect import stays a valid ES module.
		if isCssSubpath(rest) {
			ref, err := l.bundleCSS(ctx, req.Dir, instances[key], rest)
			if err != nil {
				return build.DepLayout{}, err
			}
			result.Deps[spec] = ref
			externals = append(externals, spec)
			if ref.CSSArtifact != "" {
				styles = append(styles, ref.CSSArtifact)
			}
			continue
		}
		if reason := unsupportedSubpath(rest); reason != "" {
			return build.DepLayout{}, &build.DepBuildError{Pkg: top, Hint: reason}
		}
		ref, err := l.bundle(ctx, req.Dir, instances[key], rest)
		if err != nil {
			return build.DepLayout{}, err
		}
		result.Deps[spec] = ref
		externals = append(externals, spec)
		if ref.CSSArtifact != "" {
			styles = append(styles, ref.CSSArtifact)
		}
	}

	result.Externals = uniqueSorted(externals)
	result.Styles = uniqueSorted(styles)
	return result, nil
}

// ensure fetches (if missing from the disk cache), verifies and unpacks one
// locked instance into relPath under dir (e.g. "node_modules/name" for hoisted
// packages, "node_modules/parent/node_modules/name" for nested ones).
func (l *Layout) ensure(ctx context.Context, dir string, dep domain.LockedDep, relPath string) error {
	if dep.Integrity == "" {
		return &build.DepBuildError{
			Pkg: dep.Name, Version: dep.Version,
			Hint: "registry published no sha512 integrity for this version; refusing to install unverified content",
		}
	}
	data, err := l.blob(ctx, dep)
	if err != nil {
		return err
	}
	return unpackTarball(data, filepath.Join(dir, filepath.FromSlash(relPath)))
}

// indexInstances indexes a lock's instances by their graph-unique name@version
// key and the deterministic per-package creation order. A duplicated key is a
// corrupted lock, not a valid nested layout.
func indexInstances(deps []domain.LockedDep) (map[string]domain.LockedDep, map[string][]string, error) {
	instances := make(map[string]domain.LockedDep, len(deps))
	perName := make(map[string][]string, len(deps))
	for _, dep := range deps {
		key := dep.InstanceKey()
		if _, dup := instances[key]; dup {
			return nil, nil, &build.DepBuildError{
				Pkg: dep.Name, Version: dep.Version,
				Hint: "duplicate lock entry for the same package instance (corrupted snapshot lock)",
			}
		}
		instances[key] = dep
		perName[dep.Name] = append(perName[dep.Name], key)
	}
	return instances, perName, nil
}

// layOut unpacks every instance of the lock at its physical path(s). Hoisted
// instances go to node_modules/<name>; a nested instance is placed under the
// path of every parent instance that requires it, so esbuild resolves each
// consumer to the version that satisfies its requested range. The lock must
// list parents before children (deterministic BFS order the resolver emits);
// a nested instance whose parent path is not yet known is a broken lock. In
// legacy (no markers) mode every bare instance unpacks at the root.
func (l *Layout) layOut(ctx context.Context, dir string, deps []domain.LockedDep,
	instances map[string]domain.LockedDep, hoisted func(string) (string, bool), legacy bool) error {
	paths := make(map[string][]string, len(deps))
	for _, dep := range deps {
		key := dep.InstanceKey()
		if hkey, _ := hoisted(dep.Name); hkey == key {
			paths[key] = []string{filepath.Join("node_modules", dep.Name)}
			continue
		}
		if legacy {
			paths[key] = []string{filepath.Join("node_modules", dep.Name)}
			continue
		}
		var own []string
		for _, parent := range dep.RequestedBy {
			if parent == "site" {
				return &build.DepBuildError{
					Pkg: dep.Name, Version: dep.Version,
					Hint: "a nested instance may not be requested by the site; a top-level package must be hoisted",
				}
			}
			parentPaths, ok := paths[parent]
			if !ok {
				return &build.DepBuildError{
					Pkg: dep.Name, Version: dep.Version,
					Hint: fmt.Sprintf("nested instance has unknown parent %q in the lock (parents must precede children)", parent),
				}
			}
			for _, p := range parentPaths {
				own = append(own, filepath.Join(p, "node_modules", dep.Name))
			}
		}
		paths[key] = uniqueSorted(own)
	}

	for _, dep := range deps {
		for _, relPath := range paths[dep.InstanceKey()] {
			if err := l.ensure(ctx, dir, dep, relPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// blob returns verified tarball bytes for a locked package, using the disk
// cache when present and fetching from the registry otherwise.
func (l *Layout) blob(ctx context.Context, dep domain.LockedDep) ([]byte, error) {
	if l.blobs.Has(dep.Name, dep.Version) {
		l.touch(dep.Name, dep.Version)
		if data, err := l.blobs.Open(dep.Name, dep.Version); err == nil {
			return data, nil
		}
	}
	pkg, err := l.pkgs.GetDepPackage(ctx, dep.Name, dep.Version)
	if err != nil {
		return nil, fmt.Errorf("dependency %s@%s: package metadata is not cached (%w)", dep.Name, dep.Version, err)
	}
	if pkg.Integrity != dep.Integrity {
		return nil, fmt.Errorf("dependency %s@%s: cache integrity %q does not match the snapshot lock %q", dep.Name, dep.Version, pkg.Integrity, dep.Integrity)
	}
	if pkg.TarballURL == "" {
		return nil, &build.DepBuildError{
			Pkg: dep.Name, Version: dep.Version,
			Hint: "no tarball URL was cached for this package version",
		}
	}
	data, err := l.fetch.Tarball(ctx, dep.Name, dep.Version, pkg.TarballURL, pkg.Integrity)
	if err != nil {
		return nil, err
	}
	if err := l.blobs.Save(dep.Name, dep.Version, data); err != nil {
		return nil, fmt.Errorf("dependency %s@%s: cache write: %w", dep.Name, dep.Version, err)
	}
	l.touch(dep.Name, dep.Version)
	return data, nil
}

// touch records a last-access stamp for a tarball identity when an accessor is
// configured. Read failures are best-effort (the LRU eviction still runs using
// whatever stamps exist).
func (l *Layout) touch(name, version string) {
	if l.accessor == nil {
		return
	}
	_ = l.accessor.TouchTarballAccess(context.Background(), name, version)
}

// bundle builds one _deps artifact for a top-level dependency, or for one of
// its bare subpaths (subpath != ""). The artifact is a re-export of the target
// module so any named/default import the site source makes resolves at
// runtime (esbuild omits a default re-export that has no binding, so the same
// entry works for defaultless subpath modules too). Only the shared libraries
// plus the declared extra externals (the dep-internal subpath specifiers) stay
// external (import-map-resolved); transitive deps are inlined (spec §6). The
// css loader turns every stylesheet reachable from the entry's JS graph into a
// sibling .css artifact (combined CSS, spec §12) and strips the import from
// the JS output.
func (l *Layout) bundle(ctx context.Context, dir string, dep domain.LockedDep, subpath string, extraExternal ...string) (build.DepRef, error) {
	specifier := dep.Name
	if subpath != "" {
		specifier = dep.Name + "/" + subpath
	}
	artifact := artifactName(dep.Name, dep.Version)
	if subpath != "" {
		artifact = strings.TrimSuffix(artifact, ".js") + "/" + subpath
		if !strings.HasSuffix(artifact, ".js") {
			artifact += ".js"
		}
	}

	entry := filepath.Join(dir, "deps_entries", artifactEntry(specifier))
	outfile := filepath.Join(dir, "dist", "_deps", filepath.FromSlash(artifact))
	if err := os.MkdirAll(filepath.Dir(entry), 0o755); err != nil {
		return build.DepRef{}, fmt.Errorf("dependency %s: mkdir: %w", specifier, err)
	}
	if err := os.MkdirAll(filepath.Dir(outfile), 0o755); err != nil {
		return build.DepRef{}, fmt.Errorf("dependency %s: mkdir dist: %w", specifier, err)
	}

	entrySource := fmt.Sprintf("export * from %q;\nexport { default } from %q;\n", specifier, specifier)
	if err := os.WriteFile(entry, []byte(entrySource), 0o644); err != nil {
		return build.DepRef{}, fmt.Errorf("dependency %s: write entry: %w", specifier, err)
	}

	external := append(append([]string{}, build.SharedExternals...), extraExternal...)
	result := api.Build(api.BuildOptions{
		EntryPoints:   []string{entry},
		Outfile:       outfile,
		Bundle:        true,
		Write:         true,
		Format:        api.FormatESModule,
		Platform:      api.PlatformBrowser,
		Target:        api.ES2020,
		TreeShaking:   api.TreeShakingTrue,
		AbsWorkingDir: dir,
		Loader:        map[string]api.Loader{".css": api.LoaderCSS},
		External:      external,
		LogLevel:      api.LogLevelSilent,
	})
	if len(result.Errors) > 0 {
		return build.DepRef{}, depBuildError(dep, result.Errors)
	}

	ref := build.DepRef{
		Name:           dep.Name,
		Version:        dep.Version,
		PublicArtifact: filepath.ToSlash(filepath.Join("dist", "_deps", artifact)),
		Integrity:      dep.Integrity,
	}
	// A stylesheet the dep's JS graph reaches is extracted by esbuild into a
	// sibling .css file of the JS artifact; publish it as the combined CSS.
	cssRel := strings.TrimSuffix(filepath.ToSlash(filepath.Join("dist", "_deps", artifact)), ".js") + ".css"
	if _, statErr := os.Stat(filepath.Join(dir, filepath.FromSlash(cssRel))); statErr == nil {
		ref.CSSArtifact = cssRel
	}
	return ref, nil
}

// bundleCSS builds the two artifacts a bare CSS subpath of a declared
// top-level dependency needs (spec §12): the .css bundle itself (esbuild,
// @import/url() within the package inlined) and the JS stub the import map
// maps the specifier to, so the consuming bundle's route-effect import stays a
// valid ES module while the real stylesheet is injected as <link> by the
// runtime shell.
func (l *Layout) bundleCSS(ctx context.Context, dir string, dep domain.LockedDep, subpath string) (build.DepRef, error) {
	specifier := dep.Name + "/" + subpath
	entry := filepath.Join(dir, "node_modules", filepath.FromSlash(dep.Name), filepath.FromSlash(subpath))
	cssArtifact := cssArtifactPath(dep, subpath)
	outCSS := filepath.Join(dir, filepath.FromSlash(cssArtifact))
	stubArtifact := cssArtifact + ".js"
	outStub := filepath.Join(dir, filepath.FromSlash(stubArtifact))

	if err := os.MkdirAll(filepath.Dir(outStub), 0o755); err != nil {
		return build.DepRef{}, fmt.Errorf("dependency %s: mkdir dist: %w", specifier, err)
	}
	result := api.Build(api.BuildOptions{
		EntryPoints:   []string{entry},
		Outfile:       outCSS,
		Bundle:        true,
		Write:         true,
		Loader:        map[string]api.Loader{".css": api.LoaderCSS},
		AbsWorkingDir: dir,
		LogLevel:      api.LogLevelSilent,
	})
	if len(result.Errors) > 0 {
		return build.DepRef{}, depBuildError(dep, result.Errors)
	}
	if err := os.WriteFile(outStub, []byte("export default undefined;\n"), 0o644); err != nil {
		return build.DepRef{}, fmt.Errorf("dependency %s: write css stub: %w", specifier, err)
	}
	return build.DepRef{
		Name:           dep.Name,
		Version:        dep.Version,
		PublicArtifact: stubArtifact,
		CSSArtifact:    cssArtifact,
		Integrity:      dep.Integrity,
	}, nil
}

// depBuildError turns esbuild failures for one dependency into a DepBuildError
// with a targeted hint for the phase-blocked cases: static assets with no
// loader, node builtins, and every other unresolvable module.
func depBuildError(dep domain.LockedDep, messages []api.Message) error {
	var details []string
	for _, msg := range messages {
		details = append(details, msg.Text)
		line := strings.ToLower(msg.Text)
		if strings.Contains(line, "no loader is configured") {
			return &build.DepBuildError{
				Pkg: dep.Name, Version: dep.Version,
				Hint: "the package references a static asset (font, image, ...) that the bundle pipeline does not support yet; CSS itself is supported — if a stylesheet pull fails, split it into its own CSS file",
			}
		}
		for builtin := range blockedBuiltins {
			if strings.Contains(line, "\""+builtin+"\"") || strings.Contains(line, "'"+builtin+"'") {
				return &build.DepBuildError{
					Pkg: dep.Name, Version: dep.Version, Missing: builtin,
					Hint: "the package imports the node built-in \"" + builtin +
						"\" which the browser runtime does not provide; pick an alternative without node APIs, or drop the import",
				}
			}
		}
	}
	return &build.DepBuildError{
		Pkg: dep.Name, Version: dep.Version,
		Missing: strings.Join(details, "; "),
	}
}

// blockedBuiltins is the static list of node built-ins that a browser bundle
// must treat as a failure (spec §6 hints for fs/net/child_process/...).
var blockedBuiltins = map[string]bool{
	"fs": true, "net": true, "path": true, "child_process": true,
	"process": true, "stream": true, "http": true, "https": true,
	"crypto": true, "os": true, "util": true, "events": true,
	"assert": true, "buffer": true, "dns": true, "zlib": true,
	"worker_threads": true, "perf_hooks": true,
	"readline": true, "timers": true, "tty": true, "url": true,
}

// artifactName builds a filesystem-safe _deps artifact name from a (possibly
// scoped) package identity: "@babel/core"@7.0.0 → "@babel%2Fcore@7.0.0.js".
func artifactName(name, version string) string {
	safe := strings.ReplaceAll(name, "/", "%2F")
	return safe + "@" + version + ".js"
}

// artifactEntry is the per-dep esbuild entry file name (no slashes, scoped-safe).
func artifactEntry(name string) string {
	return strings.ReplaceAll(name, "/", "__") + ".ts"
}

// splitTopLevel lives in the build package as build.SplitBareSpecifier; the
// layout classifies with it throughout.

// scanDepSubpaths walks the materialized source of one top-level dependency
// and returns (a) the bare subpath specifiers that must stay external in this
// dependency's bundle, and (b) the ones that additionally need their own
// _deps/ subpath artifact (both lists: subpaths of other declared top-level
// deps; (a) also covers exact shared-runtime specifiers). Anything else —
// plain top-level imports, relative paths, subpaths of transitive-only
// packages, self-subpaths — stays inlined, exactly as before. An unsafe or
// non-JS subpath of a declared dep fails with the same hint as site-source
// subpaths.
func (l *Layout) scanDepSubpaths(dir, name string, declared map[string]bool) (externals, bundles []string, err error) {
	pkgDir := filepath.Join(dir, "node_modules", name)
	// A top-level name absent from the lock is reported by the bundling loop
	// below ("declared dependency is absent from the snapshot lock"); nothing
	// to scan here.
	if _, statErr := os.Stat(pkgDir); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, nil, nil
		}
		return nil, nil, statErr
	}
	err = filepath.WalkDir(pkgDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path != pkgDir && d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !isJSFile(d.Name()) {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, spec := range build.ScanImportSpecifiers(string(data)) {
			top, rest, isSub := build.SplitBareSpecifier(spec)
			if !isSub {
				continue
			}
			if top == name {
				continue
			}
			if sharedRoots[top] {
				if sharedExternals[spec] {
					externals = append(externals, spec)
				}
				continue
			}
			if !declared[top] {
				continue
			}
			if !validSubpath(rest) {
				return &build.DepBuildError{
					Pkg: top, Hint: fmt.Sprintf("subpath %q is not a filesystem-safe JS module path", spec),
				}
			}
			// A CSS subpath of a declared dep is its own artifact like any
			// other subpath (spec §12); non-CSS assets stay a failure.
			if isCssSubpath(rest) {
				externals = append(externals, spec)
				bundles = append(bundles, spec)
				continue
			}
			if reason := unsupportedSubpath(rest); reason != "" {
				return &build.DepBuildError{Pkg: top, Hint: reason}
			}
			externals = append(externals, spec)
			bundles = append(bundles, spec)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return uniqueSorted(externals), uniqueSorted(bundles), nil
}

// isJSFile reports whether a package file is a JS module the bundle pipeline
// scans for bare subpath imports.
func isJSFile(name string) bool {
	return strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".mjs") || strings.HasSuffix(name, ".cjs")
}

// sharedRoots and sharedExternals are derived from the immutable shared
// libraries list: the backend-versioned runtime bundles (react, react-dom,
// jsx-runtime, ui-runtime) must never be re-bundled for a site. Specifiers
// that are part of the shared list already resolve via the import map; other
// subpaths of a shared root are not bundled either.
var (
	sharedRoots     = map[string]bool{}
	sharedExternals = map[string]bool{}
)

func init() {
	for _, ext := range build.SharedExternals {
		top, _, _ := build.SplitBareSpecifier(ext)
		sharedRoots[top] = true
		sharedExternals[ext] = true
	}
}

// validSubpath reports whether a subpath is a safe, plain relative JS-module
// path with no empty or escaping segments.
func validSubpath(subpath string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(subpath), "/") {
		if seg == "" || seg == "." || seg == ".." || strings.Contains(seg, "\\") ||
			strings.Contains(seg, "%2F") || strings.Contains(seg, "%2f") {
			return false
		}
	}
	return true
}

// isCssSubpath reports whether a bare subpath names a stylesheet
// ("agent/styles.css"), which the pipeline bundles as its own .css artifact
// plus an import-map stub instead of a JS re-export (spec §12).
func isCssSubpath(subpath string) bool {
	return strings.EqualFold(filepath.Ext(subpath), ".css")
}

// cssArtifactPath is the public artifact path of a CSS subpath bundle:
// dist/_deps/<name>@<version>/<subpath>.css (scoped packages keep the %2F
// escape, exactly like JS subpath artifacts).
func cssArtifactPath(dep domain.LockedDep, subpath string) string {
	base := strings.TrimSuffix(artifactName(dep.Name, dep.Version), ".js")
	return filepath.ToSlash(filepath.Join("dist", "_deps", base, subpath))
}

// unsupportedSubpath returns a plain-language reason when a subpath imports a
// static asset that the bundle pipeline does not support (spec §12): CSS is
// bundled (own artifact + <link>), fonts/images/… are still a failure.
func unsupportedSubpath(subpath string) string {
	ext := strings.ToLower(filepath.Ext(subpath))
	if !unsupportedSubpathExts[ext] {
		return ""
	}
	return fmt.Sprintf("subpath %q imports a static asset; CSS is supported (own .css artifact + <link>), but fonts, images and other assets are not bundled yet — import the package's JS or CSS file instead", subpath)
}

// unsupportedSubpathExts is the asset-extension guard for subpath bundles.
// .css is deliberately absent: it is handled by bundleCSS.
var unsupportedSubpathExts = map[string]bool{
	".scss": true, ".sass": true, ".less": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true,
	".woff": true, ".woff2": true, ".ttf": true, ".wasm": true,
}

// uniqueSorted dedupes and sorts a string slice, dropping empty entries.
func uniqueSorted(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

const maxUnpackBytes = 256 << 20 // 256 MiB safety cap per tarball

// unpackTarball extracts an npm-style gzipped tarball ("package/…" entries)
// into target, rejecting any entry that would escape the target (path
// traversal, absolute paths).
func unpackTarball(data []byte, target string) error {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("unpack tarball: gzip: %w", err)
	}
	defer reader.Close()

	var total int64
	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("unpack tarball: %w", err)
		}
		if hdr.Size < 0 {
			return fmt.Errorf("unpack tarball: negative size for %s", hdr.Name)
		}
		total += hdr.Size
		if total > maxUnpackBytes {
			return fmt.Errorf("unpack tarball: exceeds %d MiB safety cap", maxUnpackBytes>>20)
		}

		rel := stripPrefix(hdr.Name)
		if rel == "" {
			continue
		}
		if strings.HasPrefix(hdr.Name, "node_modules/") {
			continue // transitive deps inside a tarball: not needed in a flat layout
		}
		dst := filepath.Join(target, filepath.FromSlash(rel))
		if !within(target, dst) {
			return fmt.Errorf("unpack tarball: entry %q escapes the package directory", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			file, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			if _, err := io.CopyN(file, tr, hdr.Size); err != nil {
				file.Close()
				return fmt.Errorf("unpack tarball %s: %w", hdr.Name, err)
			}
			if err := file.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

// stripPrefix removes the leading "package/" directory npm tarballs use.
func stripPrefix(name string) string {
	cleaned := strings.TrimPrefix(filepath.ToSlash(name), "package/")
	if cleaned == name {
		return name
	}
	return cleaned
}

// within reports whether child is inside parent (lexically, after cleaning).
func within(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}
