package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/api/admin"
	buildapp "github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/application/deps"
	"github.com/liapoldus/liapoldus/backend/internal/application/gitsnapshot"
	"github.com/liapoldus/liapoldus/backend/internal/application/site"
	"github.com/liapoldus/liapoldus/backend/internal/application/snapshot"
	tokenapp "github.com/liapoldus/liapoldus/backend/internal/application/token"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/git"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
)

// slice2AppTestApp wires every aggregate the dashboard/settings/auth handlers
// need. Git gets a real on-disk repo so the dashboard reflects actual branch
// state (slice 2.2).
// requestWithAuth is request with an admin bearer token pre-attached
// (endpoints behind BearerAuth).
func requestWithAuth(t *testing.T, handler http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func slice2TestApp(t *testing.T, adminToken string) (admin.App, *storage.Memory) {
	t.Helper()
	db := storage.NewMemory()
	return admin.App{
		Sites:     site.NewService(db, site.Settings{DefaultLocale: "ru"}),
		Snapshots: snapshot.NewService(db, db, db),
		Builds: buildapp.NewService(db, db, db,
			&fakeBuilder{}, &fakeRunner{}, &fakeArtifacts{}),
		Git: gitsnapshot.NewService(
			git.NewRepo(filepath.Join(t.TempDir(), "git")),
			db, db, db, db, db, db, db, db,
			deps.NewService(db, db, &fakeRegistry{}),
		),
		Tokens:     tokenapp.NewService(db, db),
		Logger:     slog.Default(),
		AdminToken: adminToken,
	}, db
}

func TestAuthValidate(t *testing.T) {
	t.Run("valid token", func(t *testing.T) {
		app, _ := slice2TestApp(t, "secret")
		handler := admin.NewRouter(app)
		resp := request(t, handler, http.MethodPost, "/api/auth/validate", map[string]any{"token": "secret"})
		if resp.Code != http.StatusOK {
			t.Fatalf("validate status = %d, want 200", resp.Code)
		}
		var body struct {
			Valid bool `json:"valid"`
		}
		decodeResponse(t, resp, &body)
		if !body.Valid {
			t.Fatalf("valid = false, want true")
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		app, _ := slice2TestApp(t, "secret")
		handler := admin.NewRouter(app)
		resp := request(t, handler, http.MethodPost, "/api/auth/validate", map[string]any{"token": "wrong"})
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("validate status = %d, want 401", resp.Code)
		}
	})

	t.Run("open when no token configured", func(t *testing.T) {
		app, _ := slice2TestApp(t, "")
		handler := admin.NewRouter(app)
		resp := request(t, handler, http.MethodPost, "/api/auth/validate", map[string]any{"token": ""})
		if resp.Code != http.StatusOK {
			t.Fatalf("validate status = %d, want 200 when adminToken is empty", resp.Code)
		}
	})
}

func TestDashboardEmpty(t *testing.T) {
	app, _ := slice2TestApp(t, "")
	handler := admin.NewRouter(app)
	resp := request(t, handler, http.MethodGet, "/api/dashboard", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d, want 200", resp.Code)
	}
	var dash struct {
		SiteCount       int    `json:"siteCount"`
		RecentBuilds    []any  `json:"recentBuilds"`
		RecentSnapshots []any  `json:"recentSnapshots"`
		RuntimeStatus   string `json:"runtimeStatus"`
	}
	decodeResponse(t, resp, &dash)
	if dash.SiteCount != 0 {
		t.Fatalf("siteCount = %d, want 0", dash.SiteCount)
	}
	if len(dash.RecentBuilds) != 0 || len(dash.RecentSnapshots) != 0 {
		t.Fatalf("recent builds/snapshots must be empty, got %d/%d", len(dash.RecentBuilds), len(dash.RecentSnapshots))
	}
	if dash.RuntimeStatus != "no-builds" {
		t.Fatalf("runtimeStatus = %q, want no-builds", dash.RuntimeStatus)
	}
}

func TestDashboardAggregatesSitesBuildsSnapshots(t *testing.T) {
	app, db := slice2TestApp(t, "")
	ctx := context.Background()

	first, err := app.Sites.Create(ctx, "Первый сайт", "first", "", nil)
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	second, err := app.Sites.Create(ctx, "Второй сайт", "second", "", nil)
	if err != nil {
		t.Fatalf("create second site: %v", err)
	}

	if _, err := app.Git.Commit(ctx, first.ID, "initial"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	now := time.Now().UTC()
	for _, s := range []domain.Snapshot{
		{ID: "snap_1", SiteID: second.ID, Name: "Alpha", CreatedAt: now.Add(-time.Hour)},
		{ID: "snap_2", SiteID: first.ID, Name: "Beta", CreatedAt: now},
	} {
		if err := db.CreateSnapshot(ctx, s); err != nil {
			t.Fatalf("create snapshot: %v", err)
		}
	}
	for _, b := range []domain.Build{
		{ID: "build_1", SiteID: first.ID, SnapshotID: "snap_2", Environment: domain.EnvironmentProduction, Status: domain.BuildStatusFailed, CreatedAt: now.Add(-2 * time.Hour)},
		{ID: "build_2", SiteID: second.ID, SnapshotID: "snap_1", Environment: domain.EnvironmentDevelopment, Status: domain.BuildStatusReady, CreatedAt: now.Add(-time.Hour)},
	} {
		if err := db.CreateBuild(ctx, b); err != nil {
			t.Fatalf("create build: %v", err)
		}
	}

	handler := admin.NewRouter(app)
	resp := request(t, handler, http.MethodGet, "/api/dashboard", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d, want 200", resp.Code)
	}

	var dash struct {
		SiteCount int `json:"siteCount"`
		Sites     []struct {
			SiteID string `json:"siteId"`
			Name   string `json:"name"`
			Git    *struct {
				Dev struct {
					SHA string `json:"sha"`
				} `json:"dev"`
			} `json:"git"`
		} `json:"sites"`
		RecentBuilds []struct {
			ID       string `json:"id"`
			SiteName string `json:"siteName"`
			Status   string `json:"status"`
		} `json:"recentBuilds"`
		RecentSnapshots []struct {
			SiteName string `json:"siteName"`
			Name     string `json:"name"`
		} `json:"recentSnapshots"`
		RuntimeStatus string `json:"runtimeStatus"`
	}
	decodeResponse(t, resp, &dash)

	if dash.SiteCount != 2 {
		t.Fatalf("siteCount = %d, want 2", dash.SiteCount)
	}
	if len(dash.RecentBuilds) != 2 {
		t.Fatalf("recentBuilds = %d, want 2", len(dash.RecentBuilds))
	}
	if want := "Второй сайт"; dash.RecentBuilds[0].SiteName != want {
		t.Fatalf("recentBuilds[0].siteName = %q, want %q (newest first)", dash.RecentBuilds[0].SiteName, want)
	}
	if dash.RecentBuilds[0].Status != "ready" {
		t.Fatalf("recentBuilds[0].status = %q, want ready", dash.RecentBuilds[0].Status)
	}
	if dash.RuntimeStatus != "degraded" {
		t.Fatalf("runtimeStatus = %q, want degraded (failed build present)", dash.RuntimeStatus)
	}
	if len(dash.RecentSnapshots) != 2 {
		t.Fatalf("recentSnapshots = %d, want 2", len(dash.RecentSnapshots))
	}
	if dash.RecentSnapshots[0].Name != "Beta" {
		t.Fatalf("recentSnapshots[0].name = %q, want Beta (newest first)", dash.RecentSnapshots[0].Name)
	}
	if dash.RecentSnapshots[0].SiteName != "Первый сайт" {
		t.Fatalf("recentSnapshots[0].siteName = %q, want first site name", dash.RecentSnapshots[0].SiteName)
	}

	var firstGit *struct {
		Dev struct {
			SHA string `json:"sha"`
		} `json:"dev"`
	}
	for _, s := range dash.Sites {
		if s.SiteID == first.ID {
			firstGit = s.Git
		}
	}
	if firstGit == nil || firstGit.Dev.SHA == "" {
		t.Fatalf("first site git status missing or empty, got %#v", dash.Sites)
	}
}

func TestSettingsResponse(t *testing.T) {
	app, _ := slice2TestApp(t, "supersecret1234")
	app.DefaultLocale = "de"
	app.RedirectDefaultStatus = 302
	handler := admin.NewRouter(app)

	resp := requestWithAuth(t, handler, http.MethodGet, "/api/settings", nil, "supersecret1234")
	if resp.Code != http.StatusOK {
		t.Fatalf("settings status = %d, want 200", resp.Code)
	}
	var body struct {
		AdminToken            string `json:"adminToken"`
		DefaultLocale         string `json:"defaultLocale"`
		RedirectDefaultStatus int    `json:"redirectDefaultStatus"`
	}
	decodeResponse(t, resp, &body)
	if body.AdminToken != "····1234" {
		t.Fatalf("adminToken = %q, want masked form", body.AdminToken)
	}
	if body.DefaultLocale != "de" || body.RedirectDefaultStatus != 302 {
		t.Fatalf("settings = %#v, want de/302", body)
	}
}

func TestSettingsResponseMasksShortToken(t *testing.T) {
	app, _ := slice2TestApp(t, "abcd")
	handler := admin.NewRouter(app)
	resp := requestWithAuth(t, handler, http.MethodGet, "/api/settings", nil, "abcd")
	var body struct {
		AdminToken string `json:"adminToken"`
	}
	decodeResponse(t, resp, &body)
	if body.AdminToken != "····" {
		t.Fatalf("adminToken = %q, want fully masked", body.AdminToken)
	}
}
