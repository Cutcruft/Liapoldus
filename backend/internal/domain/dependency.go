package domain

import (
	"errors"
	"regexp"
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
)

var depNamePattern = regexp.MustCompile(`^(@[a-z0-9-~][a-z0-9-._~]*\/)?[a-z0-9-~][a-z0-9-._~]*$`)

// ValidDependencyName reports whether name is a legal npm package name
// (optionally scoped). Tags/aliases are not names and are rejected upstream.
func ValidDependencyName(name string) bool {
	return len(name) > 0 && depNamePattern.MatchString(name)
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
	FetchedAt    time.Time         `json:"fetchedAt"`
}
