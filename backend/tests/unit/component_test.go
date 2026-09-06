package unit

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/application/component"
	"github.com/liapoldus/liapoldus/backend/internal/application/git"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// fakeDefs is an in-memory ComponentDefinitionRepository for tests.
type fakeDefs struct {
	defs    map[string]*domain.ComponentDefinition
	saved   []*domain.ComponentDefinition
	deleted map[string]bool
}

func newFakeDefs() *fakeDefs {
	return &fakeDefs{defs: map[string]*domain.ComponentDefinition{}, deleted: map[string]bool{}}
}

func (f *fakeDefs) key(siteID, id string) string { return siteID + "\x00" + id }

func (f *fakeDefs) Save(_ context.Context, d *domain.ComponentDefinition) error {
	f.defs[f.key(d.SiteID, d.ID)] = d
	f.saved = append(f.saved, d)
	return nil
}

func (f *fakeDefs) Get(_ context.Context, siteID, id string) (*domain.ComponentDefinition, error) {
	d, ok := f.defs[f.key(siteID, id)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return d, nil
}

func (f *fakeDefs) List(_ context.Context, siteID string) ([]domain.ComponentDefinition, error) {
	var out []domain.ComponentDefinition
	for _, d := range f.defs {
		if d.SiteID == siteID {
			out = append(out, *d)
		}
	}
	return out, nil
}

func (f *fakeDefs) Delete(_ context.Context, siteID, id string) error {
	key := f.key(siteID, id)
	if _, ok := f.defs[key]; !ok {
		return domain.ErrNotFound
	}
	delete(f.defs, key)
	f.deleted[key] = true
	return nil
}

type fakeGitCommit struct {
	siteID, definitionID, message string
	files                         map[string][]byte
	sha                           string
}

type fakeGit struct {
	commits     []fakeGitCommit
	inits       []string
	head        string
	initErr     error
	commitErr   error
	checkoutErr error
}

func (f *fakeGit) Init(_ context.Context, siteID string) error {
	if f.initErr != nil {
		return f.initErr
	}
	f.inits = append(f.inits, siteID)
	return nil
}

func (f *fakeGit) Commit(_ context.Context, siteID, definitionID, message string, files map[string][]byte) (string, error) {
	if f.commitErr != nil {
		return "", f.commitErr
	}
	sha := "git-sha" + string(rune('a'+len(f.commits)))
	f.commits = append(f.commits, fakeGitCommit{siteID, definitionID, message, files, sha})
	f.head = sha
	return sha, nil
}

func (f *fakeGit) ListVersions(_ context.Context, _ string, definitionID string) ([]string, error) {
	var out []string
	for _, c := range f.commits {
		if c.definitionID == definitionID {
			out = append(out, c.sha)
		}
	}
	return out, nil
}

func (f *fakeGit) Checkout(_ context.Context, _ string, sha string) (map[string][]byte, error) {
	if f.checkoutErr != nil {
		return nil, f.checkoutErr
	}
	for _, c := range f.commits {
		if c.sha == sha {
			return c.files, nil
		}
	}
	return nil, domain.ErrVersionNotFound
}

func (f *fakeGit) Head(_ context.Context, _ string) (string, error) { return f.head, nil }

func newComponentService(t *testing.T) (*component.Service, *fakeGit, *fakeDefs) {
	t.Helper()
	fg := &fakeGit{}
	fd := newFakeDefs()
	return component.NewService(fd, git.NewService(fg, fd)), fg, fd
}

var componentSchemaFixture = mustJSONMap(`{
	"type": "object",
	"required": ["title"],
	"properties": {"title": {"type": "string"}}
}`)

func mustJSONMap(raw string) map[string]any {
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		panic("bad fixture: " + raw)
	}
	return m
}

func componentDefinition(siteID, id, name string) domain.ComponentDefinition {
	return domain.ComponentDefinition{
		SiteID:   siteID,
		ID:       id,
		Name:     name,
		Kind:     "component",
		Source:   "export default () => <div/>",
		Schema:   componentSchemaFixture,
		Metadata: map[string]any{"label": name},
	}
}

