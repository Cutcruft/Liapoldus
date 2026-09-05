package config

import (
	"strings"
	"testing"
	"time"
)

const (
	testDefaultLocale = "ru"
	testEmailPattern  = `^[^@\s]+@[^@\s]+\.[^@\s]+$`
	testTemplate      = "/files/{id}"
)

func setAllRequiredEnv(t *testing.T) {
	t.Helper()
	base := map[string]string{
		"LIAPOLDUS_ADMIN_ADDR":                  ":9090",
		"LIAPOLDUS_CLIENT_ADDR":                 ":19090",
		"LIAPOLDUS_ASSET_DIR":                   "/tmp/assets",
		"LIAPOLDUS_STORAGE":                     "memory",
		"LIAPOLDUS_DEFAULT_LOCALE":              testDefaultLocale,
		"LIAPOLDUS_REDIRECT_DEFAULT_STATUS":     "301",
		"LIAPOLDUS_REDIRECT_ALLOWED_STATUSES":   "301,302",
		"LIAPOLDUS_COMPONENT_MAX_DEPTH":         "5",
		"LIAPOLDUS_COMPONENT_TYPES":             "Container,Text",
		"LIAPOLDUS_PAGE_INITIAL_VERSION":        "1",
		"LIAPOLDUS_EMAIL_PATTERN":               testEmailPattern,
		"LIAPOLDUS_MASTER_VARIANT_NAME":         "master",
		"LIAPOLDUS_ASSET_FALLBACK_NAME":         "asset",
		"LIAPOLDUS_ASSET_FALLBACK_MIME":         "application/octet-stream",
		"LIAPOLDUS_ASSET_FILE_URL_TEMPLATE":     testTemplate,
		"LIAPOLDUS_ASSET_CACHE_MAX_AGE_SECONDS": "31536000",
		"LIAPOLDUS_MAX_UPLOAD_BYTES":            "10485760",
		"LIAPOLDUS_STARTUP_TIMEOUT":             "15s",
		"LIAPOLDUS_SHUTDOWN_TIMEOUT":            "10s",
		"LIAPOLDUS_READ_HEADER_TIMEOUT":         "5s",
	}
	for name, value := range base {
		t.Setenv(name, value)
	}
}

func TestLoadCustomConfiguration(t *testing.T) {
	setAllRequiredEnv(t)
	t.Setenv("LIAPOLDUS_ADMIN_TOKEN", "secret")
	t.Setenv("LIAPOLDUS_CLIENT_DEFAULT_SLUG", "demo")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AdminAddr != ":9090" || cfg.ClientAddr != ":19090" {
		t.Fatalf("addresses = %q, %q", cfg.AdminAddr, cfg.ClientAddr)
	}
	if cfg.AdminToken != "secret" {
		t.Fatalf("admin token = %q, want secret", cfg.AdminToken)
	}
	if cfg.ClientDefaultSlug != "demo" {
		t.Fatalf("client default slug = %q, want demo", cfg.ClientDefaultSlug)
	}
	if cfg.Storage != StorageMemory {
		t.Fatalf("storage = %q, want %q", cfg.Storage, StorageMemory)
	}
	if cfg.DefaultLocale != testDefaultLocale || cfg.RedirectDefaultStatus != 301 {
		t.Fatalf("cfg = %#v", cfg)
	}
	if len(cfg.RedirectAllowedStatuses) != 2 || cfg.RedirectAllowedStatuses[0] != 301 || cfg.RedirectAllowedStatuses[1] != 302 {
		t.Fatalf("redirect statuses = %#v", cfg.RedirectAllowedStatuses)
	}
	if cfg.ComponentMaxDepth != 5 || len(cfg.ComponentTypes) != 2 {
		t.Fatalf("component config = %#v", cfg)
	}
	if cfg.PageInitialVersion != 1 {
		t.Fatalf("page initial version = %d, want 1", cfg.PageInitialVersion)
	}
	if cfg.EmailPattern == nil || !cfg.EmailPattern.MatchString("user@example.com") {
		t.Fatalf("email pattern = %v", cfg.EmailPattern)
	}
	if cfg.MasterVariantName != "master" || cfg.AssetCacheMaxAgeSeconds != 31536000 || cfg.MaxUploadBytes != 10485760 {
		t.Fatalf("asset config = %#v", cfg)
	}
	if cfg.StartupTimeout != 15*time.Second || cfg.ShutdownTimeout != 10*time.Second || cfg.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("timeouts = %#v", cfg)
	}
}

func TestLoadRequiresEveryVariable(t *testing.T) {
	setAllRequiredEnv(t)
	t.Setenv("LIAPOLDUS_DEFAULT_LOCALE", "")
	t.Setenv("LIAPOLDUS_REDIRECT_ALLOWED_STATUSES", "")
	t.Setenv("LIAPOLDUS_ADMIN_ADDR", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want aggregated error")
	}
	message := err.Error()
	for _, missing := range []string{"LIAPOLDUS_DEFAULT_LOCALE", "LIAPOLDUS_REDIRECT_ALLOWED_STATUSES", "LIAPOLDUS_ADMIN_ADDR"} {
		if !strings.Contains(message, missing) {
			t.Fatalf("error %q does not mention %s", message, missing)
		}
	}
}

func TestLoadRejectsUnknownStorage(t *testing.T) {
	setAllRequiredEnv(t)
	t.Setenv("LIAPOLDUS_STORAGE", "mongo")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "LIAPOLDUS_STORAGE") {
		t.Fatalf("error = %v, want storage complaint", err)
	}
}

func TestLoadPostgresRequiresDatabaseURL(t *testing.T) {
	setAllRequiredEnv(t)
	t.Setenv("LIAPOLDUS_STORAGE", "postgres")
	t.Setenv("LIAPOLDUS_DATABASE_URL", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "LIAPOLDUS_DATABASE_URL") {
		t.Fatalf("error = %v, want database url complaint", err)
	}
}

func TestLoadPostgresOK(t *testing.T) {
	setAllRequiredEnv(t)
	t.Setenv("LIAPOLDUS_STORAGE", "postgres")
	t.Setenv("LIAPOLDUS_DATABASE_URL", "postgres://user:pass@localhost:5432/liapoldus?sslmode=disable")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage != StoragePostgres || cfg.DatabaseURL == "" {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestLoadRejectsTemplateWithoutPlaceholder(t *testing.T) {
	setAllRequiredEnv(t)
	t.Setenv("LIAPOLDUS_ASSET_FILE_URL_TEMPLATE", "/static/asset")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "{id}") {
		t.Fatalf("error = %v, want {id} placeholder complaint", err)
	}
}

func TestLoadRejectsInvalidEmailPattern(t *testing.T) {
	setAllRequiredEnv(t)
	t.Setenv("LIAPOLDUS_EMAIL_PATTERN", "([")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "LIAPOLDUS_EMAIL_PATTERN") {
		t.Fatalf("error = %v, want email pattern complaint", err)
	}
}
