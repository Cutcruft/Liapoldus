package config

import (
	"fmt"
	"os"
	"strings"
)

type ManagementTransport string

type StorageDriver string

const (
	TransportREST   ManagementTransport = "rest"
	TransportGRPC   ManagementTransport = "grpc"
	StorageMemory   StorageDriver       = "memory"
	StoragePostgres StorageDriver       = "postgres"
)

type Config struct {
	Addr                string
	ManagementTransport ManagementTransport
	Storage             StorageDriver
	DatabaseURL         string
}

func Load() (Config, error) {
	transport := ManagementTransport(strings.ToLower(strings.TrimSpace(os.Getenv("LIAPOLDUS_MANAGEMENT_TRANSPORT"))))
	if transport == "" {
		transport = TransportREST
	}
	if transport != TransportREST && transport != TransportGRPC {
		return Config{}, fmt.Errorf("unsupported LIAPOLDUS_MANAGEMENT_TRANSPORT %q: use rest or grpc", transport)
	}

	addr := strings.TrimSpace(os.Getenv("LIAPOLDUS_ADDR"))
	if addr == "" {
		addr = ":8080"
	}

	storage := StorageDriver(strings.ToLower(strings.TrimSpace(os.Getenv("LIAPOLDUS_STORAGE"))))
	if storage == "" {
		storage = StoragePostgres
	}
	if storage != StorageMemory && storage != StoragePostgres {
		return Config{}, fmt.Errorf("unsupported LIAPOLDUS_STORAGE %q: use memory or postgres", storage)
	}

	databaseURL := strings.TrimSpace(os.Getenv("LIAPOLDUS_DATABASE_URL"))
	if databaseURL == "" {
		databaseURL = "postgres://liapoldus:liapoldus@localhost:5432/liapoldus?sslmode=disable"
	}
	return Config{Addr: addr, ManagementTransport: transport, Storage: storage, DatabaseURL: databaseURL}, nil
}
