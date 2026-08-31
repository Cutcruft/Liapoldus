package page

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/liapoldus/liapoldus/backend/domain"
	"github.com/liapoldus/liapoldus/backend/shared"
)

type Service struct {
	repo     domain.PageRepository
	siteRepo domain.SiteRepository
}

func NewService(repo domain.PageRepository, siteRepo domain.SiteRepository) *Service {
	return &Service{repo: repo, siteRepo: siteRepo}
}

func (s *Service) Create(ctx context.Context, siteID, name, slug string, root domain.ComponentNode) (domain.Page, error) {
	if _, err := s.siteRepo.GetSite(ctx, siteID); err != nil {
		return domain.Page{}, err
	}
	name, slug = strings.TrimSpace(name), strings.TrimSpace(slug)
	if name == "" || slug == "" {
		return domain.Page{}, fmt.Errorf("%w: name and slug are required", domain.ErrInvalidRequest)
	}
	if err := root.Validate(); err != nil {
		return domain.Page{}, err
	}
	id, err := shared.NewID("page")
	if err != nil {
		return domain.Page{}, err
	}
	now := time.Now().UTC()
	page := domain.Page{ID: id, SiteID: siteID, Name: name, Slug: slug, Root: root, Version: 1, CreatedAt: now, UpdatedAt: now}
	versionID, err := shared.NewID("pagever")
	if err != nil {
		return domain.Page{}, err
	}
	version := domain.PageVersion{ID: versionID, PageID: id, Number: 1, Root: root, CreatedAt: now}
	if err := s.repo.CreatePage(ctx, page, version); err != nil {
		return domain.Page{}, err
	}
	return page, nil
}

func (s *Service) Get(ctx context.Context, id string) (domain.Page, error) {
	return s.repo.GetPage(ctx, id)
}

func (s *Service) ListBySite(ctx context.Context, siteID string) ([]domain.Page, error) {
	if _, err := s.siteRepo.GetSite(ctx, siteID); err != nil {
		return nil, err
	}
	return s.repo.ListPagesBySite(ctx, siteID)
}

func (s *Service) UpdateTree(ctx context.Context, id string, root domain.ComponentNode) (domain.Page, error) {
	if err := root.Validate(); err != nil {
		return domain.Page{}, err
	}
	current, err := s.repo.GetPage(ctx, id)
	if err != nil {
		return domain.Page{}, err
	}
	now := time.Now().UTC()
	current.Root, current.Version, current.UpdatedAt = root, current.Version+1, now
	versionID, err := shared.NewID("pagever")
	if err != nil {
		return domain.Page{}, err
	}
	version := domain.PageVersion{ID: versionID, PageID: id, Number: current.Version, Root: root, CreatedAt: now}
	if err := s.repo.UpdatePage(ctx, current, version); err != nil {
		return domain.Page{}, err
	}
	return current, nil
}

func (s *Service) Versions(ctx context.Context, id string) ([]domain.PageVersion, error) {
	return s.repo.ListPageVersions(ctx, id)
}
