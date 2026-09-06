package unit

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
)

func testNow() time.Time { return time.Now().UTC() }

// fakeBuilder materializes a workspace on demand or returns a canned error.
type fakeBuilder struct {
	err      error
	onCall   func(build.WorkspaceRequest)
	manifest build.Manifest
}

func (f *fakeBuilder) Materialize(_ context.Context, req build.WorkspaceRequest) (build.Workspace, error) {
	if f.onCall != nil {
		f.onCall(req)
	}
	if f.err != nil {
		return build.Workspace{}, f.err
	}
	return build.Workspace{Dir: req.Dir, Manifest: f.manifest}, nil
}

// fakeRunner emulates esbuild: produces dist inside the workspace or errors.
type fakeRunner struct {
	err error
	dir string
}

func (f *fakeRunner) Build(_ context.Context, ws build.Workspace) (build.BundleResult, error) {
	if f.err != nil {
		return build.BundleResult{}, f.err
	}
	return build.BundleResult{DistDir: filepath.Join(ws.Dir, "dist")}, nil
}

// fakeArtifacts records publish calls and returns a deterministic dir.
type fakeArtifacts struct {
	err      error
	calls    int
	lastSite string
}

func (f *fakeArtifacts) Publish(_ context.Context, siteID, environment, snapshotID string, _ build.Workspace, _ build.BundleResult) (string, error) {
	f.calls++
	f.lastSite = siteID
	if f.err != nil {
		return "", f.err
	}
	return "build/" + siteID + "/" + environment + "/" + snapshotID, nil
}

// buildTestHarness wires memory storage + fakes into a Build service.
func buildTestHarness(t *testing.T, mem *storage.Memory, builder *fakeBuilder, runner *fakeRunner, artifacts *fakeArtifacts) *build.Service {
	t.Helper()
	return build.NewService(mem, mem, mem, builder, runner, artifacts)
}

