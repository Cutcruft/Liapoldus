package domain

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

var (
	// ErrPackageNotFound is returned by the registry fetcher when the package
	// does not exist (or its packument is not usable).
	ErrPackageNotFound = errors.New("package not found in npm registry")
	// ErrUnresolvableSpec means no published version satisfies the requested range.
	ErrUnresolvableSpec = errors.New("no published version satisfies the requested range")
	// ErrInvalidDepSpec means the declared specifier is not a valid npm range
	// (tags, "latest" and bare "*" are rejected).
	ErrInvalidDepSpec = errors.New("invalid dependency specifier")
	// ErrVersionConflict means two packages in the graph pin the same package to
	// different versions that cannot both satisfy all requesting specs.
	ErrVersionConflict = errors.New("conflicting versions in dependency graph")
	// ErrUnsatisfiedPeer means a locked instance declares a required peer
	// dependency (peerDependencies) that no instance in the graph and no fixed
	// shared external satisfies (peer policy, spec §5). Optional peers and
	// self-references are exempt.
	ErrUnsatisfiedPeer = errors.New("unsatisfied peer dependency")
	// ErrDepNotAllowed means a package that would enter the site`s dependency
	// graph matches none of the site's allowlist entries (allowlist policy,
	// spec §5). The check runs over the whole graph — top-level and transitive.
	ErrDepNotAllowed = errors.New("dependency is not allowed by the site allowlist")
	// ErrInvalidAllowlistEntry means the submitted allowlist entry is not a
	// legal npm name, scoped package or scope wildcard ("@scope/*", "*").
	ErrInvalidAllowlistEntry = errors.New("invalid allowlist entry")
	// ErrInvalidCacheConfig means the submitted per-site dependency cache limit
	// is not a legal value (must be positive and within the platform cap).
	ErrInvalidCacheConfig = errors.New("invalid dependency cache configuration")
)

var depNamePattern = regexp.MustCompile(`^(@[a-z0-9-~][a-z0-9-._~]*\/)?[a-z0-9-~][a-z0-9-._~]*$`)
var scopeNamePattern = regexp.MustCompile(`^[a-z0-9-~][a-z0-9-._~]*$`)

// ValidDependencyName reports whether name is a legal npm package name
// (optionally scoped). Tags/aliases are not names and are rejected upstream.
func ValidDependencyName(name string) bool {
	return len(name) > 0 && depNamePattern.MatchString(name)
}

// ValidAllowlistEntry reports whether an entry is a legal allowlist rule. The
// grammar is a subset of npm naming rules plus two wildcards: an exact package
// name ("lodash"), an exact scoped package ("@acme/core"), a scope wildcard
// ("@acme/*", matching every package in that scope) or the catch-all "*".
// Entries are normalized (trimmed, lowercased) by the application layer before
// validation, so callers must pass a normalized value.
func ValidAllowlistEntry(entry string) bool {
	if entry == "*" {
		return true
	}
	if strings.HasPrefix(entry, "@") && strings.HasSuffix(entry, "/*") {
		return scopeNamePattern.MatchString(strings.TrimSuffix(strings.TrimPrefix(entry, "@"), "/*"))
	}
	return depNamePattern.MatchString(entry)
}

// AllowlistEntryMatches reports whether an allowlist entry covers a package
// name. "*" covers everything; "@scope/*" covers every package in that scope;
// anything else must match the name exactly. name must be a valid package name.
func AllowlistEntryMatches(entry, name string) bool {
	if entry == "*" {
		return true
	}
	if strings.HasSuffix(entry, "/*") {
		return strings.HasPrefix(name, strings.TrimSuffix(entry, "/*")+"/")
	}
	return entry == name
}

