// Package token manages the per-site design-token set (colors, typography,
// spacing, …). The editor always works on the whole set: Get returns it (the
// empty set when the site has none stored) and Update replaces it entirely.
package token

import (
	"context"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

type Service struct {
	repo  domain.TokenRepository
	sites domain.SiteRepository
}

func NewService(repo domain.TokenRepository, sites domain.SiteRepository) *Service {
	return &Service{repo: repo, sites: sites}
}

// Get returns the site's token set, normalizing an absent row to the empty set.
func (s *Service) Get(ctx context.Context, siteID string) (*domain.TokenSet, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return nil, err
	}
	set, err := s.repo.GetTokens(ctx, siteID)
	if err != nil {
		return nil, err
	}
	return normalize(set), nil
}

// Update replaces the site's token set with the given one and returns it.
func (s *Service) Update(ctx context.Context, siteID string, set *domain.TokenSet) (*domain.TokenSet, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return nil, err
	}
	set = normalize(set)
	if err := s.repo.UpsertTokens(ctx, siteID, set); err != nil {
		return nil, err
	}
	return set, nil
}

// normalize replaces a nil set with the empty set and keeps the Colors slice
// non-nil so the JSON wire format is a stable "colors": [].
func normalize(set *domain.TokenSet) *domain.TokenSet {
	if set == nil {
		set = &domain.TokenSet{}
	}
	if set.Colors == nil {
		set.Colors = []domain.ColorToken{}
	}
	return set
}
