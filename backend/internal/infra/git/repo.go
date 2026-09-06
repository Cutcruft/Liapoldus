// Package git implements the application/git.Repository port with go-git
// against one bare repository per site. Each repo holds the full site state
// on the `dev` (working) and `main` (published) branches; every commit is a
// complete serialized snapshot. go-git has no rebase support, so Rebase and
// the fast-forward Merge are implemented at tree level: commits are replayed
// by applying diff(parent→commit) onto the target branch's tree.
package git

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage"
	gitapp "github.com/liapoldus/liapoldus/backend/internal/application/git"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Branch names are the stable contract between the infra layer and the
// gitsnapshot service.
const (
	BranchDev  = "dev"
	BranchMain = "main"
)

// Repo implements Repository with go-git against a bare repository per site.
// Repositories live outside content and the database: the tree layout is
// <baseDir>/sites/<siteId>/repo.git.
type Repo struct {
	baseDir string
}

// NewRepo builds a Repo whose repositories live under baseDir.
func NewRepo(baseDir string) *Repo {
	return &Repo{baseDir: baseDir}
}

// Dir returns the bare repository path for a site. Exposed so tests and
// tooling can assert the on-disk layout (<baseDir>/sites/<siteId>/repo.git).
func (r *Repo) Dir(siteID string) string {
	return r.repoDir(siteID)
}

func (r *Repo) repoDir(siteID string) string {
	return filepath.Join(r.baseDir, "sites", siteID, "repo.git")
}

func branchRef(branch string) plumbing.ReferenceName {
	return plumbing.ReferenceName("refs/heads/" + branch)
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

// InitRepo ensures a bare repository exists with a `main` branch holding an
// empty initial commit, `dev` branched at that same commit, and HEAD pointing
// at main. Idempotent.
func (r *Repo) InitRepo(_ context.Context, siteID string) error {
	if _, err := git.PlainInit(r.repoDir(siteID), true); err != nil {
		if !errors.Is(err, git.ErrRepositoryAlreadyExists) {
			return fmt.Errorf("init repo for %s: %w", siteID, err)
		}
	}
	repo, err := r.openRepo(siteID)
	if err != nil {
		return err
	}
	st := repo.Storer
	if _, err := st.Reference(branchRef(BranchMain)); err != nil {
		if !errors.Is(err, plumbing.ErrReferenceNotFound) {
			return fmt.Errorf("read main ref: %w", err)
		}
		treeHash, err := encodeTree(st, &object.Tree{})
		if err != nil {
			return err
		}
		commit := &object.Commit{Author: signature(), Committer: signature(), Message: "initial state", TreeHash: treeHash}
		hash, err := storeCommit(st, commit)
		if err != nil {
			return err
		}
		if err := st.SetReference(plumbing.NewHashReference(branchRef(BranchMain), hash)); err != nil {
			return fmt.Errorf("set main ref: %w", err)
		}
		if err := st.SetReference(plumbing.NewHashReference(branchRef(BranchDev), hash)); err != nil {
			return fmt.Errorf("set dev ref: %w", err)
		}
	}
	if err := st.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, branchRef(BranchMain))); err != nil {
		return fmt.Errorf("set HEAD: %w", err)
	}
	return nil
}

// CommitOnBranch writes files as one commit on branch (parent = branch HEAD
// when present) and advances the branch ref. Files are written deterministically.
func (r *Repo) CommitOnBranch(_ context.Context, siteID, branch, message string, files map[string][]byte) (string, error) {
	repo, err := r.openRepo(siteID)
	if err != nil {
		return "", err
	}
	return commitOnBranch(repo, branch, message, files)
}

func (r *Repo) storer(siteID string) (storage.Storer, error) {
	repo, err := r.openRepo(siteID)
	if err != nil {
		return nil, err
	}
	return repo.Storer, nil
}

// HeadSHA returns the sha of the branch HEAD.
func (r *Repo) HeadSHA(ctx context.Context, siteID, branch string) (string, error) {
	repo, err := r.openRepo(siteID)
	if err != nil {
		return "", err
	}
	return headSHA(repo, branch)
}

// CheckoutBranch returns the sha of the branch HEAD (the bare repo has no
// working tree to check out; this is the branch switch point).
func (r *Repo) CheckoutBranch(ctx context.Context, siteID, branch string) (string, error) {
	return r.HeadSHA(ctx, siteID, branch)
}

