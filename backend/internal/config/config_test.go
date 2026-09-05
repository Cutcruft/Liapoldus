package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("LIAPOLDUS_ADDR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8080" {
		t.Fatalf("address = %q, want :8080", cfg.Addr)
	}
	if cfg.Storage != StoragePostgres {
		t.Fatalf("storage = %q, want %q", cfg.Storage, StoragePostgres)
	}
}

func TestLoadCustomConfiguration(t *testing.T) {
	t.Setenv("LIAPOLDUS_ADDR", ":9090")
	t.Setenv("LIAPOLDUS_STORAGE", "memory")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":9090" {
		t.Fatalf("address = %q, want :9090", cfg.Addr)
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
