// Package deps is the application service for third-party npm dependencies.
// Admins declare top-level specs per site; the service validates them, probes
// the registry best-effort and resolves a frozen, flat lock per snapshot.
package deps

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// ResolvedVersion is the application-level view of a registry resolution:
// exact version + integrity (sha512) to pin in a lock.
type ResolvedVersion struct {
	Name         string
	Version      string
	Integrity    string
	TarballURL   string
	Dependencies map[string]string
}

// Registry resolves a package range to an exact version. Implementations must
// surface the domain sentinels ErrPackageNotFound / ErrUnresolvableSpec /
// ErrInvalidDepSpec so the service can tell user errors from transient ones.
type Registry interface {
	Resolve(ctx context.Context, name, spec string) (ResolvedVersion, error)
}

// LockResolver is implemented by Service so snapshot creation can freeze the
// dependency lock ("lock в снапшот", spec §7).
type LockResolver interface {
	ResolveLock(ctx context.Context, siteID string) (domain.SnapshotLock, error)
}

type Service struct {
	deps      domain.DependencyRepository
	packages  domain.DepPackageRepository
	registry  Registry
	resolveMu sync.Mutex
}

func NewService(deps domain.DependencyRepository, packages domain.DepPackageRepository, registry Registry) *Service {
	return &Service{deps: deps, packages: packages, registry: registry}
}

type edge struct {
	name        string
	spec        string
	requestedBy string
}

// Add validates the declaration, probes the registry best-effort and upserts
// the top-level dependency. User errors (invalid name/spec, unknown package,
// range nothing satisfies) abort the write; transient registry failures only
// lose the informational resolvedVersion, the declaration is still stored.
func (s *Service) Add(ctx context.Context, siteID, name, spec string) (ResolvedVersion, error) {
	if err := validateName(name); err != nil {
		return ResolvedVersion{}, err
	}
	spec = strings.TrimSpace(spec)
	if err := validateSpec(spec); err != nil {
		return ResolvedVersion{}, err
	}

	var resolved ResolvedVersion
	if probe, err := s.registry.Resolve(ctx, name, spec); err == nil {
		resolved = probe
		s.cachePackage(ctx, probe)
	} else if isUserError(err) {
		return ResolvedVersion{}, err
	} // transient failure: best-effort, resolve later at snapshot time

	now := time.Now().UTC()
	existing, err := s.deps.GetDependency(ctx, siteID, name)
	if err == nil {
		existing.Spec = spec
		existing.UpdatedAt = now
		return resolved, s.deps.UpdateDependency(ctx, existing)
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return ResolvedVersion{}, err
	}
	dep := domain.Dependency{SiteID: siteID, Name: name, Spec: spec, CreatedAt: now, UpdatedAt: now}
	if err := s.deps.CreateDependency(ctx, dep); err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			// Lost a race: treat as an update.
			existing, getErr := s.deps.GetDependency(ctx, siteID, name)
			if getErr != nil {
				return ResolvedVersion{}, getErr
			}
			existing.Spec = spec
			existing.UpdatedAt = now
			return resolved, s.deps.UpdateDependency(ctx, existing)
		}
		return ResolvedVersion{}, err
	}
	return resolved, nil
}

func (s *Service) Remove(ctx context.Context, siteID, name string) error {
	return s.deps.DeleteDependency(ctx, siteID, name)
}

func (s *Service) List(ctx context.Context, siteID string) ([]domain.Dependency, error) {
	return s.deps.ListDependenciesBySite(ctx, siteID)
}

// ResolveLock resolves the site's top-level declarations and every transitive
// dependency into a flat frozen lock (one version per package). Two packages
// pinning the same name to incompatible versions fail with ErrVersionConflict;
// nested versioned layouts arrive in phase 2.
func (s *Service) ResolveLock(ctx context.Context, siteID string) (domain.SnapshotLock, error) {
	s.resolveMu.Lock()
	defer s.resolveMu.Unlock()

	top, err := s.deps.ListDependenciesBySite(ctx, siteID)
	if err != nil {
		return domain.SnapshotLock{}, err
	}
	lock := domain.SnapshotLock{Deps: []domain.LockedDep{}}
	if len(top) == 0 {
		return lock, nil
	}

	frozen := make(map[string]domain.LockedDep, len(top))
	queue := make([]edge, 0, len(top))
	for _, dep := range top {
		queue = append(queue, edge{name: dep.Name, spec: dep.Spec, requestedBy: "site"})
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if existing, ok := frozen[current.name]; ok && versionSatisfies(existing.Version, current.spec) {
			continue // already frozen and compatible, reuse
		}

		resolved, err := s.registry.Resolve(ctx, current.name, current.spec)
		if err != nil {
			return domain.SnapshotLock{}, err
		}
		s.cachePackage(ctx, resolved)

		if existing, ok := frozen[current.name]; ok && existing.Version != resolved.Version {
			return domain.SnapshotLock{}, fmt.Errorf(
				"%w: %s requires %s@%s but the graph already pins %s@%s",
				domain.ErrVersionConflict, current.requestedBy, current.name, current.spec, current.name, existing.Version)
		}
		next := domain.LockedDep{Name: current.name, Spec: current.spec, Version: resolved.Version, Integrity: resolved.Integrity}
		frozen[current.name] = next

		names := make([]string, 0, len(resolved.Dependencies))
		for dep := range resolved.Dependencies {
			names = append(names, dep)
		}
		sort.Strings(names)
		for _, dep := range names {
			queue = append(queue, edge{name: dep, spec: resolved.Dependencies[dep], requestedBy: current.name})
		}
	}

	lock.Deps = lockList(frozen)
	return lock, nil
}

func (s *Service) cachePackage(ctx context.Context, resolved ResolvedVersion) {
	pkg := domain.DepPackage{
		Name:         resolved.Name,
		Version:      resolved.Version,
		Integrity:    resolved.Integrity,
		TarballURL:   resolved.TarballURL,
		Dependencies: resolved.Dependencies,
		FetchedAt:    time.Now().UTC(),
	}
	_ = s.packages.CreateDepPackage(ctx, pkg) // immutable cache: no-op on duplicates
}

func validateName(name string) error {
	if !domain.ValidDependencyName(name) {
		return fmt.Errorf("%w: %q is not a valid npm package name", domain.ErrInvalidDepSpec, name)
	}
	return nil
}

func validateSpec(spec string) error {
	if spec == "" || spec == "*" || spec == "x" || spec == "X" {
		return fmt.Errorf("%w: %q (tags, latest and wildcards are rejected)", domain.ErrInvalidDepSpec, spec)
	}
	if _, err := semver.NewConstraint(spec); err != nil {
		return fmt.Errorf("%w: %q is not a valid npm range (%v)", domain.ErrInvalidDepSpec, spec, err)
	}
	return nil
}

func isUserError(err error) bool {
	return errors.Is(err, domain.ErrPackageNotFound) ||
		errors.Is(err, domain.ErrUnresolvableSpec) ||
		errors.Is(err, domain.ErrInvalidDepSpec)
}

func versionSatisfies(version, spec string) bool {
	constraint, err := semver.NewConstraint(strings.TrimSpace(spec))
	if err != nil {
		return false
	}
	if v, err := semver.NewVersion(version); err == nil && v != nil {
		return constraint.Check(v)
	}
	return false
}

func lockList(frozen map[string]domain.LockedDep) []domain.LockedDep {
	list := make([]domain.LockedDep, 0, len(frozen))
	for _, locked := range frozen {
		list = append(list, locked)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}
