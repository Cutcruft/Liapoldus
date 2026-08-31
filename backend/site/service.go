package site

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/liapoldus/liapoldus/backend/domain"
	"github.com/liapoldus/liapoldus/backend/shared"
)

type Service struct{ repo domain.SiteRepository }

func NewService(repo domain.SiteRepository) *Service { return &Service{repo: repo} }

func (s *Service) Create(ctx context.Context, name, slug string) (domain.Site, error) {
	name, slug = strings.TrimSpace(name), strings.TrimSpace(slug)
	if name == "" || slug == "" {
		return domain.Site{}, fmt.Errorf("%w: name and slug are required", domain.ErrInvalidRequest)
	}
	id, err := shared.NewID("site")
	if err != nil {
		return domain.Site{}, err
	}
	site := domain.Site{ID: id, Name: name, Slug: slug, CreatedAt: time.Now().UTC()}
	if err := s.repo.CreateSite(ctx, site); err != nil {
		return domain.Site{}, err
	}
	return site, nil
}

func (s *Service) Get(ctx context.Context, id string) (domain.Site, error) {
	return s.repo.GetSite(ctx, id)
}
