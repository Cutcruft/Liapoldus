package unit

import (
	"context"
	"log/slog"
	"net/http"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/api/admin"
	"github.com/liapoldus/liapoldus/backend/internal/application/deps"
	"github.com/liapoldus/liapoldus/backend/internal/application/site"
	"github.com/liapoldus/liapoldus/backend/internal/application/snapshot"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
)

func newDepsApp(t *testing.T, reg *fakeRegistry) (admin.App, domain.Storage, string) {
	t.Helper()
	db := storage.NewMemory()
	ctx := context.Background()

	sites := site.NewService(db, site.Settings{DefaultLocale: "ru"})
	created, err := sites.Create(ctx, "Demo", "demo", "ru", []string{"demo.test"})
	if err != nil {
		t.Fatalf("create site: %v", err)
	}

	depsSvc := deps.NewService(db, db, reg)
	app := admin.App{
		Sites:     sites,
		Snapshots: snapshot.NewService(db, db, db, depsSvc),
		Deps:      depsSvc,
		Logger:    slog.Default(),
	}
	return app, db, created.ID
}

func TestDepsAdminFlow(t *testing.T) {
	reg := &fakeRegistry{resolve: func(_ context.Context, name, spec string) (deps.ResolvedVersion, error) {
		if name == "lodash" && spec == "^4" {
			return deps.ResolvedVersion{Name: name, Version: "4.17.21", Integrity: "sha512-x"}, nil
		}
		return deps.ResolvedVersion{}, domain.ErrPackageNotFound
	}}
	app, _, siteID := newDepsApp(t, reg)
	handler := admin.NewRouter(app)

	list := request(t, handler, http.MethodGet, "/api/sites/"+siteID+"/dependencies", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("GET deps status = %d", list.Code)
	}
	var empty []map[string]any
	decodeResponse(t, list, &empty)
	if len(empty) != 0 {
		t.Errorf("initial deps = %+v, want empty", empty)
	}

	create := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/dependencies", map[string]any{"name": "lodash", "spec": "^4"})
	if create.Code != http.StatusCreated {
		t.Fatalf("POST deps status = %d (%s)", create.Code, create.Body.String())
	}
	var created struct {
		Name            string `json:"name"`
		Spec            string `json:"spec"`
		ResolvedVersion string `json:"resolvedVersion"`
	}
	decodeResponse(t, create, &created)
	if created.Name != "lodash" || created.Spec != "^4" || created.ResolvedVersion != "4.17.21" {
		t.Errorf("created = %+v", created)
	}

	list = request(t, handler, http.MethodGet, "/api/sites/"+siteID+"/dependencies", nil)
	var deps []map[string]any
	decodeResponse(t, list, &deps)
	if len(deps) != 1 || deps[0]["name"] != "lodash" {
		t.Errorf("deps after create = %+v", deps)
	}

	del := request(t, handler, http.MethodDelete, "/api/sites/"+siteID+"/dependencies/lodash", nil)
	if del.Code != http.StatusNoContent {
		t.Fatalf("DELETE deps status = %d", del.Code)
	}

	list = request(t, handler, http.MethodGet, "/api/sites/"+siteID+"/dependencies", nil)
	decodeResponse(t, list, &empty)
	if len(empty) != 0 {
		t.Errorf("deps after delete = %+v, want empty", empty)
	}
}

func TestDepsAdminScopedNameDelete(t *testing.T) {
	reg := &fakeRegistry{resolve: func(_ context.Context, name, _ string) (deps.ResolvedVersion, error) {
		return deps.ResolvedVersion{Name: name, Version: "1.2.3"}, nil
	}}
	app, _, siteID := newDepsApp(t, reg)
	handler := admin.NewRouter(app)

	create := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/dependencies", map[string]any{"name": "@acme/core", "spec": "^1"})
	if create.Code != http.StatusCreated {
		t.Fatalf("POST scoped deps status = %d (%s)", create.Code, create.Body.String())
	}

	del := request(t, handler, http.MethodDelete, "/api/sites/"+siteID+"/dependencies/%40acme%2Fcore", nil)
	if del.Code != http.StatusNoContent {
		t.Fatalf("DELETE scoped deps status = %d (%s)", del.Code, del.Body.String())
	}
}

