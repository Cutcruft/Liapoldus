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
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/shared"
)

// ResolvedVersion is the application-level view of a registry resolution:
// exact version + integrity (sha512) to pin in a lock.
type ResolvedVersion struct {
	Name                 string
	Version              string
	Integrity            string
	TarballURL           string
	Dependencies         map[string]string
	PeerDependencies     map[string]string
	PeerDependenciesMeta map[string]bool
}

// Registry resolves a package range to an exact version. Implementations must
// surface the domain sentinels ErrPackageNotFound / ErrUnresolvableSpec /
// ErrInvalidDepSpec so the service can tell user errors from transient ones.
type Registry interface {
	Resolve(ctx context.Context, name, spec string) (ResolvedVersion, error)
}

// TarballCache is the on-disk immutable tarball store the service evicts from
// when a per-site cache limit is configured (cache-limits/eviction, spec §11).
// It is optional: a nil TarballCache disables the on-disk eviction phase.
type TarballCache interface {
	Has(name, version string) bool
	Size(name, version string) int64
	Delete(name, version string) error
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
	tarballs  TarballCache
	resolveMu sync.Mutex
}

func NewService(deps domain.DependencyRepository, packages domain.DepPackageRepository, registry Registry) *Service {
	return &Service{deps: deps, packages: packages, registry: registry}
}

