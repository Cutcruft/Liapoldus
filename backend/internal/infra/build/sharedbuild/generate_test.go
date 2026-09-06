package sharedbuild

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/infra/deps/registry"
)

// sharedRegistry serves both abbreviated packuments and npm tarball bytes for
// the react family, so the generator's resolve+fetch run against genuine
// registry semantics (real sha512 SRIs over the served bytes).
type sharedRegistry struct {
	tarballs map[string]map[string][]byte
	baseURL  string
	mu       sync.Mutex
	paths    []string
}

func newSharedRegistry() *sharedRegistry {
	return &sharedRegistry{tarballs: map[string]map[string][]byte{}}
}

// add registers a package tarball (files keyed by package-relative path).
func (r *sharedRegistry) add(name, version string, files map[string]string) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for rel, content := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: "package/" + rel, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg,
		}); err != nil {
			panic(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			panic(err)
		}
	}
	if err := tw.Close(); err != nil {
		panic(err)
	}
	if err := gz.Close(); err != nil {
		panic(err)
	}
	if r.tarballs[name] == nil {
		r.tarballs[name] = map[string][]byte{}
	}
	r.tarballs[name][version] = buf.Bytes()
}

// seedReact seeds the react family (react, react-dom, scheduler) with real ESM
// entry points so esbuild can bundle the reexport surfaces.
func seedReact(r *sharedRegistry) {
	r.add("react", "18.3.1", map[string]string{
		"package.json": `{"name":"react","version":"18.3.1","main":"./index.js","module":"./index.js"}`,
		"index.js": `export const createElement = () => {};
const React = { createElement, version: "18.3.1" };
export default React;
export const Fragment = Symbol.for("react.fragment");
`,
		"jsx-runtime.js": `export const Fragment = Symbol.for("react.fragment");
export const jsx = () => {};
export const jsxs = () => {};
`,
	})
	r.add("react-dom", "18.3.1", map[string]string{
		"package.json": `{"name":"react-dom","version":"18.3.1","main":"./index.js","module":"./index.js"}`,
		"index.js": `import React from "react";
export const createRoot = () => {};
export const hydrateRoot = () => {};
const ReactDOM = { createRoot, hydrateRoot, version: "18.3.1", versionReexport: React.version };
export default ReactDOM;
`,
		"client.js": `import React from "react";
export const createRoot = () => {};
export const hydrateRoot = () => {};
export { React };
`,
	})
	r.add("scheduler", "0.23.2", map[string]string{
		"package.json": `{"name":"scheduler","version":"0.23.2","main":"./index.js","module":"./index.js"}`,
		"index.js": `export const unstable_now = () => 0;
`,
	})
}

func (r *sharedRegistry) sri(name, version string) string {
	sum := sha512.Sum512(r.tarballs[name][version])
	return "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
}

// start wires the httptest server: packuments at /<name>, tarballs at
// /<name>/-/<name>-<version>.tgz, and returns a registry client pointed at it.
func (r *sharedRegistry) start(t *testing.T) *registry.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.paths = append(r.paths, req.URL.Path)
		r.mu.Unlock()

		path := strings.TrimPrefix(req.URL.Path, "/")
		if strings.Contains(path, "/-/") {
			parts := strings.SplitN(path, "/-/", 2)
			name := parts[0]
			file := strings.TrimSuffix(parts[1], ".tgz")
			version := strings.TrimPrefix(file, name+"-")
			data, ok := r.tarballs[name][version]
			if !ok {
				http.NotFound(w, req)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(data)
			return
		}
		pack, ok := r.packument(path)
		if !ok {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(pack)
	}))
	t.Cleanup(srv.Close)
	r.baseURL = srv.URL
	return registry.New(srv.URL)
}

type packVersion struct {
	Name                 string            `json:"name"`
	Version              string            `json:"version"`
	Dependencies         map[string]string `json:"dependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies,omitempty"`
	PeerDependenciesMeta map[string]struct {
		Optional bool `json:"optional"`
	} `json:"peerDependenciesMeta,omitempty"`
	Dist struct {
		Integrity string `json:"integrity"`
		Tarball   string `json:"tarball"`
	} `json:"dist"`
}

