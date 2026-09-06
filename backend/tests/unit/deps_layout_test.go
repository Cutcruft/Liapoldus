package unit

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/deps/layout"
	"github.com/liapoldus/liapoldus/backend/internal/infra/deps/store"
)

type fakeFetcher struct {
	data  map[string][]byte
	calls []string
}

func (f *fakeFetcher) Tarball(_ context.Context, name, version, tarballURL, integrity string) ([]byte, error) {
	f.calls = append(f.calls, name+"@"+version)
	data, ok := f.data[name]
	if !ok {
		return nil, domain.ErrPackageNotFound
	}
	return data, nil
}

// pkgRepoStub is the immutable dep_packages cache stand-in: every tarball in
// scope publishes a non-empty sha512 integrity + a tarball URL, matching how
// the registry side records them.
type pkgRepoStub struct{ pkgs map[string]domain.DepPackage }

func (s pkgRepoStub) GetDepPackage(_ context.Context, name, version string) (domain.DepPackage, error) {
	pkg, ok := s.pkgs[name+"@"+version]
	if !ok {
		return domain.DepPackage{}, domain.ErrNotFound
	}
	return pkg, nil
}

func (pkgRepoStub) CreateDepPackage(context.Context, domain.DepPackage) error { return nil }

// layoutHarness wires a layout over a temp-dir blob store, a fake fetcher (for
// the given tarballs), and a fake dep_packages cache (integrity + tarball URL
// derived from the same tarballs). Returns the layout, fetcher and the
// workspace root the layout materializes into.
func layoutHarness(t *testing.T, tarballs map[string][]byte, sri map[string]string) (*layout.Layout, *fakeFetcher, string) {
	t.Helper()
	fetcher := &fakeFetcher{data: tarballs}
	blobs := store.New(t.TempDir())
	pkgs := make(map[string]domain.DepPackage, len(tarballs))
	for name := range tarballs {
		pkgs[name+"@"+"1.0.0"] = domain.DepPackage{
			Name: name, Version: "1.0.0", Integrity: sri[name],
			TarballURL: "https://registry.test/" + name + "/-/" + name + "-1.0.0.tgz",
		}
	}
	lay := layout.New(layout.LayoutOptions{
		Packages: pkgRepoStub{pkgs: pkgs},
		Store:    blobs,
		Fetch:    fetcher,
	})
	return lay, fetcher, blobs.Root()
}

// tarball builds an npm-style gzipped tarball with a "package/" root from a
// map of relative file paths to contents, and returns its sha512 SRI.
func tarball(t *testing.T, files map[string]string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for rel, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: "package/" + rel, Mode: 0o644, Size: int64(len(content)),
			Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	sum := sha512.Sum512(buf.Bytes())
	return buf.Bytes(), "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
}

func TestLayoutEmptyLockIsNoOp(t *testing.T) {
	lay, _, _ := layoutHarness(t, nil, nil)
	result, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: t.TempDir(), TopLevel: []string{"agent"},
	})
	if err != nil {
		t.Fatalf("empty lock: %v", err)
	}
	if len(result.Deps) != 0 || len(result.Externals) != 0 {
		t.Fatalf("empty lock must be a no-op, got %#v", result)
	}
}

func TestLayoutMaterializesAndBundlesTopLevel(t *testing.T) {
	helperData, helperSRI := tarball(t, map[string]string{
		"package.json": `{"name":"helper","version":"1.0.0","main":"index.js"}`,
		"index.js":     "export function doubling(x) { const marker = \"helper-double\"; return x * 2 + marker.length * 0; }\n",
	})
	agentData, agentSRI := tarball(t, map[string]string{
		"package.json": `{"name":"agent","version":"1.0.0","main":"index.js","dependencies":{"helper":"^1.0.0"}}`,
		"index.js": `import { doubling } from "helper";
import { createElement } from "react";
export const tag = "agent-title";
export default function title(props) { return createElement("h1", null, doubling(props.n)); }
`,
	})
	// "helper" is transitive but the harness only keys 1.0.0 per package; both
	// versions resolve from the same map, which is exactly the flat-lock case.
	lay, fetcher, root := layoutHarness(t,
		map[string][]byte{"agent": agentData, "helper": helperData},
		map[string]string{"agent": agentSRI, "helper": helperSRI},
	)
	result, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: agentSRI},
			{Name: "helper", Spec: "^1.0.0", Version: "1.0.0", Integrity: helperSRI},
		}},
		TopLevel: []string{"agent"},
	})
	if err != nil {
		t.Fatalf("materialize deps: %v", err)
	}

	ref, ok := result.Deps["agent"]
	if !ok {
		t.Fatalf("missing import-map entry for agent: %#v", result.Deps)
	}
	if ref.Version != "1.0.0" || ref.Integrity != agentSRI {
		t.Fatalf("dep ref = %#v", ref)
	}
	if ref.PublicArtifact != "dist/_deps/agent@1.0.0.js" {
		t.Fatalf("public artifact = %q", ref.PublicArtifact)
	}
	if len(result.Externals) != 1 || result.Externals[0] != "agent" {
		t.Fatalf("externals = %#v", result.Externals)
	}

	for _, rel := range []string{"node_modules/agent/package.json", "node_modules/helper/index.js"} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("layout missing %s: %v", rel, err)
		}
	}

	bundle, err := os.ReadFile(filepath.Join(root, "dist/_deps", "agent@1.0.0.js"))
	if err != nil {
		t.Fatalf("read _deps bundle: %v", err)
	}
	if !strings.Contains(string(bundle), "helper-double") {
		t.Fatalf("transitive helper must be inlined into the agent bundle:\n%s", bundle)
	}
	if !strings.Contains(string(bundle), `from "react"`) {
		t.Fatalf("react must stay external in the agent bundle:\n%s", bundle)
	}
	if len(fetcher.calls) != 2 {
		t.Fatalf("expected 2 fetches (agent, helper), got %#v", fetcher.calls)
	}
}

