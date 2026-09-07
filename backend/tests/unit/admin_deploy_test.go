package unit

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/api/admin"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

func seedSiteSnapshot(t *testing.T, store domain.Storage, siteID, snapshotID string, created time.Time) {
	t.Helper()
	ctx := context.Background()
	if err := store.CreateSite(ctx, domain.Site{ID: siteID, Name: "Site", Slug: "site", DefaultLocale: "ru", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("seed site: %v", err)
	}
	if err := store.CreateSnapshot(ctx, domain.Snapshot{ID: snapshotID, SiteID: siteID, Name: "Snap", CreatedAt: created}); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}
}

func TestAdminDeployReleaseAndActive(t *testing.T) {
	app, db := newAdminHandlerTestAppDB(t)
	handler := admin.NewRouter(app)
	seedSiteSnapshot(t, db, "site_d1", "snap_d1", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	resp := request(t, handler, http.MethodPost, "/api/sites/site_d1/deployments", map[string]any{"snapshotId": "snap_d1", "environment": "production"})
	if resp.Code != http.StatusCreated {
		t.Fatalf("POST deployments: %d %s", resp.Code, resp.Body.String())
	}
	var deployment domain.Deployment
	if err := json.Unmarshal(resp.Body.Bytes(), &deployment); err != nil {
		t.Fatal(err)
	}
	if deployment.SnapshotID != "snap_d1" {
		t.Fatalf("deployment.SnapshotID = %q", deployment.SnapshotID)
	}

	resp = request(t, handler, http.MethodGet, "/api/sites/site_d1/deployments/active?environment=production", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("GET active: %d", resp.Code)
	}
	var active domain.Deployment
	if err := json.Unmarshal(resp.Body.Bytes(), &active); err != nil {
		t.Fatal(err)
	}
	if active.SnapshotID != "snap_d1" || active.Environment != "production" {
		t.Fatalf("active = %+v", active)
	}
}

func TestAdminDeployList(t *testing.T) {
	app, db := newAdminHandlerTestAppDB(t)
	handler := admin.NewRouter(app)
	seedSiteSnapshot(t, db, "site_d2", "snap_dev", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	// Release to development
	resp := request(t, handler, http.MethodPost, "/api/sites/site_d2/deployments", map[string]any{"snapshotId": "snap_dev", "environment": "development"})
	if resp.Code != http.StatusCreated {
		t.Fatalf("release dev: %d %s", resp.Code, resp.Body.String())
	}

	resp = request(t, handler, http.MethodGet, "/api/sites/site_d2/deployments", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("list: %d", resp.Code)
	}
	var list []domain.Deployment
	if err := json.Unmarshal(resp.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Environment != "development" {
		t.Fatalf("list = %+v", list)
	}
}

func TestAdminDeployRollbackEarlier(t *testing.T) {
	app, db := newAdminHandlerTestAppDB(t)
	handler := admin.NewRouter(app)
	seedSiteSnapshot(t, db, "site_rb", "snap_old", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	ctx := context.Background()
	if err := db.CreateSnapshot(ctx, domain.Snapshot{ID: "snap_new", SiteID: "site_rb", Name: "New", CreatedAt: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}

	// Release snap_new first
	resp := request(t, handler, http.MethodPost, "/api/sites/site_rb/deployments", map[string]any{"snapshotId": "snap_new", "environment": "production"})
	if resp.Code != http.StatusCreated {
		t.Fatalf("release: %d %s", resp.Code, resp.Body.String())
	}

	// Rollback to snap_old (earlier)
	resp = request(t, handler, http.MethodPost, "/api/sites/site_rb/deployments/rollback", map[string]any{"snapshotId": "snap_old", "environment": "production"})
	if resp.Code != http.StatusOK {
		t.Fatalf("rollback: %d %s", resp.Code, resp.Body.String())
	}
	var rollback domain.Deployment
	if err := json.Unmarshal(resp.Body.Bytes(), &rollback); err != nil {
		t.Fatal(err)
	}
	if rollback.SnapshotID != "snap_old" {
		t.Fatalf("rollback snapshot = %q, want snap_old", rollback.SnapshotID)
	}
}

func TestAdminDeployRollbackRejectsForward(t *testing.T) {
	app, db := newAdminHandlerTestAppDB(t)
	handler := admin.NewRouter(app)
	seedSiteSnapshot(t, db, "site_rb2", "snap_old", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	ctx := context.Background()
	if err := db.CreateSnapshot(ctx, domain.Snapshot{ID: "snap_new", SiteID: "site_rb2", Name: "New", CreatedAt: time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}

	// Release snap_old first (old)
	resp := request(t, handler, http.MethodPost, "/api/sites/site_rb2/deployments", map[string]any{"snapshotId": "snap_old", "environment": "production"})
	if resp.Code != http.StatusCreated {
		t.Fatalf("release: %d", resp.Code)
	}

	// Rollback to snap_new (newer) — should fail
	resp = request(t, handler, http.MethodPost, "/api/sites/site_rb2/deployments/rollback", map[string]any{"snapshotId": "snap_new", "environment": "production"})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("rollback forward: code = %d, want 400; body = %s", resp.Code, resp.Body.String())
	}
}

func TestAdminDeployActiveNotFound(t *testing.T) {
	app, db := newAdminHandlerTestAppDB(t)
	handler := admin.NewRouter(app)
	seedSiteSnapshot(t, db, "site_none", "snap_none", time.Now().UTC())

	resp := request(t, handler, http.MethodGet, "/api/sites/site_none/deployments/active?environment=production", nil)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("active not found: code = %d, want 404", resp.Code)
	}
}
