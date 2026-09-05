package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type SiteService struct{ repo SiteRepository }

func NewSiteService(repo SiteRepository) *SiteService { return &SiteService{repo: repo} }

func (s *SiteService) Create(ctx context.Context, name, slug string) (Site, error) {
	name, slug = strings.TrimSpace(name), strings.TrimSpace(slug)
	if name == "" || slug == "" {
		return Site{}, fmt.Errorf("%w: name and slug are required", ErrInvalidRequest)
	}
	id, err := NewID("site")
	if err != nil {
		return Site{}, err
	}
	site := Site{ID: id, Name: name, Slug: slug, CreatedAt: time.Now().UTC()}
	if err := s.repo.CreateSite(ctx, site); err != nil {
		return Site{}, err
	}
	return site, nil
}

func (s *SiteService) Get(ctx context.Context, id string) (Site, error) {
	return s.repo.GetSite(ctx, id)
}