// ListBranches returns branch short names sorted.
func (r *Repo) ListBranches(_ context.Context, siteID string) ([]string, error) {
	st, err := r.storer(siteID)
	if err != nil {
		return nil, err
	}
	iter, err := st.IterReferences()
	if err != nil {
		return nil, fmt.Errorf("iterate refs: %w", err)
	}
	defer iter.Close()
	var branches []string
	if err := iter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name()
		if name.IsBranch() {
			branches = append(branches, name.Short())
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scan refs: %w", err)
	}
	sort.Strings(branches)
	return branches, nil
}

// Commits returns the commit history of branch, newest first, at most limit.
func (r *Repo) Commits(_ context.Context, siteID, branch string, limit int) ([]gitapp.CommitInfo, error) {
	sha, err := r.HeadSHA(context.Background(), siteID, branch)
	if err != nil {
		return nil, err
	}
	st, err := r.storer(siteID)
	if err != nil {
		return nil, err
	}
	var out []gitapp.CommitInfo
	cur := plumbing.NewHash(sha)
	for i := 0; i < limit && !cur.IsZero(); i++ {
		c, err := object.GetCommit(st, cur)
		if err != nil {
			return nil, fmt.Errorf("read commit %s: %w", cur, err)
		}
		out = append(out, gitapp.CommitInfo{
			SHA:     cur.String(),
			Message: c.Message,
			Author:  c.Author.Name,
			Time:    c.Author.When,
		})
		if len(c.ParentHashes) == 0 {
			break
		}
		cur = c.ParentHashes[0]
	}
	return out, nil
}

// Rebase replays sourceBranch's commits on top of targetBranch, rewriting only
// the replayed commits. Idempotent when source is already a descendant of
// target (or equal).
func (r *Repo) Rebase(ctx context.Context, siteID, sourceBranch, targetBranch string) error {
	st, err := r.storer(siteID)
	if err != nil {
		return err
	}
	srcSHA := plumbing.NewHash(mustHead(ctx, r, siteID, sourceBranch))
	tgtSHA := plumbing.NewHash(mustHead(ctx, r, siteID, targetBranch))
	if srcSHA == tgtSHA {
		return nil
	}
	base, err := mergeBase(st, srcSHA, tgtSHA)
	if err != nil {
		return err
	}
	if base == srcSHA {
		return nil // source is already an ancestor of target
	}
	replays := replayPath(st, srcSHA, base)
	newParent := tgtSHA
	for _, c := range replays {
		newHead, err := object.GetCommit(st, newParent)
		if err != nil {
			return fmt.Errorf("read commit %s: %w", newParent, err)
		}
		ours, err := treeFiles(st, newHead.TreeHash)
		if err != nil {
			return err
		}
		var pa map[string]plumbing.Hash
		if len(c.ParentHashes) > 0 {
			pa, err = treeFiles(st, c.ParentHashes[0])
			if err != nil {
				return err
			}
		}
		pb, err := treeFiles(st, c.TreeHash)
		if err != nil {
			return err
		}
		treeHash, err := buildFlatTree(st, applyDiff(ours, pa, pb))
		if err != nil {
			return err
		}
		nc := &object.Commit{
			Author:       c.Author,
			Committer:    c.Committer,
			Message:      c.Message,
			TreeHash:     treeHash,
			ParentHashes: []plumbing.Hash{newParent},
		}
		hash, err := storeCommit(st, nc)
		if err != nil {
			return err
		}
		newParent = hash
	}
	if err := st.SetReference(plumbing.NewHashReference(branchRef(sourceBranch), newParent)); err != nil {
		return fmt.Errorf("update %s ref: %w", sourceBranch, err)
	}
	return nil
}

// Merge fast-forwards targetBranch to sourceBranch. When source is not a
// descendant of target, publication must stop (no merge commits in this model).
func (r *Repo) Merge(ctx context.Context, siteID, sourceBranch, targetBranch string) (string, error) {
	st, err := r.storer(siteID)
	if err != nil {
		return "", err
	}
	srcSHA := plumbing.NewHash(mustHead(ctx, r, siteID, sourceBranch))
	tgtSHA := plumbing.NewHash(mustHead(ctx, r, siteID, targetBranch))
	if !isAncestor(st, tgtSHA, srcSHA) {
		return "", fmt.Errorf("%w: %s is not a descendant of %s", domain.ErrNotFastForward, sourceBranch, targetBranch)
	}
	if err := st.SetReference(plumbing.NewHashReference(branchRef(targetBranch), srcSHA)); err != nil {
		return "", fmt.Errorf("ff update %s: %w", targetBranch, err)
	}
	return srcSHA.String(), nil
}

