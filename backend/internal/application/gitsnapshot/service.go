// Package gitsnapshot unifies the site snapshot lifecycle with git: every
// commit on dev/main stores the complete serialized site state, publish moves
// dev onto main (rebase + fast-forward merge), and restore/rollback overwrite
// the database state from a commit tree.
package gitsnapshot

import (
	"context"
	"fmt"
	"strings"
	"time"

	gitapp "github.com/liapoldus/liapoldus/backend/internal/application/git"
	"github.com/liapoldus/liapoldus/backend/internal/application/id"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Branch names are the stable contract with the git infra layer.
const (
	BranchDev  = "dev"
	BranchMain = "main"
)

// LockResolver freezes the site's dependency graph into the snapshot. The
// same contract the snapshot service uses (dependency-service spec §7).
type LockResolver interface {
	ResolveLock(context.Context, string) (domain.SnapshotLock, error)
}

// Service serializes a site's database state into git commits and loads commit
// trees back into the database.
type Service struct {
	repo       gitapp.Repository
	sites      domain.SiteRepository
	pages      domain.PageRepository
	contents   domain.ContentRepository
	routes     domain.RouteRepository
	forms      domain.FormRepository
	components domain.ComponentDefinitionRepository
	snapshots  domain.SnapshotRepository
	tokens     domain.TokenRepository
	deps       LockResolver
	now        func() time.Time
}

func NewService(
	repo gitapp.Repository,
	sites domain.SiteRepository,
	pages domain.PageRepository,
	contents domain.ContentRepository,
	routes domain.RouteRepository,
	forms domain.FormRepository,
	components domain.ComponentDefinitionRepository,
	snapshots domain.SnapshotRepository,
	tokens domain.TokenRepository,
	deps LockResolver,
) *Service {
	return &Service{
		repo:       repo,
		sites:      sites,
		pages:      pages,
		contents:   contents,
		routes:     routes,
		forms:      forms,
		components: components,
		snapshots:  snapshots,
		tokens:     tokens,
		deps:       deps,
		now:        time.Now,
	}
}

// BranchStatus is one branch's HEAD information.
type BranchStatus struct {
	Existing bool               `json:"existing"`
	SHA      string             `json:"sha,omitempty"`
	Head     *gitapp.CommitInfo `json:"head,omitempty"`
}

// SiteStatus describes the git state of a site for the admin Git page.
type SiteStatus struct {
	SiteID   string       `json:"siteId"`
	Branches []string     `json:"branches"`
	Dev      BranchStatus `json:"dev"`
	Main     BranchStatus `json:"main"`
	Dirty    bool         `json:"dirty"`
}

// Status reports branch heads and whether the database state differs from the
// dev HEAD commit. The dirty check never resolves dependencies (network-free).
func (s *Service) Status(ctx context.Context, siteID string) (*SiteStatus, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return nil, err
	}
	status := &SiteStatus{SiteID: siteID}
	if branches, err := s.repo.ListBranches(ctx, siteID); err == nil {
		status.Branches = branches
	}
	devSHA, err := s.repo.HeadSHA(ctx, siteID, BranchDev)
	if err == nil {
		status.Dev = BranchStatus{Existing: true, SHA: devSHA}
		status.Dev.Head = s.head(ctx, siteID, BranchDev)
	}
	if !errorsIsRepoNotInitialized(err) && err != nil {
		return nil, err
	}
	mainSHA, err := s.repo.HeadSHA(ctx, siteID, BranchMain)
	if err == nil {
		status.Main = BranchStatus{Existing: true, SHA: mainSHA}
		status.Main.Head = s.head(ctx, siteID, BranchMain)
	}
	if !errorsIsRepoNotInitialized(err) && err != nil {
		return nil, err
	}
	if status.Dev.Existing {
		dirty, err := s.isDirty(ctx, siteID, status.Dev.SHA)
		if err != nil {
			return nil, err
		}
		status.Dirty = dirty
	}
	return status, nil
}

func (s *Service) head(ctx context.Context, siteID, branch string) *gitapp.CommitInfo {
	commits, err := s.repo.Commits(ctx, siteID, branch, 1)
	if err != nil || len(commits) == 0 {
		return nil
	}
	return &commits[0]
}

// Commits returns the dev branch history, newest first.
func (s *Service) Commits(ctx context.Context, siteID string, limit int) ([]gitapp.CommitInfo, error) {
	return s.repo.Commits(ctx, siteID, BranchDev, limit)
}

// Commit serializes the current database state as one commit on dev and
// returns its sha. The repository is initialized on first use.
func (s *Service) Commit(ctx context.Context, siteID, message string) (string, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return "", err
	}
	if err := s.repo.InitRepo(ctx, siteID); err != nil {
		return "", err
	}
	files, err := s.serialize(ctx, siteID)
	if err != nil {
		return "", err
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "commit site state"
	}
	return s.repo.CommitOnBranch(ctx, siteID, BranchDev, message, files)
}

