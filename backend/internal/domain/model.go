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
)

type Site struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Slug          string    `json:"slug"`
	DefaultLocale string    `json:"defaultLocale"`
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

// ComponentVersion is a commit in the site's git repo representing a released
// definition of a component.
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
	Pages     []SnapshotPage `json:"pages"`
	CreatedAt time.Time      `json:"createdAt"`
}
