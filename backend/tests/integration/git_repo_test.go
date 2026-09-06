package integrationtest

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/application/component"
	"github.com/liapoldus/liapoldus/backend/internal/application/git"
	"github.com/liapoldus/liapoldus/backend/internal/application/page"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
	"github.com/liapoldus/liapoldus/backend/internal/schema"
)

func gitRepo(t *testing.T) (*git.Repo, *storage.Memory, *git.Service) {
	t.Helper()
	mem := storage.NewMemory()
	repo := git.NewRepo(t.TempDir())
	return repo, mem, git.NewService(repo, mem)
}

// release commits a component through the service and returns its sha.
func gitRelease(t *testing.T, svc *git.Service, siteID, id, name, source string) string {
	t.Helper()
	v, err := svc.Release(context.Background(), siteID, id, name, source,
		map[string]any{"type": "object"},
		map[string]any{"label": name})
	if err != nil {
		t.Fatalf("release %s: %v", id, err)
	}
	return v.ID
}

func TestGitRepoInitCreatesBare(t *testing.T) {
	repo, _, _ := gitRepo(t)
	siteID := "site-bare"

	if err := repo.Init(context.Background(), siteID); err != nil {
		t.Fatalf("init: %v", err)
	}
	dir := repo.Dir(siteID)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("bare repository must exist on disk at %s: %v", dir, err)
	}
	// HEAD present => valid bare gitdir.
	if _, err := os.Stat(filepath.Join(dir, "HEAD")); err != nil {
		t.Fatalf("bare repo HEAD missing: %v", err)
	}
	if err := repo.Init(context.Background(), siteID); err != nil {
		t.Fatalf("repeat init must be ok, got %v", err)
	}
}

func TestGitRepoTwoComponentsTwoCommits(t *testing.T) {
	repo, _, svc := gitRepo(t)
	siteID := "site-two"
	if err := repo.Init(context.Background(), siteID); err != nil {
		t.Fatal(err)
	}
	shaCard := gitRelease(t, svc, siteID, "card", "Card", "src-card-1")
	shaLayout := gitRelease(t, svc, siteID, "layout.main", "Layout", "src-layout-1")
	if shaCard == shaLayout {
		t.Fatal("distinct definitions must produce distinct shas")
	}

	files, err := svc.CheckoutVersion(context.Background(), siteID, shaCard)
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	for _, name := range []string{git.FileDefinition, git.FileSchema, git.FileMetadata} {
		if _, ok := files[name]; !ok {
			t.Fatalf("commit must contain %s, got %v", name, files)
		}
	}
	if string(files[git.FileDefinition]) != "src-card-1" {
		t.Fatalf("definition source = %q", files[git.FileDefinition])
	}
}

func TestGitRepoListVersionsChronological(t *testing.T) {
	repo, _, svc := gitRepo(t)
	siteID := "site-vers"
	if err := repo.Init(context.Background(), siteID); err != nil {
		t.Fatal(err)
	}
	gitRelease(t, svc, siteID, "card", "Card", "v1")
	gitRelease(t, svc, siteID, "card", "Card", "v2")

	shas, err := repo.ListVersions(context.Background(), siteID, "card")
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if len(shas) != 2 {
		t.Fatalf("want 2 versions, got %d", len(shas))
	}
	if shas[0] == "" {
		t.Fatal("first sha must be non-empty")
	}
	if shas[0] == shas[1] {
		t.Fatal("versions must differ")
	}
}

func TestGitRepoCheckoutByVersion(t *testing.T) {
	repo, _, svc := gitRepo(t)
	siteID := "site-checkout"
	if err := repo.Init(context.Background(), siteID); err != nil {
		t.Fatal(err)
	}
	v1 := gitRelease(t, svc, siteID, "card", "Card", "v1-src")
	gitRelease(t, svc, siteID, "card", "Card", "v2-src")

	files, err := repo.Checkout(context.Background(), siteID, v1)
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if string(files[git.FileDefinition]) != "v1-src" {
		t.Fatalf("checkout returned %q, want v1-src (honest checkout by version)", files[git.FileDefinition])
	}
}

func TestGitRepoReleasedSchemaValid(t *testing.T) {
	_, _, svc := gitRepo(t)
	siteID := "site-schema"
	v := gitRelease(t, svc, siteID, "card", "Card", "v1-src")

	files, err := svc.CheckoutVersion(context.Background(), siteID, v)
	if err != nil {
		t.Fatal(err)
	}
	var defSchema map[string]any
	if err := json.Unmarshal(files[git.FileSchema], &defSchema); err != nil {
		t.Fatalf("schema file is not json: %v", err)
	}
	if err := schema.ValidateSchema(defSchema); err != nil {
		t.Fatalf("committed schema invalid: %v", err)
	}
}

func TestGitRepoRollbackHistoryGrows(t *testing.T) {
	repo, mem, svc := gitRepo(t)
	siteID := "site-rollback"

	base := domain.ComponentDefinition{SiteID: siteID, ID: "card", Name: "Card", Kind: "component", Source: "v1-src"}
	base.Schema = map[string]any{"type": "object"}
	base.Metadata = map[string]any{"label": "Card"}
	if err := mem.Save(context.Background(), &base); err != nil {
		t.Fatal(err)
	}

	v1 := gitRelease(t, svc, siteID, "card", "Card", "v1-src")
	gitRelease(t, svc, siteID, "card", "Card", "v2-src")

	if err := svc.Rollback(context.Background(), siteID, "card", v1); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	stored, err := mem.Get(context.Background(), siteID, "card")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Source != "v1-src" {
		t.Fatalf("registry after rollback = %q, want v1-src", stored.Source)
	}
	if stored.CurrentSHA == v1 {
		t.Fatal("rollback must create a NEW commit, not reuse the old sha")
	}
	history, err := repo.ListVersions(context.Background(), siteID, "card")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("history after rollback = %d commits, want 3 (grows, never truncated)", len(history))
	}
}

