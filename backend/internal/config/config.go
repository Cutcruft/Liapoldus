package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type StorageDriver string

const (
	StorageMemory   StorageDriver = "memory"
	StoragePostgres StorageDriver = "postgres"
)

type Config struct {
	AdminAddr               string
	ClientAddr              string
	AdminToken              string
	AssetDir                string
	ClientDefaultSlug       string
	Storage                 StorageDriver
	DatabaseURL             string
	DefaultLocale           string
	RedirectDefaultStatus   int
	RedirectAllowedStatuses []int
	ComponentMaxDepth       int
	ComponentTypes          []string
	PageInitialVersion      int32
	EmailPattern            *regexp.Regexp
	MasterVariantName       string
	AssetFallbackName       string
	AssetFallbackMime       string
	AssetFileURLTemplate    string
	AssetCacheMaxAgeSeconds int
	MaxUploadBytes          int64
	StartupTimeout          time.Duration
	ShutdownTimeout         time.Duration
	ReadHeaderTimeout       time.Duration
}

// Load is the single source of configuration: every value comes from the
// environment. There are no code defaults; missing or invalid variables are
// collected into a single error so operators see the full set at once.
func Load() (Config, error) {
	var problems []string

	adminAddr := os.Getenv("LIAPOLDUS_ADMIN_ADDR")
	clientAddr := os.Getenv("LIAPOLDUS_CLIENT_ADDR")
	adminToken := os.Getenv("LIAPOLDUS_ADMIN_TOKEN")
	assetDir := os.Getenv("LIAPOLDUS_ASSET_DIR")
	clientDefaultSlug := os.Getenv("LIAPOLDUS_CLIENT_DEFAULT_SLUG")

	storage := StorageDriver(strings.ToLower(strings.TrimSpace(os.Getenv("LIAPOLDUS_STORAGE"))))
	switch storage {
	case StorageMemory:
	case StoragePostgres:
	default:
		problems = append(problems, `LIAPOLDUS_STORAGE must be "memory" or "postgres"`)
	}

	databaseURL := os.Getenv("LIAPOLDUS_DATABASE_URL")
	if storage == StoragePostgres && strings.TrimSpace(databaseURL) == "" {
		problems = append(problems, "LIAPOLDUS_DATABASE_URL is required for postgres storage")
	}

	defaultLocale, err := required("LIAPOLDUS_DEFAULT_LOCALE")
	if err != nil {
		problems = append(problems, err.Error())
	}

	redirectDefaultStatus, err := parseInt("LIAPOLDUS_REDIRECT_DEFAULT_STATUS")
	if err != nil {
		problems = append(problems, err.Error())
	}

	redirectAllowedStatuses, err := parseIntList("LIAPOLDUS_REDIRECT_ALLOWED_STATUSES")
	if err != nil {
		problems = append(problems, err.Error())
	}

	componentMaxDepth, err := parseInt("LIAPOLDUS_COMPONENT_MAX_DEPTH")
	if err != nil {
		problems = append(problems, err.Error())
	}

	componentTypes, err := parseStringList("LIAPOLDUS_COMPONENT_TYPES")
	if err != nil {
		problems = append(problems, err.Error())
	}

	pageInitialVersion, err := parseInt("LIAPOLDUS_PAGE_INITIAL_VERSION")
	if err != nil {
		problems = append(problems, err.Error())
	}

	emailPatternRaw, err := required("LIAPOLDUS_EMAIL_PATTERN")
	if err != nil {
		problems = append(problems, err.Error())
	}

	masterVariantName, err := required("LIAPOLDUS_MASTER_VARIANT_NAME")
	if err != nil {
		problems = append(problems, err.Error())
	}

	assetFallbackName, err := required("LIAPOLDUS_ASSET_FALLBACK_NAME")
	if err != nil {
		problems = append(problems, err.Error())
	}

	assetFallbackMime, err := required("LIAPOLDUS_ASSET_FALLBACK_MIME")
	if err != nil {
		problems = append(problems, err.Error())
	}

	assetFileURLTemplate, err := required("LIAPOLDUS_ASSET_FILE_URL_TEMPLATE")
	if err != nil {
		problems = append(problems, err.Error())
	}

	assetCacheMaxAgeSeconds, err := parseInt("LIAPOLDUS_ASSET_CACHE_MAX_AGE_SECONDS")
	if err != nil {
		problems = append(problems, err.Error())
	}

	maxUploadBytes, err := parseInt64("LIAPOLDUS_MAX_UPLOAD_BYTES")
	if err != nil {
		problems = append(problems, err.Error())
	}

	startupTimeout, err := parseDuration("LIAPOLDUS_STARTUP_TIMEOUT")
	if err != nil {
		problems = append(problems, err.Error())
	}

	shutdownTimeout, err := parseDuration("LIAPOLDUS_SHUTDOWN_TIMEOUT")
	if err != nil {
		problems = append(problems, err.Error())
	}

	readHeaderTimeout, err := parseDuration("LIAPOLDUS_READ_HEADER_TIMEOUT")
	if err != nil {
		problems = append(problems, err.Error())
	}

	problems = append(problems, requireAll(
		"LIAPOLDUS_ADMIN_ADDR", "LIAPOLDUS_CLIENT_ADDR", "LIAPOLDUS_ASSET_DIR",
	)...)

	if len(problems) > 0 {
		return Config{}, aggregateError(problems)
	}

	emailPattern, err := regexp.Compile(emailPatternRaw)
	if err != nil {
		return Config{}, fmt.Errorf("LIAPOLDUS_EMAIL_PATTERN: invalid regular expression: %v", err)
	}
	if !strings.Contains(assetFileURLTemplate, "{id}") {
		return Config{}, errors.New("LIAPOLDUS_ASSET_FILE_URL_TEMPLATE must contain the {id} placeholder")
	}

	return Config{
		AdminAddr:               strings.TrimSpace(adminAddr),
		ClientAddr:              strings.TrimSpace(clientAddr),
		AdminToken:              strings.TrimSpace(adminToken),
		AssetDir:                strings.TrimSpace(assetDir),
		ClientDefaultSlug:       strings.TrimSpace(clientDefaultSlug),
		Storage:                 storage,
		DatabaseURL:             strings.TrimSpace(databaseURL),
		DefaultLocale:           defaultLocale,
		RedirectDefaultStatus:   redirectDefaultStatus,
		RedirectAllowedStatuses: redirectAllowedStatuses,
		ComponentMaxDepth:       componentMaxDepth,
		ComponentTypes:          componentTypes,
		PageInitialVersion:      int32(pageInitialVersion),
		EmailPattern:            emailPattern,
		MasterVariantName:       masterVariantName,
		AssetFallbackName:       assetFallbackName,
		AssetFallbackMime:       assetFallbackMime,
		AssetFileURLTemplate:    assetFileURLTemplate,
		AssetCacheMaxAgeSeconds: assetCacheMaxAgeSeconds,
		MaxUploadBytes:          maxUploadBytes,
		StartupTimeout:          startupTimeout,
		ShutdownTimeout:         shutdownTimeout,
		ReadHeaderTimeout:       readHeaderTimeout,
	}, nil
}

func required(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func requireAll(names ...string) []string {
	var missing []string
	for _, name := range names {
		if strings.TrimSpace(os.Getenv(name)) == "" {
			missing = append(missing, fmt.Sprintf("%s is required", name))
		}
	}
	return missing
}

func parseInt(name string) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %v", name, err)
	}
	return value, nil
}

func parseInt64(name string) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %v", name, err)
	}
	return value, nil
}

func parseIntList(name string) ([]int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil, fmt.Errorf("%s is required", name)
	}
	parts := strings.Split(raw, ",")
	result := make([]int, 0, len(parts))
	seen := map[int]bool{}
	for _, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("%s must be a comma-separated list of integers: %v", name, err)
		}
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result, nil
}

func parseStringList(name string) ([]string, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil, fmt.Errorf("%s is required", name)
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			result = append(result, value)
		}
	}
	return result, nil
}

func parseDuration(name string) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return 0, fmt.Errorf("%s is required", name)
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration (e.g. 5s, 1m): %v", name, err)
	}
	return value, nil
}

func aggregateError(problems []string) error {
	message := strings.Join(problems, "\n  - ")
	return fmt.Errorf("invalid configuration:\n  - %s", message)
}
