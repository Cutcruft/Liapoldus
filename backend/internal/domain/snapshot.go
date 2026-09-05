package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type SnapshotService struct {
	sites SiteRepository
	pages PageRepository
	repo  SnapshotRepository
}

func NewSnapshotService(sites SiteRepository, pages PageRepository, repo SnapshotRepository) *SnapshotService {
	return &SnapshotService{sites: sites, pages: pages, repo: repo}
}

func (s *SnapshotService) Create(ctx context.Context, siteID, name string) (Snapshot, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return Snapshot{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Snapshot{}, fmt.Errorf("%w: name is required", ErrInvalidRequest)
	}
	pages, err := s.pages.ListPagesBySite(ctx, siteID)
	if err != nil {
		return Snapshot{}, err
	}
	refs := make([]SnapshotPage, 0, len(pages))
	for _, p := range pages {
		versions, err := s.pages.ListPageVersions(ctx, p.ID)
		if err != nil || len(versions) == 0 {
			if err == nil {
				err = ErrNotFound
			}
			return Snapshot{}, err
		}
		latest := versions[len(versions)-1]
		refs = append(refs, SnapshotPage{PageID: p.ID, VersionID: latest.ID, Version: latest.Number})
	}
	id, err := NewID("snapshot")
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{ID: id, SiteID: siteID, Name: name, Pages: refs, CreatedAt: time.Now().UTC()}
	if err := s.repo.CreateSnapshot(ctx, snapshot); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func (s *SnapshotService) Get(ctx context.Context, id string) (Snapshot, error) {
	return s.repo.GetSnapshot(ctx, id)
}

func (s *SnapshotService) ListBySite(ctx context.Context, siteID string) ([]Snapshot, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return nil, err
	}
	return s.repo.ListSnapshotsBySite(ctx, siteID)
}

func (s *SnapshotService) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.GetSnapshot(ctx, id); err != nil {
		return err
	}
	return s.repo.DeleteSnapshot(ctx, id)
}
