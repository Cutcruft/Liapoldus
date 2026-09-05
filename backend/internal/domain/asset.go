package domain

import (
	"context"
	"io"
	"time"
)

// AssetVariant describes one servable byte stream of the asset.
type AssetVariant struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Mime string `json:"mime"`
	Size int64  `json:"size"`
}

// Asset is immutable: each upload creates a new id. This round stores exactly
// one variant (master).
type Asset struct {
	ID        string    `json:"id"`
	SiteID    string    `json:"siteId"`
	Name      string    `json:"name"`
	Mime      string    `json:"mime"`
	Size      int64     `json:"size"`
	ETag      string    `json:"etag"`
	CreatedAt time.Time `json:"createdAt"`
}

// AssetMetadata is the public representation with the master variant and a
// relative byte URL (served on both ports).
type AssetMetadata struct {
	ID        string         `json:"id"`
	SiteID    string         `json:"siteId"`
	Name      string         `json:"name"`
	Mime      string         `json:"mime"`
	Size      int64          `json:"size"`
	Variants  []AssetVariant `json:"variants"`
	ETag      string         `json:"etag"`
	CreatedAt time.Time      `json:"createdAt"`
}

// AssetBlobStore persists the raw bytes (disk directory, see infra/storage).
type AssetBlobStore interface {
	Save(context.Context, string, string, io.Reader) (int64, string, error)
	Open(context.Context, string, string) (io.ReadCloser, error)
	Delete(context.Context, string, string) error
}
