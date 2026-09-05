package unit

import (
	"bytes"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"regexp"
	"strings"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/api/admin"
	"github.com/liapoldus/liapoldus/backend/internal/application/asset"
	"github.com/liapoldus/liapoldus/backend/internal/application/content"
	"github.com/liapoldus/liapoldus/backend/internal/application/form"
	"github.com/liapoldus/liapoldus/backend/internal/application/page"
	"github.com/liapoldus/liapoldus/backend/internal/application/route"
	"github.com/liapoldus/liapoldus/backend/internal/application/site"
	"github.com/liapoldus/liapoldus/backend/internal/application/snapshot"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
)

func newAdminHandlerTestApp(t *testing.T) admin.App {
	t.Helper()
	db := storage.NewMemory()
	blobs, err := storage.NewDiskBlobStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return admin.App{
		Sites: site.NewService(db, site.Settings{DefaultLocale: "ru"}),
		Pages: page.NewService(db, db, page.Settings{
			InitialVersion: 1,
			MaxDepth:       5,
			Types:          map[string]bool{"Container": true, "Text": true},
		}),
		Snapshots: snapshot.NewService(db, db, db),
		Contents:  content.NewService(db),
		Assets: asset.NewService(db, blobs, db, asset.Settings{
			MasterVariant: "master",
			FallbackName:  "asset",
			FallbackMime:  "application/octet-stream",
			URLTemplate:   "/api/assets/{id}/file",
		}),
		Routes: route.NewService(db, route.Settings{
			DefaultStatus: 301,
			Allowed:       map[int]bool{301: true, 302: true},
		}),
		Forms:  form.NewService(db, db, form.Settings{EmailPattern: emailPattern}),
		Logger: slog.Default(),
	}
}

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func TestSiteAndPageFlow(t *testing.T) {
	handler := admin.NewRouter(newAdminHandlerTestApp(t))

	siteResponse := request(t, handler, http.MethodPost, "/api/sites", map[string]any{"name": "Demo", "slug": "demo"})
	if siteResponse.Code != http.StatusCreated {
		t.Fatalf("create site status = %d", siteResponse.Code)
	}
	var created struct {
		ID string `json:"id"`
	}
	decodeResponse(t, siteResponse, &created)

	pageResponse := request(t, handler, http.MethodPost, "/api/sites/"+created.ID+"/pages", map[string]any{
		"name": "Home", "slug": "home", "root": map[string]any{
			"id": "root", "type": "Container", "children": []any{map[string]any{"id": "title", "type": "Text", "props": map[string]any{"text": "Hello"}}},
		},
	})
	if pageResponse.Code != http.StatusCreated {
		t.Fatalf("create page status = %d", pageResponse.Code)
	}
	var createdPage struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
	}
	decodeResponse(t, pageResponse, &createdPage)
	if createdPage.Version != 1 {
		t.Fatalf("initial page version = %d, want 1", createdPage.Version)
	}

	updateResponse := request(t, handler, http.MethodPut, "/api/pages/"+createdPage.ID+"/tree", map[string]any{
		"root": map[string]any{"id": "root", "type": "Container", "children": []any{}},
	})
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update tree status = %d", updateResponse.Code)
	}
	decodeResponse(t, updateResponse, &createdPage)
	if createdPage.Version != 2 {
		t.Fatalf("updated page version = %d, want 2", createdPage.Version)
	}

	snapshotResponse := request(t, handler, http.MethodPost, "/api/sites/"+created.ID+"/snapshots", map[string]any{"name": "Initial"})
	if snapshotResponse.Code != http.StatusCreated {
		t.Fatalf("create snapshot status = %d", snapshotResponse.Code)
	}
	var snapshot struct {
		Pages []struct {
			PageID  string `json:"pageId"`
			Version int    `json:"version"`
		} `json:"pages"`
	}
	decodeResponse(t, snapshotResponse, &snapshot)
	if len(snapshot.Pages) != 1 || snapshot.Pages[0].PageID != createdPage.ID || snapshot.Pages[0].Version != 2 {
		t.Fatalf("snapshot pages = %#v, want page version 2", snapshot.Pages)
	}
}

