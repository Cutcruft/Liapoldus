package integrationtest

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/application"
	"github.com/liapoldus/liapoldus/backend/internal/config"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/db"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
)

// setupPostgres opens the database from TEST_DATABASE_URL, applies migrations
// and returns a fresh Services wired to it. Tests skip when the variable is
// not set so the suite still passes locally without a database.
func setupPostgres(t *testing.T) *application.Services {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
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

	blobDir := t.TempDir()
	blobs, err := storage.NewDiskBlobStore(blobDir)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		DefaultLocale:           "ru",
		RedirectDefaultStatus:   301,
		RedirectAllowedStatuses: []int{301, 302},
		ComponentMaxDepth:       5,
		ComponentTypes:          []string{"Container", "Text"},
		PageInitialVersion:      1,
		EmailPattern:            nil,
		MasterVariantName:       "master",
		AssetFallbackName:       "asset",
		AssetFallbackMime:       "application/octet-stream",
		AssetFileURLTemplate:    "/api/assets/{id}/file",
		AssetCacheMaxAgeSeconds: 31536000,
		MaxUploadBytes:          10485760,
	}
	return application.New(store, blobs, cfg)
}

func TestSitePageContentAssetRoundtrip(t *testing.T) {
	ctx := context.Background()
	services := setupPostgres(t)

	site, err := services.Sites.Create(ctx, "Example", "example",
		"ru", []string{"example.test"})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	root := domain.ComponentNode{
		ID:       "root",
		Type:     "Container",
		Props:    map[string]any{"title": "Home"},
		Children: []domain.ComponentNode{{ID: "kids", Type: "Text"}},
	}
	page, err := services.Pages.Create(ctx, site.ID, "Home", "index", root)
	if err != nil {
		t.Fatalf("create page: %v", err)
	}
	if page.Version != 1 {
		t.Fatalf("page version = %d, want 1", page.Version)
	}

	got, err := services.Pages.Get(ctx, page.ID)
	if err != nil {
		t.Fatalf("get page: %v", err)
	}
	if got.SiteID != site.ID || got.Slug != "index" || len(got.Root.Children) != 1 {
		t.Fatalf("page = %#v", got)
	}

	updatedRoot := domain.ComponentNode{
		ID:    "root",
		Type:  "Container",
		Props: map[string]any{"title": "About"},
	}
	updated, err := services.Pages.UpdateTree(ctx, page.ID, updatedRoot)
	if err != nil {
		t.Fatalf("update page: %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("page version = %d, want 2", updated.Version)
	}

	versions, err := services.Pages.Versions(ctx, page.ID)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("versions = %d, want 2", len(versions))
	}

	content, err := services.Contents.Create(ctx, site.ID, "posts", "",
		map[string]any{"title": "Hello"})
	if err != nil {
		t.Fatalf("create content: %v", err)
	}
	if _, err := services.Contents.SetTranslation(ctx, site.ID, content.ID, "en",
		map[string]any{"title": "Hi"}); err != nil {
		t.Fatalf("set translation: %v", err)
	}
	merged, err := services.Contents.GetMerged(ctx, site.ID, content.ID, "en")
	if err != nil {
		t.Fatalf("get merged: %v", err)
	}
	if merged.Fields["title"] != "Hi" {
		t.Fatalf("merged title = %#v, want Hi", merged.Fields["title"])
	}

	asset, err := services.Assets.Create(ctx, site.ID, "logo.png",
		"image/png", strings.NewReader("png-data"))
	if err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if !services.Assets.IsMaster("master") {
		t.Fatalf("master variant not recognized")
	}
	readBack, reader, err := services.Assets.Open(ctx, asset.ID)
	if err != nil {
		t.Fatalf("open asset: %v", err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	if string(data) != "png-data" {
		t.Fatalf("asset data = %q, want png-data", string(data))
	}
	metadata := services.Assets.Metadata(readBack)
	if len(metadata.Variants) != 1 || !strings.HasSuffix(metadata.Variants[0].URL, asset.ID+"/file") {
		t.Fatalf("asset metadata = %#v", metadata)
	}

	if err := services.Pages.Delete(ctx, page.ID); err != nil {
		t.Fatalf("delete page: %v", err)
	}
	if _, err := services.Sites.Get(ctx, site.ID); err != nil {
		t.Fatalf("site must survive page deletion: %v", err)
	}
}

func TestPostgresNotFoundAndUniqueViolations(t *testing.T) {
	ctx := context.Background()
	services := setupPostgres(t)

	if _, err := services.Sites.Get(ctx, "site_nonexistent"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("get missing site error = %v, want ErrNotFound", err)
	}

	site, err := services.Sites.Create(ctx, "Dup", "dup", "ru", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := services.Sites.Create(ctx, "Dup2", "dup", "ru", nil); !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("duplicate slug error = %v, want ErrAlreadyExists", err)
	}
	if err := services.Sites.Delete(ctx, site.ID); err != nil {
		t.Fatal(err)
	}

	update, err := services.Sites.Get(ctx, site.ID)
	if err == nil {
		t.Fatalf("deleted site still present: %#v", update)
	}
}
