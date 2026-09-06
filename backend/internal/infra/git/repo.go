// Package git implements the application/git.Repository port with go-git
// against one bare repository per site (R5/R6/R7 in docs/design/frontend.md;
// §4/§5 of docs/backend/components-test-spec.md). It is infrastructure: the
// application layer only sees the repository interface.
package git

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Repo implements Repository with go-git against a bare repository per site.
// Repositories live outside content and the database (R6): the tree layout is
// <baseDir>/sites/<siteId>/repo.git.
type Repo struct {
	baseDir string
}

// NewRepo builds a Repo whose repositories live under baseDir.
func NewRepo(baseDir string) *Repo {
	return &Repo{baseDir: baseDir}
}

// branchPrefix scopes one branch per definition so versions are listable and
// chronology is preserved through parent links.
const branchPrefix = "refs/heads/defs/"

// Dir returns the bare repository path for a site. Exposed so tests and tooling
// can assert the on-disk layout (R6: <baseDir>/sites/<siteId>/repo.git).
func (r *Repo) Dir(siteID string) string {
	return r.repoDir(siteID)
}

// repoDir returns the bare repository path for a site.
func (r *Repo) repoDir(siteID string) string {
	return filepath.Join(r.baseDir, "sites", siteID, "repo.git")
}

func (r *Repo) defRef(definitionID string) plumbing.ReferenceName {
	return plumbing.ReferenceName(branchPrefix + definitionID)
}

// Init ensures a bare repository exists for the site. Idempotent.
func (r *Repo) Init(_ context.Context, siteID string) error {
	dir := r.repoDir(siteID)
	if _, err := git.PlainInit(dir, true); err != nil {
		if errors.Is(err, git.ErrRepositoryAlreadyExists) {
			return nil
		}
		return fmt.Errorf("init repo for %s: %w", siteID, err)
	}
	return nil
}

func (r *Repo) openRepo(siteID string) (*git.Repository, error) {
	repo, err := git.PlainOpen(r.repoDir(siteID))
	if err != nil {
		if errors.Is(err, git.ErrRepositoryNotExists) {
			return nil, domain.ErrRepoNotInitialized
		}
		return nil, fmt.Errorf("open repo for %s: %w", siteID, err)
	}
	return repo, nil
}

// Commit writes files and returns the commit sha. Files are written
// deterministically (sorted). The commit links to the previous version of the
// same definition, forming an ordered per-definition history.
func (r *Repo) Commit(ctx context.Context, siteID, definitionID, message string, files map[string][]byte) (string, error) {
	repo, err := r.openRepo(siteID)
	if err != nil {
		return "", err
	}
	st := repo.Storer

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	entries := make([]object.TreeEntry, 0, len(names))
	for _, name := range names {
		obj := st.NewEncodedObject()
		obj.SetType(plumbing.BlobObject)
		w, err := obj.Writer()
		if err != nil {
			return "", fmt.Errorf("blob writer %s: %w", name, err)
		}
		if _, err := w.Write(files[name]); err != nil {
			return "", fmt.Errorf("write blob %s: %w", name, err)
		}
		if err := w.Close(); err != nil {
			return "", fmt.Errorf("close blob %s: %w", name, err)
		}
		hash, err := st.SetEncodedObject(obj)
		if err != nil {
			return "", fmt.Errorf("store blob %s: %w", name, err)
		}
		entries = append(entries, object.TreeEntry{
			Name: name,
			Mode: filemode.Regular,
			Hash: hash,
		})
	}

	tree := &object.Tree{Entries: entries}
	treeObj := st.NewEncodedObject()
	if err := tree.Encode(treeObj); err != nil {
		return "", fmt.Errorf("encode tree: %w", err)
	}
	treeHash, err := st.SetEncodedObject(treeObj)
	if err != nil {
		return "", fmt.Errorf("store tree: %w", err)
	}

	var parent plumbing.Hash
	if ref, err := st.Reference(r.defRef(definitionID)); err == nil {
		parent = ref.Hash()
	} else if !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return "", fmt.Errorf("read def ref: %w", err)
	}

	commit := &object.Commit{
		Author:    signature(),
		Committer: signature(),
		Message:   message,
		TreeHash:  treeHash,
	}
	if parent.IsZero() == false {
		commit.ParentHashes = []plumbing.Hash{parent}
	}
	commitObj := st.NewEncodedObject()
	if err := commit.Encode(commitObj); err != nil {
		return "", fmt.Errorf("encode commit: %w", err)
	}
	commitHash, err := st.SetEncodedObject(commitObj)
	if err != nil {
		return "", fmt.Errorf("store commit: %w", err)
	}
	if err := st.SetReference(plumbing.NewHashReference(r.defRef(definitionID), commitHash)); err != nil {
		return "", fmt.Errorf("update def ref: %w", err)
	}
	return commitHash.String(), nil
}

