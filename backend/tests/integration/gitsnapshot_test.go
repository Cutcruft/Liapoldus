package integrationtest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/application/gitsnapshot"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	gitrepo "github.com/liapoldus/liapoldus/backend/internal/infra/git"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
)

// seedSnapshotSite registers a minimal site with one component definition, one
// page and one content record so the serialization round-trip has substance.
func seedSnapshotSite(t *testing.T, db *storage.Memory, siteID string) {
	t.Helper()
	ctx := context.Background()
	if err := db.CreateSite(ctx, domain.Site{ID: siteID, Name: "Demo", Slug: "demo", DefaultLocale: "ru", Hosts: []string{}, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("create site: %v", err)
	}
	if err := db.Save(ctx, &domain.ComponentDefinition{
		SiteID: siteID, ID: "card", Name: "Card", Kind: "component",
		Schema:    map[string]any{"type": "object"},
		Metadata:  map[string]any{"label": "Card"},
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("save def: %v", err)
	}
	now := time.Now().UTC()
	root := domain.ComponentNode{InstanceID: "root", DefinitionID: "card", Props: map[string]any{"title": "Hello"}}
	if err := db.CreatePage(ctx, domain.Page{
		ID: "p_home", SiteID: siteID, Name: "Home", Slug: "home", Root: root, Version: 1, CreatedAt: now, UpdatedAt: now,
	}, domain.PageVersion{ID: "pv_1", PageID: "p_home", Number: 1, Root: root, CreatedAt: now}); err != nil {
		t.Fatalf("create page: %v", err)
	}
	if err := db.CreateContent(ctx, domain.Content{
		ID: "c_1", SiteID: siteID, CollectionID: "articles", Fields: map[string]any{"title": "Hello"}, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create content: %v", err)
	}
}

func newSnapshotService(t *testing.T, db *storage.Memory) (*gitsnapshot.Service, *gitrepo.Repo) {
	t.Helper()
	repo := gitrepo.NewRepo(t.TempDir())
	return gitsnapshot.NewService(repo, db, db, db, db, db, db, db, nil), repo
}

func snapshotList(t *testing.T, db *storage.Memory, siteID string) []domain.Snapshot {
	t.Helper()
	snapshots, err := db.ListSnapshotsBySite(context.Background(), siteID)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	return snapshots
}

func TestSnapshotCommitPublishRestoreLifecycle(t *testing.T) {
	ctx := context.Background()
	db := storage.NewMemory()
	svc, _ := newSnapshotService(t, db)
	siteID := "snap-site"
	seedSnapshotSite(t, db, siteID)

	// No git state yet.
	status, err := svc.Status(ctx, siteID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Dev.Existing || status.Main.Existing {
		t.Fatalf("fresh site must have no branches, got %+v", status)
	}

	// Commit the initial state.
	firstSHA, err := svc.Commit(ctx, siteID, "initial state")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	status, err = svc.Status(ctx, siteID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Dev.Existing || status.Dirty {
		t.Fatalf("after commit want clean dev, got %+v", status)
	}

	// Publish: rebase + ff merge + snapshot record.
	mainSHA, err := svc.Publish(ctx, siteID, "publish v1")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if mainSHA != firstSHA {
		t.Fatalf("publish sha = %s, want dev sha %s (fast-forward)", mainSHA, firstSHA)
	}
	status, err = svc.Status(ctx, siteID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Main.Existing || status.Main.SHA != firstSHA {
		t.Fatalf("main must be at published sha, got %+v", status)
	}
	snaps := snapshotList(t, db, siteID)
	if len(snaps) != 1 || snaps[0].GitSHA != mainSHA || snaps[0].Name != "publish v1" {
		t.Fatalf("expected one published snapshot, got %+v", snaps)
	}
	if len(snaps[0].Pages) != 1 || snaps[0].Pages[0].PageID != "p_home" || snaps[0].Pages[0].VersionID == "" {
		t.Fatalf("snapshot must reference the page with version, got %+v", snaps[0].Pages)
	}

	// Mutate state (change content), publish again.
	content, err := db.GetContent(ctx, "c_1")
	if err != nil {
		t.Fatal(err)
	}
	content.Fields = map[string]any{"title": "Changed"}
	if err := db.UpdateContent(ctx, content); err != nil {
		t.Fatalf("update content: %v", err)
	}
	status, err = svc.Status(ctx, siteID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Dirty {
		t.Fatal("mutated db must be dirty vs dev HEAD")
	}
	secondSHA, err := svc.Commit(ctx, siteID, "update heading")
	if err != nil {
		t.Fatalf("commit 2: %v", err)
	}
	if _, err := svc.Publish(ctx, siteID, "publish v2"); err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	if secondSHA == firstSHA {
		t.Fatal("second commit must differ from first")
	}
	if len(snapshotList(t, db, siteID)) != 2 {
		t.Fatal("two publishes must produce two snapshots")
	}

	// Restore the first commit: DB state reverts, a safety commit keeps dev
	// history, and a new snapshot entry records the restore.
	restored, err := svc.Restore(ctx, siteID, firstSHA, "restore initial")
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored != firstSHA {
		t.Fatalf("restore returned %s, want %s", restored, firstSHA)
	}
	restoredContent, err := db.GetContent(ctx, "c_1")
	if err != nil {
		t.Fatal(err)
	}
	if restoredContent.Fields["title"] != "Hello" {
		t.Fatalf("content after restore = %v, want initial title", restoredContent.Fields)
	}
	if len(snapshotList(t, db, siteID)) != 3 {
		t.Fatal("restore must record a snapshot (safety + restore entry)")
	}

	// Rollback to the first commit: rewinds main and re-applies its tree.
	rollbackSHA, err := svc.Rollback(ctx, siteID, firstSHA, "rollback v1")
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	status, err = svc.Status(ctx, siteID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Main.SHA != rollbackSHA {
		t.Fatalf("main after rollback = %s, want %s", status.Main.SHA, rollbackSHA)
	}
	afterRollback, err := db.GetContent(ctx, "c_1")
	if err != nil {
		t.Fatal(err)
	}
	if afterRollback.Fields["title"] != "Hello" {
		t.Fatalf("content after rollback = %v, want initial title", afterRollback.Fields)
	}
	if len(snapshotList(t, db, siteID)) != 4 {
		t.Fatal("rollback must record a snapshot entry")
	}
}

func TestSnapshotErrors(t *testing.T) {
	ctx := context.Background()
	db := storage.NewMemory()
	svc, _ := newSnapshotService(t, db)

	if _, err := svc.Status(ctx, "ghost"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("status of ghost site: want ErrNotFound, got %v", err)
	}
	if _, err := svc.Publish(ctx, "ghost", "x"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("publish of ghost site: want ErrNotFound, got %v", err)
	}
}
