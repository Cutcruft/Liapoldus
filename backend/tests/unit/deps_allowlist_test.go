package unit

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/api/admin"
	"github.com/liapoldus/liapoldus/backend/internal/application/deps"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

func TestValidAllowlistEntry(t *testing.T) {
	cases := []struct {
		entry string
		want  bool
	}{
		{"lodash", true},
		{"lodash._reinterpolate", true},
		{"@acme/core", true},
		{"@acme/*", true},
		{"*", true},
		{"react-dom/client", false},  // subpath, not a package
		{"@scope", false},            // bare scope without package
		{"@scope/name/extra", false}, // deeper than one level
		{"", false},                  // empty
		{"lodash/map", false},        // subpath
		{"UPPER", false},             // uppercased (service normalizes first)
		{"@Acme/*", false},           // scope must be lowercase in a wildcard
		{"@acme/Core", false},        // package part must be lowercased first
	}
	for _, tc := range cases {
		if got := domain.ValidAllowlistEntry(tc.entry); got != tc.want {
			t.Errorf("ValidAllowlistEntry(%q) = %v, want %v", tc.entry, got, tc.want)
		}
	}
}

func TestAllowlistEntryMatches(t *testing.T) {
	cases := []struct {
		entry string
		name  string
		want  bool
	}{
		{"*", "lodash", true},
		{"*", "@acme/core", true},
		{"lodash", "lodash", true},
		{"lodash", "lodash.map", false}, // different package
		{"lodash", "lodash/map", false}, // name is a package, subpaths normalize away
		{"@acme/*", "@acme/core", true},
		{"@acme/*", "@acme/ui-runtime", true},
		{"@acme/*", "@acmeui/runtime", false}, // different scope
		{"@acme/*", "acme/core", false},
		{"@acme/core", "@acme/core", true},
		{"@acme/core", "@acme/other", false},
	}
	for _, tc := range cases {
		if got := domain.AllowlistEntryMatches(tc.entry, tc.name); got != tc.want {
			t.Errorf("AllowlistEntryMatches(%q, %q) = %v, want %v", tc.entry, tc.name, got, tc.want)
		}
	}
}

// aDepRegistry serves a top-level "a" that depends on transitve "b".
func aDepRegistry() *fakeRegistry {
	return &fakeRegistry{resolve: func(_ context.Context, name, spec string) (deps.ResolvedVersion, error) {
		switch name {
		case "a":
			return deps.ResolvedVersion{Name: "a", Version: "1.0.0", Integrity: "sha512-a", Dependencies: map[string]string{"b": "^1"}}, nil
		case "b":
			return deps.ResolvedVersion{Name: "b", Version: "1.0.0", Integrity: "sha512-b"}, nil
		}
		return deps.ResolvedVersion{}, domain.ErrPackageNotFound
	}}
}

func TestAddAllowlistNormalizesAndFailsInvalid(t *testing.T) {
	svc, _ := newTestDepsService(t, aDepRegistry())
	ctx := context.Background()

	if err := svc.AddAllowlist(ctx, "site-1", "  Lodash "); err != nil {
		t.Fatalf("AddAllowlist(trimmed+lowercased): %v", err)
	}
	entries, err := svc.ListAllowlist(ctx, "site-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0] != "lodash" {
		t.Errorf("entries = %+v, want [lodash] (normalized)", entries)
	}

	if err := svc.AddAllowlist(ctx, "site-1", "lodash"); !errors.Is(err, domain.ErrAlreadyExists) {
		t.Errorf("duplicate AddAllowlist error = %v, want ErrAlreadyExists", err)
	}
	for _, bad := range []string{"", "@scope", "lodash/map", "@scope/name/extra"} {
		if err := svc.AddAllowlist(ctx, "site-1", bad); !errors.Is(err, domain.ErrInvalidAllowlistEntry) {
			t.Errorf("AddAllowlist(%q) error = %v, want ErrInvalidAllowlistEntry", bad, err)
		}
	}
	if _, err := svc.ListAllowlist(ctx, "site-2"); err != nil || len(entries) == 0 {
		t.Errorf("ListAllowlist(other site) = %v, err = %v; allowlist must be per-site", entries, err)
	}
}

func TestResolveLockAllowlistEmptyAllowsAll(t *testing.T) {
	svc, _ := newTestDepsService(t, aDepRegistry())
	ctx := context.Background()
	addDeps(t, svc, "site-1", "a")

	lock, err := svc.ResolveLock(ctx, "site-1")
	if err != nil {
		t.Fatalf("ResolveLock with empty allowlist: %v", err)
	}
	if len(lock.Deps) != 2 { // a + transitive b
		t.Errorf("lock.Deps = %+v, want a + b", lock.Deps)
	}
}

