package unit

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/application/deps"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

func TestAddValidatesDeclaration(t *testing.T) {
	reg := &fakeRegistry{}
	svc, _ := newTestDepsService(t, reg)
	ctx := context.Background()

	for _, tc := range []struct{ name, spec string }{
		{"lodash", "latest"},   // tags rejected
		{"lodash", "*"},        // wildcards rejected
		{"lodash", "x"},        // wildcard alias rejected
		{"lodash", ""},         // empty spec
		{"lodash", "^a.beta"},  // not a valid range
		{"", "^1"},             // empty name
		{"Lodash", "^1"},       // uppercase name
		{"@scope/", "^1"},      // unterminated scope
		{"!bad", "^1"},         // punctuation
		{"lodash/extra", "^1"}, // slash only legal for scopes
	} {
		if _, err := svc.Add(ctx, "site-1", tc.name, tc.spec); !errors.Is(err, domain.ErrInvalidDepSpec) {
			t.Errorf("Add(%q,%q) error = %v, want ErrInvalidDepSpec", tc.name, tc.spec, err)
		}
	}
	if calls := reg.callCount(); calls != 0 {
		t.Fatalf("registry was called %d times during validation, want 0", calls)
	}
}

func TestAddScopedNameAccepted(t *testing.T) {
	reg := &fakeRegistry{resolve: func(_ context.Context, name, spec string) (deps.ResolvedVersion, error) {
		return deps.ResolvedVersion{Name: name, Version: "1.2.3"}, nil
	}}
	svc, _ := newTestDepsService(t, reg)
	resolved, err := svc.Add(context.Background(), "site-1", "@acme/core", "^1")
	if err != nil {
		t.Fatalf("Add scoped: %v", err)
	}
	if resolved.Version != "1.2.3" {
		t.Errorf("resolved version = %q, want 1.2.3", resolved.Version)
	}
}

func TestAddProbesRegistryAndStores(t *testing.T) {
	reg := &fakeRegistry{resolve: func(_ context.Context, name, spec string) (deps.ResolvedVersion, error) {
		if name == "lodash" && spec == "^4" {
			return deps.ResolvedVersion{Name: "lodash", Version: "4.17.21", Integrity: "sha512-a"}, nil
		}
		return deps.ResolvedVersion{}, domain.ErrPackageNotFound
	}}
	svc, db := newTestDepsService(t, reg)

	resolved, err := svc.Add(context.Background(), "site-1", "lodash", "^4")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if resolved.Version != "4.17.21" || resolved.Integrity != "sha512-a" {
		t.Errorf("resolved = %+v", resolved)
	}
	deps, err := svc.List(context.Background(), "site-1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(deps) != 1 || deps[0].Name != "lodash" || deps[0].Spec != "^4" {
		t.Errorf("stored deps = %+v", deps)
	}
	pkg, err := db.GetDepPackage(context.Background(), "lodash", "4.17.21")
	if err != nil {
		t.Fatalf("resolve should cache dep package: %v", err)
	}
	if pkg.Integrity != "sha512-a" {
		t.Errorf("cached package integrity = %q, want sha512-a", pkg.Integrity)
	}
}

func TestAddBestEffortOnTransientRegistryError(t *testing.T) {
	reg := &fakeRegistry{resolve: func(context.Context, string, string) (deps.ResolvedVersion, error) {
		return deps.ResolvedVersion{}, errors.New("network gone")
	}}
	svc, _ := newTestDepsService(t, reg)
	ctx := context.Background()

	resolved, err := svc.Add(ctx, "site-1", "lodash", "^4")
	if err != nil {
		t.Fatalf("Add with transient registry error should still store: %v", err)
	}
	if resolved.Version != "" {
		t.Errorf("resolved.Version = %q, want empty on transient error", resolved.Version)
	}
	deps, err := svc.List(ctx, "site-1")
	if err != nil || len(deps) != 1 {
		t.Fatalf("dependency not stored despite transient error: deps=%+v err=%v", deps, err)
	}
}

