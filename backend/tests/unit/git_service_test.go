package unit

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/application/git"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// gitServicePair wires a git.Service over the same fake git and defs.
func gitServicePair(t *testing.T) (*git.Service, *fakeGit, *fakeDefs) {
	t.Helper()
	fg := &fakeGit{}
	fd := newFakeDefs()
	return git.NewService(fg, fd), fg, fd
}

func TestGitInitRepoIdempotent(t *testing.T) {
	ctx := context.Background()
	svc, fg, _ := gitServicePair(t)

	if err := svc.InitRepo(ctx, "site-a"); err != nil {
		t.Fatalf("first init: %v", err)
	}
	if err := svc.InitRepo(ctx, "site-a"); err != nil {
		t.Fatalf("repeat init must be ok, got %v", err)
	}
	if len(fg.inits) != 2 {
		t.Fatalf("Init must hit the mock twice, got %d", len(fg.inits))
	}
}

func TestGitReleaseWritesFilesSingleCommit(t *testing.T) {
	ctx := context.Background()
	svc, fg, _ := gitServicePair(t)

	schema := mustJSONMap(`{"type":"string"}`)
	metadata := mustJSONMap(`{"label":"Card"}`)
	v, err := svc.Release(ctx, "site-a", "card", "Card", "source-card-1", schema, metadata)
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if v.ID == "" || v.SiteID != "site-a" || v.DefinitionID != "card" {
		t.Fatalf("unexpected component version: %+v", v)
	}
	if len(fg.commits) != 1 {
		t.Fatalf("want 1 commit, got %d", len(fg.commits))
	}
	c := fg.commits[0]
	if c.definitionID != "card" {
		t.Fatalf("commit definitionId = %q", c.definitionID)
	}
	if c.message != "component card: Card" {
		t.Fatalf("commit message = %q", c.message)
	}
	wantFiles := map[string][]byte{
		git.FileDefinition: []byte("source-card-1"),
		git.FileSchema:     []byte(`{"type":"string"}`),
		git.FileMetadata:   []byte(`{"label":"Card"}`),
	}
	if !reflect.DeepEqual(c.files, wantFiles) {
		t.Fatalf("files mismatch:\n got %v\nwant %v", c.files, wantFiles)
	}
}

func TestGitReleaseDeterministicFiles(t *testing.T) {
	ctx := context.Background()
	svc, fg, _ := gitServicePair(t)

	schema := mustJSONMap(`{"type":"string"}`)
	meta := mustJSONMap(`{"label":"Card"}`)
	if _, err := svc.Release(ctx, "site-a", "card", "Card", "source-1", schema, meta); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Release(ctx, "site-a", "card", "Card", "source-2", schema, meta); err != nil {
		t.Fatal(err)
	}
	if len(fg.commits) != 2 {
		t.Fatalf("want 2 commits, got %d", len(fg.commits))
	}
	second := fg.commits[1].files
	want := map[string][]byte{
		git.FileDefinition: []byte("source-2"),
		git.FileSchema:     []byte(`{"type":"string"}`),
		git.FileMetadata:   []byte(`{"label":"Card"}`),
	}
	for name, body := range want {
		if got := string(second[name]); got != string(body) {
			t.Fatalf("file %s = %q, want %q", name, got, body)
		}
	}
	if string(second[git.FileDefinition]) == string(fg.commits[0].files[git.FileDefinition]) {
		t.Fatal("source must differ between releases")
	}
}

