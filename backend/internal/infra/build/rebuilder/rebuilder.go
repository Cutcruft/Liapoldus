package rebuilder

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/fsnotify/fsnotify"
	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/builder"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/shell"
)

// Options controls one development workspace (a site snapshot under the
// "development" environment). Dir must be an existing or materializable
// workspace directory.
type Options struct {
	Request  build.WorkspaceRequest
	Debounce time.Duration
	Logger   *slog.Logger
}

// Rebuilder keeps a site bundle live for development: it materializes the
// workspace once, opens an incremental esbuild context, watches src/ for
// changes and re-publishes artifacts + broadcasts a DevRebuildEvent after
// every attempt. Failed bundles broadcast status "failed" without replacing
// the last good artifact, so the browser keeps the previous working bundle.
type Rebuilder struct {
	mat       build.WorkspaceBuilder
	artifacts build.ArtifactStore
	hub       build.DevEventHub
	opts      Options
	logger    *slog.Logger
	watcher   *fsnotify.Watcher
	buildCtx  api.BuildContext
	workspace build.Workspace
	mu        sync.Mutex
}

func New(mat build.WorkspaceBuilder, artifacts build.ArtifactStore, hub build.DevEventHub, opts Options) *Rebuilder {
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Rebuilder{mat: mat, artifacts: artifacts, hub: hub, opts: opts, logger: logger}
}

// Start runs until ctx is cancelled: initial build, then the fsnotify loop.
func (r *Rebuilder) Start(ctx context.Context) error {
	if err := r.ensure(ctx); err != nil {
		return fmt.Errorf("dev rebuilder: %w", err)
	}
	defer r.Close()

	outdir := filepath.Join(r.opts.Request.Dir, "dist")
	entryPoints := append([]string{filepath.Join(r.opts.Request.Dir, "src", "entry.tsx")})
	for _, page := range r.workspace.Manifest.Pages {
		entryPoints = append(entryPoints, filepath.Join(r.opts.Request.Dir, "src", "pages", page.PageID+".tsx"))
	}
	buildCtx, ctxErr := api.Context(builder.Options(outdir, entryPoints, r.workspace.Manifest.Externals...))
	if ctxErr != nil {
		return fmt.Errorf("dev rebuilder: esbuild context: %s", strings.Join(esbuildErrors(ctxErr.Errors), "; "))
	}
	r.buildCtx = buildCtx
	defer buildCtx.Dispose()

	if err := r.rebuild(ctx); err != nil {
		return err
	}
	return r.loop(ctx)
}

// Close releases the file watcher (esbuild context is disposed by Start).
func (r *Rebuilder) Close() error {
	if r.watcher == nil {
		return nil
	}
	return r.watcher.Close()
}

// ensure materializes the workspace on the first run (or loads the existing
// manifest) and wires the fsnotify watcher on src/.
func (r *Rebuilder) ensure(ctx context.Context) error {
	entry := filepath.Join(r.opts.Request.Dir, "src", "entry.tsx")
	if _, err := os.Stat(entry); os.IsNotExist(err) {
		ws, err := r.mat.Materialize(ctx, r.opts.Request)
		if err != nil {
			return fmt.Errorf("materialize workspace: %w", err)
		}
		r.workspace = ws
	} else if err != nil {
		return err
	} else if manifest, err := readManifest(r.opts.Request.Dir); err != nil {
		return err
	} else {
		r.workspace = build.Workspace{Dir: r.opts.Request.Dir, Manifest: manifest}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	r.watcher = watcher
	for _, dir := range []string{
		filepath.Join(r.opts.Request.Dir, "src"),
		filepath.Join(r.opts.Request.Dir, "src", "definitions"),
	} {
		if err := watcher.Add(dir); err != nil {
			return fmt.Errorf("fsnotify add %s: %w", dir, err)
		}
	}
	// Page chunks exist only when the snapshot has pages (src/pages is created
	// lazily by the materializer); a missing dir would fail watcher.Add.
	if pagesDir := filepath.Join(r.opts.Request.Dir, "src", "pages"); dirExists(pagesDir) {
		if err := watcher.Add(pagesDir); err != nil {
			return fmt.Errorf("fsnotify add %s: %w", pagesDir, err)
		}
	}
	return nil
}

func dirExists(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

func (r *Rebuilder) loop(ctx context.Context) error {
	var timer *time.Timer
	var timerC <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return nil
		case err, ok := <-r.watcher.Errors:
			if !ok {
				return nil
			}
			if err != nil {
				r.logger.Warn("dev rebuilder watch error", "error", err)
			}
		case event, ok := <-r.watcher.Events:
			if !ok {
				return nil
			}
			if filepath.Ext(event.Name) != ".tsx" {
				continue
			}
			if timer == nil {
				timer = time.NewTimer(r.debounce())
				timerC = timer.C
			} else {
				timer.Reset(r.debounce())
			}
		case <-timerC:
			timer.Stop()
			timer = nil
			timerC = nil
			if err := r.rebuild(ctx); err != nil {
				return err
			}
		}
	}
}

// rebuild runs the incremental esbuild context, publishes the result and
// broadcasts the outcome. A failed bundle stays a "failed" event; a publish
// failure is a hard error (Start aborts).
func (r *Rebuilder) rebuild(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := r.buildCtx.Rebuild()
	event := build.DevRebuildEvent{
		SiteID:      r.opts.Request.SiteID,
		Environment: r.opts.Request.Environment,
		SnapshotID:  r.opts.Request.SnapshotID,
		Status:      string(domain.BuildStatusReady),
		UpdatedAt:   time.Now().UTC(),
	}
	if len(result.Errors) > 0 {
		event.Status = string(domain.BuildStatusFailed)
		event.Error = strings.Join(esbuildErrors(result.Errors), "; ")
		r.hub.Publish(event)
		r.logger.Warn("dev rebuild failed", "errors", event.Error)
		return nil
	}

	if err := shell.WriteShell(filepath.Join(r.opts.Request.Dir, "dist"), r.workspace.Manifest); err != nil {
		return err
	}
	artifactDir, err := r.artifacts.Publish(ctx, r.opts.Request.SiteID, r.opts.Request.Environment,
		r.opts.Request.SnapshotID, r.workspace, build.BundleResult{DistDir: filepath.Join(r.opts.Request.Dir, "dist")})
	if err != nil {
		return fmt.Errorf("dev rebuilder publish: %w", err)
	}
	event.ArtifactDir = artifactDir
	r.hub.Publish(event)
	r.logger.Info("dev rebuild ready", "artifact", artifactDir)
	return nil
}

func (r *Rebuilder) debounce() time.Duration {
	if r.opts.Debounce > 0 {
		return r.opts.Debounce
	}
	return 150 * time.Millisecond
}

func esbuildErrors(errs []api.Message) []string {
	messages := make([]string, 0, len(errs))
	for _, err := range errs {
		messages = append(messages, err.Text)
	}
	return messages
}

func readManifest(dir string) (build.Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return build.Manifest{}, fmt.Errorf("read manifest.json: %w", err)
	}
	var manifest build.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return build.Manifest{}, fmt.Errorf("decode manifest.json: %w", err)
	}
	return manifest, nil
}