func TestDepsAdminValidationErrors(t *testing.T) {
	reg := &fakeRegistry{resolve: func(context.Context, string, string) (deps.ResolvedVersion, error) {
		return deps.ResolvedVersion{}, domain.ErrPackageNotFound
	}}
	app, _, siteID := newDepsApp(t, reg)
	handler := admin.NewRouter(app)

	badSpec := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/dependencies", map[string]any{"name": "lodash", "spec": "latest"})
	if badSpec.Code != http.StatusBadRequest {
		t.Fatalf("POST spec=latest status = %d, want 400", badSpec.Code)
	}

	missing := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/dependencies", map[string]any{"name": "does-not-exist", "spec": "^1"})
	if missing.Code != http.StatusNotFound {
		t.Fatalf("POST unknown package status = %d, want 404", missing.Code)
	}

	noName := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/dependencies", map[string]any{"spec": "^1"})
	if noName.Code != http.StatusBadRequest {
		t.Fatalf("POST missing name status = %d, want 400", noName.Code)
	}
}

func TestSnapshotAdminStampsDepsLock(t *testing.T) {
	reg := &fakeRegistry{resolve: func(_ context.Context, name, _ string) (deps.ResolvedVersion, error) {
		switch name {
		case "lodash":
			return deps.ResolvedVersion{Name: name, Version: "4.17.21", Integrity: "sha512-x", Dependencies: map[string]string{"lodash._reinterpolate": "^3"}}, nil
		case "lodash._reinterpolate":
			return deps.ResolvedVersion{Name: name, Version: "3.2.1", Integrity: "sha512-y"}, nil
		}
		return deps.ResolvedVersion{}, domain.ErrPackageNotFound
	}}
	app, _, siteID := newDepsApp(t, reg)
	handler := admin.NewRouter(app)

	create := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/dependencies", map[string]any{"name": "lodash", "spec": "^4"})
	if create.Code != http.StatusCreated {
		t.Fatalf("POST deps status = %d", create.Code)
	}

	snap := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/snapshots", map[string]any{"name": "v1 with deps"})
	if snap.Code != http.StatusCreated {
		t.Fatalf("POST snapshot status = %d (%s)", snap.Code, snap.Body.String())
	}
	var body struct {
		DepsLock struct {
			Deps []struct {
				Name      string `json:"name"`
				Spec      string `json:"spec"`
				Version   string `json:"version"`
				Integrity string `json:"integrity"`
			} `json:"deps"`
		} `json:"depsLock"`
	}
	decodeResponse(t, snap, &body)
	if len(body.DepsLock.Deps) != 2 {
		t.Fatalf("snapshot depsLock = %+v, want 2 frozen deps", body.DepsLock.Deps)
	}
	if body.DepsLock.Deps[0].Name != "lodash" || body.DepsLock.Deps[1].Name != "lodash._reinterpolate" {
		t.Errorf("depsLock order = %+v", body.DepsLock.Deps)
	}
	if body.DepsLock.Deps[0].Version != "4.17.21" || body.DepsLock.Deps[1].Version != "3.2.1" {
		t.Errorf("depsLock versions = %+v", body.DepsLock.Deps)
	}
}

func TestSnapshotAdminFailsPublishWhenLockResolveFails(t *testing.T) {
	reg := &fakeRegistry{resolve: func(_ context.Context, name, _ string) (deps.ResolvedVersion, error) {
		switch name {
		case "a":
			return deps.ResolvedVersion{Name: name, Version: "1.0.0", Dependencies: map[string]string{"ghost": "^99"}}, nil
		case "ghost":
			return deps.ResolvedVersion{}, domain.ErrUnresolvableSpec
		}
		return deps.ResolvedVersion{}, domain.ErrPackageNotFound
	}}
	app, _, siteID := newDepsApp(t, reg)
	handler := admin.NewRouter(app)

	create := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/dependencies", map[string]any{"name": "a", "spec": "^1"})
	if create.Code != http.StatusCreated {
		t.Fatalf("POST deps status = %d", create.Code)
	}

	snap := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/snapshots", map[string]any{"name": "doomed"})
	if snap.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST snapshot status = %d, want 422 when lock cannot resolve", snap.Code)
	}
}

