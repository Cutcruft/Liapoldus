package unit

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/api/admin"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
)

// seedLayoutShell registers a section that accepts page content so a site
// settings test can reference a valid defaultLayoutSectionId.
func seedLayoutShell(t *testing.T, db *storage.Memory, siteID string) {
	t.Helper()
	def := domain.ComponentDefinition{
		SiteID:             siteID,
		ID:                 "Shell",
		Name:               "Shell",
		Kind:               "component",
		IsSection:          true,
		AcceptsPageContent: true,
		Source:             "export default () => null;",
		Schema:             mustJSONMap(`{"type":"object","properties":{}}`),
		CreatedAt:          time.Now().UTC(),
	}
	if err := db.Save(context.Background(), &def); err != nil {
		t.Fatalf("seed shell: %v", err)
	}
}

func TestSiteSettingsGetDefaultsAndPut(t *testing.T) {
	app, db := slice2TestApp(t, "secret")
	siteID := prepareTokenSite(t, app, "secret")
	seedLayoutShell(t, db, siteID)
	handler := admin.NewRouter(app)

	resp := requestWithAuth(t, handler, http.MethodGet, "/api/sites/"+siteID+"/settings", nil, "secret")
	if resp.Code != http.StatusOK {
		t.Fatalf("get defaults status = %d, want 200", resp.Code)
	}
	var defaults domain.SiteSettings
	decodeResponse(t, resp, &defaults)
	if defaults.SiteID != siteID || defaults.DefaultLocale != "ru" || defaults.Head.Meta == nil {
		t.Fatalf("defaults = %#v", defaults)
	}

	put := domain.SiteSettings{
		DefaultLocale:          "en",
		DefaultLayoutSectionID: "Shell",
		Head: domain.SiteHead{
			TitleTemplate:  "%s · Demo",
			Description:    "Demo site",
			FaviconAssetID: "asset_favicon",
			Meta:           map[string]string{"og:type": "website"},
		},
	}
	resp = requestWithAuth(t, handler, http.MethodPut, "/api/sites/"+siteID+"/settings", put, "secret")
	if resp.Code != http.StatusOK {
		t.Fatalf("put status = %d, want 200", resp.Code)
	}
	var saved domain.SiteSettings
	decodeResponse(t, resp, &saved)
	if saved.SiteID != siteID || saved.DefaultLocale != "en" || saved.Head.Meta["og:type"] != "website" {
		t.Fatalf("saved = %#v", saved)
	}
	if saved.DefaultLayoutSectionID != "Shell" {
		t.Fatalf("saved.defaultLayoutSectionId = %q, want Shell", saved.DefaultLayoutSectionID)
	}
}

func TestSiteSettingsRejectsEmptyMetaAndUnknownSite(t *testing.T) {
	app, db := slice2TestApp(t, "secret")
	handler := admin.NewRouter(app)
	invalid := domain.SiteSettings{DefaultLocale: "ru", Head: domain.SiteHead{Meta: map[string]string{"": "value"}}}
	if resp := requestWithAuth(t, handler, http.MethodPut, "/api/sites/ghost/settings", invalid, "secret"); resp.Code != http.StatusNotFound {
		t.Fatalf("unknown site status = %d, want 404", resp.Code)
	}

	siteID := prepareTokenSite(t, app, "secret")
	seedLayoutShell(t, db, siteID)
	if resp := requestWithAuth(t, handler, http.MethodPut, "/api/sites/"+siteID+"/settings", invalid, "secret"); resp.Code != http.StatusBadRequest {
		t.Fatalf("invalid settings status = %d, want 400", resp.Code)
	}
}

func TestSiteSettingsRejectsInvalidLayoutSection(t *testing.T) {
	app, db := slice2TestApp(t, "secret")
	siteID := prepareTokenSite(t, app, "secret")
	seedLayoutShell(t, db, siteID)
	handler := admin.NewRouter(app)

	cases := []struct {
		name   string
		layout string
	}{
		{"unknown layout section", "Missing"},
		{"section without acceptsPageContent", "Container"},
		{"primitive used as layout", "Text"},
	}
	seedSiteDefs(t, db, siteID) // Container/Text sections (no acceptsPageContent)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := domain.SiteSettings{DefaultLocale: "ru", DefaultLayoutSectionID: tc.layout}
			if resp := requestWithAuth(t, handler, http.MethodPut, "/api/sites/"+siteID+"/settings", req, "secret"); resp.Code != http.StatusBadRequest {
				t.Fatalf("%s: status = %d, want 400", tc.name, resp.Code)
			}
		})
	}
}
