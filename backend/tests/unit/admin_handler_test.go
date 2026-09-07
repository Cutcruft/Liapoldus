package unit

import (
	"bytes"
	"context"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/api/admin"
	"github.com/liapoldus/liapoldus/backend/internal/application/asset"
	"github.com/liapoldus/liapoldus/backend/internal/application/component"
	"github.com/liapoldus/liapoldus/backend/internal/application/content"
	deployapp "github.com/liapoldus/liapoldus/backend/internal/application/deploy"
	"github.com/liapoldus/liapoldus/backend/internal/application/form"
	"github.com/liapoldus/liapoldus/backend/internal/application/gitsnapshot"
	"github.com/liapoldus/liapoldus/backend/internal/application/infra"
	"github.com/liapoldus/liapoldus/backend/internal/application/page"
	"github.com/liapoldus/liapoldus/backend/internal/application/route"
	"github.com/liapoldus/liapoldus/backend/internal/application/site"
	"github.com/liapoldus/liapoldus/backend/internal/application/snapshot"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	gitrepo "github.com/liapoldus/liapoldus/backend/internal/infra/git"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
)

func newAdminHandlerTestApp(t *testing.T) admin.App {
	t.Helper()
	app, _ := newAdminHandlerTestAppDB(t)
	return app
}

func newAdminHandlerTestAppDB(t *testing.T) (admin.App, domain.Storage) {
	t.Helper()
	db := storage.NewMemory()
	return gitTestAppWithDir(t, db, t.TempDir()), db
}

