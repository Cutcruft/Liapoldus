package sharedbuild

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// reactBundle is the reexport surface supplied to esbuild for the react-family
// artifacts. React itself is bundled into the react bundle (the output IS the
// full React runtime with the namespace as default export); react-dom and
// react/jsx-runtime keep "react" external so the browser import-map serves one
// shared React instance to all site bundles.
//
// Source mirrors scripts/build-shared/build.mjs — the two must stay in lockstep
// (spec §10: cmd/dependency-build is the zero-node replacement).
type reactBundle struct {
	// key is the import-map key (Artifacts.Key), e.g. "react", "react-dom", "react/jsx-runtime".
	key string
	// contents is the reexport entry fed to esbuild via stdin.
	contents string
	// external lists bare specifiers kept external (import-map keys).
	external []string
	// markers are the public symbols the smoke check asserts on the output.
	markers []string
}

var reactBundles = []reactBundle{
	{
		key: "react",
		contents: `import React from "react";
export default React;
export * from "react";
`,
		markers: []string{"createElement", "default"},
	},
	{
		key: "react-dom",
		contents: `import ReactDOM from "react-dom";
import { createRoot, hydrateRoot } from "react-dom/client";
export default ReactDOM;
export { createRoot, hydrateRoot };
export * from "react-dom";
`,
		external: []string{"react"},
		markers:  []string{"createRoot", "default"},
	},
	{
		key: "react/jsx-runtime",
		contents: `import { Fragment, jsx, jsxs } from "react/jsx-runtime";
export { Fragment, jsx, jsxs };
`,
		external: []string{"react"},
		markers:  []string{"jsx", "Fragment"},
	},
}

// bundleReact builds the react-family artifacts against a materialized
// workspace: each reexport entry is compiled with esbuild (Write:false, output
// captured in memory) and returned as keyed by its embed-relative path
// (embed/react/18.3.1.js etc.).
func bundleReact(version string, wsDir string) (map[string][]byte, error) {
	out := make(map[string][]byte, len(reactBundles))
	for _, b := range reactBundles {
		opts := api.BuildOptions{
			Bundle:            true,
			Write:             false,
			Format:            api.FormatESModule,
			Platform:          api.PlatformBrowser,
			Target:            api.ES2020,
			MinifyWhitespace:  true,
			MinifyIdentifiers: true,
			MinifySyntax:      true,
			TreeShaking:       api.TreeShakingTrue,
			LogLevel:          api.LogLevelSilent,
			Stdin: &api.StdinOptions{
				Contents:   b.contents,
				ResolveDir: filepath.Join(wsDir, "src"),
				Sourcefile: "shared-reexport.ts",
				Loader:     api.LoaderTS,
			},
			External:  append([]string{}, b.external...),
			NodePaths: []string{wsDir},
		}

		result := api.Build(opts)
		if len(result.Errors) > 0 {
			messages := make([]string, 0, len(result.Errors))
			for _, err := range result.Errors {
				messages = append(messages, err.Text)
			}
			return nil, fmt.Errorf("shared bundle %s: %s", b.key, strings.Join(messages, "; "))
		}
		if len(result.OutputFiles) == 0 {
			return nil, fmt.Errorf("shared bundle %s: esbuild produced no output", b.key)
		}
		data := result.OutputFiles[0].Contents

		source := string(data)
		for _, marker := range b.markers {
			if !strings.Contains(source, marker) {
				return nil, fmt.Errorf("shared bundle %s is missing %s", b.key, marker)
			}
		}

		out[filepath.Join("embed", filepath.FromSlash(b.key), version+".js")] = data
	}
	return out, nil
}
