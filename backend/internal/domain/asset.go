package domain

import (
	"context"
	"io"
	"strings"
	"time"
)

const MasterVariant = "master"

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

func (a Asset) Metadata() AssetMetadata {
	return AssetMetadata{
		ID: a.ID, SiteID: a.SiteID, Name: a.Name, Mime: a.Mime, Size: a.Size, ETag: a.ETag, CreatedAt: a.CreatedAt,
		Variants: []AssetVariant{{Name: MasterVariant, URL: "/api/assets/" + a.ID + "/file", Mime: a.Mime, Size: a.Size}},
	}
}

func AssetMetadataList(assets []Asset) []AssetMetadata {
	out := make([]AssetMetadata, 0, len(assets))
	for _, asset := range assets {
		out = append(out, asset.Metadata())
	}
	return out
}

// AssetBlobStore persists the raw bytes (disk directory, see infra/store).
type AssetBlobStore interface {
	Save(context.Context, string, string, io.Reader) (int64, string, error)
	Open(context.Context, string, string) (io.ReadCloser, error)
	Delete(context.Context, string, string) error
}

type AssetService struct {
	repo  AssetRepository
	blobs AssetBlobStore
	sites SiteRepository
}

func NewAssetService(repo AssetRepository, blobs AssetBlobStore, sites SiteRepository) *AssetService {
	return &AssetService{repo: repo, blobs: blobs, sites: sites}
}

func (s *AssetService) Create(ctx context.Context, siteID, name, mime string, data io.Reader) (Asset, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return Asset{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "asset"
	}
	if strings.TrimSpace(mime) == "" {
		mime = "application/octet-stream"
	}
	id, err := NewID("asset")
	if err != nil {
		return Asset{}, err
	}
	size, etag, err := s.blobs.Save(ctx, siteID, id, data)
	if err != nil {
		return Asset{}, err
	}
	asset := Asset{ID: id, SiteID: siteID, Name: name, Mime: mime, Size: size, ETag: etag, CreatedAt: time.Now().UTC()}
	if err := s.repo.CreateAsset(ctx, asset); err != nil {
		// roll back the blob to keep the world consistent.
		_ = s.blobs.Delete(ctx, siteID, id)
		return Asset{}, err
	}
	return asset, nil
}

func (s *AssetService) Get(ctx context.Context, id string) (Asset, error) {
	return s.repo.GetAsset(ctx, id)
}

func (s *AssetService) List(ctx context.Context, siteID string) ([]Asset, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return nil, err
	}
	return s.repo.ListAssetsBySite(ctx, siteID)
}

func (s *AssetService) Delete(ctx context.Context, id string) error {
	asset, err := s.repo.GetAsset(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.DeleteAsset(ctx, id); err != nil {
		return err
	}
	return s.blobs.Delete(ctx, asset.SiteID, id)
}

// Open returns a reader for the asset bytes (master).
func (s *AssetService) Open(ctx context.Context, id string) (Asset, io.ReadCloser, error) {
	asset, err := s.repo.GetAsset(ctx, id)
	if err != nil {
		return Asset{}, nil, err
	}
	r, err := s.blobs.Open(ctx, asset.SiteID, id)
	if err != nil {
		return Asset{}, nil, err
	}
	return asset, r, nil
}
