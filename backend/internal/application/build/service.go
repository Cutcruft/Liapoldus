package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/application/id"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// DefinitionRef pins a materialized definition to the git commit it was built
// from, so a runtime can reproduce exactly what was released.
type DefinitionRef struct {
	File string `json:"file"`
	SHA  string `json:"sha"`
}

// Manifest is the workspace contract between the materializer and the bundle
// runner (and later, the runtime). It is written to the workspace as
// manifest.json.
type Manifest struct {
	SiteID      string                   `json:"siteId"`
	SnapshotID  string                   `json:"snapshotId"`
	Environment string                   `json:"environment"`
	Definitions map[string]DefinitionRef `json:"definitions"`
	Pages       []string                 `json:"pages"`
	Externals   []string                 `json:"externals"`
}

// SharedExternals are the runtime libraries the site bundle does not include;
// they are served separately by the backend (R3).
var SharedExternals = []string{"react", "react-dom", "@liapoldus/ui-runtime"}

// WorkspaceRequest describes the materialization target. Dir is chosen by the
// caller (typically a temporary directory) and must not exist yet or be empty.
type WorkspaceRequest struct {
	SiteID      string
	SnapshotID  string
	Environment string
	Dir         string
}

// Workspace is a materialized frontend build input: src/entry.tsx,
// src/definitions/*.tsx and manifest.json.
type Workspace struct {
	Dir      string
	Manifest Manifest
}

// BundleResult carries the compiled output directory produced by the bundle
// runner (esbuild writes dist/ inside the workspace).
type BundleResult struct {
	DistDir string
}

// WorkspaceBuilder materializes a snapshot into an esbuild-able workspace.
type WorkspaceBuilder interface {
	Materialize(context.Context, WorkspaceRequest) (Workspace, error)
}

// BundleRunner compiles a materialized workspace into the site bundle.
type BundleRunner interface {
	Build(context.Context, Workspace) (BundleResult, error)
}

// ArtifactStore publishes a finished build into the artifact directory
// build/<site>/<environment>/<snapshot>/ and returns that directory.
type ArtifactStore interface {
	Publish(context.Context, string, string, string, Workspace, BundleResult) (string, error)
}

// Service orchestrates the build pipeline: create + status machine, and the
// synchronous materialize → bundle → publish run (Этап 3).
type Service struct {
	sites     domain.SiteRepository
	snapshots domain.SnapshotRepository
	builds    domain.BuildRepository
	builder   WorkspaceBuilder
	runner    BundleRunner
	artifacts ArtifactStore
}

func NewService(sites domain.SiteRepository, snapshots domain.SnapshotRepository,
	builds domain.BuildRepository, builder WorkspaceBuilder, runner BundleRunner,
	artifacts ArtifactStore) *Service {
	return &Service{sites: sites, snapshots: snapshots, builds: builds, builder: builder, runner: runner, artifacts: artifacts}
}

// validEnvironments is the closed set of environment strings supported in
// Этап 3 (an Environment entity arrives in a later stage).
func validEnvironments() map[string]bool {
	return map[string]bool{
		domain.EnvironmentDevelopment: true,
		domain.EnvironmentProduction:  true,
	}
}

