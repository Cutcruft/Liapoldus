package integrationtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/application/route"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// getRuntimeJSON performs a GET against a /runtime/* descriptor endpoint on a
// running client server and decodes the JSON body.
func getRuntimeJSON(t *testing.T, url, endpoint, siteID string, q map[string]string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest("GET", url+endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	query := req.URL.Query()
	query.Set("siteId", siteID)
	for k, v := range q {
		if v != "" {
			query.Set(k, v)
		}
	}
	req.URL.RawQuery = query.Encode()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("%s is not JSON: %v\n%s", endpoint, err, body)
	}
	return res.StatusCode, out
}

// TestRuntimeTreeDescriptors covers the Этап 4 `tree.get` builtin on the
// client server: explicit pageId, routeId resolution, the home-page boot
// heuristic, and 404s for pages outside the snapshot.
func TestRuntimeTreeDescriptors(t *testing.T) {
	mem, site, snapshot, builds := runtimeContractFixture(t, "site_rtree")
	routes := route.NewService(mem, route.Settings{DefaultStatus: 301, Allowed: map[int]bool{301: true}})
	srv := runtimeClientServer(t, mem, routes, builds)

	// routeId → the renderPage route pointing at the snapshot page.
	siteRoutes, err := routes.List(context.Background(), site.ID)
	if err != nil || len(siteRoutes) != 1 {
		t.Fatalf("routes = %#v, err %v", siteRoutes, err)
	}

	status, body := getRuntimeJSON(t, srv.URL, "/runtime/tree", site.ID,
		map[string]string{"versionId": snapshot.ID, "routeId": siteRoutes[0].ID})
	if status != http.StatusOK {
		t.Fatalf("tree by routeId: status %d body %#v", status, body)
	}
	tree, _ := body["tree"].(map[string]any)
	if tree["pageId"] != "page_boot" || tree["snapshotId"] != snapshot.ID {
		t.Fatalf("tree by routeId = %#v", tree)
	}
	if root := tree["root"].(map[string]any); root["instanceId"] != "root" || root["definitionId"] != "container" {
		t.Fatalf("tree root = %#v", tree["root"])
	}

	// Explicit pageId.
	status, body = getRuntimeJSON(t, srv.URL, "/runtime/tree", site.ID,
		map[string]string{"versionId": snapshot.ID, "pageId": "page_boot"})
	if status != http.StatusOK || body["tree"].(map[string]any)["pageId"] != "page_boot" {
		t.Fatalf("tree by pageId: status %d body %#v", status, body)
	}

	// No pageId/routeId → the boot home-page heuristic resolves the page.
	status, body = getRuntimeJSON(t, srv.URL, "/runtime/tree", site.ID,
		map[string]string{"versionId": snapshot.ID})
	if status != http.StatusOK || body["tree"].(map[string]any)["pageId"] != "page_boot" {
		t.Fatalf("tree home: status %d body %#v", status, body)
	}

	// A page outside the snapshot is not found.
	status, body = getRuntimeJSON(t, srv.URL, "/runtime/tree", site.ID,
		map[string]string{"pageId": "page_missing"})
	if status != http.StatusNotFound {
		t.Fatalf("unknown pageId status = %d body %#v", status, body)
	}
}

// TestRuntimeRoutesDescriptors asserts /runtime/routes emits the descriptor
// form the boot contract uses (matcher + action), not raw domain routes.
func TestRuntimeRoutesDescriptors(t *testing.T) {
	mem, site, _, builds := runtimeContractFixture(t, "site_rroutes")
	routes := route.NewService(mem, route.Settings{DefaultStatus: 301, Allowed: map[int]bool{301: true}})
	srv := runtimeClientServer(t, mem, routes, builds)

	status, body := getRuntimeJSON(t, srv.URL, "/runtime/routes", site.ID, nil)
	if status != http.StatusOK {
		t.Fatalf("routes status = %d body %#v", status, body)
	}
	arr, _ := body["routes"].([]any)
	if len(arr) != 1 {
		t.Fatalf("routes = %#v", body["routes"])
	}
	routeDesc := arr[0].(map[string]any)
	if routeDesc["matcher"] != "^/$" {
		t.Fatalf("route descriptor = %#v", routeDesc)
	}
	action, _ := routeDesc["action"].(map[string]any)
	if action["type"] != "renderPage" || action["pageId"] != "page_boot" {
		t.Fatalf("route action = %#v", action)
	}
}

// TestRuntimeTokensDescriptors covers the Этап 4 `tokens.get` builtin: the
// site's TokenSet is served as a single default runtime theme.
func TestRuntimeTokensDescriptors(t *testing.T) {
	mem, site, _, builds := runtimeContractFixture(t, "site_rtokens")
	routes := route.NewService(mem, route.Settings{DefaultStatus: 301, Allowed: map[int]bool{301: true}})
	srv := runtimeClientServer(t, mem, routes, builds)

	ctx := context.Background()
	tokens := &domain.TokenSet{
		Colors: []domain.ColorToken{{Name: "accent", Value: map[domain.ColorMode]string{domain.ColorModeLight: "#0b2e4f"}}},
	}
	if err := mem.UpsertTokens(ctx, site.ID, tokens); err != nil {
		t.Fatal(err)
	}

	status, body := getRuntimeJSON(t, srv.URL, "/runtime/tokens", site.ID, nil)
	if status != http.StatusOK {
		t.Fatalf("tokens status = %d body %#v", status, body)
	}
	if body["themeId"] != "default" {
		t.Fatalf("themeId = %#v", body["themeId"])
	}
	tokenMap, _ := body["tokens"].(map[string]any)
	colors, _ := tokenMap["colors"].([]any)
	if len(colors) != 1 {
		t.Fatalf("tokens.colors = %#v", tokenMap["colors"])
	}

	// An explicit themeId passes through (only the default theme exists today).
	status, body = getRuntimeJSON(t, srv.URL, "/runtime/tokens", site.ID, map[string]string{"themeId": "dark"})
	if status != http.StatusOK || body["themeId"] != "dark" {
		t.Fatalf("themeId override: status %d body %#v", status, body)
	}
}