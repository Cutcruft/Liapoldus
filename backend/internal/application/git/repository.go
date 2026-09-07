// Package git defines the Repository port (application boundary) used by the
// snapshot service. One bare repository per site holds the full site state on
// the `dev` and `main` branches: dev is the working history, main the published
// state. Every commit is a complete, serialized snapshot of the site.
package git

import (
	"context"
	"time"
)

// Repository exposes branch-level git operations for a site ("one repo per
// site"). Files in a commit are stored as a flat map path→content; paths may
// contain "/" (e.g. "pages/p_1.json") and round-trip through ReadFiles.
type Repository interface {
	// InitRepo ensures a bare repo exists with a `main` branch holding an
	// empty initial commit. Idempotent.
	InitRepo(ctx context.Context, siteID string) error

	// CommitOnBranch writes files as one commit on branch (parent = branch
	// HEAD when present) and advances the branch ref. Returns the commit sha.
	CommitOnBranch(ctx context.Context, siteID, branch, message string, files map[string][]byte) (string, error)

	// CheckoutBranch returns the sha of the branch HEAD.
	CheckoutBranch(ctx context.Context, siteID, branch string) (string, error)

	// HeadSHA returns the sha of the branch HEAD.
	HeadSHA(ctx context.Context, siteID, branch string) (string, error)

	// Rebase replays sourceBranch's commits (those not already on targetBranch)
	// on top of targetBranch, rewriting only the replayed commits. sourceBranch
	// ends up a descendant of targetBranch. Idempotent when already rebased.
	Rebase(ctx context.Context, siteID, sourceBranch, targetBranch string) error

	// Merge fast-forwards targetBranch to sourceBranch (sourceBranch must be a
	// descendant of targetBranch). Returns the new target HEAD sha.
	Merge(ctx context.Context, siteID, sourceBranch, targetBranch string) (string, error)

	// ListBranches returns branch short names sorted.
	ListBranches(ctx context.Context, siteID string) ([]string, error)

	// Commits returns the commit history of branch, newest first, at most limit.
	Commits(ctx context.Context, siteID, branch string, limit int) ([]CommitInfo, error)

	// ReadFiles returns the file set of a single commit by sha.
	ReadFiles(ctx context.Context, siteID, sha string) (map[string][]byte, error)

	// ReadFile returns one file of a single commit by sha and path, or
	// ErrNotFound when the commit or the path does not exist.
	ReadFile(ctx context.Context, siteID, sha, path string) ([]byte, error)
}

// CommitInfo is one entry of a branch history list.
type CommitInfo struct {
	SHA     string    `json:"sha"`
	Message string    `json:"message"`
	Author  string    `json:"author"`
	Time    time.Time `json:"time"`
}