func TestInvalidComponentIsRejected(t *testing.T) {
	handler := admin.NewRouter(newAdminHandlerTestApp(t))
	var created struct {
		ID string `json:"id"`
	}
	decodeResponse(t, request(t, handler, http.MethodPost, "/api/sites", map[string]any{"name": "Demo", "slug": "demo"}), &created)
	response := request(t, handler, http.MethodPost, "/api/sites/"+created.ID+"/pages", map[string]any{
		"name": "Broken", "slug": "broken", "root": map[string]any{"id": "root", "type": "Unknown"},
	})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid component status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestContentTranslationMerge(t *testing.T) {
	handler := admin.NewRouter(newAdminHandlerTestApp(t))
	var created struct {
		ID string `json:"id"`
	}
	decodeResponse(t, request(t, handler, http.MethodPost, "/api/sites", map[string]any{"name": "Demo", "slug": "demo"}), &created)

	createContent := request(t, handler, http.MethodPost, "/api/sites/"+created.ID+"/contents", map[string]any{
		"collectionId": "col.articles", "id": "a1",
		"fields": map[string]any{"title": "Hello", "description": "Base description"},
	})
	if createContent.Code != http.StatusCreated {
		t.Fatalf("create content status = %d", createContent.Code)
	}

	putTranslation := request(t, handler, http.MethodPut, "/api/sites/"+created.ID+"/contents/a1/translations/ru", map[string]any{
		"fields": map[string]any{"title": "Привет"},
	})
	if putTranslation.Code != http.StatusOK {
		t.Fatalf("set translation status = %d", putTranslation.Code)
	}

	var decoded struct {
		Translations map[string]map[string]any `json:"translations"`
	}
	decodeResponse(t, request(t, handler, http.MethodGet, "/api/sites/"+created.ID+"/contents/a1", nil), &decoded)
	if _, ok := decoded.Translations["ru"]; !ok {
		t.Fatalf("translations = %#v, want ru entry", decoded.Translations)
	}
}

func TestAssetUploadAndDelete(t *testing.T) {
	handler := admin.NewRouter(newAdminHandlerTestApp(t))
	var created struct {
		ID string `json:"id"`
	}
	decodeResponse(t, request(t, handler, http.MethodPost, "/api/sites", map[string]any{"name": "Demo", "slug": "demo"}), &created)

	body := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(body)
	part, err := multipartWriter.CreatePart(textproto.MIMEHeader{"Content-Disposition": {"form-data; name=\"file\"; filename=\"hello.txt\""}, "Content-Type": {"text/plain"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("hello world")); err != nil {
		t.Fatal(err)
	}
	if err := multipartWriter.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/sites/"+created.ID+"/assets", body)
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusCreated {
		t.Fatalf("upload asset status = %d", response.Code)
	}
	var asset struct {
		ID       string `json:"id"`
		Mime     string `json:"mime"`
		Size     int64  `json:"size"`
		Variants []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"variants"`
	}
	decodeResponse(t, response, &asset)
	if asset.Mime != "text/plain" || asset.Size != 11 {
		t.Fatalf("asset metadata = %#v", asset)
	}
	if len(asset.Variants) != 1 || asset.Variants[0].Name != "master" || !strings.HasSuffix(asset.Variants[0].URL, "/file") {
		t.Fatalf("asset variants = %#v", asset.Variants)
	}

	fileResponse := httptest.NewRecorder()
	handler.ServeHTTP(fileResponse, httptest.NewRequest(http.MethodGet, "/api/assets/"+asset.ID+"/file", nil))
	if fileResponse.Code != http.StatusOK {
		t.Fatalf("get asset file status = %d", fileResponse.Code)
	}
	if got := fileResponse.Body.String(); got != "hello world" {
		t.Fatalf("asset bytes = %q", got)
	}
	if fileResponse.Header().Get("ETag") == "" {
		t.Fatal("asset file missing ETag")
	}

	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, httptest.NewRequest(http.MethodDelete, "/api/assets/"+asset.ID, nil))
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete asset status = %d", deleteResponse.Code)
	}
}

func TestRouteValidation(t *testing.T) {
	handler := admin.NewRouter(newAdminHandlerTestApp(t))
	var created struct {
		ID string `json:"id"`
	}
	decodeResponse(t, request(t, handler, http.MethodPost, "/api/sites", map[string]any{"name": "Demo", "slug": "demo"}), &created)

	badStatus := request(t, handler, http.MethodPost, "/api/sites/"+created.ID+"/routes", map[string]any{
		"matcher": "^/old$", "priority": 0,
		"action": map[string]any{"type": "redirect", "target": "/new", "status": 200},
	})
	if badStatus.Code != http.StatusBadRequest {
		t.Fatalf("invalid redirect status = %d, want 400", badStatus.Code)
	}

	okRoute := request(t, handler, http.MethodPost, "/api/sites/"+created.ID+"/routes", map[string]any{
		"matcher": "^/old$", "priority": 0,
		"action": map[string]any{"type": "redirect", "target": "/new"},
	})
	if okRoute.Code != http.StatusCreated {
		t.Fatalf("create route status = %d", okRoute.Code)
	}
}
