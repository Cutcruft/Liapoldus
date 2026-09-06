package shared

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed embed
var Files embed.FS

// Artifact pins one shared runtime library to the version bundled into the
// server binary. Key is the bare import specifier (import-map key); the ESM
// bundle lives at embed/<Key>/<Version>.js and is served from the artifact
// root at _shared/<Key>/<Version>.js via the client /build/ handler.
type Artifact struct {
	Key     string
	Version string
}

// Artifacts is the import-map surface of the site bundler. It MUST stay in
// sync with the generator's pins (cmd/dependency-build, which resolves and
// bundles every entry below) — bump both together and re-run
// `go run ./cmd/dependency-build generate` so the committed bundles match.
var Artifacts = []Artifact{
	{Key: "react", Version: "18.3.1"},
	{Key: "react-dom", Version: "18.3.1"},
	{Key: "react/jsx-runtime", Version: "18.3.1"},
	{Key: "@liapoldus/ui-runtime", Version: "0.1.0"},
}

// embedRel is the forward-slash path of the artifact inside the embedded FS.
func embedRel(a Artifact) string {
	return filepath.ToSlash(filepath.Join("embed", a.Key, a.Version+".js"))
}

func artifactData(a Artifact) ([]byte, error) {
	return fs.ReadFile(Files, embedRel(a))
}

// PublicRel is the artifact-root-relative path served to browsers
// (BuildDir-relative, e.g. "_shared/react/18.3.1.js").
func PublicRel(a Artifact) string {
	return filepath.ToSlash(filepath.Join("_shared", a.Key, a.Version+".js"))
}

// PublicURL is the absolute public URL the client server serves the artifact
// at (its /build/ handler maps onto the artifact root).
func PublicURL(a Artifact) string {
	return "/build/" + PublicRel(a)
}

// Versions maps the shared external specifier keys to their pinned versions.
// The dependency peer policy (spec §5) uses the map to decide whether a fixed
// shared external — react, react-dom, react/jsx-runtime, @liapoldus/ui-runtime —
// satisfies a peer range.
func Versions() map[string]string {
	m := make(map[string]string, len(Artifacts))
	for _, a := range Artifacts {
		m[a.Key] = a.Version
	}
	return m
}

// Install materialises the embedded shared bundles under root/_shared/... so
// the existing client /build/ handler can serve them without any runtime
// compilation. Idempotent: existing files are never overwritten, so an
// operator edit in the artifact root survives restart.
func Install(root string) error {
	for _, a := range Artifacts {
		data, err := artifactData(a)
		if err != nil {
			return fmt.Errorf("shared %s: %w", a.Key, err)
		}
		dest := filepath.Join(root, filepath.FromSlash(PublicRel(a)))
		if _, err := os.Stat(dest); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("shared %s: %w", a.Key, err)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("shared %s: %w", a.Key, err)
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return fmt.Errorf("shared %s: %w", a.Key, err)
		}
	}
	return nil
}

// Resolver canonicalises the shared artifacts to public URLs so the
// materializer can record the import map in the build manifest.
type Resolver struct{}

func NewResolver() *Resolver { return &Resolver{} }

// SharedURLs maps bare import specifiers (import-map keys) to the public URLs
// of the versioned shared bundles.
func (*Resolver) SharedURLs() map[string]string {
	m := make(map[string]string, len(Artifacts))
	for _, a := range Artifacts {
		m[a.Key] = PublicURL(a)
	}
	return m
}
