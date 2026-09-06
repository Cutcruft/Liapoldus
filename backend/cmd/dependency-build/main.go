// Command dependency-build regenerates the versioned shared runtime bundles
// embedded into the Go server (internal/infra/build/shared/embed) with zero
// node/npm on the host: react/react-dom/jsx-runtime are resolved and fetched
// straight from the npm registry, and @liapoldus/ui-runtime is compiled from
// the monorepo's ui-runtime/src with esbuild-as-a-Go-library — covering the
// whole Artifacts table (spec §10).
//
// Usage:
//
//	dependency-build generate [--registry URL] [--temp-dir DIR]
//	dependency-build verify   [--registry URL] [--temp-dir DIR]
//
// generate bundles every shared artifact and writes
// internal/infra/build/shared/embed/<key>/<version>.js for each one;
// verify regenerates in a temp workspace and fails if any committed artifact
// differs from the freshly generated bundle (CI gate on schema-sync).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/liapoldus/liapoldus/backend/internal/infra/build/sharedbuild"
	"github.com/liapoldus/liapoldus/backend/internal/infra/deps/registry"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	// Subcommand in argv[1]: generate | verify.
	cmd := os.Args[1]

	fs := flag.NewFlagSet("dependency-build "+cmd, flag.ExitOnError)
	registryURL := fs.String("registry", defaultRegistry(), "npm registry base URL")
	tempDir := fs.String("temp-dir", os.TempDir(), "workspace directory for resolve/materialize")
	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}

	if *registryURL == "" {
		fmt.Fprintln(os.Stderr, "registry URL is required (set --registry or LIAPOLDUS_NPM_REGISTRY)")
		os.Exit(2)
	}

	ctx := context.Background()
	gen := sharedbuild.NewGenerator(registry.New(*registryURL), *tempDir)
	src, err := uiRuntimeSrc()
	if err != nil {
		fatal(err)
	}
	gen = gen.WithUIRuntime(src)

	switch cmd {
	case "generate":
		written, err := gen.Generate(ctx)
		if err != nil {
			fatal(err)
		}
		root, err := embedRoot()
		if err != nil {
			fatal(err)
		}
		if err := writeBundles(root, written); err != nil {
			fatal(err)
		}
		fmt.Printf("generated %d shared bundles under %s\n", len(written), root)
	case "verify":
		if err := gen.Verify(ctx); err != nil {
			fatal(err)
		}
		fmt.Println("verify ok: committed shared bundles match the freshly generated ones")
	default:
		usage()
		os.Exit(2)
	}
}

// writeBundles writes generated embed-relative bundles (forward-slash paths)
// under root. Keys include the "embed/" prefix matching the shared.Files
// structure; the prefix is stripped to write files relative to the embed dir.
func writeBundles(root string, bundles map[string][]byte) error {
	const embedPrefix = "embed/"
	for rel, data := range bundles {
		relPath := strings.TrimPrefix(rel, embedPrefix)
		dest := filepath.Join(root, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// embedRoot resolves the backend's internal/infra/build/shared/embed directory
// relative to the working directory (the generator is typically run from the
// backend module root).
func embedRoot() (string, error) {
	rel := filepath.Join("internal", "infra", "build", "shared", "embed")
	if _, err := os.Stat(rel); err == nil {
		abs, aerr := filepath.Abs(rel)
		return abs, aerr
	}
	try := filepath.Join("backend", "internal", "infra", "build", "shared", "embed")
	if _, err := os.Stat(try); err == nil {
		return filepath.Abs(try)
	}
	return "", fmt.Errorf("cannot locate shared/embed (run from the backend module root): %w", os.ErrNotExist)
}

// uiRuntimeSrc locates the monorepo's ui-runtime/src by walking up from the
// working directory until an ancestor contains ui-runtime/src/index.ts. Unlike
// embedRoot it does not assume a particular starting module, so generate/verify
// work from both the repo root and the backend module root.
func uiRuntimeSrc() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 12; i++ {
		cand := filepath.Join(dir, "ui-runtime", "src")
		if info, serr := os.Stat(filepath.Join(cand, "index.ts")); serr == nil && !info.IsDir() {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("cannot locate ui-runtime/src (run dependency-build from the repo or backend module root)")
}

func defaultRegistry() string {
	return os.Getenv("LIAPOLDUS_NPM_REGISTRY")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "dependency-build:", err)
	os.Exit(1)
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  dependency-build generate [--registry URL] [--temp-dir DIR]
  dependency-build verify   [--registry URL] [--temp-dir DIR]`)
}
