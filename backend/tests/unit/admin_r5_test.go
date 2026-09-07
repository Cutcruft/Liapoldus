package unit

import (
	"context"
	"net/http"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/api/admin"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

const cardSourceV1 = `export default function Card({ title }: { title: string }) { return <div>{title}</div> }`

const cardSourceV2 = `export default function Card({ title, subtitle }: { title: string; subtitle?: string }) { return <section>{title}{subtitle}</section> }`

func r5CardRequest() map[string]any {
	return map[string]any{
		"id":        "card",
		"name":      "Карточка",
		"kind":      "component",
		"isSection": true,
		"source":    cardSourceV1,
		"schema": map[string]any{
			"type":       "object",
			"required":   []any{"title"},
			"properties": map[string]any{"title": map[string]any{"type": "string"}},
		},
	}
}

func TestAdminComponentRegistryCommitHistory(t *testing.T) {
	app, _ := newAdminHandlerTestAppDB(t)
	ctx := context.Background()
	site, err := app.Sites.Create(ctx, "Сайт", "site", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := admin.NewRouter(app)
	base := "/api/sites/" + site.ID + "/components"

	// Define a component; before any commit it must be uncommitted.
	define := request(t, handler, http.MethodPost, base, r5CardRequest())
	if define.Code != http.StatusCreated {
		t.Fatalf("define status = %d, body = %s", define.Code, define.Body.String())
	}

	reg := request(t, handler, http.MethodGet, base+"/registry", nil)
	if reg.Code != http.StatusOK {
		t.Fatalf("registry status = %d, body = %s", reg.Code, reg.Body.String())
	}
	var registry struct {
		DevSHA     string `json:"devSha"`
		Components []struct {
			ID        string `json:"id"`
			Source    string `json:"source"`
			Committed bool   `json:"committed"`
		} `json:"components"`
	}
	decodeResponse(t, reg, &registry)
	if len(registry.Components) != 1 || registry.Components[0].ID != "card" || registry.Components[0].Committed {
		t.Fatalf("registry uncommitted state wrong: %#v", registry)
	}
	if registry.DevSHA != "" {
		t.Fatalf("devSha must be empty before a commit, got %q", registry.DevSHA)
	}

	// Commit: full-site snapshot, definitions marked at the produced sha.
	commit := request(t, handler, http.MethodPost, base+"/card/commit", map[string]any{"message": "первая версия"})
	if commit.Code != http.StatusOK {
		t.Fatalf("commit status = %d, body = %s", commit.Code, commit.Body.String())
	}
	var committed struct {
		SHA   string `json:"sha"`
		Count int    `json:"count"`
	}
	decodeResponse(t, commit, &committed)
	if committed.SHA == "" || committed.Count != 1 {
		t.Fatalf("commit response wrong: %#v", committed)
	}

	reg = request(t, handler, http.MethodGet, base+"/registry", nil)
	decodeResponse(t, reg, &registry)
	if len(registry.Components) != 1 || !registry.Components[0].Committed || registry.DevSHA != committed.SHA {
		t.Fatalf("registry committed state wrong: %#v (dev %q)", registry, committed.SHA)
	}

	// Get carries the source + committed flag.
	get := request(t, handler, http.MethodGet, base+"/card", nil)
	if get.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", get.Code, get.Body.String())
	}
	var got struct {
		ID         string `json:"id"`
		Source     string `json:"source"`
		CurrentSHA string `json:"currentSha"`
		Committed  bool   `json:"committed"`
	}
	decodeResponse(t, get, &got)
	if got.Source != cardSourceV1 || !got.Committed || got.CurrentSHA != committed.SHA {
		t.Fatalf("get response wrong: %#v", got)
	}

	// Editing the source marks the component dirty again.
	update := request(t, handler, http.MethodPut, base+"/card", map[string]any{
		"name":   "Карточка v2",
		"kind":   "component",
		"source": cardSourceV2,
		"schema": map[string]any{
			"type":       "object",
			"required":   []any{"title"},
			"properties": map[string]any{"title": map[string]any{"type": "string"}, "subtitle": map[string]any{"type": "string"}},
		},
	})
	if update.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", update.Code, update.Body.String())
	}
	reg = request(t, handler, http.MethodGet, base+"/registry", nil)
	decodeResponse(t, reg, &registry)
	if len(registry.Components) != 1 || registry.Components[0].Committed {
		t.Fatalf("component must be uncommitted after edit: %#v", registry)
	}

	// Second commit produces a new sha and a 2-entry history (per-source).
	commit2 := request(t, handler, http.MethodPost, base+"/card/commit", map[string]any{"message": "вторая версия"})
	decodeResponse(t, commit2, &committed)
	if committed.Count != 1 {
		t.Fatalf("second commit count = %d, want 1", committed.Count)
	}

	hist := request(t, handler, http.MethodGet, base+"/card/history", nil)
	if hist.Code != http.StatusOK {
		t.Fatalf("history status = %d, body = %s", hist.Code, hist.Body.String())
	}
	var history []struct {
		SHA     string `json:"sha"`
		Message string `json:"message"`
		Source  string `json:"source"`
	}
	decodeResponse(t, hist, &history)
	if len(history) != 2 {
		t.Fatalf("history = %d entries, want 2 (both sources)", len(history))
	}
	if history[0].Source != cardSourceV2 || history[1].Source != cardSourceV1 {
		t.Fatalf("history order wrong: newest first v2 then v1, got %q then %q", history[0].Source, history[1].Source)
	}

	// Historical read by sha returns the version captured by that commit.
	atSHA := request(t, handler, http.MethodGet, base+"/card?sha="+history[1].SHA, nil)
	if atSHA.Code != http.StatusOK {
		t.Fatalf("get@sha status = %d, body = %s", atSHA.Code, atSHA.Body.String())
	}
	var historic struct {
		Source string `json:"source"`
		Name   string `json:"name"`
	}
	decodeResponse(t, atSHA, &historic)
	if historic.Source != cardSourceV1 {
		t.Fatalf("historic source = %q, want v1", historic.Source)
	}
	if historic.Name != "Карточка v2" {
		t.Fatalf("historic name must come from current registry, got %q", historic.Name)
	}
}