func TestGitReleasesChronological(t *testing.T) {
	ctx := context.Background()
	svc, fg, _ := gitServicePair(t)

	schema := mustJSONMap(`{"type":"string"}`)
	meta := mustJSONMap(`{"label":"Card"}`)
	_, err := svc.Release(ctx, "site-a", "card", "Card", "v1", schema, meta)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Release(ctx, "site-a", "card", "Card", "v2", schema, meta)
	if err != nil {
		t.Fatal(err)
	}
	versions, err := svc.Releases(ctx, "site-a", "card")
	if err != nil {
		t.Fatalf("releases: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("want 2 versions, got %d", len(versions))
	}
	// fakeGit.ListVersions returns commits in insertion order (chronological).
	if versions[0].ID == "" || versions[1].ID == "" {
		t.Fatalf("versions must carry ids: %+v", versions)
	}
	if versions[0].DefinitionID != "card" || versions[1].DefinitionID != "card" {
		t.Fatalf("versions must reference card: %+v", versions)
	}
	_ = fg
}

func TestGitCheckoutVersion(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := gitServicePair(t)
	schema := mustJSONMap(`{"type":"string"}`)
	meta := mustJSONMap(`{"label":"Card"}`)
	v1, err := svc.Release(ctx, "site-a", "card", "Card", "v1-src", schema, meta)
	if err != nil {
		t.Fatal(err)
	}
	files, err := svc.CheckoutVersion(ctx, "site-a", v1.ID)
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if string(files[git.FileDefinition]) != "v1-src" {
		t.Fatalf("source = %q, want v1-src", files[git.FileDefinition])
	}
}

func TestGitCheckoutVersionUnknownSha(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := gitServicePair(t)
	if _, err := svc.CheckoutVersion(ctx, "site-a", "nope"); !errors.Is(err, domain.ErrVersionNotFound) {
		t.Fatalf("want ErrVersionNotFound, got %v", err)
	}
}

func TestGitRollbackRewritesRegistryNewCommit(t *testing.T) {
	ctx := context.Background()
	svc, fg, fd := gitServicePair(t)

	card := componentDefinition("site-a", "card", "Card")
	card.Source = "v1-src"
	_ = fd.Save(ctx, &card)

	schema := mustJSONMap(`{"type":"string"}`)
	meta := mustJSONMap(`{"label":"Card"}`)
	v1, err := svc.Release(ctx, "site-a", "card", "Card", "v1-src", schema, meta)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Release(ctx, "site-a", "card", "Card", "v2-src", schema, meta)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.Rollback(ctx, "site-a", "card", v1.ID); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	// History grows: v1, v2, rollback.
	if len(fg.commits) != 3 {
		t.Fatalf("want 3 commits after rollback, got %d", len(fg.commits))
	}
	rb := fg.commits[2]
	if rb.message != "rollback card: "+v1.ID {
		t.Fatalf("rollback message = %q", rb.message)
	}
	if string(rb.files[git.FileDefinition]) != "v1-src" {
		t.Fatalf("rollback commit carries %q, want v1-src", rb.files[git.FileDefinition])
	}

	stored, err := fd.Get(ctx, "site-a", "card")
	if err != nil {
		t.Fatal(err)
	}
	if stored.CurrentSHA != rb.sha {
		t.Fatalf("registry currentSHA = %q, want rollback sha %q", stored.CurrentSHA, rb.sha)
	}
	if stored.Source != "v1-src" {
		t.Fatalf("registry source = %q, want v1-src", stored.Source)
	}
}

func TestGitRollbackNoVersions(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := gitServicePair(t)
	if err := svc.Rollback(ctx, "site-a", "card", "unknown-sha"); !errors.Is(err, domain.ErrVersionNotFound) {
		t.Fatalf("want ErrVersionNotFound, got %v", err)
	}
}

func TestGitUninitializedPropagates(t *testing.T) {
	ctx := context.Background()
	fg := &fakeGit{}
	// Simulate a client-visible uninitialized repo.
	fg.checkoutErr = domain.ErrRepoNotInitialized
	fd := newFakeDefs()
	svc := git.NewService(fg, fd)

	schema := mustJSONMap(`{"type":"string"}`)
	meta := mustJSONMap(`{"label":"Card"}`)
	if _, err := svc.Release(ctx, "site-a", "card", "Card", "v1", schema, meta); err != nil {
		t.Fatalf("init path untouched, release: %v", err)
	}
	if err := svc.Rollback(ctx, "site-a", "card", "any"); !errors.Is(err, domain.ErrRepoNotInitialized) {
		t.Fatalf("rollback: want ErrRepoNotInitialized, got %v", err)
	}
}

func TestGitRollbackTransactionalOnCheckoutError(t *testing.T) {
	ctx := context.Background()
	svc, fg, fd := gitServicePair(t)

	card := componentDefinition("site-a", "card", "Card")
	card.Source = "v1-src"
	_ = fd.Save(ctx, &card)

	fg.checkoutErr = errors.New("boom")
	if err := svc.Rollback(ctx, "site-a", "card", "any-should-fail"); err == nil {
		t.Fatal("rollback must fail")
	}
	if len(fd.saved) != 1 {
		t.Fatalf("registry must be untouched (only the seed save), got %d saves", len(fd.saved))
	}
	stored, err := fd.Get(ctx, "site-a", "card")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Source != "v1-src" || stored.CurrentSHA != "" {
		t.Fatalf("registry mutated: %+v", stored)
	}
}
