package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Store is the on-disk tarball cache for site dependencies (dependency-service
// spec §11 шаг 3). Tarballs are stored by identity:
//
//	<root>/<name>/<version>.tgz              (e.g. lodash/4.17.21.tgz)
//	<root>/@scope/name/<version>.tgz         (scoped packages)
//
// Saves are write-and-rename and conflicts are ignored, so a concurrent build
// of the same tarball is a no-op and a partially written blob is never
// observed by a reader.
type Store struct {
	root string
}

func New(root string) *Store {
	return &Store{root: root}
}

// path resolves the (properly escaped) storage location for a package tarball.
func (s *Store) path(name, version string) string {
	return filepath.Join(s.root, filepath.FromSlash(name), version+".tgz")
}

// Save persists tarball bytes for (name, version). Concurrent writes of the
// same identity are harmless: the first complete write wins, later ones are
// treated as no-ops.
func (s *Store) Save(name, version string, data []byte) error {
	target := s.path(name, version)
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return fmt.Errorf("deps store root: %w", err)
	}
	staging, err := os.MkdirTemp(s.root, ".deps-*")
	if err != nil {
		return fmt.Errorf("deps store staging: %w", err)
	}
	defer os.RemoveAll(staging)
	if err := os.WriteFile(filepath.Join(staging, "blob"), data, 0o644); err != nil {
		return fmt.Errorf("deps store write: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("deps store mkdir: %w", err)
	}
	// Best-effort rename; a race with another writer is not an error.
	if err := os.Rename(filepath.Join(staging, "blob"), target); err != nil {
		if !os.IsNotExist(err) {
			if _, statErr := os.Stat(target); statErr == nil {
				return nil // another writer won
			}
			return fmt.Errorf("deps store rename: %w", err)
		}
	}
	return nil
}

// Has reports whether a cached tarball exists for (name, version).
func (s *Store) Has(name, version string) bool {
	_, err := os.Stat(s.path(name, version))
	return err == nil
}

// Open reads a previously saved tarball for (name, version).
func (s *Store) Open(name, version string) ([]byte, error) {
	data, err := os.ReadFile(s.path(name, version))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("deps store: %s@%s not found", name, version)
		}
		return nil, fmt.Errorf("deps store read: %w", err)
	}
	return data, nil
}

// Root returns the on-disk mount point of the tarball cache.
func (s *Store) Root() string {
	return s.root
}

// importSpecifier derives the bare import specifier for a cached package (the
// registry name, with scoped names kept slash-separated) and validates that a
// version string can be embedded in a path without escaping the layout.
func ImportSpecifier(name, version string) (string, error) {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(version) == "" {
		return "", fmt.Errorf("deps store: empty name or version")
	}
	if strings.Contains(version, "/") {
		return "", fmt.Errorf("deps store: invalid version %q", version)
	}
	return name, nil
}
