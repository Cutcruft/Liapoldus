// Package sharedbuild regenerates the versioned shared runtime bundles that the
// Go server embeds and serves at /build/_shared/... (internal/infra/build/
// shared ./embed). It is the zero-node replacement for scripts/build-shared:
// react/react-dom/jsx-runtime are resolved and fetched straight from the npm
// registry over plain HTTP (no node/npm on the host), and the ui-runtime bundle
// is compiled from ui-runtime/src with Go esbuild.
//
// The generator is standalone by design: it wires its own infra (registry
// client + tarball store) and never touches the application/DB, so
// cmd/dependency-build can run it as a pure tool.
package sharedbuild

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/deps/registry"
)

// resolveSpec is a pinned npm range that enters the shared external graph.
type resolveSpec struct {
	name string
	spec string
}

// resolves maps every package that must be materialized into the workspace for
// the react family. react-dom drags scheduler; react and react/jsx-runtime are
// subpaths of the same react package.
var resolves = []resolveSpec{
	{name: "react", spec: "18.3.1"},
	{name: "react-dom", spec: "18.3.1"},
}

// edited edge of the BFS: a package range plus the instance that requested it
// (the verbatim "shared" marks a pinned generator root).
type resolveEdge struct {
	name        string
	spec        string
	requestedBy string
}

// resolveResult carries the frozen lock plus the registry resolutions for each
// instance — the tarball URL the materializer needs, which domain.LockedDep
// does not carry (the DB-backed layout reads it from dep_packages instead).
type resolveResult struct {
	lock domain.SnapshotLock
	// byKey maps "name@version" to the registry metadata used to fetch.
	byKey map[string]registry.ResolvedVersion
}

// resolveLock freezes the shared external graph by walking every range the
// react family declares, mirroring deps.Service.ResolveLock's BFS but without
// an allowlist or the DB: versions come straight from the packument cache, and
// each locked instance carries the integrity pinned at resolve time. Emission
// order is deterministic BFS (parents before children), matching the layout
// guarantees the site materializer relies on.
func resolveLock(ctx context.Context, client *registry.Client) (resolveResult, error) {
	instances := make(map[string]domain.LockedDep)
	perName := make(map[string][]string)
	byKey := make(map[string]registry.ResolvedVersion)
	var order []string

	queue := make([]resolveEdge, 0, len(resolves))
	for _, r := range resolves {
		queue = append(queue, resolveEdge{name: r.name, spec: r.spec, requestedBy: "shared"})
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Reuse the hoisted instance when its version satisfies this range
		// (as in ResolveLock); otherwise resolve a new version.
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

		resolved, err := client.Resolve(ctx, current.name, current.spec)
		if err != nil {
			return resolveResult{}, err
		}

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
		byKey[key] = resolved
		perName[current.name] = append(perName[current.name], key)
		order = append(order, key)

		names := make([]string, 0, len(resolved.Dependencies))
		for dep := range resolved.Dependencies {
			names = append(names, dep)
		}
		sort.Strings(names)
		for _, dep := range names {
			queue = append(queue, resolveEdge{name: dep, spec: resolved.Dependencies[dep], requestedBy: key})
		}
	}

	if err := checkPeers(instances, order); err != nil {
		return resolveResult{}, err
	}

	deps := make([]domain.LockedDep, 0, len(order))
	for _, key := range order {
		dep := instances[key]
		sort.Strings(dep.RequestedBy)
		deps = append(deps, dep)
	}
	return resolveResult{lock: domain.SnapshotLock{Deps: deps}, byKey: byKey}, nil
}

// checkPeers enforces the peer policy (spec §5, peer-fail): every required peer
// must be satisfiable by an instance already in the graph. The generator's root
// set (react/react-dom/scheduler) is self-consistent, so this is a guard that
// fails loudly if a future react bump drags in an unsatisfiable peer.
func checkPeers(instances map[string]domain.LockedDep, order []string) error {
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
				continue
			}
			peerRange := dep.PeerDependencies[peer]
			satisfied := false
			for _, otherKey := range order {
				other := instances[otherKey]
				if other.Name == peer && versionSatisfies(other.Version, peerRange) {
					satisfied = true
					break
				}
			}
			if !satisfied {
				return fmt.Errorf("shared lock: %s@%s requires peer %q (%s) that the graph does not satisfy",
					dep.Name, dep.Version, peer, peerRange)
			}
		}
	}
	return nil
}

func versionSatisfies(version, spec string) bool {
	constraint, err := semver.NewConstraint(strings.TrimSpace(spec))
	if err != nil {
		return false
	}
	v, err := semver.NewVersion(version)
	if err != nil {
		return false
	}
	return constraint.Check(v)
}

func appendUnique(list []string, value string) []string {
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}