func TestLayoutScopedPackageArtifactName(t *testing.T) {
	coreData, coreSRI := tarball(t, map[string]string{
		"package.json": `{"name":"@liapoldus/core","version":"1.0.0","main":"index.js"}`,
		"index.js":     "export const version = \"1\";\nexport default 1;\n",
	})
	lay, _, root := layoutHarness(t,
		map[string][]byte{"@liapoldus/core": coreData},
		map[string]string{"@liapoldus/core": coreSRI},
	)
	result, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "@liapoldus/core", Spec: "^1.0.0", Version: "1.0.0", Integrity: coreSRI},
		}},
		TopLevel: []string{"@liapoldus/core"},
	})
	if err != nil {
		t.Fatalf("materialize scoped dep: %v", err)
	}
	ref := result.Deps["@liapoldus/core"]
	if ref.PublicArtifact != "dist/_deps/@liapoldus%2Fcore@1.0.0.js" {
		t.Fatalf("scoped artifact name = %q", ref.PublicArtifact)
	}
	for _, rel := range []string{
		"node_modules/@liapoldus/core/index.js",
		"dist/_deps/@liapoldus%2Fcore@1.0.0.js",
	} {
		if _, statErr := os.Stat(filepath.Join(root, rel)); statErr != nil {
			t.Fatalf("scoped layout missing %s: %v", rel, statErr)
		}
	}
}

func TestLayoutBundledDepImportingNodeBuiltinFails(t *testing.T) {
	agentData, agentSRI := tarball(t, map[string]string{
		"package.json": `{"name":"agent","version":"1.0.0","main":"index.js"}`,
		"index.js":     "import fs from \"fs\";\nexport default fs.readFileSync;\n",
	})
	lay, _, root := layoutHarness(t, map[string][]byte{"agent": agentData}, map[string]string{"agent": agentSRI})
	_, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: agentSRI},
		}},
		TopLevel: []string{"agent"},
	})
	var depErr *build.DepBuildError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected *build.DepBuildError, got %T: %v", err, err)
	}
	if depErr.Pkg != "agent" || depErr.Missing != "fs" {
		t.Fatalf("dep error = %#v", depErr)
	}
	if depErr.Hint == "" {
		t.Fatalf("expected a hint for the fs builtin")
	}
}

func TestLayoutFetchErrorPropagates(t *testing.T) {
	_, missSRI := tarball(t, map[string]string{
		"package.json": `{"name":"agent","version":"1.0.0","main":"index.js"}`,
		"index.js":     "export default 1;\n",
	})
	lay, _, root := layoutHarness(t, map[string][]byte{}, map[string]string{})
	// The locked package is not in the fake registry: fetch fails and the
	// error must surface as-is (the registry layer owns error semantics).
	_, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: missSRI},
		}},
		TopLevel: []string{"agent"},
	})
	if err == nil {
		t.Fatal("expected fetch failure to propagate")
	}
}

func TestLayoutMissingIntegrityFailsClosed(t *testing.T) {
	lay, _, root := layoutHarness(t, nil, nil)
	_, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0"},
		}},
		TopLevel: []string{"agent"},
	})
	var depErr *build.DepBuildError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected *build.DepBuildError for missing integrity, got %T: %v", err, err)
	}
}

func TestLayoutTopLevelNotInLockFails(t *testing.T) {
	agentData, agentSRI := tarball(t, map[string]string{
		"package.json": `{"name":"agent","version":"1.0.0","main":"index.js"}`,
		"index.js":     "export default 1;\n",
	})
	lay, _, root := layoutHarness(t, map[string][]byte{"agent": agentData}, map[string]string{"agent": agentSRI})
	_, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: agentSRI},
		}},
		TopLevel: []string{"ghost"},
	})
	var depErr *build.DepBuildError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected *build.DepBuildError for missing top-level, got %T: %v", err, err)
	}
	if depErr.Pkg != "ghost" {
		t.Fatalf("Pkg = %q", depErr.Pkg)
	}
}

func TestLayoutMetadataIntegrityMismatchFails(t *testing.T) {
	helperData, helperSRI := tarball(t, map[string]string{
		"package.json": `{"name":"helper","version":"1.0.0","main":"index.js"}`,
		"index.js":     "export default 1;\n",
	})
	blobs := store.New(t.TempDir())
	lay := layout.New(layout.LayoutOptions{
		Packages: pkgRepoStub{pkgs: map[string]domain.DepPackage{
			"helper@1.0.0": {
				Name: "helper", Version: "1.0.0",
				Integrity:  "sha512-mismatched-cache-entry",
				TarballURL: "https://registry.test/helper/-/helper-1.0.0.tgz",
			},
		}},
		Store: blobs,
		Fetch: &fakeFetcher{data: map[string][]byte{"helper": helperData}},
	})
	_, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: blobs.Root(),
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "helper", Spec: "^1.0.0", Version: "1.0.0", Integrity: helperSRI},
		}},
		TopLevel: []string{"helper"},
	})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected integrity mismatch error, got %v", err)
	}
}