// gitTestAppWithDir wires the full admin app against db and a git repo rooted
// at gitDir, so two apps can share one repository for commit/restore tests.
func gitTestAppWithDir(t *testing.T, db domain.Storage, gitDir string) admin.App {
	t.Helper()
	blobs, err := storage.NewDiskBlobStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitRepo := gitrepo.NewRepo(gitDir)
	return admin.App{
		Sites: site.NewService(db, site.Settings{DefaultLocale: "ru"}),
		Pages: page.NewService(db, db, db, page.Settings{
			InitialVersion: 1,
			MaxElements:    500,
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
		Forms:      form.NewService(db, db, form.Settings{EmailPattern: emailPattern}),
		Components: component.NewService(db),
		Infra:      infra.NewService(db, db),
		Deploys: deployapp.NewService(db, db, db, &fakeReleaser{func(_ context.Context, siteID, snapshotID, environment string) (domain.Build, error) {
			return domain.Build{ID: "build_" + snapshotID, SiteID: siteID, SnapshotID: snapshotID, Environment: environment, Status: domain.BuildStatusReady, CreatedAt: time.Now().UTC()}, nil
		}}),
		Git: gitsnapshot.NewService(
			gitRepo, db, db, db, db, db, db, db, db, nil,
		),
		Logger: slog.Default(),
	}
}

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// seedSiteDefs registers component definitions so page trees referencing them
// validate. siteID must already exist in the store.
func seedSiteDefs(t *testing.T, db domain.Storage, siteID string) {
	t.Helper()
	ctx := context.Background()
	for id, schema := range map[string]string{
		"Container": `{"type":"object","properties":{"gap":{"type":"number"}}}`,
		"Text":      `{"type":"object","required":["text"],"properties":{"text":{"type":"string"}}}`,
	} {
		if err := db.Save(ctx, &domain.ComponentDefinition{
			SiteID:    siteID,
			ID:        id,
			Name:      id,
			Kind:      "component",
			IsSection: true,
			Source:    "export default () => null;",
			Schema:    mustJSONMap(schema),
			Metadata:  map[string]any{"label": id},
			CreatedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatalf("seed component %s: %v", id, err)
		}
	}
}

// seedShellDef registers a section whose acceptsPageContent is true, so it can
// be referenced as a page layout (page.layoutSectionId / site default).
func seedShellDef(t *testing.T, db domain.Storage, siteID string) {
	t.Helper()
	if err := db.Save(context.Background(), &domain.ComponentDefinition{
		SiteID:             siteID,
		ID:                 "Shell",
		Name:               "Shell",
		Kind:               "component",
		IsSection:          true,
		AcceptsPageContent: true,
		Source:             "export default () => null;",
		Schema:             mustJSONMap(`{"type":"object","properties":{"slot":{"type":"string"}}}`),
		Metadata:           map[string]any{"label": "Shell"},
		CreatedAt:          time.Now().UTC(),
	}); err != nil {
		t.Fatalf("seed layout shell: %v", err)
	}
}

func TestSiteAndPageFlow(t *testing.T) {
	app, db := newAdminHandlerTestAppDB(t)
	handler := admin.NewRouter(app)

	siteResponse := request(t, handler, http.MethodPost, "/api/sites", map[string]any{"name": "Demo", "slug": "demo"})
	if siteResponse.Code != http.StatusCreated {
		t.Fatalf("create site status = %d", siteResponse.Code)
	}
	var created struct {
		ID string `json:"id"`
	}
	decodeResponse(t, siteResponse, &created)
	seedSiteDefs(t, db, created.ID)
	seedShellDef(t, db, created.ID)

	pageResponse := request(t, handler, http.MethodPost, "/api/sites/"+created.ID+"/pages", map[string]any{
		"name": "Home", "slug": "home",
		"layoutSectionId": "Shell",
		"head": map[string]any{
			"title": "Home", "description": "Сайт", "robots": "index, follow",
			"canonical": "https://demo.test/",
			"og":        map[string]string{"og:type": "website"},
			"meta":      map[string]string{"theme-color": "#000"},
		},
		"list": []any{
			map[string]any{"componentId": "Container", "props": map[string]any{"gap": map[string]any{"kind": "literal", "value": 8}}},
			map[string]any{"componentId": "Text", "props": map[string]any{"text": map[string]any{"kind": "literal", "value": "Hello"}}},
		},
	})
	if pageResponse.Code != http.StatusCreated {
		t.Fatalf("create page status = %d", pageResponse.Code)
	}
	var createdPage struct {
		ID              string          `json:"id"`
		Version         int             `json:"version"`
		LayoutSectionID string          `json:"layoutSectionId"`
		Head            domain.PageHead `json:"head"`
	}
	decodeResponse(t, pageResponse, &createdPage)
	if createdPage.Version != 1 {
		t.Fatalf("initial page version = %d, want 1", createdPage.Version)
	}
	if createdPage.LayoutSectionID != "Shell" {
		t.Fatalf("created layoutSectionId = %q, want Shell", createdPage.LayoutSectionID)
	}
	if createdPage.Head.Title != "Home" || createdPage.Head.Meta["theme-color"] != "#000" {
		t.Fatalf("created head = %#v", createdPage.Head)
	}

	updateResponse := request(t, handler, http.MethodPut, "/api/pages/"+createdPage.ID, map[string]any{
		"name": "Home", "list": []any{},
		"layoutSectionId": "Shell",
		"head": map[string]any{
			"title": "Updated", "og": map[string]string{"og:type": "article"},
		},
	})
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update page status = %d", updateResponse.Code)
	}
	decodeResponse(t, updateResponse, &createdPage)
	if createdPage.Version != 2 {
		t.Fatalf("updated page version = %d, want 2", createdPage.Version)
	}
	if createdPage.LayoutSectionID != "Shell" {
		t.Fatalf("updated page lost layoutSectionId, got %q", createdPage.LayoutSectionID)
	}
	if createdPage.Head.Title != "Updated" || createdPage.Head.OG["og:type"] != "article" {
		t.Fatalf("updated head = %#v", createdPage.Head)
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

	// Version history carries the flat element list per version (§1.3/§3.7)
	// plus the layout override and head pinned at the moment of the write.
	type versionEntry struct {
		ID              string          `json:"id"`
		Number          int             `json:"number"`
		LayoutSectionID string          `json:"layoutSectionId"`
		Head            domain.PageHead `json:"head"`
		List            []any           `json:"list"`
	}
	var versions []versionEntry
	listVersionsResponse := request(t, handler, http.MethodGet, "/api/pages/"+createdPage.ID+"/versions", nil)
	if listVersionsResponse.Code != http.StatusOK {
		t.Fatalf("list versions status = %d", listVersionsResponse.Code)
	}
	decodeResponse(t, listVersionsResponse, &versions)
	if len(versions) != 2 {
		t.Fatalf("versions = %#v, want 2", versions)
	}
	if versions[0].Number != 1 || len(versions[0].List) != 2 {
		t.Fatalf("version 1 = %#v, want list with 2 elements", versions[0])
	}
	if versions[0].LayoutSectionID != "Shell" || versions[0].Head.Title != "Home" {
		t.Fatalf("version 1 layout/head not pinned: %#v", versions[0])
	}
	if versions[1].Number != 2 || len(versions[1].List) != 0 {
		t.Fatalf("version 2 = %#v, want empty list", versions[1])
	}
	if versions[1].LayoutSectionID != "Shell" || versions[1].Head.Title != "Updated" || versions[1].Head.OG["og:type"] != "article" {
		t.Fatalf("version 2 layout/head not pinned: %#v", versions[1])
	}

	// GetVersion returns the pinned list, not a tree.
	var single versionEntry
	getVersionResponse := request(t, handler, http.MethodGet, "/api/pages/"+createdPage.ID+"/versions/"+versions[0].ID, nil)
	if getVersionResponse.Code != http.StatusOK {
		t.Fatalf("get version status = %d", getVersionResponse.Code)
	}
	decodeResponse(t, getVersionResponse, &single)
	if len(single.List) != 2 || single.Number != 1 {
		t.Fatalf("get version = %#v, want number 1 with 2 elements", single)
	}
	if single.Head.Title != "Home" || single.LayoutSectionID != "Shell" {
		t.Fatalf("get version layout/head = %#v", single)
	}
}

func TestComponentsListCatalog(t *testing.T) {
	app, db := newAdminHandlerTestAppDB(t)
	handler := admin.NewRouter(app)
	var created struct {
		ID string `json:"id"`
	}
	decodeResponse(t, request(t, handler, http.MethodPost, "/api/sites", map[string]any{"name": "Demo", "slug": "demo"}), &created)
	seedSiteDefs(t, db, created.ID)

	var catalog []struct {
		Type      string         `json:"type"`
		Label     string         `json:"label"`
		Container bool           `json:"container"`
		Schema    map[string]any `json:"schema"`
	}
	decodeResponse(t, request(t, handler, http.MethodGet, "/api/sites/"+created.ID+"/components", nil), &catalog)

	want := []struct {
		Type      string
		Container bool
	}{
		{"Container", true},
		{"Text", false},
		{"Image", false},
		{"Button", false},
		// Seeded site definitions are appended after the builtins.
		{"Container", false},
		{"Text", false},
	}
	if len(catalog) != len(want) {
		t.Fatalf("catalog len = %d, want %d (%#v)", len(catalog), len(want), catalog)
	}
	for i, w := range want {
		if catalog[i].Type != w.Type || catalog[i].Container != w.Container {
			t.Fatalf("catalog[%d] = {type:%q container:%v}, want {type:%q container:%v}", i, catalog[i].Type, catalog[i].Container, w.Type, w.Container)
		}
	}
	// Every entry is inspector-ready.
	for _, c := range catalog {
		if c.Schema == nil {
			t.Fatalf("component %q missing schema", c.Type)
		}
		if c.Label == "" {
			t.Fatalf("component %q missing label", c.Type)
		}
	}
}

func TestInvalidComponentIsRejected(t *testing.T) {
	app, db := newAdminHandlerTestAppDB(t)
	handler := admin.NewRouter(app)
	var created struct {
		ID string `json:"id"`
	}
	decodeResponse(t, request(t, handler, http.MethodPost, "/api/sites", map[string]any{"name": "Demo", "slug": "demo"}), &created)
	seedSiteDefs(t, db, created.ID)
	response := request(t, handler, http.MethodPost, "/api/sites/"+created.ID+"/pages", map[string]any{
		"name": "Broken", "slug": "broken", "list": []any{
			map[string]any{"componentId": "Unknown", "props": map[string]any{}},
		},
	})
	if response.Code != http.StatusNotFound {
		t.Fatalf("invalid component status = %d, want %d", response.Code, http.StatusNotFound)
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

	for _, matcher := range []string{"/old", "^/old", "/old$", "^old"} {
		badMatcher := request(t, handler, http.MethodPost, "/api/sites/"+created.ID+"/routes", map[string]any{
			"matcher": matcher, "priority": 0,
			"action": map[string]any{"type": "redirect", "target": "/new"},
		})
		if badMatcher.Code != http.StatusBadRequest {
			t.Fatalf("matcher %q: status = %d, want 400", matcher, badMatcher.Code)
		}
	}

	badRegex := request(t, handler, http.MethodPost, "/api/sites/"+created.ID+"/routes", map[string]any{
		"matcher": "^/old(]", "priority": 0,
		"action": map[string]any{"type": "redirect", "target": "/new"},
	})
	if badRegex.Code != http.StatusBadRequest {
		t.Fatalf("invalid regex matcher = %d, want 400", badRegex.Code)
	}
}

func TestGitCommitRestoreCarriesPageHeadAndLayout(t *testing.T) {
	ctx := context.Background()
	gitDir := t.TempDir()

	sourceDB := storage.NewMemory()
	source := gitTestAppWithDir(t, sourceDB, gitDir)
	site, err := source.Sites.Create(ctx, "Git", "git-site", "", nil)
	if err != nil {
		t.Fatalf("create site: %v", err)
	}
	seedSiteDefs(t, sourceDB, site.ID)
	seedShellDef(t, sourceDB, site.ID)

	head := domain.PageHead{
		Title: "Главная", Description: "Описание", Robots: "noindex",
		Canonical: "https://git.test/", OG: map[string]string{"og:type": "website"},
		Meta: map[string]string{"theme-color": "#123456"},
	}
	page, err := source.Pages.Create(ctx, site.ID, "Главная", "index", []domain.Element{
		{ID: "root", ComponentID: "Shell", Props: map[string]domain.ElementProp{}},
	}, "Shell", head)
	if err != nil {
		t.Fatalf("create page with head: %v", err)
	}

	sha, err := source.Git.Commit(ctx, site.ID, "snapshot with head")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	targetDB := storage.NewMemory()
	target := gitTestAppWithDir(t, targetDB, gitDir)
	// Restore operates on an existing site record; reuse the source site id so
	// the pinned page references resolve.
	if err := targetDB.CreateSite(ctx, domain.Site{ID: site.ID, Name: site.Name, Slug: site.Slug, DefaultLocale: site.DefaultLocale, CreatedAt: site.CreatedAt}); err != nil {
		t.Fatalf("recreate site on target: %v", err)
	}
	if _, err := target.Git.Restore(ctx, site.ID, sha, "restore head"); err != nil {
		t.Fatalf("restore: %v", err)
	}

	got, err := target.Pages.Get(ctx, page.ID)
	if err != nil {
		t.Fatalf("get restored page: %v", err)
	}
	if got.LayoutSectionID != "Shell" {
		t.Fatalf("restored layoutSectionId = %q, want Shell", got.LayoutSectionID)
	}
	if !reflect.DeepEqual(got.Head, head) {
		t.Fatalf("restored head = %#v, want %#v", got.Head, head)
	}

	versions, err := target.Pages.Versions(ctx, page.ID)
	if err != nil {
		t.Fatalf("list restored versions: %v", err)
	}
	if len(versions) != 1 || versions[0].LayoutSectionID != "Shell" || !reflect.DeepEqual(versions[0].Head, head) {
		t.Fatalf("restored version head/layout = %#v", versions)
	}
}