// ReadFiles returns the file set of a single commit by sha.
func (r *Repo) ReadFiles(_ context.Context, siteID, sha string) (map[string][]byte, error) {
	st, err := r.storer(siteID)
	if err != nil {
		return nil, err
	}
	commit, err := object.GetCommit(st, plumbing.NewHash(sha))
	if err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			return nil, domain.ErrVersionNotFound
		}
		return nil, fmt.Errorf("get commit %s: %w", sha, err)
	}
	return readTreeContents(st, commit.TreeHash)
}

func commitOnBranch(repo *git.Repository, branch, message string, files map[string][]byte) (string, error) {
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
		entries = append(entries, object.TreeEntry{Name: name, Mode: filemode.Regular, Hash: hash})
	}
	treeHash, err := encodeTree(st, &object.Tree{Entries: entries})
	if err != nil {
		return "", err
	}
	var parent plumbing.Hash
	if ref, err := st.Reference(branchRef(branch)); err == nil {
		parent = ref.Hash()
	} else if !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return "", fmt.Errorf("read %s ref: %w", branch, err)
	}
	commit := &object.Commit{
		Author:    signature(),
		Committer: signature(),
		Message:   message,
		TreeHash:  treeHash,
	}
	if !parent.IsZero() {
		commit.ParentHashes = []plumbing.Hash{parent}
	}
	hash, err := storeCommit(st, commit)
	if err != nil {
		return "", err
	}
	if err := st.SetReference(plumbing.NewHashReference(branchRef(branch), hash)); err != nil {
		return "", fmt.Errorf("update %s ref: %w", branch, err)
	}
	return hash.String(), nil
}

// headSHA returns the branch HEAD sha, or ErrRepoNotInitialized when the
// branch is missing.
func headSHA(repo *git.Repository, branch string) (string, error) {
	ref, err := repo.Storer.Reference(branchRef(branch))
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return "", fmt.Errorf("%w: branch %q", domain.ErrRepoNotInitialized, branch)
		}
		return "", fmt.Errorf("read %s ref: %w", branch, err)
	}
	return ref.Hash().String(), nil
}

// mustHead resolves a branch HEAD sha, returning "" when the branch is
// missing so the caller surfaces ErrRepoNotInitialized.
func mustHead(ctx context.Context, r *Repo, siteID, branch string) string {
	sha, err := r.HeadSHA(ctx, siteID, branch)
	if err != nil {
		return ""
	}
	return sha
}

// encodeTree stores a tree and returns its hash.
func encodeTree(st storage.Storer, tree *object.Tree) (plumbing.Hash, error) {
	obj := st.NewEncodedObject()
	if err := tree.Encode(obj); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("encode tree: %w", err)
	}
	return st.SetEncodedObject(obj)
}

// buildFlatTree stores one flat tree (path→blob hash) and returns its hash.
func buildFlatTree(st storage.Storer, files map[string]plumbing.Hash) (plumbing.Hash, error) {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := make([]object.TreeEntry, 0, len(names))
	for _, name := range names {
		entries = append(entries, object.TreeEntry{Name: name, Mode: filemode.Regular, Hash: files[name]})
	}
	return encodeTree(st, &object.Tree{Entries: entries})
}

// storeCommit encodes a commit and returns its hash.
func storeCommit(st storage.Storer, commit *object.Commit) (plumbing.Hash, error) {
	obj := st.NewEncodedObject()
	if err := commit.Encode(obj); err != nil {
		return plumbing.ZeroHash, fmt.Errorf("encode commit: %w", err)
	}
	return st.SetEncodedObject(obj)
}

// treeFiles walks a tree (recursing into subdirectories) into a flat
// path→blob-hash map.
func treeFiles(st storage.Storer, treeHash plumbing.Hash) (map[string]plumbing.Hash, error) {
	if treeHash.IsZero() {
		return map[string]plumbing.Hash{}, nil
	}
	tree, err := object.GetTree(st, treeHash)
	if err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			return map[string]plumbing.Hash{}, nil
		}
		return nil, fmt.Errorf("get tree %s: %w", treeHash, err)
	}
	files := map[string]plumbing.Hash{}
	var walk func(t *object.Tree, prefix string) error
	walk = func(t *object.Tree, prefix string) error {
		for _, e := range t.Entries {
			name := e.Name
			if prefix != "" {
				name = prefix + "/" + e.Name
			}
			if e.Mode == filemode.Dir {
				sub, err := object.GetTree(st, e.Hash)
				if err != nil {
					return fmt.Errorf("get subtree %s: %w", e.Hash, err)
				}
				if err := walk(sub, name); err != nil {
					return err
				}
				continue
			}
			files[name] = e.Hash
		}
		return nil
	}
	if err := walk(tree, ""); err != nil {
		return nil, err
	}
	return files, nil
}