// Publish rebases dev onto main, fast-forwards main to dev and loads the
// published state into the database, recording a Snapshot with the resulting
// main sha. The build pipeline stays a separate, explicit frontend action.
func (s *Service) Publish(ctx context.Context, siteID, message string) (string, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return "", err
	}
	if err := s.repo.InitRepo(ctx, siteID); err != nil {
		return "", err
	}
	if err := s.repo.Rebase(ctx, siteID, BranchDev, BranchMain); err != nil {
		return "", fmt.Errorf("rebase dev on main: %w", err)
	}
	mainSHA, err := s.repo.Merge(ctx, siteID, BranchDev, BranchMain)
	if err != nil {
		return "", fmt.Errorf("merge dev into main: %w", err)
	}
	files, err := s.repo.ReadFiles(ctx, siteID, mainSHA)
	if err != nil {
		return "", fmt.Errorf("read main tree: %w", err)
	}
	pageFiles, lock, err := s.loadState(ctx, siteID, files)
	if err != nil {
		return "", fmt.Errorf("load published state: %w", err)
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "publish " + shortSHA(mainSHA)
	}
	if err := s.recordSnapshot(ctx, siteID, message, mainSHA, pageFiles, lock); err != nil {
		return "", err
	}
	return mainSHA, nil
}

// Restore overwrites the database state from a commit tree. The current
// database state is committed to dev first as a safety snapshot, so nothing is
// lost. Returns the applied commit sha.
func (s *Service) Restore(ctx context.Context, siteID, sha, message string) (string, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return "", err
	}
	if _, err := s.Commit(ctx, siteID, "restore safety: before restore to "+shortSHA(sha)); err != nil {
		return "", err
	}
	files, err := s.repo.ReadFiles(ctx, siteID, sha)
	if err != nil {
		return "", fmt.Errorf("read commit %s: %w", sha, err)
	}
	pageFiles, lock, err := s.loadState(ctx, siteID, files)
	if err != nil {
		return "", fmt.Errorf("restore state: %w", err)
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "restore " + shortSHA(sha)
	}
	if err := s.recordSnapshot(ctx, siteID, message, sha, pageFiles, lock); err != nil {
		return "", err
	}
	return sha, nil
}

// Rollback rewinds main to a snapshot commit: the current main state becomes a
// dev commit (kept as history), the snapshot tree becomes a new main commit,
// and main is loaded into the database. Returns the new main sha.
func (s *Service) Rollback(ctx context.Context, siteID, snapshotSHA, message string) (string, error) {
	if _, err := s.sites.GetSite(ctx, siteID); err != nil {
		return "", err
	}
	if err := s.repo.InitRepo(ctx, siteID); err != nil {
		return "", err
	}
	mainSHA, err := s.repo.HeadSHA(ctx, siteID, BranchMain)
	if err != nil {
		return "", fmt.Errorf("%w: rollback needs a published main", domain.ErrRepoNotInitialized)
	}
	mainFiles, err := s.repo.ReadFiles(ctx, siteID, mainSHA)
	if err != nil {
		return "", fmt.Errorf("read main tree: %w", err)
	}
	if _, err := s.repo.CommitOnBranch(ctx, siteID, BranchDev, "rollback: current prod", mainFiles); err != nil {
		return "", fmt.Errorf("back up main on dev: %w", err)
	}
	snapFiles, err := s.repo.ReadFiles(ctx, siteID, snapshotSHA)
	if err != nil {
		return "", fmt.Errorf("read snapshot commit %s: %w", snapshotSHA, err)
	}
	message = strings.TrimSpace(message)
	if message == "" {
		message = "rollback to " + shortSHA(snapshotSHA)
	}
	newMainSHA, err := s.repo.CommitOnBranch(ctx, siteID, BranchMain, message, snapFiles)
	if err != nil {
		return "", fmt.Errorf("rewind main: %w", err)
	}
	pageFiles, lock, err := s.loadState(ctx, siteID, snapFiles)
	if err != nil {
		return "", fmt.Errorf("load rolled-back state: %w", err)
	}
	if err := s.recordSnapshot(ctx, siteID, message, newMainSHA, pageFiles, lock); err != nil {
		return "", err
	}
	return newMainSHA, nil
}

// recordSnapshot persists a Snapshot entry capturing the applied git state.
func (s *Service) recordSnapshot(ctx context.Context, siteID, name, gitSHA string, pageFiles map[string]pageFile, lock domain.SnapshotLock) error {
	snapshotID, err := id.New(id.Snapshot)
	if err != nil {
		return err
	}
	refs := make([]domain.SnapshotPage, 0, len(pageFiles))
	for pageID := range pageFiles {
		ref := domain.SnapshotPage{PageID: pageID}
		if versions, err := s.pages.ListPageVersions(ctx, pageID); err == nil && len(versions) > 0 {
			ref.VersionID = versions[len(versions)-1].ID
			ref.Version = versions[len(versions)-1].Number
		}
		refs = append(refs, ref)
	}
	sortSnapshotPages(refs)
	snapshot := domain.Snapshot{
		ID:        snapshotID,
		SiteID:    siteID,
		Name:      name,
		GitSHA:    gitSHA,
		Pages:     refs,
		DepsLock:  lock,
		CreatedAt: s.now().UTC(),
	}
	if err := s.snapshots.CreateSnapshot(ctx, snapshot); err != nil {
		return fmt.Errorf("record snapshot: %w", err)
	}
	return nil
}

// isDirty reports whether the database state differs from a commit tree.
func (s *Service) isDirty(ctx context.Context, siteID, sha string) (bool, error) {
	current, err := s.serializeSansDeps(ctx, siteID)
	if err != nil {
		return false, err
	}
	committed, err := s.repo.ReadFiles(ctx, siteID, sha)
	if err != nil {
		return false, err
	}
	delete(committed, "deps-lock.json")
	return !filesEqual(current, committed), nil
}

func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

func errorsIsRepoNotInitialized(err error) bool {
	return err != nil && strings.Contains(err.Error(), domain.ErrRepoNotInitialized.Error())
}
