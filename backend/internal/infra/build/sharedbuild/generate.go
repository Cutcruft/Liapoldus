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
	client  *registry.Client
	tempDir string // preserved across Generate calls so a verify reuses the fetch+materialize
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

// Generate resolves + materializes the react family and bundles the reexport
// entries, returning them keyed by the embed-relative path (forward slash) the
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
	return out, nil
}

// Verify regenerates the react family and compares every produced bundle
// against the committed artifact bytes embedded in the binary. Artifacts
// outside the generator's scope (e.g. @liapoldus/ui-runtime before slice 8b)
// are skipped — the full-surface verify is gated on `dependency-build verify`
// after all families are implemented.
func (g *Generator) Verify(ctx context.Context) error {
	generated, err := g.Generate(ctx)
	if err != nil {
		return err
	}
	generatedKeys := make(map[string]bool, len(generated))
	for rel := range generated {
		generatedKeys[rel] = true
	}
	for _, a := range shared.Artifacts {
		rel := "embed/" + a.Key + "/" + a.Version + ".js"
		if !generatedKeys[rel] {
			continue // not in this generator's scope; skip
		}
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