func mustSchema(t *testing.T, raw string) map[string]any {
	t.Helper()
	return decodeSchema(t, raw)
}

func TestComponentDefine(t *testing.T) {
	ctx := context.Background()
	svc, fg, fd := newComponentService(t)
	d := componentDefinition("site_1", "card", "Карточка статьи")

	ver, err := svc.Define(ctx, d)
	if err != nil {
		t.Fatalf("Define: %v", err)
	}
	if ver.ID == "" || ver.DefinitionID != "card" || ver.SiteID != "site_1" {
		t.Fatalf("unexpected version: %+v", ver)
	}
	if len(fg.commits) != 1 {
		t.Fatalf("want 1 commit, got %d", len(fg.commits))
	}
	c := fg.commits[0]
	if c.message != "component card: Карточка статьи" {
		t.Fatalf("commit message = %q", c.message)
	}
	if string(c.files[git.FileDefinition]) != d.Source {
		t.Fatal("definition.tsx does not carry the source")
	}
	var committedSchema map[string]any
	if err := json.Unmarshal(c.files[git.FileSchema], &committedSchema); err != nil {
		t.Fatalf("schema.json invalid: %v", err)
	}
	if !reflect.DeepEqual(committedSchema, d.Schema) {
		t.Fatalf("schema.json mismatch:\n got %v\nwant %v", committedSchema, d.Schema)
	}
	if len(fd.saved) != 1 {
		t.Fatalf("want 1 saved definition, got %d", len(fd.saved))
	}
	if fd.saved[0].CurrentSHA != ver.ID {
		t.Fatalf("saved CurrentSHA = %q, version = %q", fd.saved[0].CurrentSHA, ver.ID)
	}
}

func TestComponentDefineInvalidSchema(t *testing.T) {
	ctx := context.Background()
	svc, fg, fd := newComponentService(t)
	d := componentDefinition("site_1", "card", "Карточка")
	d.Schema = mustSchema(t, `{"type":"object","properties":{"title":{"type":"bogus"}}}`)

	_, err := svc.Define(ctx, d)
	if !errors.Is(err, domain.ErrSchemaInvalid) {
		t.Fatalf("want ErrSchemaInvalid, got %v", err)
	}
	if len(fg.commits) != 0 || len(fd.saved) != 0 {
		t.Fatal("invalid schema must not write anything")
	}
}

func TestComponentDefineDuplicate(t *testing.T) {
	ctx := context.Background()
	svc, fg, _ := newComponentService(t)
	d := componentDefinition("site_1", "card", "Карточка")

	if _, err := svc.Define(ctx, d); err != nil {
		t.Fatalf("first define: %v", err)
	}
	if _, err := svc.Define(ctx, d); !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("want ErrAlreadyExists, got %v", err)
	}
	if len(fg.commits) != 1 {
		t.Fatalf("duplicate define must not commit, got %d commits", len(fg.commits))
	}
}

