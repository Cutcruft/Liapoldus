package unit

import (
	"net/http"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/api/admin"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

func TestSiteSettingsGetDefaultsAndPut(t *testing.T) {
	app, _ := slice2TestApp(t, "secret")
	siteID := prepareTokenSite(t, app, "secret")
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
		DefaultLayoutSectionID: "layout.shell",
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
}

func TestSiteSettingsRejectsEmptyMetaAndUnknownSite(t *testing.T) {
	app, _ := slice2TestApp(t, "secret")
	handler := admin.NewRouter(app)
	invalid := domain.SiteSettings{DefaultLocale: "ru", Head: domain.SiteHead{Meta: map[string]string{"": "value"}}}
	if resp := requestWithAuth(t, handler, http.MethodPut, "/api/sites/ghost/settings", invalid, "secret"); resp.Code != http.StatusNotFound {
		t.Fatalf("unknown site status = %d, want 404", resp.Code)
	}

	siteID := prepareTokenSite(t, app, "secret")
	if resp := requestWithAuth(t, handler, http.MethodPut, "/api/sites/"+siteID+"/settings", invalid, "secret"); resp.Code != http.StatusBadRequest {
		t.Fatalf("invalid settings status = %d, want 400", resp.Code)
	}
}
