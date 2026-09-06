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

	// 2. Bundle the top-level deps (the site's declared bare imports) one by
	// one. Only the shared libraries stay external: transitive dependencies
	// are inlined into the top-level bundle, per spec §6.
	result := build.DepLayout{Deps: map[string]build.DepRef{}}
	externals := make([]string, 0, len(req.TopLevel)+len(req.Subpaths))
	for _, name := range req.TopLevel {
		dep, ok := byName[name]
		if !ok {
			return build.DepLayout{}, &build.DepBuildError{
				Pkg: name, Hint: "declared dependency is absent from the snapshot lock (lock was frozen before the declaration)",
			}
		}
		ref, err := l.bundle(ctx, req.Dir, dep, "")
		if err != nil {
			return build.DepLayout{}, err
		}
		result.Deps[dep.Name] = ref
		externals = append(externals, dep.Name)
	}

	// 3. Bundle each bare subpath of a declared top-level dep the site source
	// imports directly (spec §6): one _deps/<name>@<ver>/<subpath>.js artifact
	// + import-map entry. Shared externals stay external; everything else is
	// inlined into the subpath bundle exactly like the top-level one. Subpaths
	// of undeclared (transitive) packages are not materialized — they resolve
	// from node_modules and inline where imported.
	for _, spec := range uniqueSorted(req.Subpaths) {
		top, rest, ok := splitTopLevel(spec)
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
// stay external (import-map-resolved); transitive deps are inlined (spec §6).
func (l *Layout) bundle(ctx context.Context, dir string, dep domain.LockedDep, subpath string) (build.DepRef, error) {
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

	external := append([]string{}, build.SharedExternals...)
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

// splitTopLevel splits a bare import specifier into its package name and the
// remaining subpath (if any): "x" → ("x", "", false), "x/y/z" → ("x", "y/z",
// true), "@scope/x/y" → ("@scope/x", "y", true).
func splitTopLevel(spec string) (top, rest string, isSub bool) {
	if strings.HasPrefix(spec, "@") {
		parts := strings.Split(spec, "/")
		if len(parts) < 3 {
			return spec, "", false
		}
		return parts[0] + "/" + parts[1], strings.Join(parts[2:], "/"), true
	}
	idx := strings.Index(spec, "/")
	if idx < 0 {
		return spec, "", false
	}
	return spec[:idx], spec[idx+1:], true
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
