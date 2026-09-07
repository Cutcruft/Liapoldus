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

// Element is a single component instance in a page's linear composition list
// (docs/redesign/backend.md §1.3). Position in Page.List is the order of
// rendering; ID is stable (assigned by the server on create, unchanged across
// insert/move) so bindings and drag/drop can reference it reliably.
type Element struct {
	ID          string                `json:"id"`
	ComponentID string                `json:"componentId"`
	Props       map[string]ElementProp `json:"props,omitempty"`
	Bindings    []BindingSource       `json:"bindings,omitempty"`
}

// ElementProp is a single property value: either a literal or a binding to a
// runtime source (§1.3).
type ElementProp struct {
	Kind   string        `json:"kind"` // "literal" | "binding"
	Value  any           `json:"value,omitempty"`
	Source *BindingSource `json:"source,omitempty"`
}

// BindingSource identifies where a property value comes from (§1.3). It is a
// discriminated union; exactly one variant's fields must be set.
type BindingSource struct {
	Kind        string `json:"kind"` // "content" | "form" | "operation" | "query" | "routeGroup"
	ContentID   string `json:"contentId,omitempty"`
	Field       string `json:"field,omitempty"`
	FormID      string `json:"formId,omitempty"`
	OperationID string `json:"operationId,omitempty"`
	Param       string `json:"param,omitempty"`
	Index       *int   `json:"index,omitempty"` // routeGroup
}

// ComponentDefinition is the persisted registry entry for a component. It
// carries the source alongside the parsed schema/metadata (R5); the site git
// history keeps the durable per-version record. Source is json:"-" so it
// never leaks into page/snapshot payloads — it is served explicitly by the
// component get/history endpoints.
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
	ID        string      `json:"id"`
	SiteID    string      `json:"siteId"`
	Name      string      `json:"name"`
	Slug      string      `json:"slug"`
	List      []Element   `json:"list"`
	Version   int32       `json:"version"`
	CreatedAt time.Time   `json:"createdAt"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

type PageVersion struct {
	ID        string      `json:"id"`
	PageID    string      `json:"pageId"`
	Number    int32       `json:"number"`
	List      []Element   `json:"list"`
	CreatedAt time.Time   `json:"createdAt"`
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
