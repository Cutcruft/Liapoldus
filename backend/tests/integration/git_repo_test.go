package integrationtest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
	gitrepo "github.com/liapoldus/liapoldus/backend/internal/infra/git"
)

func itoa(i int) string { return strconv.Itoa(i) }

func testRepo(t *testing.T, siteID string) *gitrepo.Repo {
	t.Helper()
	repo := gitrepo.NewRepo(t.TempDir())
	if err := repo.InitRepo(context.Background(), siteID); err != nil {
		t.Fatalf("init repo: %v", err)
	}
	return repo
}

func TestInitRepoCreatesBareWithDevAndMain(t *testing.T) {
	repo := gitrepo.NewRepo(t.TempDir())
	siteID := "site-bare"

	if err := repo.InitRepo(context.Background(), siteID); err != nil {
		t.Fatalf("init: %v", err)
	}
	dir := repo.Dir(siteID)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("bare repository must exist on disk at %s: %v", dir, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "HEAD")); err != nil {
		t.Fatalf("bare repo HEAD missing: %v", err)
	}
	devSHA, err := repo.HeadSHA(context.Background(), siteID, "dev")
	if err != nil {
		t.Fatalf("dev head: %v", err)
	}
	mainSHA, err := repo.HeadSHA(context.Background(), siteID, "main")
	if err != nil {
		t.Fatalf("main head: %v", err)
	}
	if devSHA != mainSHA {
		t.Fatalf("dev and main must share the initial commit, dev=%s main=%s", devSHA, mainSHA)
	}
	if err := repo.InitRepo(context.Background(), siteID); err != nil {
		t.Fatalf("repeat init must be ok, got %v", err)
	}
	branches, err := repo.ListBranches(context.Background(), siteID)
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}
	if len(branches) != 2 || branches[0] != "dev" || branches[1] != "main" {
		t.Fatalf("branches = %v, want [dev main]", branches)
	}
}

func TestCommitOnBranchTwoCommitsAndReadFiles(t *testing.T) {
	repo := testRepo(t, "site-two")
	ctx := context.Background()

	shaCard, err := repo.CommitOnBranch(ctx, "site-two", "dev", "add card", map[string][]byte{
		"components/card/source.tsx": []byte("src-card-1"),
		"pages/p_1.json":             []byte(`{"tree":{}}`),
	})
	if err != nil {
		t.Fatalf("commit 1: %v", err)
	}
	shaLayout, err := repo.CommitOnBranch(ctx, "site-two", "dev", "add layout", map[string][]byte{
		"components/layout.main/source.tsx": []byte("src-layout-1"),
	})
	if err != nil {
		t.Fatalf("commit 2: %v", err)
	}
	if shaCard == shaLayout {
		t.Fatal("distinct commits must produce distinct shas")
	}

	files, err := repo.ReadFiles(ctx, "site-two", shaCard)
	if err != nil {
		t.Fatalf("read files: %v", err)
	}
	if string(files["components/card/source.tsx"]) != "src-card-1" {
		t.Fatalf("card source = %q, want src-card-1", files["components/card/source.tsx"])
	}
	if _, ok := files["components/layout.main/source.tsx"]; ok {
		t.Fatal("commit 1 must not contain the layout file (immutable history)")
	}
}

func TestReadFilesUnknownSha(t *testing.T) {
	repo := testRepo(t, "site-unknown")
	if _, err := repo.ReadFiles(context.Background(), "site-unknown", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"); !errors.Is(err, domain.ErrVersionNotFound) {
		t.Fatalf("want ErrVersionNotFound, got %v", err)
	}
}

func TestCommitsNewestFirst(t *testing.T) {
	repo := testRepo(t, "site-vers")
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		msg := map[int]string{0: "zero", 1: "one", 2: "two"}[i]
		if _, err := repo.CommitOnBranch(ctx, "site-vers", "dev", msg, map[string][]byte{
			"pages/p.json": []byte(`{"n":"` + itoa(i) + `"}`),
		}); err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
	}
	commits, err := repo.Commits(ctx, "site-vers", "dev", 10)
	if err != nil {
		t.Fatalf("commits: %v", err)
	}
	if len(commits) != 4 { // empty initial commit + 3
		t.Fatalf("history = %d commits, want 4", len(commits))
	}
	if commits[0].Message != "two" || commits[1].Message != "one" || commits[2].Message != "zero" {
		t.Fatalf("order = [%s %s %s], want newest first", commits[0].Message, commits[1].Message, commits[2].Message)
	}
	limited, err := repo.Commits(ctx, "site-vers", "dev", 2)
	if err != nil {
		t.Fatalf("commits limited: %v", err)
	}
	if len(limited) != 2 {
		t.Fatalf("limit 2 returned %d commits", len(limited))
	}
}

