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
	pkgs  domain.DepPackageRepository
	blobs *store.Store
	fetch TarballFetcher
}

type LayoutOptions struct {
	Packages domain.DepPackageRepository
	Store    *store.Store
	Fetch    TarballFetcher
}

func New(opts LayoutOptions) *Layout {
	return &Layout{
		pkgs:  opts.Packages,
		blobs: opts.Store,
		fetch: opts.Fetch,
	}
}

var _ build.DepLayouter = (*Layout)(nil)

// MaterializeDeps implements build.DepLayouter. An empty lock is a no-op.
func (l *Layout) MaterializeDeps(ctx context.Context, req build.DepLayoutRequest) (build.DepLayout, error) {
	if len(req.Lock.Deps) == 0 {
		return build.DepLayout{Deps: map[string]build.DepRef{}, Externals: []string{}}, nil
	}

	// 1. Ensure every locked package is laid out into node_modules/.
	byName := make(map[string]domain.LockedDep, len(req.Lock.Deps))
	for _, dep := range req.Lock.Deps {
		byName[dep.Name] = dep
		if err := l.ensure(ctx, req.Dir, dep); err != nil {
			return build.DepLayout{}, err
		}
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
	// bundle, per spec §6.
	result := build.DepLayout{Deps: map[string]build.DepRef{}}
	externals := make([]string, 0, len(req.TopLevel)+len(req.Subpaths)+len(discovered))
	for _, name := range req.TopLevel {
		dep, ok := byName[name]
		if !ok {
			return build.DepLayout{}, &build.DepBuildError{
				Pkg: name, Hint: "declared dependency is absent from the snapshot lock (lock was frozen before the declaration)",
			}
		}
		ref, err := l.bundle(ctx, req.Dir, dep, "", perDepExternals[name]...)
		if err != nil {
			return build.DepLayout{}, err
		}
		result.Deps[dep.Name] = ref
		externals = append(externals, dep.Name)
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
		if reason := unsupportedSubpath(rest); reason != "" {
			return build.DepLayout{}, &build.DepBuildError{Pkg: top, Hint: reason}
		}
		dep, ok := byName[top]
		if !ok {
			return build.DepLayout{}, &build.DepBuildError{
				Pkg: top, Hint: fmt.Sprintf("subpath import %q names a package absent from the snapshot lock", spec),
			}
		}
		ref, err := l.bundle(ctx, req.Dir, dep, rest)
		if err != nil {
			return build.DepLayout{}, err
		}
		result.Deps[spec] = ref
		externals = append(externals, spec)
	}

	result.Externals = uniqueSorted(externals)
	return result, nil
}

// ensure fetches (if missing from the disk cache), verifies and unpacks one
// locked package into node_modules/<name>.
func (l *Layout) ensure(ctx context.Context, dir string, dep domain.LockedDep) error {
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
	return unpackTarball(data, filepath.Join(dir, "node_modules", dep.Name))
}

// blob returns verified tarball bytes for a locked package, using the disk
// cache when present and fetching from the registry otherwise.
func (l *Layout) blob(ctx context.Context, dep domain.LockedDep) ([]byte, error) {
	if l.blobs.Has(dep.Name, dep.Version) {
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
	return data, nil
}

// bundle builds one _deps artifact for a top-level dependency, or for one of
// its bare subpaths (subpath != ""). The artifact is a re-export of the target
// module so any named/default import the site source makes resolves at
// runtime (esbuild omits a default re-export that has no binding, so the same
// entry works for defaultless subpath modules too). Only the shared libraries
// plus the declared extra externals (the dep-internal subpath specifiers) stay
// external (import-map-resolved); transitive deps are inlined (spec §6).
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
		External:      external,
		LogLevel:      api.LogLevelSilent,
	})
	if len(result.Errors) > 0 {
		return build.DepRef{}, depBuildError(dep, result.Errors)
	}

	return build.DepRef{
		Name:           dep.Name,
		Version:        dep.Version,
		PublicArtifact: filepath.ToSlash(filepath.Join("dist", "_deps", artifact)),
		Integrity:      dep.Integrity,
	}, nil
}

// depBuildError turns esbuild failures for one dependency into a DepBuildError
// with a targeted hint for the phase-1 blocked cases (node builtins).
func depBuildError(dep domain.LockedDep, messages []api.Message) error {
	var details []string
	for _, msg := range messages {
		details = append(details, msg.Text)
		line := strings.ToLower(msg.Text)
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

// unsupportedSubpath returns a plain-language reason when a subpath imports a
// non-JS asset, which the current bundle pipeline does not support (spec §12).
func unsupportedSubpath(subpath string) string {
	ext := strings.ToLower(filepath.Ext(subpath))
	if !unsupportedSubpathExts[ext] {
		return ""
	}
	return fmt.Sprintf("subpath %q imports a non-JS asset; CSS and static assets are not bundled yet — import the package's JS module instead", subpath)
}

// unsupportedSubpathExts is the asset-extension guard for subpath bundles.
var unsupportedSubpathExts = map[string]bool{
	".css": true, ".scss": true, ".sass": true, ".less": true,
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
