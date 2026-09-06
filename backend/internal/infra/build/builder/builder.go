package builder

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/liapoldus/liapoldus/backend/internal/application/build"
)

// Builder compiles a materialized workspace with esbuild as a Go library
// (R2): the site bundle declares the shared libraries external and esbuild
// writes dist/ inside the workspace directory.
type Builder struct{}

func New() *Builder { return &Builder{} }

var _ build.BundleRunner = (*Builder)(nil)

// Options is the shared esbuild configuration for site bundles. The dev
// rebuilder reuses it so the incremental context and the one-shot build never
// drift apart.
func Options(entryPoint, outdir string) api.BuildOptions {
	return api.BuildOptions{
		EntryPoints:       []string{entryPoint},
		Outdir:            outdir,
		Bundle:            true,
		Write:             true,
		Format:            api.FormatESModule,
		Platform:          api.PlatformBrowser,
		Target:            api.ES2020,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		TreeShaking:       api.TreeShakingTrue,
		External:          build.SharedExternals,
		LogLevel:          api.LogLevelSilent,
	}
}

func (b *Builder) Build(_ context.Context, workspace build.Workspace) (build.BundleResult, error) {
	outdir := filepath.Join(workspace.Dir, "dist")
	result := api.Build(Options(filepath.Join(workspace.Dir, "src", "entry.tsx"), outdir))
	if len(result.Errors) > 0 {
		messages := make([]string, 0, len(result.Errors))
		for _, err := range result.Errors {
			messages = append(messages, err.Text)
		}
		return build.BundleResult{}, fmt.Errorf("esbuild failed: %s", strings.Join(messages, "; "))
	}
	return build.BundleResult{DistDir: outdir}, nil
}