func TestGitRepoCheckoutUnknownSha(t *testing.T) {
	repo, _, _ := gitRepo(t)
	siteID := "site-unknown"
	if err := repo.Init(context.Background(), siteID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Checkout(context.Background(), siteID, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"); !errors.Is(err, domain.ErrVersionNotFound) {
		t.Fatalf("want ErrVersionNotFound, got %v", err)
	}
}

func TestGitRepoDeleteKeepsHistory(t *testing.T) {
	repo, mem, svc := gitRepo(t)
	siteID := "site-delete"

	card := domain.ComponentDefinition{SiteID: siteID, ID: "card", Name: "Card", Kind: "component", Source: "v1-src", Schema: schemaObject()}
	if err := mem.Save(context.Background(), &card); err != nil {
		t.Fatal(err)
	}
	gitRelease(t, svc, siteID, "card", "Card", "v1-src")
	gitRelease(t, svc, siteID, "card", "Card", "v2-src")

	if err := mem.Delete(context.Background(), siteID, "card"); err != nil {
		t.Fatal(err)
	}
	history, err := repo.ListVersions(context.Background(), siteID, "card")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("git history must survive registry Delete, got %d commits", len(history))
	}
	if _, err := mem.Get(context.Background(), siteID, "card"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("registry entry must be gone, got %v", err)
	}
}

func TestGitRepoPageTreeStaysStableAcrossReleases(t *testing.T) {
	repo, mem, svc := gitRepo(t)
	siteID := "site-page"

	if err := repo.Init(context.Background(), siteID); err != nil {
		t.Fatal(err)
	}
	card := domain.ComponentDefinition{SiteID: siteID, ID: "card", Name: "Card", Kind: "component", Source: "v1-src", Schema: schemaObject()}
	if err := mem.Save(context.Background(), &card); err != nil {
		t.Fatal(err)
	}
	if err := mem.CreateSite(context.Background(), domain.Site{ID: siteID, Slug: "site-page"}); err != nil {
		t.Fatal(err)
	}
	gitRelease(t, svc, siteID, "card", "Card", "v1-src")

	pageSvc := page.NewService(mem, mem, mem, page.Settings{InitialVersion: 1, MaxDepth: 32, MaxChildren: 100})

	root := domain.ComponentNode{
		InstanceID:   "root",
		DefinitionID: "card",
		Props:        map[string]any{"title": "Hello"},
	}
	p, err := pageSvc.Create(context.Background(), siteID, "Home", "home", root)
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	// Release the next version: the stored page tree must stay exactly as
	// created (reference + props, version-agnostic).
	gitRelease(t, svc, siteID, "card", "Card", "v2-src")

	stored, err := pageSvc.Get(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !jsonEqual(stored.Root, root) {
		t.Fatalf("page tree changed after component release:\n got %+v\nwant %+v", stored.Root, root)
	}
	history, err := repo.ListVersions(context.Background(), siteID, "card")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("history = %d, want 2 (v1 still retriable)", len(history))
	}
}

func TestGitRepoServiceLevelFlowRollbackCanonical(t *testing.T) {
	repo, mem, svc := gitRepo(t)
	siteID := "site-flow"

	compSvc := component.NewService(mem, svc)
	card := componentDefinitionInt(siteID, "card")
	card.Source = "v1-src"
	v1, err := compSvc.Define(context.Background(), card)
	if err != nil {
		t.Fatalf("define card: %v", err)
	}
	if v1.ID == "" {
		t.Fatal("define must return the release sha")
	}
	card.Source = "v2-src"
	if _, err := compSvc.Update(context.Background(), card); err != nil {
		t.Fatalf("update card: %v", err)
	}
	layout := componentDefinitionInt(siteID, "layout.main")
	if _, err := compSvc.Define(context.Background(), layout); err != nil {
		t.Fatalf("define layout.main: %v", err)
	}

	if err := svc.Rollback(context.Background(), siteID, "card", v1.ID); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	stored, err := mem.Get(context.Background(), siteID, "card")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Source != "v1-src" {
		t.Fatalf("final card source = %q, want v1-src (v1 canonical)", stored.Source)
	}
	history, err := repo.ListVersions(context.Background(), siteID, "card")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("final card history = %d, want 3 ([define, update, rollback], canonical)", len(history))
	}
	layoutHistory, err := repo.ListVersions(context.Background(), siteID, "layout.main")
	if err != nil {
		t.Fatal(err)
	}
	if len(layoutHistory) != 1 {
		t.Fatalf("layout.main history = %d, want 1 (untouched by card rollback)", len(layoutHistory))
	}
}

func componentDefinitionInt(siteID, id string) domain.ComponentDefinition {
	return domain.ComponentDefinition{
		SiteID:   siteID,
		ID:       id,
		Name:     id,
		Kind:     "component",
		Source:   "export default () => <div/>",
		Schema:   map[string]any{"type": "object", "properties": map[string]any{"title": map[string]any{"type": "string"}}},
		Metadata: map[string]any{"label": id},
	}
}

func schemaObject() map[string]any {
	return map[string]any{"type": "object"}
}

func jsonEqual(a, b any) bool {
	aj, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bj, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(aj) == string(bj)
}