func TestLayoutDiskCacheSkipsFetchAndMetadata(t *testing.T) {
	helperData, helperSRI := tarball(t, map[string]string{
		"package.json": `{"name":"helper","version":"1.0.0","main":"index.js"}`,
		"index.js":     "export const n = 1;\nexport default 1;\n",
	})

	// Seed the disk cache with the verified blob directly.
	blobs := store.New(t.TempDir())
	if err := blobs.Save("helper", "1.0.0", helperData); err != nil {
		t.Fatal(err)
	}
	// With the blob cached, the layout must never touch registry metadata (nil
	// repo) nor fetch (counting fetcher stays empty).
	fetcher := &fakeFetcher{data: map[string][]byte{}}
	lay := layout.New(layout.LayoutOptions{Packages: nil, Store: blobs, Fetch: fetcher})

	result, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: blobs.Root(),
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "helper", Spec: "^1.0.0", Version: "1.0.0", Integrity: helperSRI},
		}},
		TopLevel: []string{"helper"},
	})
	if err != nil {
		t.Fatalf("materialize from disk cache: %v", err)
	}
	if ref := result.Deps["helper"]; ref.PublicArtifact != "dist/_deps/helper@1.0.0.js" {
		t.Fatalf("dep ref = %#v", ref)
	}
	if len(fetcher.calls) != 0 {
		t.Fatalf("disk cache must skip the registry, got %#v", fetcher.calls)
	}
	if _, statErr := os.Stat(filepath.Join(blobs.Root(), "node_modules", "helper", "index.js")); statErr != nil {
		t.Fatalf("layout from cache missing node_modules: %v", statErr)
	}
}

// versionedFetcher serves tarballs keyed by name@version so a test can host
// two versions of the same package (the nested-versioned layout case).
type versionedFetcher struct {
	data  map[string][]byte
	calls []string
}

func (f *versionedFetcher) Tarball(_ context.Context, name, version, _, _ string) ([]byte, error) {
	key := name + "@" + version
	f.calls = append(f.calls, key)
	data, ok := f.data[key]
	if !ok {
		return nil, domain.ErrPackageNotFound
	}
	return data, nil
}

// layoutHarnessVersioned wires a layout whose registry serves distinct
// tarballs per name@version, including cache metadata for each.
func layoutHarnessVersioned(t *testing.T, tarballs map[string][]byte, sri map[string]string) (*layout.Layout, *versionedFetcher, string) {
	t.Helper()
	fetcher := &versionedFetcher{data: tarballs}
	blobs := store.New(t.TempDir())
	pkgs := make(map[string]domain.DepPackage, len(tarballs))
	for key := range tarballs {
		name, version, _ := strings.Cut(key, "@")
		pkgs[key] = domain.DepPackage{
			Name: name, Version: version, Integrity: sri[key],
			TarballURL: "https://registry.test/" + name + "/-/" + name + "-" + version + ".tgz",
		}
	}
	lay := layout.New(layout.LayoutOptions{
		Packages: pkgRepoStub{pkgs: pkgs},
		Store:    blobs,
		Fetch:    fetcher,
	})
	return lay, fetcher, blobs.Root()
}

func TestLayoutNestedVersionsPlaceAndResolve(t *testing.T) {
	b1Data, b1SRI := tarball(t, map[string]string{
		"package.json": `{"name":"b","version":"1.0.0","main":"index.js"}`,
		"index.js":     "export const bMarker = \"b-ver-1\";\n",
	})
	b2Data, b2SRI := tarball(t, map[string]string{
		"package.json": `{"name":"b","version":"2.0.0","main":"index.js"}`,
		"index.js":     "export const bMarker = \"b-ver-2\";\n",
	})
	t1Data, t1SRI := tarball(t, map[string]string{
		"package.json": `{"name":"t1","version":"1.0.0","main":"index.js","dependencies":{"b":"^1.0.0"}}`,
		"index.js": `import { bMarker } from "b";
export const t1ID = "t1-" + bMarker;
export default t1ID;
`,
	})
	t2Data, t2SRI := tarball(t, map[string]string{
		"package.json": `{"name":"t2","version":"1.0.0","main":"index.js","dependencies":{"b":"^2.0.0"}}`,
		"index.js": `import { bMarker } from "b";
export const t2ID = "t2-" + bMarker;
export default t2ID;
`,
	})
	lay, fetcher, root := layoutHarnessVersioned(t, map[string][]byte{
		"t1@1.0.0": t1Data, "t2@1.0.0": t2Data, "b@1.0.0": b1Data, "b@2.0.0": b2Data,
	}, map[string]string{
		"t1@1.0.0": t1SRI, "t2@1.0.0": t2SRI, "b@1.0.0": b1SRI, "b@2.0.0": b2SRI,
	})
	_, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "t1", Spec: "^1.0.0", Version: "1.0.0", Integrity: t1SRI, Hoisted: true, RequestedBy: []string{"site"}},
			{Name: "t2", Spec: "^1.0.0", Version: "1.0.0", Integrity: t2SRI, Hoisted: true, RequestedBy: []string{"site"}},
			{Name: "b", Spec: "^1.0.0", Version: "1.0.0", Integrity: b1SRI, Hoisted: true, RequestedBy: []string{"t1@1.0.0"}},
			{Name: "b", Spec: "^2.0.0", Version: "2.0.0", Integrity: b2SRI, RequestedBy: []string{"t2@1.0.0"}},
		}},
		TopLevel: []string{"t1", "t2"},
	})
	if err != nil {
		t.Fatalf("materialize deps: %v", err)
	}

	// The hoisted b@1.0.0 sits at the root; b@2.0.0 nests under its requester.
	rootB, err := os.ReadFile(filepath.Join(root, "node_modules", "b", "index.js"))
	if err != nil {
		t.Fatalf("read hoisted b: %v", err)
	}
	if !strings.Contains(string(rootB), "b-ver-1") {
		t.Fatalf("hoisted node_modules/b must be version 1:\n%s", rootB)
	}
	nestedB, err := os.ReadFile(filepath.Join(root, "node_modules", "t2", "node_modules", "b", "index.js"))
	if err != nil {
		t.Fatalf("read nested b under t2: %v", err)
	}
	if !strings.Contains(string(nestedB), "b-ver-2") {
		t.Fatalf("node_modules/t2/node_modules/b must be version 2:\n%s", nestedB)
	}
	if _, err := os.Stat(filepath.Join(root, "node_modules", "t1", "node_modules", "b", "index.js")); err == nil {
		t.Fatalf("t1 must not get its own nested b (it reuses the hoisted v1)")
	}

	// esbuild resolves each bundle to the version whose physical path is
	// closest — the right version per consumer.
	t1Bundle, err := os.ReadFile(filepath.Join(root, "dist/_deps", "t1@1.0.0.js"))
	if err != nil {
		t.Fatalf("read t1 bundle: %v", err)
	}
	if !strings.Contains(string(t1Bundle), "b-ver-1") || strings.Contains(string(t1Bundle), "b-ver-2") {
		t.Fatalf("t1 bundle must inline b@1.0.0 only:\n%s", t1Bundle)
	}
	t2Bundle, err := os.ReadFile(filepath.Join(root, "dist/_deps", "t2@1.0.0.js"))
	if err != nil {
		t.Fatalf("read t2 bundle: %v", err)
	}
	if !strings.Contains(string(t2Bundle), "b-ver-2") || strings.Contains(string(t2Bundle), "b-ver-1") {
		t.Fatalf("t2 bundle must inline b@2.0.0 only:\n%s", t2Bundle)
	}

	wantCalls := []string{"t1@1.0.0", "t2@1.0.0", "b@1.0.0", "b@2.0.0"}
	for _, want := range wantCalls {
		found := false
		for _, got := range fetcher.calls {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing fetch for %s in %#v", want, fetcher.calls)
		}
	}
}

