package unit

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/api/admin"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

func TestAdminDeleteSubmission(t *testing.T) {
	app, _ := newAdminHandlerTestAppDB(t)
	ctx := context.Background()
	site, err := app.Sites.Create(ctx, "Сайт", "site", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	form, err := app.Forms.Create(ctx, site.ID, "contact", map[string]any{"fields": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	first, err := app.Forms.Submit(ctx, site.ID, form.ID, map[string]any{"values": map[string]any{"name": "Иван"}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.Forms.Submit(ctx, site.ID, form.ID, map[string]any{"values": map[string]any{"name": "Пётр"}})
	if err != nil {
		t.Fatal(err)
	}

	// A submission that belongs to a different site must not be reachable.
	otherSite, err := app.Sites.Create(ctx, "Другой", "other", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	otherForm, err := app.Forms.Create(ctx, otherSite.ID, "other", map[string]any{"fields": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	otherSub, err := app.Forms.Submit(ctx, otherSite.ID, otherForm.ID, map[string]any{"values": map[string]any{"x": "y"}})
	if err != nil {
		t.Fatal(err)
	}

	handler := admin.NewRouter(app)

	cross := request(t, handler, http.MethodDelete, "/api/sites/"+site.ID+"/forms/"+otherForm.ID+"/submissions/"+otherSub.ID, nil)
	if cross.Code != http.StatusNotFound {
		t.Fatalf("cross-site delete status = %d, want 404", cross.Code)
	}

	del := request(t, handler, http.MethodDelete, "/api/sites/"+site.ID+"/forms/"+form.ID+"/submissions/"+first.ID, nil)
	if del.Code != http.StatusNoContent {
		t.Fatalf("site-scoped delete status = %d, want 204", del.Code)
	}

	delFormScoped := request(t, handler, http.MethodDelete, "/api/forms/"+form.ID+"/submissions/"+second.ID+"?siteId="+site.ID, nil)
	if delFormScoped.Code != http.StatusNoContent {
		t.Fatalf("form-scoped delete status = %d, want 204", delFormScoped.Code)
	}

	list := request(t, handler, http.MethodGet, "/api/sites/"+site.ID+"/forms/"+form.ID+"/submissions", nil)
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", list.Code)
	}
	var subs []domain.Submission
	decodeResponse(t, list, &subs)
	if len(subs) != 0 {
		t.Fatalf("submissions left = %d, want 0", len(subs))
	}

	again := request(t, handler, http.MethodDelete, "/api/forms/"+form.ID+"/submissions/"+first.ID, nil)
	if again.Code != http.StatusNotFound {
		t.Fatalf("repeat delete status = %d, want 404", again.Code)
	}
}

func TestAdminAssetUsage(t *testing.T) {
	app, _ := newAdminHandlerTestAppDB(t)
	ctx := context.Background()
	site, err := app.Sites.Create(ctx, "Сайт", "site", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	uploaded, err := app.Assets.Create(ctx, site.ID, "photo.png", "image/png", bytes.NewReader([]byte("bytes")))
	if err != nil {
		t.Fatal(err)
	}

	referencing, err := app.Contents.Create(ctx, site.ID, "col.news", "n1", map[string]any{
		"title":   "Новость",
		"hero":    uploaded.ID,
		"gallery": []any{uploaded.ID, "/api/assets/" + uploaded.ID + "/file"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Contents.SetTranslation(ctx, site.ID, referencing.ID, "ru", map[string]any{"hero": uploaded.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Contents.Create(ctx, site.ID, "col.news", "n2", map[string]any{"title": "Без картинки"}); err != nil {
		t.Fatal(err)
	}

	referencingForm, err := app.Forms.Create(ctx, site.ID, "contact", map[string]any{
		"fields": []any{map[string]any{"name": "avatar", "type": "asset", "defaultValue": uploaded.ID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Forms.Create(ctx, site.ID, "plain", map[string]any{"fields": []any{}}); err != nil {
		t.Fatal(err)
	}

	handler := admin.NewRouter(app)
	resp := request(t, handler, http.MethodGet, "/api/sites/"+site.ID+"/assets/"+uploaded.ID+"/usage", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("usage status = %d, body = %s", resp.Code, resp.Body.String())
	}
	var usage struct {
		AssetID  string `json:"assetId"`
		Contents []struct {
			ID           string `json:"id"`
			CollectionID string `json:"collectionId"`
		} `json:"contents"`
		Forms []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"forms"`
	}
	decodeResponse(t, resp, &usage)
	if usage.AssetID != uploaded.ID {
		t.Fatalf("assetId = %q, want %q", usage.AssetID, uploaded.ID)
	}
	if len(usage.Contents) != 1 || usage.Contents[0].ID != referencing.ID || usage.Contents[0].CollectionID != "col.news" {
		t.Fatalf("contents = %#v, want single %q", usage.Contents, referencing.ID)
	}
	if len(usage.Forms) != 1 || usage.Forms[0].ID != referencingForm.ID || usage.Forms[0].Name != "contact" {
		t.Fatalf("forms = %#v, want single %q", usage.Forms, referencingForm.ID)
	}

	unused := request(t, handler, http.MethodGet, "/api/sites/"+site.ID+"/assets/does-not-exist/usage", nil)
	if unused.Code != http.StatusNotFound {
		t.Fatalf("unknown asset status = %d, want 404", unused.Code)
	}
}