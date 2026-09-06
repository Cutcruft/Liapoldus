package integrationtest

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/db"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
)

func osGetenv(key string) string { return os.Getenv(key) }

func isNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }

// seedStoreSite creates a site through the given repository and returns it.
// It registers a cleanup that deletes the site (cascade removes child rows) so
// the shared test database stays repeatable.
func seedStoreSite(t *testing.T, repo domain.SiteRepository) domain.Site {
	t.Helper()
	site := domain.Site{
		ID:   "site_build_" + t.Name(),
		Slug: "build-" + t.Name(), Name: "Build site",
		DefaultLocale: "ru", CreatedAt: time.Now().UTC(),
	}
	if err := repo.CreateSite(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.DeleteSite(context.Background(), site.ID) })
	return site
}

// openPostgresStore opens the database and applies migrations, then returns the
// adapter + an isolated memory store for reference. Skips without
// TEST_DATABASE_URL so the suite still passes locally without a database.
func openPostgresStore(t *testing.T) (*db.Postgres, *storage.Memory) {
	t.Helper()
	databaseURL := osGetenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping postgres integration test")
	}
	ctx := context.Background()
	store, err := db.NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return store, storage.NewMemory()
}

func TestPostgresBuildStoreRoundtrip(t *testing.T) {
	ctx := context.Background()
	pg, mem := openPostgresStore(t)
	now := time.Now().UTC()

	site := seedStoreSite(t, pg)
	snapshot := domain.Snapshot{
		ID: "snapshot_build_1", SiteID: site.ID, Name: "v1",
		Pages: []domain.SnapshotPage{{PageID: "page_1", VersionID: "pagever_1", Version: 1}},
	}
	if err := pg.CreateSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}

	memBuild := domain.Build{
		ID: "build_1", SiteID: site.ID, SnapshotID: snapshot.ID, Environment: domain.EnvironmentDevelopment,
		Status: domain.BuildStatusQueued, Log: []string{"build queued"}, CreatedAt: now,
	}
	if err := pg.CreateBuild(ctx, memBuild); err != nil {
		t.Fatalf("create build: %v", err)
	}
	got, err := pg.GetBuild(ctx, memBuild.ID)
	if err != nil {
		t.Fatalf("get build: %v", err)
	}
	if got.Status != domain.BuildStatusQueued || len(got.Log) != 1 || got.SiteID != site.ID {
		t.Fatalf("build = %#v", got)
	}
	if _, err := mem.GetBuild(ctx, memBuild.ID); !isNotFound(err) {
		t.Fatalf("memory store must be independent (error = %v)", err)
	}

	started := now
	got.Status = domain.BuildStatusBuilding
	got.StartedAt = &started
	got.Log = append(got.Log, "materializing workspace")
	if err := pg.UpdateBuild(ctx, got); err != nil {
		t.Fatalf("update build: %v", err)
	}
	updated, err := pg.GetBuild(ctx, got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != domain.BuildStatusBuilding || len(updated.Log) != 2 || updated.StartedAt == nil {
		t.Fatalf("updated build = %#v", updated)
	}

	list, err := pg.ListBuildsBySite(ctx, site.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != memBuild.ID {
		t.Fatalf("list = %#v", list)
	}
	bySnapshot, err := pg.GetBuildBySnapshot(ctx, site.ID, domain.EnvironmentDevelopment, snapshot.ID)
	if err != nil {
		t.Fatalf("get by snapshot: %v", err)
	}
	if bySnapshot.ID != memBuild.ID {
		t.Fatalf("by snapshot = %#v", bySnapshot)
	}
	if _, err := pg.GetBuildBySnapshot(ctx, site.ID, domain.EnvironmentProduction, snapshot.ID); !isNotFound(err) {
		t.Fatalf("by snapshot other env error = %v, want NotFound", err)
	}
}

func TestPostgresBuildNotFoundAndStatusUnique(t *testing.T) {
	ctx := context.Background()
	pg, _ := openPostgresStore(t)
	site := seedStoreSite(t, pg)

	if _, err := pg.GetBuild(ctx, "build_missing"); !isNotFound(err) {
		t.Fatalf("missing build error = %v, want NotFound", err)
	}

	snapshot := domain.Snapshot{
		ID: "snapshot_build_status", SiteID: site.ID,
		Pages: []domain.SnapshotPage{{PageID: "page_1", VersionID: "pagever_1", Version: 1}},
	}
	if err := pg.CreateSnapshot(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for i := 0; i < 2; i++ {
		b := domain.Build{
			ID: "build_status_dup", SiteID: site.ID, SnapshotID: snapshot.ID,
			Environment: domain.EnvironmentDevelopment, Status: domain.BuildStatusReady, Log: nil, CreatedAt: now,
		}
		if err := pg.CreateBuild(ctx, b); err != nil {
			t.Fatalf("create build: %v", err)
		}
	}
	if err := pg.UpdateBuild(ctx, domain.Build{ID: "build_missing"}); !isNotFound(err) {
		t.Fatalf("update missing build error = %v, want NotFound", err)
	}
}