func seedBuildSite(t *testing.T, mem *storage.Memory) domain.Site {
	t.Helper()
	site := domain.Site{ID: "site_1", Slug: "example", Name: "Example", CreatedAt: testNow()}
	if err := mem.CreateSite(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	return site
}

func seedBuildSnapshot(t *testing.T, mem *storage.Memory, siteID string) domain.Snapshot {
	t.Helper()
	snapshot := domain.Snapshot{
		ID: "snapshot_1", SiteID: siteID, Name: "v1",
		Pages: []domain.SnapshotPage{{PageID: "page_1", VersionID: "pagever_1", Version: 1}},
	}
	if err := mem.CreateSnapshot(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestBuildCreateHappyPath(t *testing.T) {
	ctx := context.Background()
	mem := storage.NewMemory()
	site := seedBuildSite(t, mem)
	snapshot := seedBuildSnapshot(t, mem, site.ID)
	builder := &fakeBuilder{manifest: build.Manifest{SiteID: site.ID, SnapshotID: snapshot.ID}}
	runner := &fakeRunner{}
	artifacts := &fakeArtifacts{}
	service := buildTestHarness(t, mem, builder, runner, artifacts)

	got, err := service.Create(ctx, site.ID, snapshot.ID, domain.EnvironmentDevelopment)
	if err != nil {
		t.Fatalf("create build: %v", err)
	}
	if !strings.HasPrefix(got.ID, "build_") {
		t.Fatalf("build id = %q, want build_*", got.ID)
	}
	if got.Status != domain.BuildStatusReady {
		t.Fatalf("status = %q, want ready", got.Status)
	}
	if got.ArtifactDir != "build/"+site.ID+"/development/"+snapshot.ID {
		t.Fatalf("artifactDir = %q", got.ArtifactDir)
	}
	if got.StartedAt == nil || got.FinishedAt == nil {
		t.Fatalf("startedAt/finishedAt must be set, got %v/%v", got.StartedAt, got.FinishedAt)
	}
	if want := "build ready"; want != got.Log[len(got.Log)-1] {
		t.Fatalf("last log = %q, want %q", got.Log[len(got.Log)-1], want)
	}
	if artifacts.calls != 1 {
		t.Fatalf("publish calls = %d, want 1", artifacts.calls)
	}

	// Persisted with the same state.
	stored, err := mem.GetBuild(ctx, got.ID)
	if err != nil {
		t.Fatalf("get stored build: %v", err)
	}
	if stored.Status != domain.BuildStatusReady || stored.ArtifactDir != got.ArtifactDir {
		t.Fatalf("stored build = %#v", stored)
	}
}

func TestBuildCreateIsNoOpForExistingReady(t *testing.T) {
	ctx := context.Background()
	mem := storage.NewMemory()
	site := seedBuildSite(t, mem)
	snapshot := seedBuildSnapshot(t, mem, site.ID)
	builder := &fakeBuilder{manifest: build.Manifest{SiteID: site.ID}}
	service := buildTestHarness(t, mem, builder, &fakeRunner{}, &fakeArtifacts{})

	first, err := service.Create(ctx, site.ID, snapshot.ID, domain.EnvironmentDevelopment)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	builder.onCall = func(req build.WorkspaceRequest) {
		t.Fatalf("materializer must not run on no-op republish")
	}
	second, err := service.Create(ctx, site.ID, snapshot.ID, domain.EnvironmentDevelopment)
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("no-op republish returned new build %q, want same %q", second.ID, first.ID)
	}
	if second.ArtifactDir != first.ArtifactDir {
		t.Fatalf("artifactDir changed on republish: %q != %q", second.ArtifactDir, first.ArtifactDir)
	}
	if len(memBuilds(mem)) != 1 {
		t.Fatalf("build count = %d, want 1 (no-op)", len(memBuilds(mem)))
	}
}

func TestBuildCreateFailedIsSucceededByRetry(t *testing.T) {
	ctx := context.Background()
	mem := storage.NewMemory()
	site := seedBuildSite(t, mem)
	snapshot := seedBuildSnapshot(t, mem, site.ID)
	builder := &fakeBuilder{err: errors.New("boom")}
	service := buildTestHarness(t, mem, builder, &fakeRunner{}, &fakeArtifacts{})

	failed, err := service.Create(ctx, site.ID, snapshot.ID, domain.EnvironmentProduction)
	if err == nil || !errors.Is(err, domain.ErrBuildFailed) {
		t.Fatalf("error = %v, want ErrBuildFailed", err)
	}
	if failed.Status != domain.BuildStatusFailed {
		t.Fatalf("status = %q, want failed", failed.Status)
	}
	if len(failed.Log) == 0 || !strings.Contains(failed.Log[len(failed.Log)-1], "failed") {
		t.Fatalf("failed build must carry a log entry, got %#v", failed.Log)
	}
	if failed.ArtifactDir != "" {
		t.Fatalf("failed build artifactDir = %q, want empty", failed.ArtifactDir)
	}

	// A retry creates a fresh attempt (not a no-op) and succeeds.
	builder.err = nil
	ok, err := service.Create(ctx, site.ID, snapshot.ID, domain.EnvironmentProduction)
	if err != nil {
		t.Fatalf("retry create: %v", err)
	}
	if ok.ID == failed.ID {
		t.Fatalf("retry reused failed build id %q", failed.ID)
	}
	if ok.Status != domain.BuildStatusReady {
		t.Fatalf("retry status = %q, want ready", ok.Status)
	}
}

func TestBuildCreateValidation(t *testing.T) {
	ctx := context.Background()
	mem := storage.NewMemory()
	site := seedBuildSite(t, mem)
	snapshot := seedBuildSnapshot(t, mem, site.ID)
	service := buildTestHarness(t, mem, &fakeBuilder{}, &fakeRunner{}, &fakeArtifacts{})

	if _, err := service.Create(ctx, site.ID, snapshot.ID, "staging"); !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("bad environment error = %v, want ErrInvalidRequest", err)
	}
	if _, err := service.Create(ctx, "site_nope", snapshot.ID, domain.EnvironmentDevelopment); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing site error = %v, want ErrNotFound", err)
	}
	if _, err := service.Create(ctx, site.ID, "snapshot_nope", domain.EnvironmentDevelopment); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing snapshot error = %v, want ErrNotFound", err)
	}

	// Snapshot from another site is not accessible through this site.
	other := seedBuildSiteAt(t, mem, "site_2")
	foreign := domain.Snapshot{ID: "snapshot_2", SiteID: other.ID}
	if err := mem.CreateSnapshot(ctx, foreign); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(ctx, site.ID, foreign.ID, domain.EnvironmentDevelopment); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign snapshot error = %v, want ErrNotFound", err)
	}
}

