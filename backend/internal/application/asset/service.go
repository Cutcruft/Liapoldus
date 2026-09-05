package asset

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/application/id"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Settings carries the externally-configured defaults and the byte URL shape
// (previously hardcoded constants in the domain layer).
type Settings struct {
	MasterVariant string
	FallbackName  string
	FallbackMime  string
	URLTemplate   string // byte URL template with a {id} placeholder
}

type Service struct {
	repo     domain.AssetRepository
	blobs    domain.AssetBlobStore
	sites    domain.SiteRepository
	settings Settings
}

func NewService(repo domain.AssetRepository, blobs domain.AssetBlobStore, sites domain.SiteRepository, settings Settings) *Service {
	return &Service{repo: repo, blobs: blobs, sites: sites, settings: settings}
}

func (s *Service) Create(ctx context.Context, siteID, name, mime string, data io.Reader) (domain.Asset, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return domain.Asset{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = s.settings.FallbackName
	}
	if strings.TrimSpace(mime) == "" {
		mime = s.settings.FallbackMime
	}
	id, err := id.New(id.Asset)
	if err != nil {
		return domain.Asset{}, err
	}
	size, etag, err := s.blobs.Save(ctx, siteID, id, data)
	if err != nil {
		return domain.Asset{}, err
	}
	asset := domain.Asset{ID: id, SiteID: siteID, Name: name, Mime: mime, Size: size, ETag: etag, CreatedAt: time.Now().UTC()}
	if err := s.repo.CreateAsset(ctx, asset); err != nil {
		// roll back the blob to keep the world consistent.
		_ = s.blobs.Delete(ctx, siteID, id)
		return domain.Asset{}, err
	}
	return asset, nil
}

func (s *Service) Get(ctx context.Context, id string) (domain.Asset, error) {
	return s.repo.GetAsset(ctx, id)
}

func (s *Service) List(ctx context.Context, siteID string) ([]domain.Asset, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return nil, err
	}
	return s.repo.ListAssetsBySite(ctx, siteID)
}

func (s *Service) Delete(ctx context.Context, id string) error {
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
func (s *Service) Open(ctx context.Context, id string) (domain.Asset, io.ReadCloser, error) {
	asset, err := s.repo.GetAsset(ctx, id)
	if err != nil {
		return domain.Asset{}, nil, err
	}
	r, err := s.blobs.Open(ctx, asset.SiteID, id)
	if err != nil {
		return domain.Asset{}, nil, err
	}
	return asset, r, nil
}

// Metadata builds the public representation with the master variant and the
// configured relative byte URL.
func (s *Service) Metadata(a domain.Asset) domain.AssetMetadata {
	return domain.AssetMetadata{
		ID: a.ID, SiteID: a.SiteID, Name: a.Name, Mime: a.Mime, Size: a.Size, ETag: a.ETag, CreatedAt: a.CreatedAt,
		Variants: []domain.AssetVariant{{Name: s.settings.MasterVariant, URL: s.byteURL(a.ID), Mime: a.Mime, Size: a.Size}},
	}
}

func (s *Service) MetadataList(assets []domain.Asset) []domain.AssetMetadata {
	out := make([]domain.AssetMetadata, 0, len(assets))
	for _, a := range assets {
		out = append(out, s.Metadata(a))
	}
	return out
}

// IsMaster reports whether the given variant name is the master variant.
func (s *Service) IsMaster(variant string) bool {
	return variant == s.settings.MasterVariant
}

func (s *Service) byteURL(assetID string) string {
	return strings.ReplaceAll(s.settings.URLTemplate, "{id}", assetID)
}
