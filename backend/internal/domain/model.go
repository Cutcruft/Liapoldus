package domain

import (
	"errors"
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