func TestRebaseReplaysDevOntoMain(t *testing.T) {
	repo := testRepo(t, "site-rebase")
	ctx := context.Background()

	// published state on main
	if _, err := repo.CommitOnBranch(ctx, "site-rebase", "main", "initial publish", map[string][]byte{
		"site.json": []byte(`{"name":"S","slug":"s"}`),
	}); err != nil {
		t.Fatalf("main commit: %v", err)
	}
	// dev history diverges
	if _, err := repo.CommitOnBranch(ctx, "site-rebase", "dev", "work 1", map[string][]byte{
		"pages/a.json": []byte(`{"a":1}`),
	}); err != nil {
		t.Fatalf("dev work 1: %v", err)
	}
	if _, err := repo.CommitOnBranch(ctx, "site-rebase", "dev", "work 2", map[string][]byte{
		"pages/b.json": []byte(`{"b":2}`),
	}); err != nil {
		t.Fatalf("dev work 2: %v", err)
	}
	// main moves forward independently
	if _, err := repo.CommitOnBranch(ctx, "site-rebase", "main", "prod fix", map[string][]byte{
		"site.json": []byte(`{"name":"S","slug":"s","hosts":["x"]}`),
	}); err != nil {
		t.Fatalf("main fix: %v", err)
	}

	if err := repo.Rebase(ctx, "site-rebase", "dev", "main"); err != nil {
		t.Fatalf("rebase: %v", err)
	}

	devSHA, _ := repo.HeadSHA(ctx, "site-rebase", "dev")
	mainSHA, _ := repo.HeadSHA(ctx, "site-rebase", "main")
	mainFiles, err := repo.ReadFiles(ctx, "site-rebase", mainSHA)
	if err != nil {
		t.Fatal(err)
	}
	if string(mainFiles["site.json"]) != `{"name":"S","slug":"s","hosts":["x"]}` {
		t.Fatalf("prod fix lost on main: %s", mainFiles["site.json"])
	}
	devFiles, err := repo.ReadFiles(ctx, "site-rebase", devSHA)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"site.json", "pages/a.json", "pages/b.json"} {
		if _, ok := devFiles[name]; !ok {
			t.Fatalf("rebased dev HEAD missing %s: %v", name, devFiles)
		}
	}
	if string(devFiles["site.json"]) != string(mainFiles["site.json"]) {
		t.Fatal("rebase must keep dev a descendant of main (site.json identical)")
	}

	// idempotent
	if err := repo.Rebase(ctx, "site-rebase", "dev", "main"); err != nil {
		t.Fatalf("repeat rebase: %v", err)
	}
}

func TestMergeFastForwardAndReject(t *testing.T) {
	repo := testRepo(t, "site-merge")
	ctx := context.Background()

	if _, err := repo.CommitOnBranch(ctx, "site-merge", "dev", "work", map[string][]byte{
		"pages/a.json": []byte(`{"a":1}`),
	}); err != nil {
		t.Fatal(err)
	}
	mainSHA, err := repo.Merge(ctx, "site-merge", "dev", "main")
	if err != nil {
		t.Fatalf("ff merge: %v", err)
	}
	devSHA, _ := repo.HeadSHA(ctx, "site-merge", "dev")
	if mainSHA != devSHA {
		t.Fatalf("main after ff merge = %s, want dev %s", mainSHA, devSHA)
	}
	if _, err := repo.HeadSHA(ctx, "site-merge", "dev"); err != nil {
		t.Fatal(err)
	}

	// main now ahead of dev by one commit -> merge must reject (non-FF)
	if _, err := repo.CommitOnBranch(ctx, "site-merge", "main", "ahead", map[string][]byte{
		"site.json": []byte(`{"x":1}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Merge(ctx, "site-merge", "dev", "main"); !errors.Is(err, domain.ErrNotFastForward) {
		t.Fatalf("want ErrNotFastForward, got %v", err)
	}
}

func TestHeadSHAAndTypeErrors(t *testing.T) {
	repo := gitrepo.NewRepo(t.TempDir())
	ctx := context.Background()
	if _, err := repo.HeadSHA(ctx, "no-such-site", "main"); !errors.Is(err, domain.ErrRepoNotInitialized) {
		t.Fatalf("want ErrRepoNotInitialized, got %v", err)
	}
	if _, err := repo.ReadFiles(ctx, "no-such-site", "abc"); !errors.Is(err, domain.ErrRepoNotInitialized) {
		t.Fatalf("want ErrRepoNotInitialized, got %v", err)
	}
}
