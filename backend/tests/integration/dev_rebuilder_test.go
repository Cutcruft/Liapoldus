package integrationtest

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/artifactstore"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/materializer"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/rebuilder"
	"github.com/liapoldus/liapoldus/backend/internal/infra/storage"
)

// TestDevRebuilderRebuildsOnSourceChange drives the real watcher: a workspace
// + esbuild incremental context + on-disk artifacts. Editing a definition must
// produce a new artifact and a broadcast event; a syntax error must broadcast
// "failed" without replacing the last good artifact.
func TestDevRebuilderRebuildsOnSourceChange(t *testing.T) {
	mem := storage.NewMemory()
	site := seedBuildSiteInMemory(t, mem, "site_dev")
	dir := filepath.Join(t.TempDir(), "ws")
	materializeTestWorkspace(t, mem, site.ID, dir)

	hub := rebuilder.NewHub()
	artifacts := artifactstore.New(filepath.Join(t.TempDir(), "build"))
	rb := rebuilder.New(materializer.New(mem, mem, mem, mem, nil, nil, mem), artifacts, hub, rebuilder.Options{
		Request: build.WorkspaceRequest{
			SiteID: site.ID, SnapshotID: "snapshot_e2e",
			Environment: domain.EnvironmentDevelopment, Dir: dir,
		},
		Debounce: 30 * time.Millisecond,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		if err := rb.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("dev rebuilder: %v", err)
		}
	}()

	// Initial build: the materialized text source makes it into the bundle.
	waitBundleContaining(t, hub, site.ID, ".title")

	// Edit the definition source → rebuild → new artifact + broadcast.
	textFile := filepath.Join(dir, "src", "definitions", "text.tsx")
	sourceV2 := "import React from \"react\";\nexport default (props) => props?.title ?? \"MARKER_V2\";\n"
	if err := os.WriteFile(textFile, []byte(sourceV2), 0o644); err != nil {
		t.Fatal(err)
	}
	waitBundleContaining(t, hub, site.ID, "MARKER_V2")

	// Syntax error → failed event, artifact directory left untouched.
	bad := "export default (props => { return props..title; };\n"
	if err := os.WriteFile(textFile, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, hub, site.ID, "failed")
}

// waitBundleContaining polls the hub until the latest "ready" artifact bundle
// contains token (guard against fsnotify/esbuild timing).
func waitBundleContaining(t *testing.T, hub *rebuilder.Hub, siteID, token string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		event, ok := hub.Current(siteID)
		if ok && event.Status == "ready" && event.ArtifactDir != "" {
			for _, file := range distBundleFiles(t, event.ArtifactDir) {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(data), token) {
					return
				}
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("artifact never contained %q (latest event: %+v)", token, mustCurrent(t, hub, siteID))
}

// distBundleFiles lists every build artifact under dist/ (shallow: root
// bundles + dist/pages/ split chunks, minus non-bundle files like index.html).
func distBundleFiles(t *testing.T, artifactDir string) []string {
	t.Helper()
	var files []string
	root := filepath.Join(artifactDir, "dist")
	for _, entry := range mustReadDir(t, root) {
		if entry.IsDir() {
			for _, sub := range mustReadDir(t, filepath.Join(root, entry.Name())) {
				files = append(files, filepath.Join(root, entry.Name(), sub.Name()))
			}
			continue
		}
		files = append(files, filepath.Join(root, entry.Name()))
	}
	return files
}

func waitStatus(t *testing.T, hub *rebuilder.Hub, siteID, status string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		event, ok := hub.Current(siteID)
		if ok && event.Status == status {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("status never became %q (latest event: %+v)", status, mustCurrent(t, hub, siteID))
}

func mustCurrent(t *testing.T, hub *rebuilder.Hub, siteID string) build.DevRebuildEvent {
	t.Helper()
	event, ok := hub.Current(siteID)
	if !ok {
		t.Fatalf("hub has no event for site %s", siteID)
	}
	return event
}

func mustReadDir(t *testing.T, dir string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	return entries
}