func TestLayoutNestedInstanceWithUnknownParentFails(t *testing.T) {
	lay, _, root := layoutHarnessVersioned(t, nil, nil)
	_, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "t1", Spec: "^1.0.0", Version: "1.0.0", Hoisted: true, RequestedBy: []string{"site"}},
			{Name: "b", Spec: "^1.0.0", Version: "1.0.0", RequestedBy: []string{"ghost@1.0.0"}},
		}},
		TopLevel: []string{"t1"},
	})
	var depErr *build.DepBuildError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected *build.DepBuildError for unknown parent, got %T: %v", err, err)
	}
	if depErr.Pkg != "b" || !strings.Contains(depErr.Hint, "ghost@1.0.0") {
		t.Fatalf("dep error = %#v", depErr)
	}
}

func TestLayoutNestedInstanceRequestedBySiteFails(t *testing.T) {
	lay, _, root := layoutHarnessVersioned(t, nil, nil)
	_, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "t1", Spec: "^1.0.0", Version: "1.0.0", Hoisted: true, RequestedBy: []string{"site"}},
			{Name: "b", Spec: "^1.0.0", Version: "1.0.0", RequestedBy: []string{"site"}},
		}},
		TopLevel: []string{"t1"},
	})
	var depErr *build.DepBuildError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected *build.DepBuildError for site-requested nested instance, got %T: %v", err, err)
	}
	if depErr.Pkg != "b" || !strings.Contains(depErr.Hint, "hoisted") {
		t.Fatalf("dep error = %#v", depErr)
	}
}

func TestLayoutSoleNestedInstanceNotMisplacedTowardsRoot(t *testing.T) {
	// A package whose only version in the whole graph is nested (a grandchild
	// dependency nobody else shares) must NOT be treated as hoisted just
	// because it is the sole instance of its name.
	bData, bSRI := tarball(t, map[string]string{
		"package.json": `{"name":"b","version":"1.0.0","main":"index.js"}`,
		"index.js":     "export const bMarker = \"deep-b\";\n",
	})
	aData, aSRI := tarball(t, map[string]string{
		"package.json": `{"name":"a","version":"1.0.0","main":"index.js","dependencies":{"b":"^1.0.0"}}`,
		"index.js": `import { bMarker } from "b";
export const aID = "a-" + bMarker;
export default aID;
`,
	})
	t1Data, t1SRI := tarball(t, map[string]string{
		"package.json": `{"name":"t1","version":"1.0.0","main":"index.js","dependencies":{"a":"^1.0.0"}}`,
		"index.js": `import { aID } from "a";
export default aID;
`,
	})
	lay, _, root := layoutHarnessVersioned(t, map[string][]byte{
		"t1@1.0.0": t1Data, "a@1.0.0": aData, "b@1.0.0": bData,
	}, map[string]string{
		"t1@1.0.0": t1SRI, "a@1.0.0": aSRI, "b@1.0.0": bSRI,
	})
	_, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "t1", Spec: "^1.0.0", Version: "1.0.0", Integrity: t1SRI, Hoisted: true, RequestedBy: []string{"site"}},
			{Name: "a", Spec: "^1.0.0", Version: "1.0.0", Integrity: aSRI, Hoisted: true, RequestedBy: []string{"t1@1.0.0"}},
			{Name: "b", Spec: "^1.0.0", Version: "1.0.0", Integrity: bSRI, RequestedBy: []string{"a@1.0.0"}},
		}},
		TopLevel: []string{"t1"},
	})
	if err != nil {
		t.Fatalf("materialize deps: %v", err)
	}
	nestedB, err := os.ReadFile(filepath.Join(root, "node_modules", "a", "node_modules", "b", "index.js"))
	if err != nil {
		t.Fatalf("b must nest under a despite being the sole instance: %v", err)
	}
	if !strings.Contains(string(nestedB), "deep-b") {
		t.Fatalf("nested b content wrong:\n%s", nestedB)
	}
	if _, err := os.Stat(filepath.Join(root, "node_modules", "b", "index.js")); err == nil {
		t.Fatalf("sole nested b must not be hoisted to the root")
	}
	bundle, err := os.ReadFile(filepath.Join(root, "dist/_deps", "t1@1.0.0.js"))
	if err != nil {
		t.Fatalf("read t1 bundle: %v", err)
	}
	if !strings.Contains(string(bundle), "deep-b") {
		t.Fatalf("t1 bundle must inline the deeply nested b:\n%s", bundle)
	}
}