// Dependency is a top-level npm dependency declared by an admin for a site.
// Spec is a semver range in npm notation ("^1.2.3", "~4.0", ">=1 <2");
// resolvedVersion carries what the last best-effort probe saw.
type Dependency struct {
	SiteID    string    `json:"siteId"`
	Name      string    `json:"name"`
	Spec      string    `json:"spec"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// LockedDep is one frozen dependency instance of a snapshot: the exact version
// chosen for a range plus its integrity (sha512), so the build is reproducible.
// A package may appear several times under the same Name when two parts of the
// graph pin incompatible ranges (nested-versioned layout, spec §5): the
// Hoisted instance lives at node_modules/<name> and every other instance is
// nested under each parent instance that requires it (RequestedBy lists the
// parent instance keys "name@version"; the verbatim value "site" marks a
// top-level declaration). Emission order is deterministic BFS (parents before
// children) so the layout reproduces the same physical placement.
type LockedDep struct {
	Name        string   `json:"name"`
	Spec        string   `json:"spec"`
	Version     string   `json:"version"`
	Integrity   string   `json:"integrity"`
	Hoisted     bool     `json:"hoisted,omitempty"`
	RequestedBy []string `json:"requestedBy,omitempty"`
	// PeerDependencies records this instance's declared peers with their ranges
	// (SBOM transparency, spec §5). PeerDependenciesMeta lists only the peers
	// marked optional in peerDependenciesMeta. Required peers are guaranteed by
	// the peer policy at lock time, so a lock only ever contains satisfiable
	// graphs.
	PeerDependencies     map[string]string `json:"peerDependencies,omitempty"`
	PeerDependenciesMeta map[string]bool   `json:"peerDependenciesMeta,omitempty"`
}

// InstanceKey returns the graph-unique key of a locked instance.
func (l LockedDep) InstanceKey() string { return l.Name + "@" + l.Version }

// SnapshotLock is the complete frozen dependency graph of a snapshot. It is a
// flat list (one version per package) and acts as the snapshot's SBOM.
type SnapshotLock struct {
	Deps []LockedDep `json:"deps"`
}

// DepPackage caches immutable registry metadata for one (name, version). The
// cache grows monotonically; entries are never rewritten. Dependencies maps a
// transitive package name to the range requested by this version's manifest.
type DepPackage struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Integrity    string            `json:"integrity"`
	TarballURL   string            `json:"tarballUrl"`
	Dependencies map[string]string `json:"dependencies"`
	// PeerDependencies / PeerDependenciesMeta mirror the manifest's peer
	// metadata so the peer policy and later audits can rely on the immutable
	// cache instead of re-fetching.
	PeerDependencies     map[string]string `json:"peerDependencies,omitempty"`
	PeerDependenciesMeta map[string]bool   `json:"peerDependenciesMeta,omitempty"`
	FetchedAt            time.Time         `json:"fetchedAt"`
}

// MaxDepsCacheBytes is the platform-wide upper bound for a single site's
// dependency tarball cache limit (channel 7, cache-limits/eviction). Values
// above it are rejected as invalid.
const MaxDepsCacheBytes = 8 << 30 // 8 GiB

// SiteCacheConfig carries the per-site dependency tarball cache limit
// (cache-limits/eviction, spec §11). A site without a configuration entry has
// no limit of its own; the effective limit of the shared cache is the maximum
// across all sites that have one. Empty config authorizes unlimited caching.
type SiteCacheConfig struct {
	SiteID       string `json:"siteId"`
	MaxDepsBytes int64  `json:"maxDepsBytes"`
}

// ValidCacheLimit reports whether a per-site dependency cache limit is legal.
func ValidCacheLimit(maxDepsBytes int64) bool {
	return maxDepsBytes > 0 && maxDepsBytes <= MaxDepsCacheBytes
}

// TarballAccess is one (name, version) tarball access stamp used to drive the
// LRU eviction of the shared on-disk cache. A more recent LastAccess keeps the
// tarball on disk longer; eviction removes the least recently used entries
// first until the shared cache fits its effective size limit.
type TarballAccess struct {
	Name       string    `json:"name"`
	Version    string    `json:"version"`
	LastAccess time.Time `json:"lastAccess"`
}