func TestBuildRunnerFailureFlipsStatus(t *testing.T) {
	ctx := context.Background()
	mem := storage.NewMemory()
	site := seedBuildSite(t, mem)
	snapshot := seedBuildSnapshot(t, mem, site.ID)
	service := buildTestHarness(t, mem, &fakeBuilder{manifest: build.Manifest{SiteID: site.ID}},
		&fakeRunner{err: errors.New("ts tsx syntax error")}, &fakeArtifacts{})

	got, err := service.Create(ctx, site.ID, snapshot.ID, domain.EnvironmentDevelopment)
	if err == nil || !errors.Is(err, domain.ErrBuildFailed) {
		t.Fatalf("error = %v, want ErrBuildFailed", err)
	}
	if got.Status != domain.BuildStatusFailed {
		t.Fatalf("status = %q, want failed", got.Status)
	}
	stored, err := mem.GetBuild(ctx, got.ID)
	if err != nil {
		t.Fatalf("get stored: %v", err)
	}
	if stored.Status != domain.BuildStatusFailed {
		t.Fatalf("persisted status = %q, want failed", stored.Status)
	}
}

func TestBuildPublishFailureFlipsStatus(t *testing.T) {
	ctx := context.Background()
	mem := storage.NewMemory()
	site := seedBuildSite(t, mem)
	snapshot := seedBuildSnapshot(t, mem, site.ID)
	service := buildTestHarness(t, mem, &fakeBuilder{manifest: build.Manifest{SiteID: site.ID}},
		&fakeRunner{}, &fakeArtifacts{err: errors.New("disk full")})

	got, err := service.Create(ctx, site.ID, snapshot.ID, domain.EnvironmentDevelopment)
	if err == nil || !errors.Is(err, domain.ErrBuildFailed) {
		t.Fatalf("error = %v, want ErrBuildFailed", err)
	}
	if got.Status != domain.BuildStatusFailed {
		t.Fatalf("status = %q, want failed", got.Status)
	}
}

func TestBuildGetAndList(t *testing.T) {
	ctx := context.Background()
	mem := storage.NewMemory()
	site := seedBuildSite(t, mem)
	snapshot := seedBuildSnapshot(t, mem, site.ID)
	service := buildTestHarness(t, mem, &fakeBuilder{manifest: build.Manifest{SiteID: site.ID}}, &fakeRunner{}, &fakeArtifacts{})

	created, err := service.Create(ctx, site.ID, snapshot.ID, domain.EnvironmentDevelopment)
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != created.ID {
		t.Fatalf("get = %q, want %q", got.ID, created.ID)
	}
	list, err := service.ListBySite(ctx, site.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("list = %#v", list)
	}
	if _, err := service.Get(ctx, "build_nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing get error = %v, want ErrNotFound", err)
	}
}

func memBuilds(mem *storage.Memory) []domain.Build {
	list, _ := mem.ListBuildsBySite(context.Background(), "site_1")
	return list
}

func seedBuildSiteAt(t *testing.T, mem *storage.Memory, id string) domain.Site {
	t.Helper()
	site := domain.Site{ID: id, Slug: "slug-" + id, Name: "Site " + id, CreatedAt: testNow()}
	if err := mem.CreateSite(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	return site
}