// subpathFixture returns an agent tarball whose package entry imports ./lib/util.js
// (relative, inlined into the main bundle) and whose lib/util.js exposes only a
// named export — so the subpath artifact exercises the named-only fallback.
func subpathAgentTarball(t *testing.T) (data []byte, sri string) {
	t.Helper()
	return tarball(t, map[string]string{
		"package.json": `{"name":"agent","version":"1.0.0","main":"index.js"}`,
		"index.js": `import { utilMarker } from "./lib/util.js";
export const agentName = "agent-live";
export default function Agent() { return utilMarker; }
`,
		"lib/util.js": "export const utilMarker = \"agent-subpath-live\";\n",
	})
}

func TestLayoutSubpathBundledAndDebouncedExternals(t *testing.T) {
	agentData, agentSRI := subpathAgentTarball(t)
	lay, _, root := layoutHarness(t, map[string][]byte{"agent": agentData}, map[string]string{"agent": agentSRI})
	result, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: agentSRI},
		}},
		TopLevel: []string{"agent"},
		Subpaths: []string{"agent/lib/util", "agent/lib/util"}, // duplicated on purpose
	})
	if err != nil {
		t.Fatalf("materialize deps with subpath: %v", err)
	}

	subRef, ok := result.Deps["agent/lib/util"]
	if !ok {
		t.Fatalf("missing import-map entry for agent/lib/util: %#v", result.Deps)
	}
	if subRef.Version != "1.0.0" || subRef.Integrity != agentSRI {
		t.Fatalf("subpath dep ref = %#v", subRef)
	}
	if subRef.PublicArtifact != "dist/_deps/agent@1.0.0/lib/util.js" {
		t.Fatalf("subpath public artifact = %q", subRef.PublicArtifact)
	}
	if mainRef, ok := result.Deps["agent"]; !ok || mainRef.PublicArtifact != "dist/_deps/agent@1.0.0.js" {
		t.Fatalf("main dep ref must survive alongside the subpath: %#v", result.Deps)
	}
	if got, want := result.Externals, []string{"agent", "agent/lib/util"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("externals = %#v, want %#v", got, want)
	}

	bundle, err := os.ReadFile(filepath.Join(root, "dist/_deps", "agent@1.0.0", "lib", "util.js"))
	if err != nil {
		t.Fatalf("read subpath bundle: %v", err)
	}
	if !strings.Contains(string(bundle), "agent-subpath-live") {
		t.Fatalf("subpath bundle must carry the module exports:\n%s", bundle)
	}
}

func TestLayoutScopedSubpathArtifactName(t *testing.T) {
	coreData, coreSRI := tarball(t, map[string]string{
		"package.json": `{"name":"@liapoldus/core","version":"1.0.0","main":"index.js"}`,
		"index.js":     "export const version = \"1\";\nexport default 1;\n",
		"lib/parser.js": `export const parse = (x) => x;
`,
	})
	lay, _, root := layoutHarness(t,
		map[string][]byte{"@liapoldus/core": coreData},
		map[string]string{"@liapoldus/core": coreSRI},
	)
	result, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "@liapoldus/core", Spec: "^1.0.0", Version: "1.0.0", Integrity: coreSRI},
		}},
		TopLevel: []string{"@liapoldus/core"},
		Subpaths: []string{"@liapoldus/core/lib/parser"},
	})
	if err != nil {
		t.Fatalf("materialize scoped subpath: %v", err)
	}
	ref := result.Deps["@liapoldus/core/lib/parser"]
	if ref.PublicArtifact != "dist/_deps/@liapoldus%2Fcore@1.0.0/lib/parser.js" {
		t.Fatalf("scoped subpath artifact = %q", ref.PublicArtifact)
	}
	if _, statErr := os.Stat(filepath.Join(root, "dist/_deps", "@liapoldus%2Fcore@1.0.0", "lib", "parser.js")); statErr != nil {
		t.Fatalf("scoped subpath layout missing artifact: %v", statErr)
	}
}

func TestLayoutSubpathOfUndeclaredDepIgnored(t *testing.T) {
	helperData, helperSRI := tarball(t, map[string]string{
		"package.json": `{"name":"helper","version":"1.0.0","main":"index.js"}`,
		"index.js":     "export default 1;\n",
	})
	agentData, agentSRI := subpathAgentTarball(t)
	lay, _, root := layoutHarness(t,
		map[string][]byte{"agent": agentData, "helper": helperData},
		map[string]string{"agent": agentSRI, "helper": helperSRI},
	)
	// agent is locked (transitive) but only helper is declared top-level: the
	// agent/lib/util subpath must be ignored, not bundled.
	result, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "helper", Spec: "^1.0.0", Version: "1.0.0", Integrity: helperSRI},
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: agentSRI},
		}},
		TopLevel: []string{"helper"},
		Subpaths: []string{"agent/lib/util"},
	})
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if _, ok := result.Deps["agent/lib/util"]; ok {
		t.Fatalf("subpath of an undeclared dep must not be materialized: %#v", result.Deps)
	}
	if got, want := result.Externals, []string{"helper"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("externals = %#v, want %#v", got, want)
	}
}

func TestLayoutSubpathMissingModuleFails(t *testing.T) {
	agentData, agentSRI := subpathAgentTarball(t)
	lay, _, root := layoutHarness(t, map[string][]byte{"agent": agentData}, map[string]string{"agent": agentSRI})
	_, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: agentSRI},
		}},
		TopLevel: []string{"agent"},
		Subpaths: []string{"agent/nope"},
	})
	var depErr *build.DepBuildError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected *build.DepBuildError for unresolvable subpath, got %T: %v", err, err)
	}
	if depErr.Pkg != "agent" || !strings.Contains(depErr.Missing, "agent/nope") {
		t.Fatalf("dep error = %#v", depErr)
	}
}