func TestAdminComponentUsage(t *testing.T) {
	app, _ := newAdminHandlerTestAppDB(t)
	ctx := context.Background()
	site, err := app.Sites.Create(ctx, "Сайт", "site", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler := admin.NewRouter(app)
	base := "/api/sites/" + site.ID + "/components"

	define := request(t, handler, http.MethodPost, base, r5CardRequest())
	if define.Code != http.StatusCreated {
		t.Fatalf("define status = %d, body = %s", define.Code, define.Body.String())
	}

	list := []domain.Element{
		{ID: "r1", ComponentID: "card", Props: map[string]domain.ElementProp{"title": {Kind: "literal", Value: "Главная"}}},
		{ID: "c1", ComponentID: "card", Props: map[string]domain.ElementProp{"title": {Kind: "literal", Value: "Вложенная"}}},
	}
	createPage := request(t, handler, http.MethodPost, "/api/sites/"+site.ID+"/pages", map[string]any{
		"name": "Главная",
		"slug": "/",
		"list": list,
	})
	if createPage.Code != http.StatusCreated {
		t.Fatalf("create page status = %d, body = %s", createPage.Code, createPage.Body.String())
	}

	usage := request(t, handler, http.MethodGet, base+"/card/usage", nil)
	if usage.Code != http.StatusOK {
		t.Fatalf("usage status = %d, body = %s", usage.Code, usage.Body.String())
	}
	var used struct {
		ComponentID string `json:"componentId"`
		Pages       []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"pages"`
	}
	decodeResponse(t, usage, &used)
	if used.ComponentID != "card" || len(used.Pages) != 1 {
		t.Fatalf("usage wrong: %#v", used)
	}
	if used.Pages[0].Count != 2 || used.Pages[0].Name != "Главная" {
		t.Fatalf("usage page wrong: %#v", used.Pages[0])
	}

	// The registry reflects the reference count too, and an unknown component
	// is a 404.
	reg := request(t, handler, http.MethodGet, base+"/registry", nil)
	var registry struct {
		Components []struct {
			ID         string `json:"id"`
			UsageCount int    `json:"usageCount"`
		} `json:"components"`
	}
	decodeResponse(t, reg, &registry)
	if len(registry.Components) != 1 || registry.Components[0].UsageCount != 2 {
		t.Fatalf("registry usage count wrong: %#v", registry)
	}

	missing := request(t, handler, http.MethodGet, base+"/ghost/usage", nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown component usage status = %d, want 404", missing.Code)
	}
}