// Create publishes a snapshot for an environment. Re-publication of the same
// snapshot is a no-op: an existing ready build is returned unchanged (same id,
// same artifact directory). A failed build is superseded by a fresh attempt.
func (s *Service) Create(ctx context.Context, siteID, snapshotID, environment string) (domain.Build, error) {
	if !validEnvironments()[environment] {
		return domain.Build{}, fmt.Errorf("%w: environment must be one of development, production", domain.ErrInvalidRequest)
	}
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return domain.Build{}, err
	}
	snapshot, err := s.snapshots.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return domain.Build{}, err
	}
	if snapshot.SiteID != siteID {
		return domain.Build{}, fmt.Errorf("%w: snapshot does not belong to site", domain.ErrNotFound)
	}

	existing, err := s.builds.GetBuildBySnapshot(ctx, siteID, environment, snapshotID)
	if err == nil {
		switch existing.Status {
		case domain.BuildStatusReady, domain.BuildStatusQueued, domain.BuildStatusBuilding:
			return existing, nil
		}
	} else if !errors.Is(err, domain.ErrNotFound) {
		return domain.Build{}, err
	}

	buildID, err := id.New(id.Build)
	if err != nil {
		return domain.Build{}, err
	}
	build := domain.Build{
		ID:          buildID,
		SiteID:      siteID,
		SnapshotID:  snapshotID,
		Environment: environment,
		Status:      domain.BuildStatusQueued,
		Log:         []string{"build queued"},
		CreatedAt:   time.Now().UTC(),
	}
	if err := s.builds.CreateBuild(ctx, build); err != nil {
		return domain.Build{}, err
	}
	// Re-read so a concurrent attempt for the same (site, env, snapshot) wins
	// and both callers finish on the same row (DB unique on status).
	if bySnapshot, err := s.builds.GetBuildBySnapshot(ctx, siteID, environment, snapshotID); err == nil {
		build = bySnapshot
	}

	return s.run(ctx, build)
}

// run executes the synchronous pipeline against an already-persisted build.
// Every failure transitions the build to failed and surfaces ErrBuildFailed.
func (s *Service) run(ctx context.Context, build domain.Build) (domain.Build, error) {
	now := time.Now().UTC()
	build.Status = domain.BuildStatusBuilding
	build.StartedAt = &now
	build.Log = append(build.Log, "materializing workspace")
	if err := s.builds.UpdateBuild(ctx, build); err != nil {
		return domain.Build{}, err
	}

	wsDir, err := os.MkdirTemp("", "liapoldus-ws-*")
	if err != nil {
		return s.fail(ctx, build, "create temp workspace", err)
	}
	defer os.RemoveAll(wsDir)

	workspace, err := s.builder.Materialize(ctx, WorkspaceRequest{
		SiteID: build.SiteID, SnapshotID: build.SnapshotID,
		Environment: build.Environment, Dir: wsDir,
	})
	if err != nil {
		return s.fail(ctx, build, "materialize workspace", err)
	}

	build.Log = append(build.Log, "bundling")
	if err := s.builds.UpdateBuild(ctx, build); err != nil {
		return domain.Build{}, err
	}

	result, err := s.runner.Build(ctx, workspace)
	if err != nil {
		return s.fail(ctx, build, "esbuild", err)
	}

	build.Log = append(build.Log, "publishing artifacts")
	if err := s.builds.UpdateBuild(ctx, build); err != nil {
		return domain.Build{}, err
	}

	artifactDir, err := s.artifacts.Publish(ctx, build.SiteID, build.Environment, build.SnapshotID, workspace, result)
	if err != nil {
		return s.fail(ctx, build, "publish artifacts", err)
	}

	build.Status = domain.BuildStatusReady
	build.ArtifactDir = artifactDir
	build.FinishedAt = &now
	build.Log = append(build.Log, "build ready")
	if err := s.builds.UpdateBuild(ctx, build); err != nil {
		return domain.Build{}, err
	}
	return build, nil
}

// fail persists the failed status and returns the failed build with the
// wrapped domain.ErrBuildFailed so callers can map it onto an HTTP error.
func (s *Service) fail(ctx context.Context, build domain.Build, stage string, cause error) (domain.Build, error) {
	now := time.Now().UTC()
	build.Status = domain.BuildStatusFailed
	build.FinishedAt = &now
	build.Log = append(build.Log, fmt.Sprintf("%s failed: %v", stage, cause))
	_ = s.builds.UpdateBuild(ctx, build)
	return build, fmt.Errorf("%w: %s: %v", domain.ErrBuildFailed, stage, cause)
}

func (s *Service) Get(ctx context.Context, id string) (domain.Build, error) {
	return s.builds.GetBuild(ctx, id)
}

func (s *Service) ListBySite(ctx context.Context, siteID string) ([]domain.Build, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return nil, err
	}
	return s.builds.ListBuildsBySite(ctx, siteID)
}