func TestLayoutDepInternalSubpathBundledAndExternal(t *testing.T) {
	helperData, helperSRI := tarball(t, map[string]string{
		"package.json": `{"name":"helper","version":"1.0.0","main":"index.js"}`,
		"index.js":     "export function doubling(x) { return x * 2; }\n",
		"lib/math.js":  "export const mathMarker = \"hv-math\";\nexport const scale = (n) => n * 10;\n",
	})
	agentData, agentSRI := tarball(t, map[string]string{
		"package.json": `{"name":"agent","version":"1.0.0","main":"index.js"}`,
		"index.js": `import { scale, mathMarker } from "helper/lib/math";
import { createElement } from "react";
export const tag = mathMarker;
export default function agent() { return createElement("span", null, scale(2)); }
`,
	})
	lay, _, root := layoutHarness(t,
		map[string][]byte{"agent": agentData, "helper": helperData},
		map[string]string{"agent": agentSRI, "helper": helperSRI},
	)
	result, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: agentSRI},
			{Name: "helper", Spec: "^1.0.0", Version: "1.0.0", Integrity: helperSRI},
		}},
		TopLevel: []string{"agent", "helper"},
	})
	if err != nil {
		t.Fatalf("materialize deps: %v", err)
	}

	subRef, ok := result.Deps["helper/lib/math"]
	if !ok {
		t.Fatalf("missing import-map entry for helper/lib/math: %#v", result.Deps)
	}
	if subRef.PublicArtifact != "dist/_deps/helper@1.0.0/lib/math.js" {
		t.Fatalf("subpath public artifact = %q", subRef.PublicArtifact)
	}
	found := false
	for _, ext := range result.Externals {
		if ext == "helper/lib/math" {
			found = true
		}
	}
	if !found {
		t.Fatalf("externals must include helper/lib/math: %#v", result.Externals)
	}

	// The agent bundle keeps the subpath external instead of inlining it.
	agentBundle, err := os.ReadFile(filepath.Join(root, "dist/_deps", "agent@1.0.0.js"))
	if err != nil {
		t.Fatalf("read agent bundle: %v", err)
	}
	if !strings.Contains(string(agentBundle), `from "helper/lib/math"`) {
		t.Fatalf("agent bundle must import helper/lib/math externally:\n%s", agentBundle)
	}
	if strings.Contains(string(agentBundle), "hv-math") {
		t.Fatalf("helper/lib/math must not be inlined into the agent bundle:\n%s", agentBundle)
	}

	// Its own artifact carries the module exports.
	subpathBundle, err := os.ReadFile(filepath.Join(root, "dist/_deps", "helper@1.0.0", "lib", "math.js"))
	if err != nil {
		t.Fatalf("read helper subpath bundle: %v", err)
	}
	if !strings.Contains(string(subpathBundle), "hv-math") {
		t.Fatalf("helper subpath bundle must carry the module exports:\n%s", subpathBundle)
	}
}

func TestLayoutDepInternalSubpathOfTransitiveStaysInlined(t *testing.T) {
	helperData, helperSRI := tarball(t, map[string]string{
		"package.json": `{"name":"helper","version":"1.0.0","main":"index.js"}`,
		"index.js":     "export default 1;\n",
		"lib/math.js":  "export const mathMarker = \"hv-math\";\n",
	})
	agentData, agentSRI := tarball(t, map[string]string{
		"package.json": `{"name":"agent","version":"1.0.0","main":"index.js"}`,
		"index.js": `import { mathMarker } from "helper/lib/math";
export const tag = mathMarker;
export default tag;
`,
	})
	lay, _, root := layoutHarness(t,
		map[string][]byte{"agent": agentData, "helper": helperData},
		map[string]string{"agent": agentSRI, "helper": helperSRI},
	)
	// helper is locked but NOT declared top-level: its subpath must stay
	// inlined into the agent bundle, with no artifact of its own.
	result, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: agentSRI},
			{Name: "helper", Spec: "^1.0.0", Version: "1.0.0", Integrity: helperSRI},
		}},
		TopLevel: []string{"agent"},
	})
	if err != nil {
		t.Fatalf("materialize deps: %v", err)
	}
	if _, ok := result.Deps["helper/lib/math"]; ok {
		t.Fatalf("subpath of a transitive-only dep must not be materialized: %#v", result.Deps)
	}
	if got, want := result.Externals, []string{"agent"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("externals = %#v, want %#v", got, want)
	}
	agentBundle, err := os.ReadFile(filepath.Join(root, "dist/_deps", "agent@1.0.0.js"))
	if err != nil {
		t.Fatalf("read agent bundle: %v", err)
	}
	if !strings.Contains(string(agentBundle), "hv-math") {
		t.Fatalf("transitive subpath must be inlined into the agent bundle:\n%s", agentBundle)
	}
}

func TestLayoutDepInternalSelfSubpathInlined(t *testing.T) {
	agentData, agentSRI := tarball(t, map[string]string{
		"package.json": `{"name":"agent","version":"1.0.0","main":"index.js"}`,
		"index.js": `import { utilMarker } from "agent/lib/util";
export const tag = utilMarker;
export default tag;
`,
		"lib/util.js": "export const utilMarker = \"self-subpath\";\n",
	})
	lay, _, root := layoutHarness(t, map[string][]byte{"agent": agentData}, map[string]string{"agent": agentSRI})
	// A dep importing one of its own subpaths bare is a self-reference: it
	// resolves from node_modules and inlines (no self-external artifact).
	result, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: agentSRI},
		}},
		TopLevel: []string{"agent"},
	})
	if err != nil {
		t.Fatalf("materialize deps: %v", err)
	}
	if len(result.Deps) != 1 {
		t.Fatalf("self-subpath must not create an artifact: %#v", result.Deps)
	}
	agentBundle, err := os.ReadFile(filepath.Join(root, "dist/_deps", "agent@1.0.0.js"))
	if err != nil {
		t.Fatalf("read agent bundle: %v", err)
	}
	if !strings.Contains(string(agentBundle), "self-subpath") {
		t.Fatalf("self-subpath must be inlined into the agent bundle:\n%s", agentBundle)
	}
}