func TestResolveLockAllowlistStarAllowsAll(t *testing.T) {
	svc, _ := newTestDepsService(t, aDepRegistry())
	ctx := context.Background()
	if err := svc.AddAllowlist(ctx, "site-1", "*"); err != nil {
		t.Fatal(err)
	}
	addDeps(t, svc, "site-1", "a")

	if _, err := svc.ResolveLock(ctx, "site-1"); err != nil {
		t.Fatalf("ResolveLock with \"*\" allowlist: %v", err)
	}
}

func TestResolveLockAllowlistFailsVertexNotAllowed(t *testing.T) {
	svc, _ := newTestDepsService(t, aDepRegistry())
	ctx := context.Background()
	addDeps(t, svc, "site-1", "a")
	// Narrow the allowlist to only "b": "a" was declared before, so the POST
	// early-reject is bypassed — the authoritative ResolveLock check applies.
	if err := svc.AddAllowlist(ctx, "site-1", "b"); err != nil {
		t.Fatal(err)
	}

	_, err := svc.ResolveLock(ctx, "site-1")
	if !errors.Is(err, domain.ErrDepNotAllowed) {
		t.Fatalf("ResolveLock error = %v, want ErrDepNotAllowed", err)
	}
	if msg := err.Error(); !strings.Contains(msg, `"a"`) || !strings.Contains(msg, "b") {
		t.Errorf("error = %q, want it to name the offending package a and the rule b", msg)
	}
}

func TestResolveLockAllowlistFailsTransitiveNotAllowed(t *testing.T) {
	svc, _ := newTestDepsService(t, aDepRegistry())
	ctx := context.Background()
	addDeps(t, svc, "site-1", "a")
	if err := svc.AddAllowlist(ctx, "site-1", "a"); err != nil {
		t.Fatal(err)
	}

	// The vertex "a" is allowed but its transitive "b" is not: the policy
	// covers the whole graph, so the lock must fail naming "b".
	_, err := svc.ResolveLock(ctx, "site-1")
	if !errors.Is(err, domain.ErrDepNotAllowed) {
		t.Fatalf("ResolveLock error = %v, want ErrDepNotAllowed on transitive", err)
	}
	if msg := err.Error(); !strings.Contains(msg, `"b"`) {
		t.Errorf("error = %q, want it to name the transitive package b", msg)
	}
}

func TestResolveLockAllowlistScopeWildcard(t *testing.T) {
	reg := &fakeRegistry{resolve: func(_ context.Context, name, _ string) (deps.ResolvedVersion, error) {
		switch name {
		case "@acme/kit":
			return deps.ResolvedVersion{Name: name, Version: "1.0.0", Integrity: "sha512-k"}, nil
		case "@other/x":
			return deps.ResolvedVersion{Name: name, Version: "1.0.0", Integrity: "sha512-x"}, nil
		}
		return deps.ResolvedVersion{}, domain.ErrPackageNotFound
	}}
	svc, _ := newTestDepsService(t, reg)
	ctx := context.Background()
	if err := svc.AddAllowlist(ctx, "site-1", "@acme/*"); err != nil {
		t.Fatal(err)
	}
	addDeps(t, svc, "site-1", "@acme/kit")

	if _, err := svc.ResolveLock(ctx, "site-1"); err != nil {
		t.Fatalf("ResolveLock with @acme/* allowlist: %v", err)
	}
}

func TestAddAllowlistEarlyRejectsNotAllowed(t *testing.T) {
	svc, _ := newTestDepsService(t, aDepRegistry())
	ctx := context.Background()

	if err := svc.AddAllowlist(ctx, "site-1", "b"); err != nil {
		t.Fatal(err)
	}
	// "a" is not allowed; the POST-time probe resolves it and the service
	// rejects the declaration immediately instead of storing it.
	if _, err := svc.Add(ctx, "site-1", "a", "^1"); !errors.Is(err, domain.ErrDepNotAllowed) {
		t.Fatalf("Add(a) with b-only allowlist error = %v, want ErrDepNotAllowed", err)
	}
	declared, err := svc.List(ctx, "site-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(declared) != 0 {
		t.Errorf("declared deps = %+v, want none (rejected before write)", declared)
	}
}

