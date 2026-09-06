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
// dependency into the frozen instance graph of a snapshot (nested-versioned
// layout, spec §5). One instance per (name, version) is emitted; the first
// instance created for a name is hoisted to node_modules/<name>, and a second
// instance of the same name — when a parent's range is not satisfied by any
// existing version — is nested under every parent instance that requires it
// (RequestedBy keeps those parent edges). The emission order is a
// deterministic BFS (parents before children) so the layout reproduces the
// same physical placement at build time. An edge whose range no published
// version satisfies still fails with ErrUnresolvableSpec.
func (s *Service) ResolveLock(ctx context.Context, siteID string) (domain.SnapshotLock, error) {
	s.resolveMu.Lock()
	defer s.resolveMu.Unlock()

	top, err := s.deps.ListDependenciesBySite(ctx, siteID)
	if err != nil {
		return domain.SnapshotLock{}, err
	}
	if len(top) == 0 {
		return domain.SnapshotLock{Deps: []domain.LockedDep{}}, nil
	}
	sort.Slice(top, func(i, j int) bool { return top[i].Name < top[j].Name })

	// instances keyed by name@version; perName keeps the deterministic
	// creation order per package (the first is the hoisted candidate).
	instances := make(map[string]domain.LockedDep, len(top))
	perName := make(map[string][]string, len(top))
	order := make([]string, 0, len(top)+8)

	queue := make([]edge, 0, len(top))
	for _, dep := range top {
		queue = append(queue, edge{name: dep.Name, spec: dep.Spec, requestedBy: "site"})
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Reuse an existing instance whose version already satisfies this
		// range (the hoisted one is always first in creation order); record
		// the new parent edge. Otherwise resolve a new version for the range
		// and nest it under the requesting parent.
		reused := false
		for _, key := range perName[current.name] {
			if instance, ok := instances[key]; ok && versionSatisfies(instance.Version, current.spec) {
				instance.RequestedBy = appendUnique(instance.RequestedBy, current.requestedBy)
				instances[key] = instance
				reused = true
				break
			}
		}
		if reused {
			continue
		}

		resolved, err := s.registry.Resolve(ctx, current.name, current.spec)
		if err != nil {
			return domain.SnapshotLock{}, err
		}
		s.cachePackage(ctx, resolved)

		key := current.name + "@" + resolved.Version
		dep := domain.LockedDep{
			Name:        current.name,
			Spec:        current.spec,
			Version:     resolved.Version,
			Integrity:   resolved.Integrity,
			Hoisted:     len(perName[current.name]) == 0,
			RequestedBy: []string{current.requestedBy},
		}
		instances[key] = dep
		perName[current.name] = append(perName[current.name], key)
		order = append(order, key)

		names := make([]string, 0, len(resolved.Dependencies))
		for dep := range resolved.Dependencies {
			names = append(names, dep)
		}
		sort.Strings(names)
		for _, dep := range names {
			queue = append(queue, edge{name: dep, spec: resolved.Dependencies[dep], requestedBy: key})
		}
	}

	deps := make([]domain.LockedDep, 0, len(order))
	for _, key := range order {
		dep := instances[key]
		sort.Strings(dep.RequestedBy)
		deps = append(deps, dep)
	}
	return domain.SnapshotLock{Deps: deps}, nil
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

func appendUnique(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}