func TestLayoutDepInternalSharedRootStaysExternal(t *testing.T) {
	agentData, agentSRI := tarball(t, map[string]string{
		"package.json": `{"name":"agent","version":"1.0.0","main":"index.js"}`,
		"index.js": `import { jsx } from "react/jsx-runtime";
export const tag = jsx;
export default tag;
`,
	})
	lay, _, root := layoutHarness(t, map[string][]byte{"agent": agentData}, map[string]string{"agent": agentSRI})
	// react isn't in the lock and must not be materialized: it is a shared
	// runtime library, already external by default. The scan must neither
	// bundle a react subpath nor fail on the unresolvable bare specifier.
	result, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: agentSRI},
		}},
		TopLevel: []string{"agent"},
	})
	if err != nil {
		t.Fatalf("materialize deps with shared-root subpath: %v", err)
	}
	if len(result.Deps) != 1 {
		t.Fatalf("shared-root subpath must not create an artifact: %#v", result.Deps)
	}
	agentBundle, err := os.ReadFile(filepath.Join(root, "dist/_deps", "agent@1.0.0.js"))
	if err != nil {
		t.Fatalf("read agent bundle: %v", err)
	}
	if !strings.Contains(string(agentBundle), `from "react/jsx-runtime"`) {
		t.Fatalf("react/jsx-runtime must stay external:\n%s", agentBundle)
	}
}

func TestLayoutDepInternalStaticAssetSubpathFailsWithHint(t *testing.T) {
	helperData, helperSRI := tarball(t, map[string]string{
		"package.json": `{"name":"helper","version":"1.0.0","main":"index.js"}`,
		"index.js":     "export default 1;\n",
		"logo.png":     "not a png",
	})
	agentData, agentSRI := tarball(t, map[string]string{
		"package.json": `{"name":"agent","version":"1.0.0","main":"index.js"}`,
		"index.js": `import "helper/logo.png";
export default 1;
`,
	})
	lay, _, root := layoutHarness(t,
		map[string][]byte{"agent": agentData, "helper": helperData},
		map[string]string{"agent": agentSRI, "helper": helperSRI},
	)
	// A top-level dep importing a static asset of another declared dep fails
	// with the same hint as a site-source asset subpath import (CSS itself is
	// supported; images/fonts are not).
	_, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: agentSRI},
			{Name: "helper", Spec: "^1.0.0", Version: "1.0.0", Integrity: helperSRI},
		}},
		TopLevel: []string{"agent", "helper"},
	})
	var depErr *build.DepBuildError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected *build.DepBuildError for asset subpath, got %T: %v", err, err)
	}
	if depErr.Pkg != "helper" || !strings.Contains(depErr.Hint, "asset") {
		t.Fatalf("asset subpath must carry a hint on the owning package, got %#v", depErr)
	}
}

func TestLayoutStaticAssetSubpathFailsWithHint(t *testing.T) {
	agentData, agentSRI := tarball(t, map[string]string{
		"package.json": `{"name":"agent","version":"1.0.0","main":"index.js"}`,
		"index.js":     "export default 1;\n",
		"logo.png":     "not a png",
	})
	lay, _, root := layoutHarness(t, map[string][]byte{"agent": agentData}, map[string]string{"agent": agentSRI})
	_, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: agentSRI},
		}},
		TopLevel: []string{"agent"},
		Subpaths: []string{"agent/logo.png"},
	})
	var depErr *build.DepBuildError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected *build.DepBuildError for asset subpath, got %T: %v", err, err)
	}
	if !strings.Contains(depErr.Hint, "asset") {
		t.Fatalf("asset subpath must carry a hint, got %#v", depErr)
	}
}

func TestLayoutTopLevelDepCombinedCSSArtifact(t *testing.T) {
	// The site imports the package's main entry; the entry pulls ./styles.css
	// relatively. esbuild's css loader strips the import from the JS bundle and
	// emits a sibling combined .css artifact, which the layout publishes.
	agentData, agentSRI := tarball(t, map[string]string{
		"package.json": `{"name":"agent","version":"1.0.0","main":"index.js"}`,
		"index.js": `import "./styles.css";
export const tag = "agent-with-css";
export default tag;
`,
		"styles.css": ".agent-with-css { color: #123456; }\n",
	})
	lay, _, root := layoutHarness(t, map[string][]byte{"agent": agentData}, map[string]string{"agent": agentSRI})
	result, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: agentSRI},
		}},
		TopLevel: []string{"agent"},
	})
	if err != nil {
		t.Fatalf("materialize dep with css: %v", err)
	}
	ref, ok := result.Deps["agent"]
	if !ok {
		t.Fatalf("missing import-map entry for agent: %#v", result.Deps)
	}
	if ref.CSSArtifact != "dist/_deps/agent@1.0.0.css" {
		t.Fatalf("combined css artifact = %q", ref.CSSArtifact)
	}
	if got := result.Styles; len(got) != 1 || got[0] != ref.CSSArtifact {
		t.Fatalf("styles = %#v, want only the combined css", got)
	}

	bundle, err := os.ReadFile(filepath.Join(root, "dist/_deps", "agent@1.0.0.js"))
	if err != nil {
		t.Fatalf("read agent bundle: %v", err)
	}
	if strings.Contains(string(bundle), ".agent-with-css") || strings.Contains(string(bundle), "#123456") {
		t.Fatalf("css must be stripped from the JS bundle:\n%s", bundle)
	}
	css, err := os.ReadFile(filepath.Join(root, "dist/_deps", "agent@1.0.0.css"))
	if err != nil {
		t.Fatalf("read combined css artifact: %v", err)
	}
	if !strings.Contains(string(css), ".agent-with-css") {
		t.Fatalf("combined css artifact must carry the rule:\n%s", css)
	}
}

