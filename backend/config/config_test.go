package config

import "testing"

func TestLoadDefaultsToREST(t *testing.T) {
	t.Setenv("LIAPOLDUS_MANAGEMENT_TRANSPORT", "")
	t.Setenv("LIAPOLDUS_ADDR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ManagementTransport != TransportREST {
		t.Fatalf("transport = %q, want %q", cfg.ManagementTransport, TransportREST)
	}
	if cfg.Addr != ":8080" {
		t.Fatalf("address = %q, want :8080", cfg.Addr)
	}
	if cfg.Storage != StoragePostgres {
		t.Fatalf("storage = %q, want %q", cfg.Storage, StoragePostgres)
	}
}

func TestLoadGRPCConfiguration(t *testing.T) {
	t.Setenv("LIAPOLDUS_MANAGEMENT_TRANSPORT", "gRPC")
	t.Setenv("LIAPOLDUS_ADDR", ":9090")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ManagementTransport != TransportGRPC {
		t.Fatalf("transport = %q, want %q", cfg.ManagementTransport, TransportGRPC)
	}
	if cfg.Addr != ":9090" {
		t.Fatalf("address = %q, want :9090", cfg.Addr)
	}
	if cfg.Storage != StoragePostgres {
		t.Fatalf("storage = %q, want %q", cfg.Storage, StoragePostgres)
	}
}

func TestLoadRejectsUnknownTransport(t *testing.T) {
	t.Setenv("LIAPOLDUS_MANAGEMENT_TRANSPORT", "websocket")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}
