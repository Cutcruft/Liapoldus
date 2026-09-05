package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrNotFound       = errors.New("resource not found")
	ErrAlreadyExists  = errors.New("resource already exists")
	ErrInvalidRequest = errors.New("invalid request")
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
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Props    map[string]any  `json:"props,omitempty"`
	Children []ComponentNode `json:"children,omitempty"`
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

// Validate checks the structural invariants required by the first renderer.
// The list of component types is intentionally small until ComponentDefinition
// is introduced as a persisted domain object.
func (n ComponentNode) Validate() error {
	return n.validate(0)
}

func (n ComponentNode) validate(depth int) error {
	if depth > 64 {
		return fmt.Errorf("%w: component tree depth exceeds 64", ErrInvalidRequest)
	}
	if strings.TrimSpace(n.ID) == "" {
		return fmt.Errorf("%w: component id is required", ErrInvalidRequest)
	}
	if !SupportedComponentTypes[n.Type] {
		return fmt.Errorf("%w: unsupported component type %q", ErrInvalidRequest, n.Type)
	}
	for _, child := range n.Children {
		if err := child.validate(depth + 1); err != nil {
			return err
		}
	}
	return nil
}

var SupportedComponentTypes = map[string]bool{
	"Container": true,
	"Text":      true,
	"Image":     true,
	"Button":    true,
}
