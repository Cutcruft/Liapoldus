package config

import (
	"fmt"
	"os"
	"strings"
)

type StorageDriver string

const (
	StorageMemory   StorageDriver = "memory"
	StoragePostgres StorageDriver = "postgres"
)

type Config struct {
	AdminAddr         string
	ClientAddr        string
	AdminToken        string
	AssetDir          string
	ClientDefaultSlug string
	Storage           StorageDriver
	DatabaseURL       string
}

func Load() (Config, error) {
	adminAddr := strings.TrimSpace(os.Getenv("LIAPOLDUS_ADMIN_ADDR"))
	if adminAddr == "" {
		// backward-compatible alias for the management/editor API.
		adminAddr = strings.TrimSpace(os.Getenv("LIAPOLDUS_ADDR"))
	}
	if adminAddr == "" {
		adminAddr = ":8080"
	}

	clientAddr := strings.TrimSpace(os.Getenv("LIAPOLDUS_CLIENT_ADDR"))
	if clientAddr == "" {
		clientAddr = ":18080"
	}

	adminToken := strings.TrimSpace(os.Getenv("LIAPOLDUS_ADMIN_TOKEN"))

	assetDir := strings.TrimSpace(os.Getenv("LIAPOLDUS_ASSET_DIR"))
	if assetDir == "" {
		assetDir = "./data/assets"
	}

	clientDefaultSlug := strings.TrimSpace(os.Getenv("LIAPOLDUS_CLIENT_DEFAULT_SLUG"))

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
	return Config{
		AdminAddr:         adminAddr,
		ClientAddr:        clientAddr,
		AdminToken:        adminToken,
		AssetDir:          assetDir,
		ClientDefaultSlug: clientDefaultSlug,
		Storage:           storage,
		DatabaseURL:       databaseURL,
	}, nil
}
