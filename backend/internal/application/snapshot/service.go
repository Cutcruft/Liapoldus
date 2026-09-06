package snapshot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/application/id"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

type Service struct {
	sites domain.SiteRepository
	pages domain.PageRepository
	repo  domain.SnapshotRepository
	locks LockResolver
}

// LockResolver freezes the site's dependency lock into a new snapshot
// (dependency-service spec §7). Optional: created without one, snapshots get
// an empty lock.
type LockResolver interface {
	ResolveLock(context.Context, string) (domain.SnapshotLock, error)
}

func NewService(sites domain.SiteRepository, pages domain.PageRepository, repo domain.SnapshotRepository, locks ...LockResolver) *Service {
	svc := &Service{sites: sites, pages: pages, repo: repo}
	if len(locks) > 0 && locks[0] != nil {
		svc.locks = locks[0]
	}
	return svc
}

func (s *Service) Create(ctx context.Context, siteID, name string) (domain.Snapshot, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return domain.Snapshot{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Snapshot{}, fmt.Errorf("%w: name is required", domain.ErrInvalidRequest)
	}
	pages, err := s.pages.ListPagesBySite(ctx, siteID)
	if err != nil {
		return domain.Snapshot{}, err
	}
	refs := make([]domain.SnapshotPage, 0, len(pages))
	for _, p := range pages {
		versions, err := s.pages.ListPageVersions(ctx, p.ID)
		if err != nil || len(versions) == 0 {
			if err == nil {
				err = domain.ErrNotFound
			}
			return domain.Snapshot{}, err
		}
		latest := versions[len(versions)-1]
		refs = append(refs, domain.SnapshotPage{PageID: p.ID, VersionID: latest.ID, Version: latest.Number})
	}
	id, err := id.New(id.Snapshot)
	if err != nil {
		return domain.Snapshot{}, err
	}
	snapshot := domain.Snapshot{ID: id, SiteID: siteID, Name: name, Pages: refs, CreatedAt: time.Now().UTC()}
	if s.locks != nil {
		lock, err := s.locks.ResolveLock(ctx, siteID)
		if err != nil {
			// Publication must never proceed with an unresolved dependency
			// graph: invalid/nonexistent packages fail the snapshot now.
			return domain.Snapshot{}, fmt.Errorf("resolve dependencies: %w", err)
		}
		snapshot.DepsLock = lock
	}
	if err := s.repo.CreateSnapshot(ctx, snapshot); err != nil {
		return domain.Snapshot{}, err
	}
	return snapshot, nil
}

func (s *Service) Get(ctx context.Context, id string) (domain.Snapshot, error) {
	return s.repo.GetSnapshot(ctx, id)
}

func (s *Service) ListBySite(ctx context.Context, siteID string) ([]domain.Snapshot, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return nil, err
	}
	return s.repo.ListSnapshotsBySite(ctx, siteID)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.GetSnapshot(ctx, id); err != nil {
		return err
	}
	return s.repo.DeleteSnapshot(ctx, id)
}
