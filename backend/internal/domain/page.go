package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type PageService struct {
	repo     PageRepository
	siteRepo SiteRepository
}

func NewPageService(repo PageRepository, siteRepo SiteRepository) *PageService {
	return &PageService{repo: repo, siteRepo: siteRepo}
}

func (s *PageService) Create(ctx context.Context, siteID, name, slug string, root ComponentNode) (Page, error) {
	if _, err := s.siteRepo.GetSite(ctx, siteID); err != nil {
		return Page{}, err
	}
	name, slug = strings.TrimSpace(name), strings.TrimSpace(slug)
	if name == "" || slug == "" {
		return Page{}, fmt.Errorf("%w: name and slug are required", ErrInvalidRequest)
	}
	if err := root.Validate(); err != nil {
		return Page{}, err
	}
	id, err := NewID("page")
	if err != nil {
		return Page{}, err
	}
	now := time.Now().UTC()
	page := Page{ID: id, SiteID: siteID, Name: name, Slug: slug, Root: root, Version: 1, CreatedAt: now, UpdatedAt: now}
	versionID, err := NewID("pagever")
	if err != nil {
		return Page{}, err
	}
	version := PageVersion{ID: versionID, PageID: id, Number: 1, Root: root, CreatedAt: now}
	if err := s.repo.CreatePage(ctx, page, version); err != nil {
		return Page{}, err
	}
	return page, nil
}

func (s *PageService) Get(ctx context.Context, id string) (Page, error) {
	return s.repo.GetPage(ctx, id)
}

func (s *PageService) ListBySite(ctx context.Context, siteID string) ([]Page, error) {
	if _, err := s.siteRepo.GetSite(ctx, siteID); err != nil {
		return nil, err
	}
	return s.repo.ListPagesBySite(ctx, siteID)
}

func (s *PageService) UpdateTree(ctx context.Context, id string, root ComponentNode) (Page, error) {
	if err := root.Validate(); err != nil {
		return Page{}, err
	}
	current, err := s.repo.GetPage(ctx, id)
	if err != nil {
		return Page{}, err
	}
	now := time.Now().UTC()
	current.Root, current.Version, current.UpdatedAt = root, current.Version+1, now
	versionID, err := NewID("pagever")
	if err != nil {
		return Page{}, err
	}
	version := PageVersion{ID: versionID, PageID: id, Number: current.Version, Root: root, CreatedAt: now}
	if err := s.repo.UpdatePage(ctx, current, version); err != nil {
		return Page{}, err
	}
	return current, nil
}

func (s *PageService) Versions(ctx context.Context, id string) ([]PageVersion, error) {
	return s.repo.ListPageVersions(ctx, id)
}
