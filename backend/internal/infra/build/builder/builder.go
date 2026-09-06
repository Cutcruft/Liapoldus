package builder

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/shell"
)

// Builder compiles a materialized workspace with esbuild as a Go library
// (R2): the site bundle declares the shared libraries external and esbuild
// writes dist/ inside the workspace directory.
type Builder struct{}

func New() *Builder { return &Builder{} }

var _ build.BundleRunner = (*Builder)(nil)

// Options is the shared esbuild configuration for site bundles. The dev
// rebuilder reuses it so the incremental context and the one-shot build never
// drift apart. External libraries (react, site deps, ...) come from the
// materialized manifest so the site bundle keeps them as runtime imports.
// With multiple entry points (split page chunks), esbuild code-splits shared
// code automatically (spec §16); outdir holds dist/.
func Options(outdir string, entryPoints []string, externals ...string) api.BuildOptions {
	external := append(append([]string{}, build.SharedExternals...), externals...)
	return api.BuildOptions{
		EntryPoints:       entryPoints,
		Outdir:            outdir,
		Bundle:            true,
		Write:             true,
		Splitting:         true,
		Format:            api.FormatESModule,
		Platform:          api.PlatformBrowser,
		Target:            api.ES2020,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		TreeShaking:       api.TreeShakingTrue,
		External:          external,
		LogLevel:          api.LogLevelSilent,
	}
}

func (b *Builder) Build(_ context.Context, workspace build.Workspace) (build.BundleResult, error) {
	outdir := filepath.Join(workspace.Dir, "dist")
	entryPoints := append([]string{filepath.Join(workspace.Dir, "src", "entry.tsx")})
	for _, page := range workspace.Manifest.Pages {
		entryPoints = append(entryPoints, filepath.Join(workspace.Dir, "src", "pages", page.PageID+".tsx"))
	}
	result := api.Build(Options(outdir, entryPoints, workspace.Manifest.Externals...))
	if len(result.Errors) > 0 {
		messages := make([]string, 0, len(result.Errors))
		for _, err := range result.Errors {
			messages = append(messages, err.Text)
		}
		return build.BundleResult{}, fmt.Errorf("esbuild failed: %s", strings.Join(messages, "; "))
	}
	if err := shell.WriteShell(outdir, workspace.Manifest); err != nil {
		return build.BundleResult{}, fmt.Errorf("build shell: %w", err)
	}
	return build.BundleResult{DistDir: outdir}, nil
}
