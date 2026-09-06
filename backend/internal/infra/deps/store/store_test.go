package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreSaveOpenHas(t *testing.T) {
	root := t.TempDir()
	s := New(root)

	if s.Has("lodash", "4.17.21") {
		t.Fatal("must not have before save")
	}
	if err := s.Save("lodash", "4.17.21", []byte("tarball-a")); err != nil {
		t.Fatalf("save: %v", err)
	}
	if !s.Has("lodash", "4.17.21") {
		t.Fatal("must have after save")
	}
	got, err := s.Open("lodash", "4.17.21")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if string(got) != "tarball-a" {
		t.Fatalf("open = %q", got)
	}

	// Layout: <root>/<name>/<version>.tgz
	path := filepath.Join(root, "lodash", "4.17.21.tgz")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("layout %s: %v", path, err)
	}
}

func TestStoreScopedLayout(t *testing.T) {
	root := t.TempDir()
	s := New(root)

	if err := s.Save("@babel/core", "7.23.0", []byte("scoped")); err != nil {
		t.Fatalf("save scoped: %v", err)
	}
	if !s.Has("@babel/core", "7.23.0") {
		t.Fatal("must have scoped")
	}
	path := filepath.Join(root, "@babel", "core", "7.23.0.tgz")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("scoped layout %s: %v", path, err)
	}
}

func TestStoreSaveIdempotentConcurrent(t *testing.T) {
	root := t.TempDir()
	s := New(root)

	done := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func() {
			done <- s.Save("react", "18.3.1", []byte("same-tarball"))
		}()
	}
	for i := 0; i < 4; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent save: %v", err)
		}
	}
	got, err := s.Open("react", "18.3.1")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if string(got) != "same-tarball" {
		t.Fatalf("open = %q", got)
	}
}

func TestStoreOpenMissing(t *testing.T) {
	s := New(t.TempDir())
	if _, err := s.Open("no", "such"); err == nil {
		t.Fatal("expected error for missing tarball")
	}
}

func TestImportSpecifier(t *testing.T) {
	if _, err := ImportSpecifier("", "1.0.0"); err == nil {
		t.Fatal("expected error for empty name")
	}
	if _, err := ImportSpecifier("lodash", ""); err == nil {
		t.Fatal("expected error for empty version")
	}
	if _, err := ImportSpecifier("lodash", "1/2"); err == nil {
		t.Fatal("expected error for version with slash")
	}
	got, err := ImportSpecifier("@scope/pkg", "1.0.0")
	if err != nil {
		t.Fatalf("import specifier: %v", err)
	}
	if got != "@scope/pkg" {
		t.Fatalf("import specifier = %q", got)
	}
}