// readTreeContents reads every file of a tree (recursing) into a flat map.
func readTreeContents(st storage.Storer, treeHash plumbing.Hash) (map[string][]byte, error) {
	files := map[string][]byte{}
	var read func(e object.TreeEntry, prefix string) error
	read = func(e object.TreeEntry, prefix string) error {
		name := e.Name
		if prefix != "" {
			name = prefix + "/" + e.Name
		}
		if e.Mode == filemode.Dir {
			sub, err := object.GetTree(st, e.Hash)
			if err != nil {
				return fmt.Errorf("get subtree %s: %w", e.Hash, err)
			}
			for _, se := range sub.Entries {
				if err := read(se, name); err != nil {
					return err
				}
			}
			return nil
		}
		content, err := fileContents(st, e)
		if err != nil {
			return err
		}
		files[name] = content
		return nil
	}
	tree, err := object.GetTree(st, treeHash)
	if err != nil {
		return nil, fmt.Errorf("get tree %s: %w", treeHash, err)
	}
	for _, e := range tree.Entries {
		if err := read(e, ""); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func fileContents(st storage.Storer, e object.TreeEntry) ([]byte, error) {
	blob, err := object.GetBlob(st, e.Hash)
	if err != nil {
		return nil, fmt.Errorf("get blob %s: %w", e.Hash, err)
	}
	reader, err := blob.Reader()
	if err != nil {
		return nil, fmt.Errorf("read blob %s: %w", e.Hash, err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read blob %s: %w", e.Hash, err)
	}
	return data, nil
}

// mergeBase computes the most recent common ancestor of two commits.
func mergeBase(st storage.Storer, a, b plumbing.Hash) (plumbing.Hash, error) {
	ancestors := map[plumbing.Hash]bool{}
	stack := []plumbing.Hash{a}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if ancestors[cur] {
			continue
		}
		ancestors[cur] = true
		c, err := object.GetCommit(st, cur)
		if err != nil {
			return plumbing.ZeroHash, fmt.Errorf("read commit %s: %w", cur, err)
		}
		stack = append(stack, c.ParentHashes...)
	}
	queue := []plumbing.Hash{b}
	visited := map[plumbing.Hash]bool{}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if ancestors[cur] {
			return cur, nil
		}
		if visited[cur] {
			continue
		}
		visited[cur] = true
		c, err := object.GetCommit(st, cur)
		if err != nil {
			return plumbing.ZeroHash, fmt.Errorf("read commit %s: %w", cur, err)
		}
		queue = append(queue, c.ParentHashes...)
	}
	return plumbing.ZeroHash, fmt.Errorf("%w: no merge base", domain.ErrRepoNotInitialized)
}

// isAncestor reports whether anc is an ancestor of (or equal to) desc.
func isAncestor(st storage.Storer, anc, desc plumbing.Hash) bool {
	if anc == desc {
		return true
	}
	visited := map[plumbing.Hash]bool{}
	queue := []plumbing.Hash{desc}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] {
			continue
		}
		visited[cur] = true
		c, err := object.GetCommit(st, cur)
		if err != nil {
			return false
		}
		for _, p := range c.ParentHashes {
			if p == anc {
				return true
			}
			queue = append(queue, p)
		}
	}
	return false
}

// replayPath returns the linear commits between tip and base (exclusive),
// oldest first.
func replayPath(st storage.Storer, tip, base plumbing.Hash) []*object.Commit {
	var newestFirst []*object.Commit
	cur := tip
	for !cur.IsZero() {
		c, err := object.GetCommit(st, cur)
		if err != nil {
			break
		}
		if cur == base {
			break
		}
		newestFirst = append(newestFirst, c)
		if len(c.ParentHashes) == 0 {
			break
		}
		cur = c.ParentHashes[0]
	}
	out := make([]*object.Commit, 0, len(newestFirst))
	for i := len(newestFirst) - 1; i >= 0; i-- {
		out = append(out, newestFirst[i])
	}
	return out
}

// applyDiff re-applies the diff between two trees (pa→pb) onto ours.
func applyDiff(ours, pa, pb map[string]plumbing.Hash) map[string]plumbing.Hash {
	result := make(map[string]plumbing.Hash, len(ours))
	for k, v := range ours {
		result[k] = v
	}
	for path := range pa {
		pbVal, inPb := pb[path]
		paVal := pa[path]
		if inPb {
			if paVal != pbVal {
				result[path] = pbVal
			}
		} else {
			delete(result, path)
		}
	}
	for path, val := range pb {
		if _, inPa := pa[path]; !inPa {
			result[path] = val
		}
	}
	return result
}

func signature() object.Signature {
	return object.Signature{
		Name:  "Liapoldus",
		Email: "noreply@liapoldus.local",
	}
}