func TestDepsAdminResolveReturnsFullLock(t *testing.T) {
	reg := &fakeRegistry{resolve: func(_ context.Context, name, _ string) (deps.ResolvedVersion, error) {
		switch name {
		case "a":
			return deps.ResolvedVersion{Name: name, Version: "1.0.0", Integrity: "sha512-a", Dependencies: map[string]string{"b": "^1", "c": "~2"}}, nil
		case "b":
			return deps.ResolvedVersion{Name: name, Version: "1.2.0", Integrity: "sha512-b"}, nil
		case "c":
			return deps.ResolvedVersion{Name: name, Version: "2.0.1", Integrity: "sha512-c", Dependencies: map[string]string{"b": "^1"}}, nil
		}
		return deps.ResolvedVersion{}, domain.ErrPackageNotFound
	}}
	app, _, siteID := newDepsApp(t, reg)
	handler := admin.NewRouter(app)

	for _, d := range []struct{ name, spec string }{{"a", "^1"}, {"b", "^1"}} {
		create := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/dependencies", map[string]any{"name": d.name, "spec": d.spec})
		if create.Code != http.StatusCreated {
			t.Fatalf("POST deps(%s) status = %d (%s)", d.name, create.Code, create.Body.String())
		}
	}

	res := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/dependencies/resolve", nil)
	if res.Code != http.StatusOK {
		t.Fatalf("POST resolve status = %d (%s)", res.Code, res.Body.String())
	}
	var lock struct {
		Deps []struct {
			Name        string   `json:"name"`
			Version     string   `json:"version"`
			Hoisted     bool     `json:"hoisted"`
			RequestedBy []string `json:"requestedBy"`
		} `json:"deps"`
	}
	decodeResponse(t, res, &lock)
	if len(lock.Deps) != 3 { // a, b (hoisted), c; b only hoisted once even though a and c both need it
		t.Fatalf("resolve deps = %+v, want 3 instances (a, hoisted b, c)", lock.Deps)
	}
	got := map[string]bool{}
	for _, d := range lock.Deps {
		got[d.Name+"@"+d.Version] = true
	}
	for _, want := range []string{"a@1.0.0", "b@1.2.0", "c@2.0.1"} {
		if !got[want] {
			t.Errorf("resolve missing instance %s; got %+v", want, lock.Deps)
		}
	}
	// b is a top-level decl and the hoisted instance of the shared transitive dep.
	for _, d := range lock.Deps {
		if d.Name == "b" && !d.Hoisted {
			t.Errorf("b expected hoisted, got %+v", d)
		}
		if d.Name == "a" {
			found := false
			for _, rb := range d.RequestedBy {
				if rb == "site" {
					found = true
				}
			}
			if !found {
				t.Errorf("a.requestedBy = %+v, want it to include 'site' (top-level decl)", d.RequestedBy)
			}
		}
	}
}

func TestDepsAdminResolveFailsUnsatisfiable(t *testing.T) {
	reg := &fakeRegistry{resolve: func(_ context.Context, name, _ string) (deps.ResolvedVersion, error) {
		switch name {
		case "a":
			return deps.ResolvedVersion{Name: name, Version: "1.0.0", Dependencies: map[string]string{"ghost": "^99"}}, nil
		case "ghost":
			return deps.ResolvedVersion{}, domain.ErrUnresolvableSpec
		}
		return deps.ResolvedVersion{}, domain.ErrPackageNotFound
	}}
	app, _, siteID := newDepsApp(t, reg)
	handler := admin.NewRouter(app)

	create := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/dependencies", map[string]any{"name": "a", "spec": "^1"})
	if create.Code != http.StatusCreated {
		t.Fatalf("POST deps status = %d", create.Code)
	}

	res := request(t, handler, http.MethodPost, "/api/sites/"+siteID+"/dependencies/resolve", nil)
	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST resolve status = %d, want 422 on unsatisfiable graph", res.Code)
	}
}
