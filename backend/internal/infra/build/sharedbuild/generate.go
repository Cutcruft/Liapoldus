package sharedbuild

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/liapoldus/liapoldus/backend/internal/infra/build/shared"
	"github.com/liapoldus/liapoldus/backend/internal/infra/deps/registry"
)

// Generator regenerates the React-family shared bundles (react, react-dom,
// react/jsx-runtime) from the npm registry with zero node/npm on the host. Each
// run resolves and fetches whatever the pinned ranges select, materializes the
// frozen lock into a fresh temp workspace, bundles the reexport entries and
// returns the embed-relative path -> bytes for the caller to write (Generate) or
// compare (Verify).
type Generator struct {
	client *registry.Client
	// tempDir is preserved across Generate calls so a verify reuses the
	// fetch+materialize (the ui-runtime source is compiled in place).
	tempDir string
	// uiRuntimeSrc is the absolute path to the monorepo's ui-runtime/src.
	// When set, Generate also compiles @liapoldus/ui-runtime (slice 8b);
	// cmd/dependency-build always wires it, so generate covers the full
	// Artifacts table.
	uiRuntimeSrc string
}

// NewGenerator wires the standalone infra (registry client + temp workspace).
// tempDir must already exist and is reused across Generate calls so a verify
// reuses the fetch+materialize.
func NewGenerator(client *registry.Client, tempDir string) *Generator {
	return &Generator{
		client:  client,
		tempDir: tempDir,
	}
}

// WithUIRuntime points the generator at the checkout of ui-runtime/src (the
// monorepo's runtime source). Required for generate/verify to cover the whole
// Artifacts table.
func (g *Generator) WithUIRuntime(src string) *Generator {
	g.uiRuntimeSrc = src
	return g
}

// Generate resolves + materializes the react family, bundles the reexport
// entries and — when wired via WithUIRuntime — compiles @liapoldus/ui-runtime,
// returning the bundles keyed by embed-relative path (forward slash) the
// shared package expects (embed/react/18.3.1.js, ...).
func (g *Generator) Generate(ctx context.Context) (map[string][]byte, error) {
	result, err := resolveLock(ctx, g.client)
	if err != nil {
		return nil, err
	}

	wsDir := filepath.Join(g.tempDir, "ws")
	if err := os.MkdirAll(filepath.Join(wsDir, "src"), 0o755); err != nil {
		return nil, err
	}
	if err := materialize(ctx, g.client, wsDir, result); err != nil {
		return nil, err
	}

	version := ""
	for _, dep := range result.lock.Deps {
		if dep.Name == "react" {
			version = dep.Version
			break
		}
	}
	if version == "" {
		return nil, fmt.Errorf("shared lock resolved no react version")
	}

	bundled, err := bundleReact(version, wsDir)
	if err != nil {
		return nil, err
	}

	out := make(map[string][]byte, len(bundled))
	for rel, data := range bundled {
		out[filepath.ToSlash(rel)] = data
	}

	if g.uiRuntimeSrc != "" {
		art, err := uiRuntimeArtifact()
		if err != nil {
			return nil, err
		}
		uiBundle, err := bundleUIRuntime(art.Version, g.uiRuntimeSrc)
		if err != nil {
			return nil, err
		}
		uiRel := filepath.ToSlash(filepath.Join("embed", art.Key, art.Version+".js"))
		out[uiRel] = uiBundle
	}
	return out, nil
}

// Verify regenerates every shared artifact and compares each produced bundle
// against the committed artifact bytes embedded in the binary. It is the
// full-surface gate on the Artifacts table: an artifact the generator did not
// produce — e.g. a ui-runtime pin when WithUIRuntime was not wired — fails the
// verify instead of being skipped.
func (g *Generator) Verify(ctx context.Context) error {
	generated, err := g.Generate(ctx)
	if err != nil {
		return err
	}
	generatedKeys := make(map[string]bool, len(generated))
	for rel := range generated {
		generatedKeys[rel] = true
	}

	var missing []string
	for _, a := range shared.Artifacts {
		rel := "embed/" + a.Key + "/" + a.Version + ".js"
		if !generatedKeys[rel] {
			missing = append(missing, a.Key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("verify: generator scope does not cover artifacts %s (full-surface gate; wire its build step)",
			strings.Join(missing, ", "))
	}

	for _, a := range shared.Artifacts {
		rel := "embed/" + a.Key + "/" + a.Version + ".js"
		data, err := fs.ReadFile(shared.Files, rel)
		if err != nil {
			return fmt.Errorf("verify %s: committed artifact missing: %w", a.Key, err)
		}
		want, ok := generated[rel]
		if !ok {
			return fmt.Errorf("verify %s: generator produced no artifact", a.Key)
		}
		if string(data) != string(want) {
			return fmt.Errorf("verify %s: committed artifact differs from the freshly generated bundle (run generate and commit)", a.Key)
		}
	}
	return nil
}

// verifyBundles compares freshly generated bundles (embed-relative paths) to
// the committed bytes. A key present in committed but absent in generated — or
// bytes that differ — is reported with the artifact name.
func verifyBundles(generated, committed map[string][]byte) error {
	for rel, want := range committed {
		key := strings.TrimSuffix(strings.TrimPrefix(rel, "embed/"), ".js")
		data, ok := generated[rel]
		if !ok {
			return fmt.Errorf("verify %s: generator produced no artifact", key)
		}
		if string(data) != string(want) {
			return fmt.Errorf("verify %s: committed artifact differs from the freshly generated bundle (run generate and commit)", key)
		}
	}
	return nil
}
