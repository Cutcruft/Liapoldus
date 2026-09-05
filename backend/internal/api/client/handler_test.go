package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/store"
)

func newTestApp(t *testing.T, defaultSlug string) *App {
	t.Helper()
	db := store.NewMemory()
	blobs, err := store.NewDiskBlobStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &App{
		Sites:       domain.NewSiteService(db),
		Contents:    domain.NewContentService(db),
		Assets:      domain.NewAssetService(db, blobs, db),
		Routes:      domain.NewRouteService(db),
		Forms:       domain.NewFormService(db, db),
		Logger:      slog.Default(),
		DefaultSlug: defaultSlug,
	}
}

func createSite(t *testing.T, app *App, slug string) domain.Site {
	t.Helper()
	site, err := app.Sites.Create(context.Background(), slug+" site", slug, "ru", nil)
	if err != nil {
		t.Fatal(err)
	}
	return site
}

func TestContentGetMergedWithHostFallback(t *testing.T) {
	app := newTestApp(t, "demo")
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

	handler := NewRouter(app)
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
	app := newTestApp(t, "demo")
	site := createSite(t, app, "demo")

	form, err := app.Forms.Create(context.Background(), site.ID, "contact", map[string]any{
		"fields": []any{map[string]any{"name": "email", "type": "email", "required": true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Routes.Create(context.Background(), site.ID, "^/old$", 0, domain.RouteAction{
		Type: domain.RouteRedirect, Target: "/new", Status: 301,
	}); err != nil {
		t.Fatal(err)
	}
	handler := NewRouter(app)

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
	app := newTestApp(t, "demo")
	site := createSite(t, app, "demo")

	if _, err := app.Routes.Create(context.Background(), site.ID, `^/products/([a-z0-9-]+)$`, 0, domain.RouteAction{
		Type: domain.RouteRedirect, Target: "/shop/$1", Status: 302,
	}); err != nil {
		t.Fatal(err)
	}
	handler := NewRouter(app)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/products/sneakers", nil))
	if response.Code != http.StatusFound {
		t.Fatalf("status = %d", response.Code)
	}
	if location := response.Header().Get("Location"); location != "/shop/sneakers" {
		t.Fatalf("location = %q", location)
	}
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("decode %q: %v", string(data), err)
	}
}

var _ = bytes.MinRead
