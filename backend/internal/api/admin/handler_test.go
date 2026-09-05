package admin

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/store"
)

func newTestApp(t *testing.T) App {
	t.Helper()
	db := store.NewMemory()
	blobs, err := store.NewDiskBlobStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return App{
		Sites:     domain.NewSiteService(db),
		Pages:     domain.NewPageService(db, db),
		Snapshots: domain.NewSnapshotService(db, db, db),
		Contents:  domain.NewContentService(db),
		Assets:    domain.NewAssetService(db, blobs, db),
		Routes:    domain.NewRouteService(db),
		Forms:     domain.NewFormService(db, db),
		Logger:    slog.Default(),
	}
}

func TestSiteAndPageFlow(t *testing.T) {
	handler := NewRouter(newTestApp(t))

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
	handler := NewRouter(newTestApp(t))
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
	handler := NewRouter(newTestApp(t))
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
	handler := NewRouter(newTestApp(t))
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
	handler := NewRouter(newTestApp(t))
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

func request(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
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
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
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
