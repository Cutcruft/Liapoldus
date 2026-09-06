package unit

import (
	"context"
	"net/http"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/api/admin"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

func prepareTokenSite(t *testing.T, app admin.App, token string) string {
	t.Helper()
	site, err := app.Sites.Create(context.Background(), "Токены", "token-site", "", nil)
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	return site.ID
}

func TestTokensGetEmpty(t *testing.T) {
	app, _ := slice2TestApp(t, "secret")
	siteID := prepareTokenSite(t, app, "secret")
	handler := admin.NewRouter(app)

	resp := requestWithAuth(t, handler, http.MethodGet, "/api/sites/"+siteID+"/tokens", nil, "secret")
	if resp.Code != http.StatusOK {
		t.Fatalf("get tokens status = %d, want 200", resp.Code)
	}
	var body domain.TokenSet
	decodeResponse(t, resp, &body)
	if body.Colors == nil || len(body.Colors) != 0 {
		t.Fatalf("colors = %#v, want empty list", body.Colors)
	}
	if body.Typography != nil || body.Spacing != nil {
		t.Fatalf("fresh set must have no scalar groups, got %#v", body)
	}
}

func TestTokensPutGet(t *testing.T) {
	app, _ := slice2TestApp(t, "secret")
	siteID := prepareTokenSite(t, app, "secret")
	handler := admin.NewRouter(app)

	set := domain.TokenSet{
		Colors: []domain.ColorToken{
			{Name: "accent", Value: map[domain.ColorMode]string{domain.ColorModeLight: "#0b2e4f", domain.ColorModeDark: "#7fd0ff"}},
			{Name: "bg", Value: map[domain.ColorMode]string{domain.ColorModeLight: "#ffffff", domain.ColorModeDark: "#111827"}},
		},
		Typography: map[string]string{"font-sans": "'Inter', sans-serif", "text-xl": "2rem"},
		Spacing:    map[string]string{"pad": "1rem"},
	}
	resp := requestWithAuth(t, handler, http.MethodPut, "/api/sites/"+siteID+"/tokens", set, "secret")
	if resp.Code != http.StatusOK {
		t.Fatalf("put tokens status = %d, want 200", resp.Code)
	}
	var putBody domain.TokenSet
	decodeResponse(t, resp, &putBody)
	if len(putBody.Colors) != 2 || putBody.Colors[0].Name != "accent" {
		t.Fatalf("put response = %#v, want 2 colors starting with accent", putBody.Colors)
	}
	if putBody.Colors[0].Value[domain.ColorModeDark] != "#7fd0ff" {
		t.Fatalf("accent dark = %q, want #7fd0ff", putBody.Colors[0].Value[domain.ColorModeDark])
	}

	resp = requestWithAuth(t, handler, http.MethodGet, "/api/sites/"+siteID+"/tokens", nil, "secret")
	if resp.Code != http.StatusOK {
		t.Fatalf("get tokens status = %d, want 200", resp.Code)
	}
	var got domain.TokenSet
	decodeResponse(t, resp, &got)
	if len(got.Colors) != 2 || got.Typography["text-xl"] != "2rem" || got.Spacing["pad"] != "1rem" {
		t.Fatalf("round-tripped set = %#v", got)
	}
}

func TestTokensPutReplacesEntireSet(t *testing.T) {
	app, _ := slice2TestApp(t, "secret")
	siteID := prepareTokenSite(t, app, "secret")
	handler := admin.NewRouter(app)

	first := domain.TokenSet{Colors: []domain.ColorToken{{Name: "a", Value: map[domain.ColorMode]string{domain.ColorModeLight: "#111111"}}}}
	requestWithAuth(t, handler, http.MethodPut, "/api/sites/"+siteID+"/tokens", first, "secret")

	second := domain.TokenSet{Spacing: map[string]string{"pad": "2rem"}}
	resp := requestWithAuth(t, handler, http.MethodPut, "/api/sites/"+siteID+"/tokens", second, "secret")
	if resp.Code != http.StatusOK {
		t.Fatalf("put tokens status = %d, want 200", resp.Code)
	}
	resp = requestWithAuth(t, handler, http.MethodGet, "/api/sites/"+siteID+"/tokens", nil, "secret")
	var got domain.TokenSet
	decodeResponse(t, resp, &got)
	if len(got.Colors) != 0 {
		t.Fatalf("colors after replacing set = %d, want 0", len(got.Colors))
	}
	if got.Spacing["pad"] != "2rem" {
		t.Fatalf("spacing after replace = %v, want pad:2rem", got.Spacing)
	}
}

func TestTokensGhostSite(t *testing.T) {
	app, _ := slice2TestApp(t, "secret")
	handler := admin.NewRouter(app)

	if resp := requestWithAuth(t, handler, http.MethodGet, "/api/sites/ghost/tokens", nil, "secret"); resp.Code != http.StatusNotFound {
		t.Fatalf("get tokens on ghost site status = %d, want 404", resp.Code)
	}
	if resp := requestWithAuth(t, handler, http.MethodPut, "/api/sites/ghost/tokens", domain.TokenSet{}, "secret"); resp.Code != http.StatusNotFound {
		t.Fatalf("put tokens on ghost site status = %d, want 404", resp.Code)
	}
}

func TestTokensRequiresAuth(t *testing.T) {
	app, _ := slice2TestApp(t, "secret")
	siteID := prepareTokenSite(t, app, "secret")
	handler := admin.NewRouter(app)

	if resp := request(t, handler, http.MethodGet, "/api/sites/"+siteID+"/tokens", nil); resp.Code != http.StatusUnauthorized {
		t.Fatalf("get tokens without auth status = %d, want 401", resp.Code)
	}
	if resp := requestWithAuth(t, handler, http.MethodPut, "/api/sites/"+siteID+"/tokens", domain.TokenSet{}, "wrong"); resp.Code != http.StatusUnauthorized {
		t.Fatalf("put tokens with wrong token status = %d, want 401", resp.Code)
	}
}
