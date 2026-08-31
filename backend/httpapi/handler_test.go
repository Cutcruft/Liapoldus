package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/liapoldus/liapoldus/backend/page"
	"github.com/liapoldus/liapoldus/backend/site"
	"github.com/liapoldus/liapoldus/backend/snapshot"
	"github.com/liapoldus/liapoldus/backend/store"
)

func TestSiteAndPageFlow(t *testing.T) {
	db := store.NewMemory()
	handler := NewHandler(site.NewService(db), page.NewService(db, db), snapshot.NewService(db, db, db), slog.Default())

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
	db := store.NewMemory()
	handler := NewHandler(site.NewService(db), page.NewService(db, db), snapshot.NewService(db, db, db), slog.Default())
	siteResponse := request(t, handler, http.MethodPost, "/api/sites", map[string]any{"name": "Demo", "slug": "demo"})
	var created struct {
		ID string `json:"id"`
	}
	decodeResponse(t, siteResponse, &created)
	response := request(t, handler, http.MethodPost, "/api/sites/"+created.ID+"/pages", map[string]any{
		"name": "Broken", "slug": "broken", "root": map[string]any{"id": "root", "type": "Unknown"},
	})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid component status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func request(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
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
		t.Fatal(err)
	}
}
