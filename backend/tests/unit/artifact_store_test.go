package unit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/artifactstore"
)

func seededArtifactStore(t *testing.T) (*artifactstore.Store, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "build")
	return artifactstore.New(root), root
}

func seedWorkspaceDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ws")
	dist := filepath.Join(dir, "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "site.js"), []byte("var booted=1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"siteId":"site_1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestArtifactStorePublishAndRead(t *testing.T) {
	store, root := seededArtifactStore(t)
	wsDir := seedWorkspaceDir(t)
	ws := build.Workspace{Dir: wsDir}

	// Publish → both dist and manifest land under build/<site>/<env>/<snapshot>/.
	if _, err := store.Publish(context.Background(), "site_as", "development", "snap_as", ws, build.BundleResult{}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	bundle, err := store.GetBundle(context.Background(), "site_as", "development", "snap_as", "site.js")
	if err != nil {
		t.Fatalf("get bundle: %v", err)
	}
	if string(bundle) != "var booted=1;" {
		t.Fatalf("bundle = %q", bundle)
	}
	manifest, err := store.GetManifest(context.Background(), "site_as", "development", "snap_as")
	if err != nil {
		t.Fatalf("get manifest: %v", err)
	}
	if string(manifest) != `{"siteId":"site_1"}` {
		t.Fatalf("manifest = %q", manifest)
	}
	if _, err := os.Stat(filepath.Join(root, "site_as", "development", "snap_as", "manifest.json")); err != nil {
		t.Fatalf("published manifest missing: %v", err)
	}
}

func TestArtifactStoreNoOpRepublish(t *testing.T) {
	store, _ := seededArtifactStore(t)
	wsDir := seedWorkspaceDir(t)
	if _, err := store.Publish(context.Background(), "site_as", "development", "snap_re", build.Workspace{Dir: wsDir}, build.BundleResult{}); err != nil {
		t.Fatal(err)
	}
	// Re-publishing the same snapshot id is idempotent.
	if _, err := store.Publish(context.Background(), "site_as", "development", "snap_re", build.Workspace{Dir: wsDir}, build.BundleResult{}); err != nil {
		t.Fatalf("republish: %v", err)
	}
	data, err := store.GetBundle(context.Background(), "site_as", "development", "snap_re", "site.js")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "var booted=1;" {
		t.Fatalf("bundle = %q", data)
	}
}

func TestArtifactStoreErrors(t *testing.T) {
	store, _ := seededArtifactStore(t)
	ctx := context.Background()

	if _, err := store.GetBundle(ctx, "site_as", "development", "snap_missing", "site.js"); err == nil {
		t.Fatal("missing bundle must error")
	}
	if _, err := store.GetManifest(ctx, "site_as", "development", "snap_missing"); err == nil {
		t.Fatal("missing manifest must error")
	}

	// Path traversal is rejected.
	if _, err := store.GetBundle(ctx, "site_as", "development", "snap_as", "../../etc/passwd"); err == nil {
		t.Fatal("traversal must be rejected")
	}

	// Publish from a workspace without dist fails and leaves no partial dir.
	if _, err := store.Publish(ctx, "site_as", "development", "snap_bad", build.Workspace{Dir: t.TempDir()}, build.BundleResult{}); err == nil {
		t.Fatal("publish without dist must fail")
	}
	if _, err := store.GetManifest(ctx, "site_as", "development", "snap_bad"); err == nil {
		t.Fatal("failed publish must not leave artifacts")
	}
}