func TestAddAbortsOnPackageNotFound(t *testing.T) {
	reg := &fakeRegistry{resolve: func(context.Context, string, string) (deps.ResolvedVersion, error) {
		return deps.ResolvedVersion{}, domain.ErrPackageNotFound
	}}
	svc, _ := newTestDepsService(t, reg)
	ctx := context.Background()

	if _, err := svc.Add(ctx, "site-1", "does-not-exist-xyz", "^1"); !errors.Is(err, domain.ErrPackageNotFound) {
		t.Fatalf("Add unknown package error = %v, want ErrPackageNotFound", err)
	}
	deps, _ := svc.List(ctx, "site-1")
	if len(deps) != 0 {
		t.Errorf("unknown package was stored: %+v", deps)
	}
}

func TestAddUpdatesExisting(t *testing.T) {
	reg := &fakeRegistry{resolve: func(_ context.Context, _, spec string) (deps.ResolvedVersion, error) {
		return deps.ResolvedVersion{Name: "lodash", Version: spec}, nil
	}}
	svc, _ := newTestDepsService(t, reg)
	ctx := context.Background()

	if _, err := svc.Add(ctx, "site-1", "lodash", "^4"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Add(ctx, "site-1", "lodash", "4.0.0"); err != nil {
		t.Fatal(err)
	}
	deps, _ := svc.List(ctx, "site-1")
	if len(deps) != 1 || deps[0].Spec != "4.0.0" {
		t.Errorf("updated deps = %+v, want single lodash with spec 4.0.0", deps)
	}
}

func TestRemove(t *testing.T) {
	reg := &fakeRegistry{resolve: func(_ context.Context, name, _ string) (deps.ResolvedVersion, error) {
		return deps.ResolvedVersion{Name: name, Version: "1.0.0"}, nil
	}}
	svc, _ := newTestDepsService(t, reg)
	ctx := context.Background()

	if _, err := svc.Add(ctx, "site-1", "lodash", "^1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Remove(ctx, "site-1", "lodash"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	deps, _ := svc.List(ctx, "site-1")
	if len(deps) != 0 {
		t.Errorf("deps after remove = %+v, want empty", deps)
	}
}

func TestResolveLockEmpty(t *testing.T) {
	reg := &fakeRegistry{}
	svc, _ := newTestDepsService(t, reg)
	lock, err := svc.ResolveLock(context.Background(), "site-1")
	if err != nil {
		t.Fatalf("ResolveLock: %v", err)
	}
	if len(lock.Deps) != 0 {
		t.Errorf("lock.Deps = %+v, want empty", lock.Deps)
	}
	if calls := reg.callCount(); calls != 0 {
		t.Fatalf("registry called %d times for empty graph", calls)
	}
}

func TestResolveLockResolvesTransitives(t *testing.T) {
	reg := &fakeRegistry{resolve: func(_ context.Context, name, spec string) (deps.ResolvedVersion, error) {
		switch name {
		case "a":
			return deps.ResolvedVersion{Name: "a", Version: "1.0.0", Integrity: "sha512-a", Dependencies: map[string]string{"b": "^2"}}, nil
		case "b":
			return deps.ResolvedVersion{Name: "b", Version: "2.5.0", Integrity: "sha512-b"}, nil
		}
		return deps.ResolvedVersion{}, domain.ErrPackageNotFound
	}}
	svc, _ := newTestDepsService(t, reg)
	ctx := context.Background()
	if _, err := svc.Add(ctx, "site-1", "a", "^1"); err != nil {
		t.Fatal(err)
	}

	lock, err := svc.ResolveLock(ctx, "site-1")
	if err != nil {
		t.Fatalf("ResolveLock: %v", err)
	}
	want := []domain.LockedDep{
		{Name: "a", Spec: "^1", Version: "1.0.0", Integrity: "sha512-a", Hoisted: true, RequestedBy: []string{"site"}},
		{Name: "b", Spec: "^2", Version: "2.5.0", Integrity: "sha512-b", Hoisted: true, RequestedBy: []string{"a@1.0.0"}},
	}
	if len(lock.Deps) != len(want) {
		t.Fatalf("lock.Deps = %+v, want %+v", lock.Deps, want)
	}
	for i := range want {
		if !reflect.DeepEqual(lock.Deps[i], want[i]) {
			t.Errorf("lock.Deps[%d] = %+v, want %+v", i, lock.Deps[i], want[i])
		}
	}
}

func TestResolveLockReusesFrozenCompatibleVersion(t *testing.T) {
	var calls int
	reg := &fakeRegistry{resolve: func(_ context.Context, name, spec string) (deps.ResolvedVersion, error) {
		calls++
		switch name {
		case "t1":
			return deps.ResolvedVersion{Name: "t1", Version: "1.0.0", Dependencies: map[string]string{"b": "^1"}}, nil
		case "t2":
			return deps.ResolvedVersion{Name: "t2", Version: "1.5.0", Dependencies: map[string]string{"b": "^1"}}, nil
		case "b":
			return deps.ResolvedVersion{Name: "b", Version: "1.9.2"}, nil
		}
		return deps.ResolvedVersion{}, domain.ErrPackageNotFound
	}}
	svc, _ := newTestDepsService(t, reg)
	ctx := context.Background()
	for _, name := range []string{"t1", "t2"} {
		if _, err := svc.Add(ctx, "site-1", name, "^1"); err != nil {
			t.Fatal(err)
		}
	}

	// Add probes the registry (2 calls). Zero the counter so ResolveLock alone
	// reports how many resolutions the graph needs: t1, t2 and b exactly once.
	calls = 0

	lock, err := svc.ResolveLock(ctx, "site-1")
	if err != nil {
		t.Fatalf("ResolveLock: %v", err)
	}
	if len(lock.Deps) != 3 {
		t.Errorf("lock.Deps = %+v, want 3 entries (b deduped)", lock.Deps)
	}
	if calls != 3 {
		t.Errorf("registry resolved %d times, want 3 (b reused from cache)", calls)
	}
}

func TestResolveLockNestsConflictingVersions(t *testing.T) {
	var resolvedB []string
	reg := &fakeRegistry{resolve: func(_ context.Context, name, spec string) (deps.ResolvedVersion, error) {
		switch {
		case name == "c1":
			return deps.ResolvedVersion{Name: "c1", Version: "1.0.0", Dependencies: map[string]string{"b": "^1.0"}}, nil
		case name == "c2":
			return deps.ResolvedVersion{Name: "c2", Version: "2.0.0", Dependencies: map[string]string{"b": "^2.0"}}, nil
		case name == "b" && spec == "^1.0":
			resolvedB = append(resolvedB, "1.4.0")
			return deps.ResolvedVersion{Name: "b", Version: "1.4.0"}, nil
		case name == "b" && spec == "^2.0":
			resolvedB = append(resolvedB, "2.0.0")
			return deps.ResolvedVersion{Name: "b", Version: "2.0.0"}, nil
		}
		return deps.ResolvedVersion{}, domain.ErrPackageNotFound
	}}
	svc, _ := newTestDepsService(t, reg)
	ctx := context.Background()
	for _, name := range []string{"c1", "c2"} {
		if _, err := svc.Add(ctx, "site-1", name, "^1"); err != nil {
			t.Fatal(err)
		}
	}

	// Unlike the flat layout, an incompatible transitive range no longer fails
	// the whole graph: b resolves twice and the second version is nested under
	// the parent that requires it (RequestedBy keeps the edge).
	lock, err := svc.ResolveLock(ctx, "site-1")
	if err != nil {
		t.Fatalf("ResolveLock: %v", err)
	}
	want := []domain.LockedDep{
		{Name: "c1", Spec: "^1", Version: "1.0.0", Hoisted: true, RequestedBy: []string{"site"}},
		{Name: "c2", Spec: "^1", Version: "2.0.0", Hoisted: true, RequestedBy: []string{"site"}},
		{Name: "b", Spec: "^1.0", Version: "1.4.0", Hoisted: true, RequestedBy: []string{"c1@1.0.0"}},
		{Name: "b", Spec: "^2.0", Version: "2.0.0", Hoisted: false, RequestedBy: []string{"c2@2.0.0"}},
	}
	if len(lock.Deps) != len(want) {
		t.Fatalf("lock.Deps = %+v, want %+v", lock.Deps, want)
	}
	for i := range want {
		if !reflect.DeepEqual(lock.Deps[i], want[i]) {
			t.Errorf("lock.Deps[%d] = %+v, want %+v", i, lock.Deps[i], want[i])
		}
	}
	if got := len(resolvedB); got != 2 {
		t.Errorf("b resolved %d times, want 2 (one per incompatible range)", got)
	}
}

func TestResolveLockNestedInstanceResolvesItsOwnGraph(t *testing.T) {
	var count int
	reg := &fakeRegistry{resolve: func(_ context.Context, name, spec string) (deps.ResolvedVersion, error) {
		count++
		switch {
		case name == "a":
			return deps.ResolvedVersion{Name: "a", Version: "1.0.0", Dependencies: map[string]string{"b": "^1.0"}}, nil
		case name == "b" && spec == "^1.0":
			return deps.ResolvedVersion{Name: "b", Version: "1.0.0", Dependencies: map[string]string{"c": "^2"}}, nil
		case name == "b" && spec == "^2.0":
			return deps.ResolvedVersion{Name: "b", Version: "2.0.0", Dependencies: map[string]string{"c": "^2"}}, nil
		case name == "c" && spec == "^1":
			return deps.ResolvedVersion{Name: "c", Version: "1.0.0"}, nil
		case name == "c" && spec == "^2":
			return deps.ResolvedVersion{Name: "c", Version: "2.0.0"}, nil
		}
		return deps.ResolvedVersion{}, domain.ErrPackageNotFound
	}}
	svc, _ := newTestDepsService(t, reg)
	ctx := context.Background()
	// c@2 is required by both b instances; a site-level c@1 exists too, so the
	// b@2 subtree resolves its own c@2 instance.
	if _, err := svc.Add(ctx, "site-1", "a", "^1"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Add(ctx, "site-1", "b", "^2.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Add(ctx, "site-1", "c", "^1"); err != nil {
		t.Fatal(err)
	}

	lock, err := svc.ResolveLock(ctx, "site-1")
	if err != nil {
		t.Fatalf("ResolveLock: %v", err)
	}
	sawB2 := false
	for _, dep := range lock.Deps {
		if dep.Name == "b" && dep.Version == "2.0.0" {
			sawB2 = true
		}
		if dep.Name == "a" {
			if !reflect.DeepEqual(dep.RequestedBy, []string{"site"}) {
				t.Errorf("a RequestedBy = %v, want [site]", dep.RequestedBy)
			}
		}
	}
	if !sawB2 {
		t.Fatalf("lock.Deps missing the nested b@2.0.0 instance: %+v", lock.Deps)
	}
	if count != 8 {
		t.Errorf("registry resolved %d times, want 8 (a, b@1, b@2 from site, c@1, c@2 once deduped, plus three Add probes)", count)
	}
}

func TestResolveLockFailsBuildOnUnresolvable(t *testing.T) {
	reg := &fakeRegistry{resolve: func(_ context.Context, name, _ string) (deps.ResolvedVersion, error) {
		if name == "a" {
			return deps.ResolvedVersion{Name: "a", Version: "1.0.0", Dependencies: map[string]string{"b": "^7"}}, nil
		}
		return deps.ResolvedVersion{}, domain.ErrUnresolvableSpec
	}}
	svc, _ := newTestDepsService(t, reg)
	ctx := context.Background()
	if _, err := svc.Add(ctx, "site-1", "a", "^1"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.ResolveLock(ctx, "site-1"); !errors.Is(err, domain.ErrUnresolvableSpec) {
		t.Fatalf("ResolveLock error = %v, want ErrUnresolvableSpec", err)
	}
}