func TestDepsAllowlistAdminCRUD(t *testing.T) {
	reg := &fakeRegistry{resolve: func(_ context.Context, name, _ string) (deps.ResolvedVersion, error) {
		if name == "lodash" {
			return deps.ResolvedVersion{Name: name, Version: "4.17.21", Integrity: "sha512-x"}, nil
		}
		return deps.ResolvedVersion{}, domain.ErrPackageNotFound
	}}
	app, _, siteID := newDepsApp(t, reg)
	handler := admin.NewRouter(app)

	add := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/dependencies/allowlist", map[string]any{"entry": "lodash"})
	if add.Code != http.StatusCreated {
		t.Fatalf("POST allowlist status = %d (%s)", add.Code, add.Body.String())
	}

	dup := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/dependencies/allowlist", map[string]any{"entry": "lodash"})
	if dup.Code != http.StatusConflict {
		t.Fatalf("duplicate POST allowlist status = %d, want 409", dup.Code)
	}

	bad := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/dependencies/allowlist", map[string]any{"entry": "@scope"})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("invalid POST allowlist status = %d, want 400", bad.Code)
	}

	list := request(t, handler, http.MethodGet, "/api/sites/"+siteID+"/dependencies/allowlist", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("GET allowlist status = %d (%s)", list.Code, list.Body.String())
	}
	var entries []string
	decodeResponse(t, list, &entries)
	if len(entries) != 1 || entries[0] != "lodash" {
		t.Errorf("allowlist entries = %+v, want [lodash]", entries)
	}

	del := request(t, handler, http.MethodDelete, "/api/sites/"+siteID+"/dependencies/allowlist?entry=lodash", nil)
	if del.Code != http.StatusNoContent {
		t.Fatalf("DELETE allowlist status = %d (%s)", del.Code, del.Body.String())
	}
	delAgain := request(t, handler, http.MethodDelete, "/api/sites/"+siteID+"/dependencies/allowlist?entry=lodash", nil)
	if delAgain.Code != http.StatusNotFound {
		t.Fatalf("DELETE missing allowlist status = %d, want 404", delAgain.Code)
	}

	list = request(t, handler, http.MethodGet, "/api/sites/"+siteID+"/dependencies/allowlist", nil)
	decodeResponse(t, list, &entries)
	if len(entries) != 0 {
		t.Errorf("allowlist entries after delete = %+v, want empty", entries)
	}
}

func TestDepsAdminCreateRejectsWhenNotAllowed(t *testing.T) {
	reg := &fakeRegistry{resolve: func(_ context.Context, name, _ string) (deps.ResolvedVersion, error) {
		if name == "lodash" {
			return deps.ResolvedVersion{Name: name, Version: "4.17.21", Integrity: "sha512-x"}, nil
		}
		return deps.ResolvedVersion{}, domain.ErrPackageNotFound
	}}
	app, _, siteID := newDepsApp(t, reg)
	handler := admin.NewRouter(app)

	add := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/dependencies/allowlist", map[string]any{"entry": "@only/*"})
	if add.Code != http.StatusCreated {
		t.Fatalf("POST allowlist status = %d (%s)", add.Code, add.Body.String())
	}
	create := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/dependencies", map[string]any{"name": "lodash", "spec": "^4"})
	if create.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST deps status = %d, want 422 when package not allowed", create.Code)
	}
}

func TestDepsAllowlistAdminScopedDelete(t *testing.T) {
	reg := &fakeRegistry{resolve: func(_ context.Context, name, _ string) (deps.ResolvedVersion, error) {
		if name == "lodash" {
			return deps.ResolvedVersion{Name: name, Version: "4.17.21", Integrity: "sha512-x"}, nil
		}
		return deps.ResolvedVersion{}, domain.ErrPackageNotFound
	}}
	app, _, siteID := newDepsApp(t, reg)
	handler := admin.NewRouter(app)

	add := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/dependencies/allowlist", map[string]any{"entry": "@acme/core"})
	if add.Code != http.StatusCreated {
		t.Fatalf("POST scoped allowlist status = %d (%s)", add.Code, add.Body.String())
	}
	// The "/" in the scoped entry must be URL-encoded (%2F) in the query value.
	del := request(t, handler, http.MethodDelete, "/api/sites/"+siteID+"/dependencies/allowlist?entry=%40acme%2Fcore", nil)
	if del.Code != http.StatusNoContent {
		t.Fatalf("DELETE scoped allowlist status = %d (%s)", del.Code, del.Body.String())
	}
}
