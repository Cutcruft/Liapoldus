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

// DepRef points a bare dependency specifier to its frozen, separately bundled
// artifact (dependency-service spec §11 шаг 4). Site bundles keep dependencies
// external; the manifest is the import-map source for the runtime shell.
type DepRef struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	// PublicArtifact is the artifact-relative path of the bundle, e.g.
	// "dist/_deps/lodash@4.17.21.js" (served at /build/<site>/<env>/<snap>/…).
	// For a CSS subpath specifier ("agent/styles.css") this is the JS stub the
	// import map maps the specifier to; the real stylesheet lives in CSSArtifact
	// and is loaded via <link> by the runtime shell.
	PublicArtifact string `json:"publicArtifact"`
	// CSSArtifact is the artifact-relative path of the stylesheet the dep's JS
	// graph reaches (spec §12): either the dep's combined CSS for the main
	// bundle ("dist/_deps/agent@1.0.0.css") or the per-subpath CSS bundle
	// ("dist/_deps/agent@1.0.0/styles.css"). The shell injects a <link> for it.
	CSSArtifact string `json:"cssArtifact,omitempty"`
	// Integrity is the sha512 of the source tarball the bundle was built from
	// (supply chain, spec §9).
	Integrity string `json:"integrity"`
}

// DepBuildError is the phase-1 failure mode for a dependency that cannot be
// bundled: a package importing node builtins, a missing module, or an esbuild
// failure. Hint carries actionable guidance for the site author.
type DepBuildError struct {
	Pkg     string
	Version string
	Missing string
	Hint    string
}

func (e *DepBuildError) Error() string {
	if e.Pkg == "" {
		return e.Hint
	}
	at := e.Pkg
	if e.Version != "" {
		at += "@" + e.Version
	}
	msg := at
	if e.Missing != "" {
		msg += ": cannot resolve " + e.Missing
	}
	if e.Hint != "" {
		msg += " (" + e.Hint + ")"
	}
	return msg
}

// DepLayoutRequest is the materialization target for a snapshot's frozen
// dependency lock. TopLevel lists the site's declared bare specifiers (the
// only ones that get their own _deps bundles); Subpaths lists the bare subpath
// specifiers the site code imports directly (e.g. "lodash/map"), restricted to
// top-level deps — each gets its own _deps/<name>@<ver>/<subpath>.js bundle and
// import-map entry. Every locked package (including transitives) is still laid
// out into node_modules/.
type DepLayoutRequest struct {
	Dir      string
	Lock     domain.SnapshotLock
	TopLevel []string
	Subpaths []string
}

// DepLayout is the outcome of materializing a snapshot lock: the import-map
// entries for the manifest and the bare external specifiers the site bundle
// must keep external.
type DepLayout struct {
	Deps      map[string]DepRef
	Externals []string
	// Styles is the deduped, sorted list of public CSS artifact paths the
	// manifest publishes for the runtime shell's <link> injection (spec §12).
	Styles []string
}

// DepLayouter fetches, verifies and unpacks every package of a frozen lock
// into the workspace's node_modules/, bundles the top-level deps into
// dist/_deps/ and returns the manifest import-map. Implemented by the infra
// deps materializer (spec §11 шаг 4).
type DepLayouter interface {
	MaterializeDeps(context.Context, DepLayoutRequest) (DepLayout, error)
}

// Manifest is the workspace contract between the materializer and the bundle
// runner (and later, the runtime). It is written to the workspace as
// manifest.json.
type Manifest struct {
	SiteID      string                   `json:"siteId"`
	SnapshotID  string                   `json:"snapshotId"`
	Environment string                   `json:"environment"`
	Definitions map[string]DefinitionRef `json:"definitions"`
	// Pages maps every snapshot page to its code-split chunk (relative to
	// dist/, i.e. "pages/<PageID>.js"); mount() dynamic-imports the current
	// page's chunk at navigation (spec §16).
	Pages     []PageRef `json:"pages"`
	Externals []string  `json:"externals"`
	// Shared is the import map of the versioned shared bundles (bare import
	// specifier → public URL); the boot shell turns it into a
	// <script type="importmap">.
	Shared map[string]string `json:"shared,omitempty"`
	// Deps is the import map for site dependencies (bare specifier → frozen
	// bundle artifact, published under this snapshot's dist/_deps/).
	Deps map[string]DepRef `json:"deps,omitempty"`
	// Styles lists the public CSS artifact paths to inject as <link rel=…>
	// before the site bundle mounts (spec §12): one per reachable stylesheet
	// of a dependency (combined per-bundle CSS or a bare CSS subpath).
	Styles []string `json:"styles,omitempty"`
	// HomePage is the chunk-targeted snapshot page computed by the same
	// heuristic as the runtime boot contract: the most specific renderPage
	// route referencing a snapshot page, else the first snapshot page. The
	// shell modulepreloads its chunk so the first screen has no JS round-trip.
	HomePage string `json:"homePage,omitempty"`
}

