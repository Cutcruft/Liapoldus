package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound           = errors.New("resource not found")
	ErrAlreadyExists      = errors.New("resource already exists")
	ErrInvalidRequest     = errors.New("invalid request")
	ErrRepoNotInitialized = errors.New("repository not initialized")
	ErrSchemaInvalid      = errors.New("invalid JSON schema")
	ErrVersionNotFound    = errors.New("component version not found")
	ErrBuildFailed        = errors.New("build failed")
	ErrNotFastForward     = errors.New("not a fast-forward merge")
)

type Site struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	DefaultLocale string    `json:"defaultLocale"`
	DefaultBranch string    `json:"defaultBranch,omitempty"`
	Hosts         []string  `json:"hosts"`
	CreatedAt     time.Time `json:"createdAt"`
}

type ComponentNode struct {
	InstanceID   string             `json:"instanceId"`
	DefinitionID string             `json:"definitionId"`
	Props        map[string]any     `json:"props,omitempty"`
	Bindings     []ComponentBinding `json:"bindings,omitempty"`
	Children     []ComponentNode    `json:"children,omitempty"`
}

// ComponentBinding mirrors the ui-runtime binding contract (§6 of
// docs/ui-runtime/json-descriptors.md).
type ComponentBinding struct {
	Property string        `json:"property"`
	Source   BindingSource `json:"source"`
}

// BindingSource identifies where an instance property value comes from.
// Source must carry the discriminant fields of exactly one source type.
type BindingSource struct {
	Type        string `json:"type"`
	ContentID   string `json:"contentId,omitempty"`
	Name        string `json:"name,omitempty"`
	OperationID string `json:"operationId,omitempty"`
	FormID      string `json:"formId,omitempty"`
	Path        string `json:"path,omitempty"`
	Source      string `json:"source,omitempty"`
}

// ComponentDefinition is the persisted registry entry for a component. The
// actual source is stored in git (R5); the registry carries the parsed schema
// and metadata for validation and listing.
type ComponentDefinition struct {
	SiteID     string         `json:"siteId"`
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Kind       string         `json:"kind"`
	Source     string         `json:"-"`
	Schema     map[string]any `json:"schema"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CurrentSHA string         `json:"currentSha"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

// ComponentVersion is a released definition of a component. Its ID is a
// generated opaque identifier (not a git sha); concrete versions are captured
// by the site-wide snapshot git history (slice 1).
type ComponentVersion struct {
	ID           string    `json:"id"`
	SiteID       string    `json:"siteId"`
	DefinitionID string    `json:"definitionId"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Page struct {
	ID        string        `json:"id"`
	SiteID    string        `json:"siteId"`
	Name      string        `json:"name"`
	Slug      string        `json:"slug"`
	Root      ComponentNode `json:"root"`
	Version   int32         `json:"version"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
}

type PageVersion struct {
	ID        string        `json:"id"`
	PageID    string        `json:"pageId"`
	Number    int32         `json:"number"`
	Root      ComponentNode `json:"root"`
	CreatedAt time.Time     `json:"createdAt"`
}

type SnapshotPage struct {
	PageID    string `json:"pageId"`
	VersionID string `json:"versionId"`
	Version   int32  `json:"version"`
}

type Snapshot struct {
	ID        string         `json:"id"`
	SiteID    string         `json:"siteId"`
	Name      string         `json:"name"`
	GitSHA    string         `json:"gitSha,omitempty"`
	Pages     []SnapshotPage `json:"pages"`
	DepsLock  SnapshotLock   `json:"depsLock,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

// BuildStatus describes the lifecycle of a snapshot compilation. Transitions
// are strictly queued → building → ready | failed.
type BuildStatus string

const (
	BuildStatusQueued   BuildStatus = "queued"
	BuildStatusBuilding BuildStatus = "building"
	BuildStatusReady    BuildStatus = "ready"
	BuildStatusFailed   BuildStatus = "failed"
)

// Environments are fixed strings on a Build (Этап 3); an Environment entity
// with deploy configuration arrives in a later stage (README §25).
const (
	EnvironmentDevelopment = "development"
	EnvironmentProduction  = "production"
)

// Build is the result of compiling a Site Snapshot for an Environment with the
// esbuild-based bundler. The ArtifactDir points at the published static output.
type Build struct {
	ID          string      `json:"id"`
	SiteID      string      `json:"siteId"`
	SnapshotID  string      `json:"snapshotId"`
	Environment string      `json:"environment"`
	Status      BuildStatus `json:"status"`
	Log         []string    `json:"log"`
	ArtifactDir string      `json:"artifactDir"`
	CreatedAt   time.Time   `json:"createdAt"`
	StartedAt   *time.Time  `json:"startedAt,omitempty"`
	FinishedAt  *time.Time  `json:"finishedAt,omitempty"`
}
