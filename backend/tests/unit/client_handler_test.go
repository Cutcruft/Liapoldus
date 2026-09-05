package unit

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/api/client"
	"github.com/liapoldus/liapoldus/backend/internal/application/asset"
	"github.com/liapoldus/liapoldus/backend/internal/application/content"
	"github.com/liapoldus/liapoldus/backend/internal/application/form"
	"github.com/liapoldus/liapoldus/backend/internal/application/route"
	"github.com/liapoldus/liapoldus/backend/internal/application/site"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
)

func newClientHandlerTestApp(t *testing.T, defaultSlug string) *client.App {
	t.Helper()
	db := storage.NewMemory()
	blobs, err := storage.NewDiskBlobStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &client.App{
		Sites:       site.NewService(db, site.Settings{DefaultLocale: "ru"}),
		Contents:    content.NewService(db),
		Assets:      asset.NewService(db, blobs, db, asset.Settings{MasterVariant: "master", FallbackName: "asset", FallbackMime: "application/octet-stream", URLTemplate: "/files/{id}"}),
		Routes:      route.NewService(db, route.Settings{DefaultStatus: 301, Allowed: map[int]bool{301: true, 302: true}}),
		Forms:       form.NewService(db, db, form.Settings{EmailPattern: emailPattern}),
		Logger:      slog.Default(),
		DefaultSlug: defaultSlug,

		DefaultRedirectStatus:   301,
		AssetCacheMaxAgeSeconds: 31536000,
	}
}

func createSite(t *testing.T, app *client.App, slug string) domain.Site {
	t.Helper()
	site, err := app.Sites.Create(context.Background(), slug+" site", slug, "ru", nil)
	if err != nil {
		t.Fatal(err)
	}
	return site
}

func TestContentGetMergedWithHostFallback(t *testing.T) {
	app := newClientHandlerTestApp(t, "demo")
	site := createSite(t, app, "demo")

	content, err := app.Contents.Create(context.Background(), site.ID, "col.articles", "a1", map[string]any{
		"title": "Base title", "description": "Base description",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Contents.SetTranslation(context.Background(), site.ID, content.ID, "ru", map[string]any{
		"title": "Привет",
	}); err != nil {
		t.Fatal(err)
	}

	handler := client.NewRouter(app)
	req := httptest.NewRequest(http.MethodGet, "/api/contents/a1?locale=ru", nil)
	req.Host = "shop.example.org"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var decoded struct {
		ID           string         `json:"id"`
		Locale       string         `json:"locale"`
		Fields       map[string]any `json:"fields"`
		CollectionID string         `json:"collectionId"`
	}
	decodeResponse(t, response, &decoded)
	if decoded.ID != content.ID || decoded.Locale != "ru" {
		t.Fatalf("decoded = %#v", decoded)
	}
	if decoded.Fields["title"] != "Привет" || decoded.Fields["description"] != "Base description" {
		t.Fatalf("merged fields = %#v", decoded.Fields)
	}
}

func TestFormSubmitValidationAndRedirectEdge(t *testing.T) {
	app := newClientHandlerTestApp(t, "demo")
	site := createSite(t, app, "demo")

	form, err := app.Forms.Create(context.Background(), site.ID, "contact", map[string]any{
		"fields": []any{map[string]any{"name": "email", "type": "email", "required": true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Routes.Create(context.Background(), site.ID, "^/old$", 0, domain.RouteAction{
		Type: route.Redirect, Target: "/new", Status: 301,
	}); err != nil {
		t.Fatal(err)
	}
	handler := client.NewRouter(app)

	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, httptest.NewRequest(http.MethodPost, "/api/forms/"+form.ID+"/submissions", strings.NewReader(
		`{"values":{"email":"not-an-email"}}`)))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid submission status = %d, body = %s", invalid.Code, invalid.Body.String())
	}

	valid := httptest.NewRecorder()
	handler.ServeHTTP(valid, httptest.NewRequest(http.MethodPost, "/api/forms/"+form.ID+"/submissions", strings.NewReader(
		`{"values":{"email":"user@example.com"}}`)))
	if valid.Code != http.StatusCreated {
		t.Fatalf("valid submission status = %d, body = %s", valid.Code, valid.Body.String())
	}

	redirect := httptest.NewRecorder()
	redirectReq := httptest.NewRequest(http.MethodGet, "/old", nil)
	handler.ServeHTTP(redirect, redirectReq)
	if redirect.Code != http.StatusMovedPermanently {
		t.Fatalf("redirect status = %d", redirect.Code)
	}
	if location := redirect.Header().Get("Location"); location != "/new" {
		t.Fatalf("location = %q", location)
	}
}

func TestRedirectGroupSubstitution(t *testing.T) {
	app := newClientHandlerTestApp(t, "demo")
	site := createSite(t, app, "demo")

	if _, err := app.Routes.Create(context.Background(), site.ID, `^/products/([a-z0-9-]+)$`, 0, domain.RouteAction{
		Type: route.Redirect, Target: "/shop/$1", Status: 302,
	}); err != nil {
		t.Fatal(err)
	}
	handler := client.NewRouter(app)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/products/sneakers", nil))
	if response.Code != http.StatusFound {
		t.Fatalf("status = %d", response.Code)
	}
	if location := response.Header().Get("Location"); location != "/shop/sneakers" {
		t.Fatalf("location = %q", location)
	}
}