// WithTarballCache attaches the on-disk tarball store so the service can LRU-
// evict cached tarballs against per-site cache limits (cache-limits/eviction,
// spec §11). It is safe to call before the service is used; the store is
// optional and, when absent, eviction only records the access/limits.
func (s *Service) WithTarballCache(tarballs TarballCache) *Service {
	s.tarballs = tarballs
	return s
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
		if err := s.assertAllowed(ctx, siteID, name); err != nil {
			return ResolvedVersion{}, err
		}
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

// AddAllowlist persists one normalized allowlist entry for the site. Entries
// are trimmed, lowercased and validated — an npm name, a scoped package, a
// scope wildcard ("@scope/*") or the catch-all "*" (allowlist policy, spec §5).
// A duplicate entry fails with ErrAlreadyExists.
func (s *Service) AddAllowlist(ctx context.Context, siteID, entry string) error {
	entry = strings.ToLower(strings.TrimSpace(entry))
	if !domain.ValidAllowlistEntry(entry) {
		return fmt.Errorf("%w: %q (use an npm name, a scoped package, \"@scope/*\" or \"*\")", domain.ErrInvalidAllowlistEntry, entry)
	}
	return s.deps.AddAllowlist(ctx, siteID, entry)
}

// RemoveAllowlist deletes one allowlist entry (case-insensitive against the
// normalized stored value); unknown entries fail with ErrNotFound.
func (s *Service) RemoveAllowlist(ctx context.Context, siteID, entry string) error {
	return s.deps.RemoveAllowlist(ctx, siteID, strings.ToLower(strings.TrimSpace(entry)))
}

// ListAllowlist returns the site's allowlist entries, sorted.
func (s *Service) ListAllowlist(ctx context.Context, siteID string) ([]string, error) {
	return s.deps.ListAllowlist(ctx, siteID)
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
//
// After the graph is frozen, the peer policy (spec §5, peer-fail) runs over
// every instance: a required peer must be satisfiable — by an instance already
// in the graph whose version falls in the peer range, or by a fixed shared
// external (react/react-dom/react/jsx-runtime/ui-runtime) whose pinned version
// is in the range. Optional peers and self-references are exempt. An
// unsatisfiable peer fails lock creation with ErrUnsatisfiedPeer and names the
// consumer plus the offending range, so a snapshot never freezes a graph whose
// peers resolve wrong at build time.
//
// The allowlist policy (spec §5) runs over the whole graph: when the site has
// at least one allowlist entry, every package that would enter the lock —
// top-level or transitive — must match one of them, otherwise lock creation
// fails with ErrDepNotAllowed. An empty allowlist authorizes everything.
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
	allowlist, err := s.deps.ListAllowlist(ctx, siteID)
	if err != nil {
		return domain.SnapshotLock{}, err
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

		if !depsAllowed(current.name, allowlist) {
			return domain.SnapshotLock{}, fmt.Errorf("%w: %q matches none of the site's allowlist entries: %s",
				domain.ErrDepNotAllowed, current.name, strings.Join(allowlist, ", "))
		}

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
			Name:                 current.name,
			Spec:                 current.spec,
			Version:              resolved.Version,
			Integrity:            resolved.Integrity,
			Hoisted:              len(perName[current.name]) == 0,
			RequestedBy:          []string{current.requestedBy},
			PeerDependencies:     resolved.PeerDependencies,
			PeerDependenciesMeta: resolved.PeerDependenciesMeta,
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

	if err := checkPeers(instances, order); err != nil {
		return domain.SnapshotLock{}, err
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
		Name:                 resolved.Name,
		Version:              resolved.Version,
		Integrity:            resolved.Integrity,
		TarballURL:           resolved.TarballURL,
		Dependencies:         resolved.Dependencies,
		PeerDependencies:     resolved.PeerDependencies,
		PeerDependenciesMeta: resolved.PeerDependenciesMeta,
		FetchedAt:            time.Now().UTC(),
	}
	_ = s.packages.CreateDepPackage(ctx, pkg) // immutable cache: no-op on duplicates
}

// assertAllowed early-rejects a top-level declaration when the site's allowlist
// is active and the package matches none of its entries (allowlist policy,
// spec §5). Called from Add after a successful probe; the authoritative check
// still happens on ResolveLock, which also covers transitives.
func (s *Service) assertAllowed(ctx context.Context, siteID, name string) error {
	entries, err := s.deps.ListAllowlist(ctx, siteID)
	if err != nil {
		return err
	}
	if depsAllowed(name, entries) {
		return nil
	}
	return fmt.Errorf("%w: %q matches none of the site's allowlist entries: %s",
		domain.ErrDepNotAllowed, name, strings.Join(entries, ", "))
}

// depsAllowed applies the allowlist to a single package name. An empty entry
// list authorizes everything (backward compatible); otherwise the name must
// match at least one entry.
func depsAllowed(name string, entries []string) bool {
	if len(entries) == 0 {
		return true
	}
	for _, entry := range entries {
		if domain.AllowlistEntryMatches(entry, name) {
			return true
		}
	}
	return false
}

// checkPeers enforces the peer policy (spec §5, peer-fail). It walks the full
// deterministic instance order — so a provider discovered later in the graph
// still counts — and for every required peer either finds an instance in the
// graph whose version is inside the peer range or a fixed shared external
// whose pinned version matches. The check is name/version level: the physical
// reachability of the satisfier from the consumer position is left to the
// nested-versioned layout's walk-up resolution at build time.
func checkPeers(instances map[string]domain.LockedDep, order []string) error {
	sharedVersions := shared.Versions()
	for _, key := range order {
		dep := instances[key]
		if len(dep.PeerDependencies) == 0 {
			continue
		}
		names := make([]string, 0, len(dep.PeerDependencies))
		for name := range dep.PeerDependencies {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, peer := range names {
			if dep.PeerDependenciesMeta[peer] || peer == dep.Name {
				continue // optional peer / self-reference: exempt
			}
			peerRange := dep.PeerDependencies[peer]

			present := make(map[string]bool) // dedupes the "could have served" list
			satisfied := false
			for _, otherKey := range order {
				other := instances[otherKey]
				if other.Name != peer {
					continue
				}
				if versionSatisfies(other.Version, peerRange) {
					satisfied = true
					break
				}
				present[other.Version] = true
			}
			if !satisfied {
				if sharedVersion, ok := sharedVersions[peer]; ok {
					present[sharedVersion+" (shared)"] = true
					satisfied = versionSatisfies(sharedVersion, peerRange)
				}
			}
			if satisfied {
				continue
			}

			versions := make([]string, 0, len(present))
			for v := range present {
				versions = append(versions, v)
			}
			sort.Strings(versions)
			detail := "none of its versions are present"
			if len(versions) > 0 {
				detail = "present versions: " + strings.Join(versions, ", ")
			}
			return fmt.Errorf("%w: %s@%s requires peer %q (%s); %s",
				domain.ErrUnsatisfiedPeer, dep.Name, dep.Version, peer, peerRange, detail)
		}
	}
	return nil
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

// SetCacheConfig persists a per-site dependency tarball cache limit
// (cache-limits/eviction, spec §11). The value must be positive and within the
// platform cap (domain.ValidCacheLimit), otherwise ErrInvalidCacheConfig.
func (s *Service) SetCacheConfig(ctx context.Context, siteID string, maxDepsBytes int64) error {
	if !domain.ValidCacheLimit(maxDepsBytes) {
		return fmt.Errorf("%w: %d (must be in (0, %d])", domain.ErrInvalidCacheConfig, maxDepsBytes, domain.MaxDepsCacheBytes)
	}
	return s.deps.SetCacheConfig(ctx, domain.SiteCacheConfig{SiteID: siteID, MaxDepsBytes: maxDepsBytes})
}

// GetCacheConfig returns the per-site dependency cache limit and whether the
// site has one configured.
func (s *Service) GetCacheConfig(ctx context.Context, siteID string) (domain.SiteCacheConfig, bool, error) {
	return s.deps.GetCacheConfig(ctx, siteID)
}

// EffectiveCacheLimit returns the effective limit of the shared tarball cache:
// the maximum max_deps_bytes over all sites that configured one. The second
// return is false when no site has a limit (unlimited cache).
func (s *Service) EffectiveCacheLimit(ctx context.Context) (int64, bool, error) {
	configs, err := s.deps.ListCacheConfigs(ctx)
	if err != nil {
		return 0, false, err
	}
	var max int64
	found := false
	for _, cfg := range configs {
		if !found || cfg.MaxDepsBytes > max {
			max = cfg.MaxDepsBytes
			found = true
		}
	}
	return max, found, nil
}

// EvictTarballs runs one LRU-eviction pass against the effective cache limit.
// Cached tarballs are removed oldest-access-first until the sum of the retained
// tarball sizes is at or under the limit. It is a no-op when no site has a
// limit or no tarball store is attached. It returns the bytes evicted and the
// number of blobs removed.
func (s *Service) EvictTarballs(ctx context.Context) (evictedBytes int64, evicted int, err error) {
	limit, limited, err := s.EffectiveCacheLimit(ctx)
	if err != nil {
		return 0, 0, err
	}
	if !limited || s.tarballs == nil {
		return 0, 0, nil
	}
	access, err := s.deps.ListTarballAccess(ctx)
	if err != nil {
		return 0, 0, err
	}
	var total int64
	for _, acc := range access {
		total += s.tarballs.Size(acc.Name, acc.Version)
	}
	if total <= limit {
		return 0, 0, nil
	}
	// ListTarballAccess is oldest-first, so walk from the front: the least
	// recently used tarball gets removed first, until the retained total is at
	// or under the limit.
	for _, acc := range access {
		if total <= limit {
			break
		}
		size := s.tarballs.Size(acc.Name, acc.Version)
		if size == 0 || !s.tarballs.Has(acc.Name, acc.Version) {
			continue
		}
		if err := s.tarballs.Delete(acc.Name, acc.Version); err != nil {
			return evictedBytes, evicted, err
		}
		total -= size
		evictedBytes += size
		evicted++
	}
	return evictedBytes, evicted, nil
}

// ManualEvictTarballs runs an explicit LRU-eviction pass down to a target
// retained size (cache-limits/eviction, spec §11). A non-positive targetBytes
// falls back to the effective limit (max over sites; smoke no-op when no site
// has a limit). Only the on-disk tarball blobs are deleted; the DB metadata
// (dep_packages, access records) is left intact so packages are re-fetched on
// the next build. Returns the bytes evicted and the number of blobs removed.
func (s *Service) ManualEvictTarballs(ctx context.Context, targetBytes int64) (evictedBytes int64, evicted int, err error) {
	if s.tarballs == nil {
		return 0, 0, nil
	}
	limit := targetBytes
	if limit <= 0 {
		var ok bool
		limit, ok, err = s.EffectiveCacheLimit(ctx)
		if err != nil {
			return 0, 0, err
		}
		if !ok || limit <= 0 {
			return 0, 0, nil
		}
	}
	access, err := s.deps.ListTarballAccess(ctx)
	if err != nil {
		return 0, 0, err
	}
	var total int64
	for _, acc := range access {
		total += s.tarballs.Size(acc.Name, acc.Version)
	}
	if total <= limit {
		return 0, 0, nil
	}
	for _, acc := range access {
		if total <= limit {
			break
		}
		size := s.tarballs.Size(acc.Name, acc.Version)
		if size == 0 || !s.tarballs.Has(acc.Name, acc.Version) {
			continue
		}
		if err := s.tarballs.Delete(acc.Name, acc.Version); err != nil {
			return evictedBytes, evicted, err
		}
		total -= size
		evictedBytes += size
		evicted++
	}
	return evictedBytes, evicted, nil
}

// StartCacheEviction runs the periodic LRU-eviction sweep on an interval until
// the context is cancelled (cache-limits/eviction, spec §11). A non-positive
// interval disables the periodic tick; on the context the function returns.
// Intended to run as a single long-lived goroutine from the server entrypoint.
func (s *Service) StartCacheEviction(ctx context.Context, interval time.Duration, onEvict func(evictedBytes int64, evicted int)) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			evictedBytes, evicted, err := s.EvictTarballs(ctx)
			if err != nil {
				continue // leave error handling to the next tick; no logger here
			}
			if evicted > 0 && onEvict != nil {
				onEvict(evictedBytes, evicted)
			}
		}
	}
}