func (r *sharedRegistry) packument(name string) ([]byte, bool) {
	versions := r.tarballs[name]
	if len(versions) == 0 {
		return nil, false
	}
	doc := struct {
		Versions map[string]json.RawMessage `json:"versions"`
	}{Versions: map[string]json.RawMessage{}}
	for version := range versions {
		pv := packVersion{Name: name, Version: version}
		pv.Dist.Integrity = r.sri(name, version)
		pv.Dist.Tarball = r.baseURL + "/" + name + "/-/" + name + "-" + version + ".tgz"
		switch name {
		case "react-dom":
			pv.Dependencies = map[string]string{"scheduler": "^0.23.2"}
			pv.PeerDependencies = map[string]string{"react": "^18.3.1"}
		}
		raw, err := json.Marshal(pv)
		if err != nil {
			return nil, false
		}
		doc.Versions[version] = raw
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, false
	}
	return raw, true
}

func (r *sharedRegistry) requestPaths() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.paths...)
}

// fixtureUIRuntimeSrc returns the absolute path of the bundled ui-runtime test
// fixture (testdata/ui-runtime/src) wired into the generator for the whole
// Artifacts-table coverage path.
func fixtureUIRuntimeSrc(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("testdata", "ui-runtime", "src"))
	if err != nil {
		t.Fatalf("fixtureUIRuntimeSrc: %v", err)
	}
	return abs
}

// newFullGenerator wires a generator whose scope covers the whole Artifacts
// table (react family from the fake registry + ui-runtime from the fixture),
// returning it alongside the fake registry for request assertions.
func newFullGenerator(t *testing.T, tempDir string) (*Generator, *sharedRegistry) {
	t.Helper()
	reg := newSharedRegistry()
	seedReact(reg)
	return NewGenerator(reg.start(t), tempDir).WithUIRuntime(fixtureUIRuntimeSrc(t)), reg
}

