package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/application/id"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Releaser publishes a snapshot artifact for an environment. build.Service
// satisfies it and is idempotent per (site, environment, snapshot).
type Releaser interface {
	Create(context.Context, string, string, string) (domain.Build, error)
}

// Service manages the active release pins of a site (R9, spec §2.6).
// A deployment is the (site, environment) → snapshotId pin; releasing swaps
// the pin and re-publishes the snapshot artifact. Rollback pins an earlier
// snapshot. Both release and rollback are idempotent: re-publishing the same
// snapshot returns the current pin unchanged.
type Service struct {
	sites     domain.SiteRepository
	snapshots domain.SnapshotRepository
	repo      domain.DeploymentRepository
	builds    Releaser
}

func NewService(sites domain.SiteRepository, snapshots domain.SnapshotRepository,
	repo domain.DeploymentRepository, builds Releaser) *Service {
	return &Service{sites: sites, snapshots: snapshots, repo: repo, builds: builds}
}

func validEnvironment(environment string) bool {
	return environment == domain.EnvironmentDevelopment || environment == domain.EnvironmentProduction
}

// resolveSnapshot validates the snapshot belongs to the site and returns it.
func (s *Service) resolveSnapshot(ctx context.Context, siteID, snapshotID string) (domain.Snapshot, error) {
	snapshot, err := s.snapshots.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return domain.Snapshot{}, err
	}
	if snapshot.SiteID != siteID {
		return domain.Snapshot{}, fmt.Errorf("%w: snapshot does not belong to site", domain.ErrNotFound)
	}
	return snapshot, nil
}

// Release publishes a snapshot to an environment: republishes the artifact
// (idempotent build) and pins it as the active deployment. Releasing an
// already-pinned snapshot is a no-op returning the current pin.
func (s *Service) Release(ctx context.Context, siteID, snapshotID, environment string) (domain.Deployment, error) {
	if !validEnvironment(environment) {
		return domain.Deployment{}, fmt.Errorf("%w: environment must be one of development, production", domain.ErrInvalidRequest)
	}
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return domain.Deployment{}, err
	}
	if _, err := s.resolveSnapshot(ctx, siteID, snapshotID); err != nil {
		return domain.Deployment{}, err
	}

	current, currentErr := s.repo.GetDeployment(ctx, siteID, environment)
	if currentErr == nil && current.SnapshotID == snapshotID {
		return current, nil
	}
	if currentErr != nil && currentErr != domain.ErrNotFound {
		return domain.Deployment{}, currentErr
	}

	// Re-publishing reuses the build pipeline: it is idempotent per
	// (site, environment, snapshot), so a repeat release is cheap.
	if _, err := s.builds.Create(ctx, siteID, snapshotID, environment); err != nil {
		return domain.Deployment{}, err
	}

	deployment := domain.Deployment{
		SiteID:      siteID,
		Environment: environment,
		SnapshotID:  snapshotID,
		CreatedAt:   time.Now().UTC(),
	}
	if currentErr == domain.ErrNotFound {
		depID, err := id.New(id.Deployment)
		if err != nil {
			return domain.Deployment{}, err
		}
		deployment.ID = depID
	} else {
		deployment.ID = current.ID
	}
	if err := s.repo.SetDeployment(ctx, deployment); err != nil {
		return domain.Deployment{}, err
	}
	return deployment, nil
}

// Rollback pins an earlier snapshot for the environment. The target must be a
// snapshot released strictly before the currently pinned one (an equal
// snapshot is an idempotent no-op); releasing forward goes through Release.
func (s *Service) Rollback(ctx context.Context, siteID, snapshotID, environment string) (domain.Deployment, error) {
	if !validEnvironment(environment) {
		return domain.Deployment{}, fmt.Errorf("%w: environment must be one of development, production", domain.ErrInvalidRequest)
	}
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return domain.Deployment{}, err
	}
	snapshot, err := s.resolveSnapshot(ctx, siteID, snapshotID)
	if err != nil {
		return domain.Deployment{}, err
	}

	current, err := s.repo.GetDeployment(ctx, siteID, environment)
	if err != nil {
		if err == domain.ErrNotFound {
			return domain.Deployment{}, fmt.Errorf("%w: no deployment to roll back", domain.ErrInvalidRequest)
		}
		return domain.Deployment{}, err
	}
	if current.SnapshotID == snapshotID {
		return current, nil
	}
	pinned, err := s.snapshots.GetSnapshot(ctx, current.SnapshotID)
	if err != nil {
		return domain.Deployment{}, err
	}
	if !snapshot.CreatedAt.Before(pinned.CreatedAt) {
		return domain.Deployment{}, fmt.Errorf("%w: rollback target must be an earlier snapshot", domain.ErrInvalidRequest)
	}

	if _, err := s.builds.Create(ctx, siteID, snapshotID, environment); err != nil {
		return domain.Deployment{}, err
	}

	deployment := domain.Deployment{
		ID:          current.ID,
		SiteID:      siteID,
		Environment: environment,
		SnapshotID:  snapshotID,
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.repo.SetDeployment(ctx, deployment); err != nil {
		return domain.Deployment{}, err
	}
	return deployment, nil
}

// Active returns the current deployment pin for the environment, or
// ErrNotFound when the environment was never published.
func (s *Service) Active(ctx context.Context, siteID, environment string) (domain.Deployment, error) {
	return s.repo.GetDeployment(ctx, siteID, environment)
}

// ListBySite returns every environment pin of the site.
func (s *Service) ListBySite(ctx context.Context, siteID string) ([]domain.Deployment, error) {
	return s.repo.ListDeploymentsBySite(ctx, siteID)
}