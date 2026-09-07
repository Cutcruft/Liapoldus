// Package sitesettings manages presentation defaults owned by a site.
package sitesettings

import (
	"context"
	"fmt"
	"strings"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

type Service struct {
	repo  domain.SiteSettingsRepository
	sites domain.SiteRepository
}

func NewService(repo domain.SiteSettingsRepository, sites domain.SiteRepository) *Service {
	return &Service{repo: repo, sites: sites}
}

// Get returns persisted settings or normalized defaults derived from Site.
func (s *Service) Get(ctx context.Context, siteID string) (*domain.SiteSettings, error) {
	site, err := s.sites.GetSite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	settings, err := s.repo.GetSiteSettings(ctx, siteID)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return defaults(site), nil
	}
	return normalize(settings, site), nil
}

// Update replaces settings atomically after normalizing optional maps and
// rejecting malformed locale/head values before persistence.
func (s *Service) Update(ctx context.Context, siteID string, next *domain.SiteSettings) (*domain.SiteSettings, error) {
	site, err := s.sites.GetSite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	if next == nil {
		return nil, fmt.Errorf("%w: settings are required", domain.ErrInvalidRequest)
	}
	next.SiteID = siteID
	next = normalize(next, site)
	if err := validate(next); err != nil {
		return nil, err
	}
	if err := s.repo.UpsertSiteSettings(ctx, next); err != nil {
		return nil, err
	}
	return next, nil
}

func defaults(site domain.Site) *domain.SiteSettings {
	return &domain.SiteSettings{
		SiteID:        site.ID,
		DefaultLocale: site.DefaultLocale,
		Head:          domain.SiteHead{Meta: map[string]string{}},
	}
}

func normalize(settings *domain.SiteSettings, site domain.Site) *domain.SiteSettings {
	copy := *settings
	copy.SiteID = site.ID
	copy.DefaultLocale = strings.TrimSpace(copy.DefaultLocale)
	if copy.DefaultLocale == "" {
		copy.DefaultLocale = site.DefaultLocale
	}
	copy.DefaultLayoutSectionID = strings.TrimSpace(copy.DefaultLayoutSectionID)
	copy.Head.TitleTemplate = strings.TrimSpace(copy.Head.TitleTemplate)
	copy.Head.Description = strings.TrimSpace(copy.Head.Description)
	copy.Head.FaviconAssetID = strings.TrimSpace(copy.Head.FaviconAssetID)
	if copy.Head.Meta == nil {
		copy.Head.Meta = map[string]string{}
	}
	return &copy
}

func validate(settings *domain.SiteSettings) error {
	if strings.TrimSpace(settings.DefaultLocale) == "" {
		return fmt.Errorf("%w: defaultLocale is required", domain.ErrInvalidRequest)
	}
	for key, value := range settings.Head.Meta {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: head.meta keys and values must not be empty", domain.ErrInvalidRequest)
		}
	}
	return nil
}
