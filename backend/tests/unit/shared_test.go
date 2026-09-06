package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/infra/build/shared"
)

// TestSharedArtifactsAreEmbedded guards against drift between the build script
// and the Go Artifacts table: every declared artifact must ship inside the
// binary, otherwise Install fails at bootstrap.
func TestSharedArtifactsAreEmbedded(t *testing.T) {
	for _, a := range shared.Artifacts {
		if a.Key == "" || a.Version == "" {
			t.Fatalf("artifact %+v must pin a concrete version", a)
		}
		root := t.TempDir()
		if err := shared.Install(root); err != nil {
			t.Fatalf("install %s: %v", a.Key, err)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(shared.PublicRel(a)))); err != nil {
			t.Fatalf("installed artifact %s missing: %v", a.Key, err)
		}
	}
}

func TestSharedPublicURLs(t *testing.T) {
	cases := map[string]string{
		"react":                 "/build/_shared/react/18.3.1.js",
		"react-dom":             "/build/_shared/react-dom/18.3.1.js",
		"react/jsx-runtime":     "/build/_shared/react/jsx-runtime/18.3.1.js",
		"@liapoldus/ui-runtime": "/build/_shared/@liapoldus/ui-runtime/0.1.0.js",
	}
	for key, want := range cases {
		var found *shared.Artifact
		for i := range shared.Artifacts {
			if shared.Artifacts[i].Key == key {
				found = &shared.Artifacts[i]
				break
			}
		}
		if found == nil {
			t.Fatalf("artifact %s not declared", key)
		}
		if got := shared.PublicURL(*found); got != want {
			t.Fatalf("PublicURL(%s) = %s, want %s", key, got, want)
		}
	}
}

func TestSharedInstallIsIdempotentAndWriteOnce(t *testing.T) {
	root := t.TempDir()
	if err := shared.Install(root); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(root, "_shared", "react", "18.3.1.js")
	data, err := os.ReadFile(probe)
	if err != nil {
		t.Fatalf("read installed react: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("installed react bundle is empty")
	}
	// A second install must not overwrite a manually-tuned artifact.
	if err := os.WriteFile(probe, []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := shared.Install(root); err != nil {
		t.Fatalf("second install: %v", err)
	}
	again, err := os.ReadFile(probe)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != "custom" {
		t.Fatalf("install must not overwrite existing files, got %q", again)
	}
}

func TestSharedResolverImportMap(t *testing.T) {
	m := shared.NewResolver().SharedURLs()
	for _, a := range shared.Artifacts {
		if m[a.Key] != shared.PublicURL(a) {
			t.Fatalf("import map[%s] = %q, want %q", a.Key, m[a.Key], shared.PublicURL(a))
		}
	}
	if len(m) != len(shared.Artifacts) {
		t.Fatalf("import map has %d keys, want %d", len(m), len(shared.Artifacts))
	}
}