// PageRef links a snapshot page to its code-split chunk artifact (relative to
// dist/) and to the definition ids that chunk registers.
type PageRef struct {
	PageID      string   `json:"pageId"`
	Chunk       string   `json:"chunk"`
	Definitions []string `json:"definitions"`
}

// SharedExternals are the runtime libraries the site bundle does not include;
// they are served separately by the backend. react/jsx-runtime is included so
// esbuild's automatic JSX transform never inlines React into a site bundle.
var SharedExternals = []string{"react", "react-dom", "react/jsx-runtime", "@liapoldus/ui-runtime"}

// SharedResolver maps the bare import specifiers of the shared externals to
// the public URLs of their versioned bundles (the manifest import map).
type SharedResolver interface {
	SharedURLs() map[string]string
}

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

// DevRebuildEvent is pushed after every finished (re)build of a development
// workspace. Status is one of the domain BuildStatus "ready" / "failed"
// values; Runtime clients refresh the bundle at ArtifactDir on status ready.
type DevRebuildEvent struct {
	SiteID      string    `json:"siteId"`
	Environment string    `json:"environment"`
	SnapshotID  string    `json:"snapshotId"`
	ArtifactDir string    `json:"artifactDir,omitempty"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// DevEventHub is the broadcast channel between the dev rebuilder and live
// runtime clients (WS on the client server). Subscribe returns the event
// stream and an unsubscribe function; Current serves a late join relay.
type DevEventHub interface {
	Publish(DevRebuildEvent)
	Subscribe() (<-chan DevRebuildEvent, func())
	Current(siteID string) (DevRebuildEvent, bool)
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
	events    DevEventHub
}

func NewService(sites domain.SiteRepository, snapshots domain.SnapshotRepository,
	builds domain.BuildRepository, builder WorkspaceBuilder, runner BundleRunner,
	artifacts ArtifactStore, events ...DevEventHub) *Service {
	svc := &Service{sites: sites, snapshots: snapshots, builds: builds, builder: builder, runner: runner, artifacts: artifacts}
	if len(events) > 0 && events[0] != nil {
		svc.events = events[0]
	}
	return svc
}

// publish fans the build's current lifecycle state out to hub subscribers
// (admin `/api/builds/ws`). Optional: a nil hub is a no-op.
func (s *Service) publish(build domain.Build, buildErr string) {
	if s.events == nil {
		return
	}
	s.events.Publish(DevRebuildEvent{
		SiteID:      build.SiteID,
		Environment: build.Environment,
		SnapshotID:  build.SnapshotID,
		ArtifactDir: build.ArtifactDir,
		Status:      string(build.Status),
		Error:       buildErr,
		UpdatedAt:   time.Now().UTC(),
	})
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
	s.publish(build, "")

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
	s.publish(build, "")
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
	s.publish(build, fmt.Sprintf("%s: %v", stage, cause))
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

// Published returns the newest ready build of a site/environment — the boot
// release the runtime binds to when no versionId is given. ErrNotFound when
// nothing has been published for the environment yet.
func (s *Service) Published(ctx context.Context, siteID, environment string) (domain.Build, error) {
	if !validEnvironments()[environment] {
		return domain.Build{}, fmt.Errorf("%w: environment must be one of development, production", domain.ErrInvalidRequest)
	}
	all, err := s.builds.ListBuildsBySite(ctx, siteID)
	if err != nil {
		return domain.Build{}, err
	}
	var best domain.Build
	found := false
	for _, b := range all {
		if b.Environment != environment || b.Status != domain.BuildStatusReady {
			continue
		}
		if !found || b.CreatedAt.After(best.CreatedAt) {
			best, found = b, true
		}
	}
	if !found {
		return domain.Build{}, fmt.Errorf("%w: no published build for site %s environment %s", domain.ErrNotFound, siteID, environment)
	}
	return best, nil
}
