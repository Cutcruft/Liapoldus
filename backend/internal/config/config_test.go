package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("LIAPOLDUS_ADDR", "")
	t.Setenv("LIAPOLDUS_ADMIN_ADDR", "")
	t.Setenv("LIAPOLDUS_CLIENT_ADDR", "")
	t.Setenv("LIAPOLDUS_ADMIN_TOKEN", "")
	t.Setenv("LIAPOLDUS_ASSET_DIR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AdminAddr != ":8080" {
		t.Fatalf("admin address = %q, want :8080", cfg.AdminAddr)
	}
	if cfg.ClientAddr != ":18080" {
		t.Fatalf("client address = %q, want :18080", cfg.ClientAddr)
	}
	if cfg.Storage != StoragePostgres {
		t.Fatalf("storage = %q, want %q", cfg.Storage, StoragePostgres)
	}
	if cfg.AssetDir != "./data/assets" {
		t.Fatalf("asset dir = %q, want ./data/assets", cfg.AssetDir)
	}
}

func TestLoadCustomConfiguration(t *testing.T) {
	t.Setenv("LIAPOLDUS_ADDR", "")
	t.Setenv("LIAPOLDUS_ADMIN_ADDR", ":9090")
	t.Setenv("LIAPOLDUS_CLIENT_ADDR", ":19090")
	t.Setenv("LIAPOLDUS_ADMIN_TOKEN", "secret")
	t.Setenv("LIAPOLDUS_ASSET_DIR", "/tmp/assets")
	t.Setenv("LIAPOLDUS_CLIENT_DEFAULT_SLUG", "demo")
	t.Setenv("LIAPOLDUS_STORAGE", "memory")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AdminAddr != ":9090" {
		t.Fatalf("admin address = %q, want :9090", cfg.AdminAddr)
	}
	if cfg.ClientAddr != ":19090" {
		t.Fatalf("client address = %q, want :19090", cfg.ClientAddr)
	}
	if cfg.AdminToken != "secret" {
		t.Fatalf("admin token = %q, want secret", cfg.AdminToken)
	}
	if cfg.AssetDir != "/tmp/assets" {
		t.Fatalf("asset dir = %q, want /tmp/assets", cfg.AssetDir)
	}
	if cfg.ClientDefaultSlug != "demo" {
		t.Fatalf("client default slug = %q, want demo", cfg.ClientDefaultSlug)
	}
	if cfg.Storage != StorageMemory {
		t.Fatalf("storage = %q, want %q", cfg.Storage, StorageMemory)
	}
}

func TestLoadRejectsUnknownStorage(t *testing.T) {
	t.Setenv("LIAPOLDUS_STORAGE", "mongo")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}