// ListVersions returns commit shas for one definition in chronological order
// (oldest first) by walking the definition branch's parent chain.
func (r *Repo) ListVersions(_ context.Context, siteID, definitionID string) ([]string, error) {
	repo, err := r.openRepo(siteID)
	if err != nil {
		return nil, err
	}
	ref, err := repo.Storer.Reference(r.defRef(definitionID))
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("read def ref: %w", err)
	}
	var newestFirst []string
	cur := ref.Hash()
	for i := 0; i < 100000; i++ {
		newestFirst = append(newestFirst, cur.String())
		commit, err := object.GetCommit(repo.Storer, cur)
		if err != nil {
			return nil, fmt.Errorf("read commit %s: %w", cur, err)
		}
		if len(commit.ParentHashes) == 0 {
			break
		}
		cur = commit.ParentHashes[0]
	}
	for i, j := 0, len(newestFirst)-1; i < j; i, j = i+1, j-1 {
		newestFirst[i], newestFirst[j] = newestFirst[j], newestFirst[i]
	}
	return newestFirst, nil
}

// Checkout returns the file set of a single commit, identified by sha.
func (r *Repo) Checkout(_ context.Context, siteID, sha string) (map[string][]byte, error) {
	repo, err := r.openRepo(siteID)
	if err != nil {
		return nil, err
	}
	commit, err := object.GetCommit(repo.Storer, plumbing.NewHash(sha))
	if err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			return nil, domain.ErrVersionNotFound
		}
		return nil, fmt.Errorf("get commit %s: %w", sha, err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree of %s: %w", sha, err)
	}
	files := make(map[string][]byte)
	if err := tree.Files().ForEach(func(f *object.File) error {
		content, err := f.Contents()
		if err != nil {
			return err
		}
		files[f.Name] = []byte(content)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("read files of %s: %w", sha, err)
	}
	return files, nil
}

// Head returns the sha of the most recently committed version across the site
// repository.
func (r *Repo) Head(_ context.Context, siteID string) (string, error) {
	repo, err := r.openRepo(siteID)
	if err != nil {
		return "", err
	}
	iter, err := repo.Storer.IterReferences()
	if err != nil {
		return "", fmt.Errorf("iterate refs: %w", err)
	}
	defer iter.Close()
	var latest string
	var latestWhen int64 = -1
	if err := iter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference || !strings.HasPrefix(ref.Name().String(), branchPrefix) {
			return nil
		}
		commit, err := object.GetCommit(repo.Storer, ref.Hash())
		if err != nil {
			return nil
		}
		if when := commit.Committer.When.Unix(); when > latestWhen {
			latestWhen = when
			latest = ref.Hash().String()
		}
		return nil
	}); err != nil {
		return "", fmt.Errorf("scan refs: %w", err)
	}
	if latest == "" {
		return "", domain.ErrRepoNotInitialized
	}
	return latest, nil
}

func signature() object.Signature {
	return object.Signature{
		Name:  "Liapoldus",
		Email: "noreply@liapoldus.local",
	}
}