func TestComponentGet(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newComponentService(t)
	if _, err := svc.Define(ctx, componentDefinition("site_1", "card", "Карточка")); err != nil {
		t.Fatalf("define: %v", err)
	}
	got, err := svc.Get(ctx, "site_1", "card")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "card" || got.Name != "Карточка" {
		t.Fatalf("unexpected definition: %+v", got)
	}
	if _, err := svc.Get(ctx, "site_1", "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestComponentList(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newComponentService(t)
	for _, d := range []domain.ComponentDefinition{
		componentDefinition("site_1", "card", "Карточка"),
		componentDefinition("site_1", "layout.main", "Layout"),
		componentDefinition("site_2", "card", "Карточка"),
	} {
		if _, err := svc.Define(ctx, d); err != nil {
			t.Fatalf("define %s: %v", d.ID, err)
		}
	}
	list, err := svc.List(ctx, "site_1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("site_1: want 2 definitions, got %d", len(list))
	}
	for _, d := range list {
		if d.SiteID != "site_1" {
			t.Fatalf("cross-site leak: %+v", d)
		}
	}
}

func TestComponentUpdate(t *testing.T) {
	ctx := context.Background()
	svc, fg, fd := newComponentService(t)
	if _, err := svc.Define(ctx, componentDefinition("site_1", "card", "Карточка")); err != nil {
		t.Fatalf("define: %v", err)
	}

	updated := componentDefinition("site_1", "card", "Карточка v2")
	updated.Source = "export default () => <section/>"
	updated.Schema = mustSchema(t, `{"type":"object","required":["title","subtitle"],"properties":{"title":{"type":"string"},"subtitle":{"type":"string"}}}`)
	updated.Kind = "" // must be preserved from the existing definition

	ver, err := svc.Update(ctx, updated)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if ver.ID == "git-shaa" {
		t.Fatal("update must produce a new commit")
	}
	if len(fg.commits) != 2 {
		t.Fatalf("want 2 commits, got %d", len(fg.commits))
	}
	if fg.commits[1].message != "component card: Карточка v2" {
		t.Fatalf("commit message = %q", fg.commits[1].message)
	}
	last := fd.saved[len(fd.saved)-1]
	if last.Name != "Карточка v2" || last.Kind != "component" {
		t.Fatalf("saved definition lost data: %+v", last)
	}
	if _, ok := last.Schema["required"]; !ok {
		t.Fatal("schema update did not persist")
	}
}

func TestComponentUpdateNotFound(t *testing.T) {
	ctx := context.Background()
	svc, fg, _ := newComponentService(t)
	_, err := svc.Update(ctx, componentDefinition("site_1", "ghost", "Ghost"))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if len(fg.commits) != 0 {
		t.Fatal("update of missing component must not commit")
	}
}

func TestComponentUpdateStricterSchemaAllowed(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newComponentService(t)
	if _, err := svc.Define(ctx, componentDefinition("site_1", "card", "Карточка")); err != nil {
		t.Fatalf("define: %v", err)
	}
	// Making a field required is accepted: page trees are validated lazily (R4).
	d := componentDefinition("site_1", "card", "Карточка")
	d.Schema = mustSchema(t, `{"type":"object","required":["title","rating"],"properties":{"title":{"type":"string"},"rating":{"type":"number"}}}`)
	if _, err := svc.Update(ctx, d); err != nil {
		t.Fatalf("stricter schema update must be allowed, got %v", err)
	}
}

func TestComponentDelete(t *testing.T) {
	ctx := context.Background()
	svc, fg, fd := newComponentService(t)
	if _, err := svc.Define(ctx, componentDefinition("site_1", "card", "Карточка")); err != nil {
		t.Fatalf("define: %v", err)
	}
	if err := svc.Delete(ctx, "site_1", "card"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get(ctx, "site_1", "card"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
	// Delete must not touch git: history (the source of truth) stays intact.
	if len(fg.commits) != 1 {
		t.Fatalf("delete must not create commits, got %d", len(fg.commits))
	}
	if !fd.deleted[fd.key("site_1", "card")] {
		t.Fatal("registry entry was not deleted")
	}
}

func TestComponentDefineEmptySource(t *testing.T) {
	ctx := context.Background()
	svc, fg, fd := newComponentService(t)
	d := componentDefinition("site_1", "card", "Карточка")
	d.Source = ""
	if _, err := svc.Define(ctx, d); !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("want ErrInvalidRequest, got %v", err)
	}
	if len(fg.commits) != 0 || len(fd.saved) != 0 {
		t.Fatal("empty source must not write anything")
	}
}

func TestComponentDefineInvalidID(t *testing.T) {
	ctx := context.Background()
	cases := map[string]bool{
		"-card": false, "a/b": false, "..": false, "a b": false,
		"": false, "0abc": false,
		"a.b": true, "a_b": true,
	}
	for id, valid := range cases {
		svc, fg, _ := newComponentService(t)
		d := componentDefinition("site_1", id, "X")
		_, err := svc.Define(ctx, d)
		if valid {
			if err != nil {
				t.Fatalf("id %q should be valid, got %v", id, err)
			}
			continue
		}
		if !errors.Is(err, domain.ErrInvalidRequest) {
			t.Fatalf("id %q: want ErrInvalidRequest, got %v", id, err)
		}
		if len(fg.commits) != 0 {
			t.Fatalf("id %q: invalid id must not commit", id)
		}
	}
}