func TestGenerateProducesSharedArtifacts(t *testing.T) {
	gen, reg := newFullGenerator(t, t.TempDir())
	bundles, err := gen.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// The whole Artifacts table is produced (slice 8b full-surface).
	for _, rel := range []string{
		"embed/react/18.3.1.js",
		"embed/react-dom/18.3.1.js",
		"embed/react/jsx-runtime/18.3.1.js",
		"embed/@liapoldus/ui-runtime/0.1.0.js",
	} {
		if _, ok := bundles[rel]; !ok {
			t.Errorf("Generate produced no %s", rel)
		}
	}

	react := string(bundles["embed/react/18.3.1.js"])
	if !strings.Contains(react, "createElement") {
		t.Errorf("react bundle missing createElement")
	}
	if strings.Contains(react, `from "react"`) || strings.Contains(react, `from"react"`) {
		t.Errorf("react bundle left react external (expected self-contained)")
	}

	reactDOM := string(bundles["embed/react-dom/18.3.1.js"])
	if !strings.Contains(reactDOM, "createRoot") {
		t.Errorf("react-dom bundle missing createRoot")
	}
	// react-dom keeps react external so the import map serves one React.
	if !strings.Contains(reactDOM, `from "react"`) && !strings.Contains(reactDOM, `from"react"`) {
		t.Errorf("react-dom bundle did not keep react external")
	}

	jsxRuntime := string(bundles["embed/react/jsx-runtime/18.3.1.js"])
	if !strings.Contains(jsxRuntime, "jsx") {
		t.Errorf("jsx-runtime bundle missing jsx")
	}

	// ui-runtime (slice 8b): compiled from ui-runtime/src with the React
	// surface externalized (shared import map), public exports and the
	// automatic-JSX import intact.
	uiRuntime := string(bundles["embed/@liapoldus/ui-runtime/0.1.0.js"])
	if !strings.Contains(uiRuntime, "boot") {
		t.Errorf("ui-runtime bundle missing boot export")
	}
	if strings.Contains(uiRuntime, `from "react"`) || strings.Contains(uiRuntime, `from"react"`) {
		t.Errorf("ui-runtime bundle bundled react instead of keeping it external")
	}
	if !strings.Contains(uiRuntime, "react/jsx-runtime") {
		t.Errorf("ui-runtime bundle did not use the shared react/jsx-runtime")
	}

	// react-dom's dependency scheduler must have been fetched.
	var sawScheduler bool
	for _, p := range reg.requestPaths() {
		if strings.Contains(p, "scheduler") && strings.Contains(p, ".tgz") {
			sawScheduler = true
		}
	}
	if !sawScheduler {
		t.Errorf("registry never fetched scheduler (react-dom dependency)")
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	gen, _ := newFullGenerator(t, t.TempDir())
	ctx := context.Background()
	first, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	second, err := gen.Generate(ctx)
	if err != nil {
		t.Fatalf("second Generate: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("Generate returned different artifact counts: %d vs %d", len(first), len(second))
	}
	for rel, a := range first {
		if b, ok := second[rel]; !ok || !bytes.Equal(a, b) {
			t.Errorf("Generate non-deterministic for %s", rel)
		}
	}
}

func TestGenerateMaterializesNodeModules(t *testing.T) {
	reg := newSharedRegistry()
	seedReact(reg)
	client := reg.start(t)

	tempDir := t.TempDir()
	if _, err := NewGenerator(client, tempDir).Generate(context.Background()); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, pkg := range []string{"react", "react-dom", "scheduler"} {
		index := filepath.Join(tempDir, "ws", "node_modules", pkg, "index.js")
		if _, err := os.Stat(index); err != nil {
			t.Errorf("materialized node_modules/%s/index.js missing: %v", pkg, err)
		}
	}
}

func TestVerifyBundlesCompares(t *testing.T) {
	got := map[string][]byte{
		"embed/react/18.3.1.js":             []byte("react-bytes"),
		"embed/react-dom/18.3.1.js":         []byte("react-dom-bytes"),
		"embed/react/jsx-runtime/18.3.1.js": []byte("jsx-runtime-bytes"),
	}
	wantErr := verifyBundles(got, got)
	if wantErr != nil {
		t.Errorf("verifyBundles(identical) = %v, want nil", wantErr)
	}

	// Committed contains an entry the generator did not produce.
	extra := map[string][]byte{}
	for k, v := range got {
		extra[k] = append([]byte(nil), v...)
	}
	extra["embed/react/17.0.2.js"] = []byte("stale")
	if err := verifyBundles(got, extra); err == nil {
		t.Errorf("verifyBundles(stale committed) = nil, want error")
	}

	tampered := map[string][]byte{}
	for k, v := range got {
		tampered[k] = append([]byte(nil), v...)
	}
	one := "embed/react-dom/18.3.1.js"
	tampered[one] = append(tampered[one], []byte("// tampered")...)
	if err := verifyBundles(got, tampered); err == nil {
		t.Errorf("verifyBundles(different bytes) = nil, want error")
	}
	if err := verifyBundles(got, tampered); !strings.Contains(err.Error(), "react-dom") {
		t.Errorf("verifyBundles(different bytes) error %q does not name react-dom", err)
	}
}

// TestVerifyEnforcesFullArtifactCoverage proves the verify gate covers the whole
// Artifacts table (slice 8b): a generator without the ui-runtime source fails
// exactly because @liapoldus/ui-runtime is outside its scope, never because it
// is skipped.
func TestVerifyEnforcesFullArtifactCoverage(t *testing.T) {
	reg := newSharedRegistry()
	seedReact(reg)

	t.Run("missing ui-runtime source fails the gate", func(t *testing.T) {
		gen := NewGenerator(reg.start(t), t.TempDir())
		err := gen.Verify(context.Background())
		if err == nil {
			t.Fatalf("Verify without ui-runtime source = nil, want coverage error")
		}
		for _, want := range []string{"does not cover", "@liapoldus/ui-runtime"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("Verify error %q does not mention %q", err.Error(), want)
			}
		}
	})

	t.Run("wired ui-runtime reaches bytes comparison", func(t *testing.T) {
		gen := NewGenerator(reg.start(t), t.TempDir()).
			WithUIRuntime(fixtureUIRuntimeSrc(t))
		err := gen.Verify(context.Background())
		if err == nil {
			t.Fatalf("Verify with fixture ui-runtime = nil, want committed-bytes mismatch")
		}
		if strings.Contains(err.Error(), "does not cover") {
			t.Errorf("Verify error %q still reports an uncovered artifact (ui-runtime was in scope)", err.Error())
		}
	})
}
