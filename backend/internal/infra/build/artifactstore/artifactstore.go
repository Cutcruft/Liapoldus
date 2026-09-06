package artifactstore

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// Store is the on-disk build artifact directory (R3). Layout:
//
//	<root>/<site>/<environment>/<snapshot>/dist/<bundle>.js
//	<root>/<site>/<environment>/<snapshot>/manifest.json
//	<root>/_shared/<name>/<version>.js
//
// Publishing is write-and-rename: a build lands atomically, staging dirs are
// removed, and re-publishing the same snapshot id (no-op) replaces the
// previous artifact without corrupting the target.
type Store struct {
	root string
}

func New(root string) *Store {
	return &Store{root: root}
}

// DirFor is the artifact directory for one released snapshot.
func (s *Store) DirFor(siteID, environment, snapshotID string) string {
	return s.dirFor(siteID, environment, snapshotID)
}

// Root is the mount point for build static serving (/build/).
func (s *Store) Root() string {
	return s.root
}

func (s *Store) dirFor(siteID, environment, snapshotID string) string {
	return filepath.Join(s.root, siteID, environment, snapshotID)
}

func (s *Store) Publish(_ context.Context, siteID, environment, snapshotID string, workspace build.Workspace, _ build.BundleResult) (string, error) {
	fail := func(err error) (string, error) {
		return "", err
	}
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return fail(fmt.Errorf("artifactstore root: %w", err))
	}
	target := s.dirFor(siteID, environment, snapshotID)
	staging, err := os.MkdirTemp(s.root, ".publish-*")
	if err != nil {
		return fail(fmt.Errorf("artifactstore staging: %w", err))
	}
	defer os.RemoveAll(staging)

	dist := filepath.Join(workspace.Dir, "dist")
	if err := copyTree(dist, filepath.Join(staging, "dist")); err != nil {
		return fail(fmt.Errorf("artifactstore dist: %w", err))
	}
	manifestSrc := filepath.Join(workspace.Dir, "manifest.json")
	if _, err := os.Stat(manifestSrc); err != nil {
		return fail(fmt.Errorf("artifactstore manifest: %w", err))
	}
	if err := copyFile(manifestSrc, filepath.Join(staging, "manifest.json")); err != nil {
		return fail(fmt.Errorf("artifactstore manifest: %w", err))
	}

	// Atomic replacement of the previous artifact for this snapshot.
	if _, err := os.Stat(target); err == nil {
		if err := os.RemoveAll(target); err != nil {
			return fail(fmt.Errorf("artifactstore replace: %w", err))
		}
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fail(fmt.Errorf("artifactstore mkdir: %w", err))
	}
	if err := os.Rename(staging, target); err != nil {
		return fail(fmt.Errorf("artifactstore publish: %w", err))
	}
	return target, nil
}

func (s *Store) GetBundle(_ context.Context, siteID, environment, snapshotID, fileName string) ([]byte, error) {
	name := filepath.Base(fileName)
	if filepath.Clean(fileName) != name {
		return nil, fmt.Errorf("%w: invalid artifact name %q", domain.ErrInvalidRequest, fileName)
	}
	path := filepath.Join(s.dirFor(siteID, environment, snapshotID), "dist", name)
	return readFile(path)
}

func (s *Store) GetManifest(_ context.Context, siteID, environment, snapshotID string) ([]byte, error) {
	return readFile(filepath.Join(s.dirFor(siteID, environment, snapshotID), "manifest.json"))
}

func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", domain.ErrNotFound, path)
		}
		return nil, err
	}
	return data, nil
}

func copyTree(src, dst string) error {
	return fs.WalkDir(os.DirFS(src), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(path))
		if d.IsDir() {
			if path == "." {
				return os.MkdirAll(dst, 0o755)
			}
			return os.MkdirAll(target, 0o755)
		}
		if path == "." {
			return nil
		}
		return copyFile(filepath.Join(src, filepath.FromSlash(path)), target)
	})
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