func TestLayoutSiteCSSSubpathBundledWithStub(t *testing.T) {
	// The site pulls agent/styles.css directly: it becomes its own .css
	// artifact + a JS stub the import map maps the specifier to.
	agentData, agentSRI := tarball(t, map[string]string{
		"package.json": `{"name":"agent","version":"1.0.0","main":"index.js"}`,
		"index.js":     "export default 1;\n",
		"styles.css":   ".agent-css-subpath { color: #654321; }\n",
	})
	lay, _, root := layoutHarness(t, map[string][]byte{"agent": agentData}, map[string]string{"agent": agentSRI})
	result, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: agentSRI},
		}},
		TopLevel: []string{"agent"},
		Subpaths: []string{"agent/styles.css"},
	})
	if err != nil {
		t.Fatalf("materialize css subpath: %v", err)
	}
	ref, ok := result.Deps["agent/styles.css"]
	if !ok {
		t.Fatalf("missing import-map entry for agent/styles.css: %#v", result.Deps)
	}
	if ref.PublicArtifact != "dist/_deps/agent@1.0.0/styles.css.js" {
		t.Fatalf("css subpath stub artifact = %q", ref.PublicArtifact)
	}
	if ref.CSSArtifact != "dist/_deps/agent@1.0.0/styles.css" {
		t.Fatalf("css subpath artifact = %q", ref.CSSArtifact)
	}
	if mainRef, ok := result.Deps["agent"]; !ok || mainRef.PublicArtifact != "dist/_deps/agent@1.0.0.js" {
		t.Fatalf("main dep ref must survive alongside the css subpath: %#v", result.Deps)
	}
	if got, want := result.Externals, []string{"agent", "agent/styles.css"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("externals = %#v, want %#v", got, want)
	}
	if got := result.Styles; len(got) != 1 || got[0] != ref.CSSArtifact {
		t.Fatalf("styles = %#v, want the css subpath artifact", got)
	}

	stub, err := os.ReadFile(filepath.Join(root, "dist/_deps", "agent@1.0.0", "styles.css.js"))
	if err != nil {
		t.Fatalf("read css stub: %v", err)
	}
	if string(stub) != "export default undefined;\n" {
		t.Fatalf("css stub must be a bare valid ESM module, got %q", stub)
	}
	css, err := os.ReadFile(filepath.Join(root, "dist/_deps", "agent@1.0.0", "styles.css"))
	if err != nil {
		t.Fatalf("read css artifact: %v", err)
	}
	if !strings.Contains(string(css), ".agent-css-subpath") {
		t.Fatalf("css artifact must carry the rule:\n%s", css)
	}
}

func TestLayoutDepInternalBareCSSSubpathBundled(t *testing.T) {
	// helper/styles.css is a css subpath of a declared dep; agent pulls it via
	// a bare specifier. The scan must detect it, the agent bundle must keep the
	// import external, and the css subpath gets its own artifact + stub.
	helperData, helperSRI := tarball(t, map[string]string{
		"package.json": `{"name":"helper","version":"1.0.0","main":"index.js"}`,
		"index.js":     "export default 1;\n",
		"styles.css":   ".helper-dep-css { color: #abcdef; }\n",
	})
	agentData, agentSRI := tarball(t, map[string]string{
		"package.json": `{"name":"agent","version":"1.0.0","main":"index.js"}`,
		"index.js": `import "helper/styles.css";
export const tag = "agent-uses-css";
export default tag;
`,
	})
	lay, _, root := layoutHarness(t,
		map[string][]byte{"agent": agentData, "helper": helperData},
		map[string]string{"agent": agentSRI, "helper": helperSRI},
	)
	result, err := lay.MaterializeDeps(context.Background(), build.DepLayoutRequest{
		Dir: root,
		Lock: domain.SnapshotLock{Deps: []domain.LockedDep{
			{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: agentSRI},
			{Name: "helper", Spec: "^1.0.0", Version: "1.0.0", Integrity: helperSRI},
		}},
		TopLevel: []string{"agent", "helper"},
	})
	if err != nil {
		t.Fatalf("materialize dep-internal css subpath: %v", err)
	}
	ref, ok := result.Deps["helper/styles.css"]
	if !ok {
		t.Fatalf("missing import-map entry for helper/styles.css: %#v", result.Deps)
	}
	if ref.CSSArtifact != "dist/_deps/helper@1.0.0/styles.css" {
		t.Fatalf("dep-internal css subpath artifact = %q", ref.CSSArtifact)
	}
	if got := result.Styles; len(got) != 1 || got[0] != ref.CSSArtifact {
		t.Fatalf("styles = %#v, want the dep-internal css artifact", got)
	}

	agentBundle, err := os.ReadFile(filepath.Join(root, "dist/_deps", "agent@1.0.0.js"))
	if err != nil {
		t.Fatalf("read agent bundle: %v", err)
	}
	if !strings.Contains(string(agentBundle), `"helper/styles.css"`) {
		t.Fatalf("helper/styles.css must stay external in the agent bundle:\n%s", agentBundle)
	}
	css, err := os.ReadFile(filepath.Join(root, "dist/_deps", "helper@1.0.0", "styles.css"))
	if err != nil {
		t.Fatalf("read dep-internal css artifact: %v", err)
	}
	if !strings.Contains(string(css), ".helper-dep-css") {
		t.Fatalf("dep-internal css artifact must carry the rule:\n%s", css)
	}
}
