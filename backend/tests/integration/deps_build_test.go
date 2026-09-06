package integrationtest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/artifactstore"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/builder"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/materializer"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/shared"
	"github.com/liapoldus/liapoldus/backend/internal/infra/deps/layout"
	"github.com/liapoldus/liapoldus/backend/internal/infra/deps/registry"
	"github.com/liapoldus/liapoldus/backend/internal/infra/deps/store"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
)

// fakeRegistry serves npm-format tarballs for the e2e dependency build. The
// integrity values are real sha512 of the served bytes, so the layout's fetch
// verification runs against genuine registry semantics.
type fakeRegistry struct {
	tarballs map[string]map[string][]byte
	baseURL  string
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{tarballs: map[string]map[string][]byte{}}
}

func (r *fakeRegistry) add(t *testing.T, name, version string, files map[string]string) (data []byte, sri string) {
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
	if r.tarballs[name] == nil {
		r.tarballs[name] = map[string][]byte{}
	}
	r.tarballs[name][version] = buf.Bytes()
	sum := sha512.Sum512(buf.Bytes())
	return buf.Bytes(), "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
}

func (r *fakeRegistry) URLFor(name, version string) string {
	return "/" + name + "/-/" + name + "-" + version + ".tgz"
}

// start serves the tarballs and returns the client pointed at it.
func (r *fakeRegistry) start(t *testing.T) *registry.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// /<name>/-/<name>-<version>.tgz
		trimmed := strings.TrimPrefix(req.URL.Path, "/")
		parts := strings.SplitN(trimmed, "/-/", 2)
		if len(parts) != 2 {
			http.Error(w, "not a tarball path", http.StatusNotFound)
			return
		}
		name := parts[0]
		file := strings.TrimSuffix(parts[1], ".tgz")
		version := strings.TrimPrefix(file, name+"-")
		versions, ok := r.tarballs[name]
		if !ok {
			http.Error(w, "unknown package", http.StatusNotFound)
			return
		}
		data, ok := versions[version]
		if !ok {
			http.Error(w, "unknown version", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	r.baseURL = srv.URL
	return registry.New(srv.URL)
}

// depsE2EFixture seeds a site whose snapshot has a frozen dependency lock:
// agent (top-level) pulls in helper (transitive) and keeps react external. It
// returns the in-memory domain, the layout, the artifacts store and the
// snapshot id.
type depsE2EFixture struct {
	mem        *storage.Memory
	siteID     string
	snapshotID string
	layout     *layout.Layout
	artifacts  *artifactstore.Store
}

func seedDepsBuildFixture(t *testing.T, reg *fakeRegistry, regClient *registry.Client) depsE2EFixture {
	t.Helper()
	ctx := context.Background()
	mem := storage.NewMemory()
	site := seedBuildSiteInMemory(t, mem, "site_dep_e2e")

	_, helperSRI := reg.add(t, "helper", "1.0.0", map[string]string{
		"package.json": `{"name":"helper","version":"1.0.0","main":"index.js"}`,
		"index.js":     "export function doubling(x) { const marker = \"helper-squared\"; return x * 2 + marker.length * 0; }\n",
	})
	_, agentSRI := reg.add(t, "agent", "1.0.0", map[string]string{
		"package.json": `{"name":"agent","version":"1.0.0","main":"index.js","dependencies":{"helper":"^1.0.0"}}`,
		"index.js": `import { doubling } from "helper";
import { createElement } from "react";
export const agentName = "agent-live";
export default function Agent(props) { return createElement("span", null, agentName + ":" + doubling(props.n)); }
`,
	})

	seedDefinitionSource(t, mem, site.ID, "text",
		"import { agentName } from \"agent\";\nexport default (props) => props?.title ?? agentName;\n")
	seedDefinitionSource(t, mem, site.ID, "container",
		"import React from \"react\";\nexport default (props) => React.createElement(\"div\", null, props?.children ?? null);\n")

	root := domain.ComponentNode{
		InstanceID: "root", DefinitionID: "container",
		Children: []domain.ComponentNode{{InstanceID: "kids", DefinitionID: "text"}},
	}
	page := domain.Page{ID: "page_dep_e2e", SiteID: site.ID, Name: "Home", Slug: "index", Root: root, Version: 1,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	version := domain.PageVersion{ID: "pagever_dep_e2e", PageID: "page_dep_e2e", Number: 1, Root: root, CreatedAt: time.Now().UTC()}
	if err := mem.CreatePage(ctx, page, version); err != nil {
		t.Fatal(err)
	}

	lock := domain.SnapshotLock{Deps: []domain.LockedDep{
		{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: agentSRI},
		{Name: "helper", Spec: "^1.0.0", Version: "1.0.0", Integrity: helperSRI},
	}}
	snapshot := domain.Snapshot{ID: "snapshot_dep_e2e", SiteID: site.ID,
		Pages:    []domain.SnapshotPage{{PageID: "page_dep_e2e", VersionID: "pagever_dep_e2e", Version: 1}},
		DepsLock: lock, CreatedAt: time.Now().UTC()}
	if err := mem.CreateSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}

	if err := mem.CreateDependency(ctx, domain.Dependency{
		SiteID: site.ID, Name: "agent", Spec: "^1.0.0",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	for _, locked := range lock.Deps {
		if err := mem.CreateDepPackage(ctx, domain.DepPackage{
			Name: locked.Name, Version: locked.Version, Integrity: locked.Integrity,
			TarballURL: reg.baseURL + reg.URLFor(locked.Name, locked.Version),
			FetchedAt:  time.Now().UTC(),
		}); err != nil {
			t.Fatal(err)
		}
	}

	depsLayout := layout.New(layout.LayoutOptions{
		Packages: mem,
		Store:    store.New(t.TempDir()),
		Fetch:    tarballFetcherAdapter{client: regClient},
	})
	return depsE2EFixture{
		mem: mem, siteID: site.ID, snapshotID: snapshot.ID,
		layout: depsLayout, artifacts: artifactstore.New(t.TempDir()),
	}
}

// tarballFetcherAdapter bridges the real registry client (which verifies the
// tarball sha512) onto the layout component.
type tarballFetcherAdapter struct{ client *registry.Client }

func (t tarballFetcherAdapter) Tarball(ctx context.Context, name, version, tarballURL, integrity string) ([]byte, error) {
	return t.client.Fetch(ctx, registry.ResolvedVersion{
		Name: name, Version: version, TarballURL: tarballURL, Integrity: integrity,
	})
}

func TestDependencyBuildFullCycle(t *testing.T) {
	ctx := context.Background()
	reg := newFakeRegistry()
	regClient := reg.start(t)
	fx := seedDepsBuildFixture(t, reg, regClient)

	builds := build.NewService(fx.mem, fx.mem, fx.mem,
		materializer.New(fx.mem, fx.mem, fx.mem, fx.mem, shared.NewResolver(), fx.layout),
		builder.New(), fx.artifacts)

	result, err := builds.Create(ctx, fx.siteID, fx.snapshotID, domain.EnvironmentDevelopment)
	if err != nil {
		t.Fatalf("build.Create: %v", err)
	}
	if result.Status != domain.BuildStatusReady {
		t.Fatalf("status = %s, log = %#v", result.Status, result.Log)
	}

	artifactRoot := fx.artifacts.DirFor(fx.siteID, domain.EnvironmentDevelopment, fx.snapshotID)

	// The dependency bundle was built, verified and published under _deps/.
	depBundle, err := os.ReadFile(filepath.Join(artifactRoot, "dist/_deps", "agent@1.0.0.js"))
	if err != nil {
		t.Fatalf("published _deps bundle: %v", err)
	}
	if !strings.Contains(string(depBundle), "helper-squared") {
		t.Fatalf("transitive helper must be inlined into the agent bundle")
	}
	if !strings.Contains(string(depBundle), `from "react"`) {
		t.Fatalf("react must stay external in the agent bundle:\n%s", depBundle)
	}

	// The site bundle keeps the top-level dep external for the import map.
	siteBundle, err := os.ReadFile(filepath.Join(artifactRoot, "dist", "entry.js"))
	if err != nil {
		t.Fatalf("published site bundle: %v", err)
	}
	if !strings.Contains(string(siteBundle), `from"agent"`) {
		t.Fatalf("site bundle must import the dependency externally:\n%s", siteBundle[:min(len(siteBundle), 600)])
	}

	// The manifest is the import-map source: dist/_deps artifact + externals.
	manifestData, err := os.ReadFile(filepath.Join(artifactRoot, "manifest.json"))
	if err != nil {
		t.Fatalf("published manifest: %v", err)
	}
	var manifest build.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("manifest decode: %v", err)
	}
	ref, ok := manifest.Deps["agent"]
	if !ok {
		t.Fatalf("manifest deps = %#v", manifest.Deps)
	}
	if ref.PublicArtifact != "dist/_deps/agent@1.0.0.js" {
		t.Fatalf("dep public artifact = %q", ref.PublicArtifact)
	}
	found := false
	for _, ext := range manifest.Externals {
		if ext == "agent" {
			found = true
		}
	}
	if !found {
		t.Fatalf("manifest externals must include agent: %#v", manifest.Externals)
	}
}

func TestDependencyBuildFailsOnNodeBuiltin(t *testing.T) {
	ctx := context.Background()
	reg := newFakeRegistry()
	regClient := reg.start(t)
	_, agentBadSRI := reg.add(t, "agent", "1.0.0", map[string]string{
		"package.json": `{"name":"agent","version":"1.0.0","main":"index.js"}`,
		"index.js":     "import fs from \"fs\";\nexport default fs.readFileSync;\n",
	})

	// Directory layout without the helper package: the agent package itself
	// imports a node builtin, which must fail the whole build with a hint.
	mem := storage.NewMemory()
	site := seedBuildSiteInMemory(t, mem, "site_dep_fail")
	seedDefinitionSource(t, mem, site.ID, "text",
		"import { agentName } from \"agent\";\nexport default (props) => props?.title ?? agentName;\n")

	root := domain.ComponentNode{InstanceID: "root", DefinitionID: "text"}
	page := domain.Page{ID: "page_dep_fail", SiteID: site.ID, Name: "Home", Root: root, Version: 1,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	version := domain.PageVersion{ID: "pagever_dep_fail", PageID: "page_dep_fail", Number: 1, Root: root, CreatedAt: time.Now().UTC()}
	if err := mem.CreatePage(ctx, page, version); err != nil {
		t.Fatal(err)
	}
	lock := domain.SnapshotLock{Deps: []domain.LockedDep{
		{Name: "agent", Spec: "^1.0.0", Version: "1.0.0", Integrity: agentBadSRI},
	}}
	snapshot := domain.Snapshot{ID: "snapshot_dep_fail", SiteID: site.ID,
		Pages:    []domain.SnapshotPage{{PageID: "page_dep_fail", VersionID: "pagever_dep_fail", Version: 1}},
		DepsLock: lock, CreatedAt: time.Now().UTC()}
	if err := mem.CreateSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := mem.CreateDependency(ctx, domain.Dependency{
		SiteID: site.ID, Name: "agent", Spec: "^1.0.0",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := mem.CreateDepPackage(ctx, domain.DepPackage{
		Name: "agent", Version: "1.0.0", Integrity: agentBadSRI,
		TarballURL: reg.baseURL + reg.URLFor("agent", "1.0.0"),
		FetchedAt:  time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	depsLayout := layout.New(layout.LayoutOptions{
		Packages: mem,
		Store:    store.New(t.TempDir()),
		Fetch:    tarballFetcherAdapter{client: regClient},
	})
	builds := build.NewService(mem, mem, mem,
		materializer.New(mem, mem, mem, mem, shared.NewResolver(), depsLayout),
		builder.New(), artifactstore.New(t.TempDir()))

	_, err := builds.Create(ctx, site.ID, snapshot.ID, domain.EnvironmentDevelopment)
	if !errors.Is(err, domain.ErrBuildFailed) {
		t.Fatalf("error = %v, want ErrBuildFailed", err)
	}
	if !strings.Contains(err.Error(), "fs") {
		t.Fatalf("error must name the node builtin: %v", err)
	}
}
