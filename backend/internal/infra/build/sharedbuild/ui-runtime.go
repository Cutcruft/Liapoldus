package sharedbuild

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/shared"
)

// uiRuntimeKey is the import-map key of the site runtime artifact (the
// Artifacts table in internal/infra/build/shared). Unlike the react family,
// the ui-runtime bundle is NOT resolved from the npm registry: it is the
// monorepo's own runtime compiled from ui-runtime/src, so the generator's
// ui-runtime phase is off-network (spec §10).
const uiRuntimeKey = "@liapoldus/ui-runtime"

// uiRuntimeArtifact returns the Artifacts-table pin for @liapoldus/ui-runtime —
// the single source of truth for the version embedded beside the react family
// (shared.Artifacts MUST stay in sync with ui-runtime/package.json).
func uiRuntimeArtifact() (shared.Artifact, error) {
	for _, a := range shared.Artifacts {
		if a.Key == uiRuntimeKey {
			return a, nil
		}
	}
	return shared.Artifact{}, fmt.Errorf("shared.Artifacts has no %s entry", uiRuntimeKey)
}

// bundleUIRuntime compiles ui-runtime/src/index.ts with esbuild-as-a-Go-library
// into one ESM module. react and react-dom stay external so the import map
// serves one shared React instance to both the runtime and the site bundles
// (react/jsx-runtime is externalized with react automatically, matching the
// node build.mjs behavior the committed artifact was produced with).
func bundleUIRuntime(version, srcDir string) ([]byte, error) {
	opts := api.BuildOptions{
		Bundle:            true,
		Write:             false,
		Format:            api.FormatESModule,
		Platform:          api.PlatformBrowser,
		Target:            api.ES2020,
		JSX:               api.JSXAutomatic,
		TreeShaking:       api.TreeShakingTrue,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		LogLevel:          api.LogLevelSilent,
		EntryPoints:       []string{filepath.Join(srcDir, "index.ts")},
		External:          []string{"react", "react-dom"},
	}

	result := api.Build(opts)
	if len(result.Errors) > 0 {
		messages := make([]string, 0, len(result.Errors))
		for _, err := range result.Errors {
			messages = append(messages, err.Text)
		}
		return nil, fmt.Errorf("shared bundle %s: %s", uiRuntimeKey, strings.Join(messages, "; "))
	}
	if len(result.OutputFiles) == 0 {
		return nil, fmt.Errorf("shared bundle %s: esbuild produced no output", uiRuntimeKey)
	}
	data := result.OutputFiles[0].Contents

	source := string(data)
	// Smoke check on the ESM surface: public exports survive minification and
	// the React surface is served by the shared import map — esbuild
	// externalizes react/jsx-runtime together with the external "react" root,
	// so a self-contained ui-runtime (no bare import) means a duplicated React
	// instance and an inconsistent hook identity.
	for _, marker := range []string{"boot", "react/jsx-runtime"} {
		if !strings.Contains(source, marker) {
			return nil, fmt.Errorf("shared bundle %s is missing %s", uiRuntimeKey, marker)
		}
	}

	return data, nil
}
