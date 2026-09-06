package unit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/infra/build/shell"
)

func TestShellRenderImportMap(t *testing.T) {
	m := build.Manifest{
		Shared: map[string]string{
			"react":                 "/build/_shared/react/18.3.1.js",
			"@liapoldus/ui-runtime": "/build/_shared/@liapoldus/ui-runtime/0.1.0.js",
		},
		Deps: map[string]build.DepRef{
			"lodash":           {Name: "lodash", Version: "4.17.21", PublicArtifact: "dist/_deps/lodash@4.17.21.js"},
			"lodash/map":       {Name: "lodash", Version: "4.17.21", PublicArtifact: "dist/_deps/lodash@4.17.21/map.js"},
			"agent/styles.css": {Name: "agent", Version: "1.0.0", PublicArtifact: "dist/_deps/agent@1.0.0/styles.js", CSSArtifact: "dist/_deps/agent@1.0.0/styles.css"},
		},
		Styles: []string{"dist/_deps/agent@1.0.0/styles.css", "dist/_deps/agent@1.0.0.css"},
	}

	html, err := shell.Render(m)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	for _, want := range []string{
		`<script type="importmap">`,
		`"react":"/build/_shared/react/18.3.1.js"`,
		`"@liapoldus/ui-runtime":"/build/_shared/@liapoldus/ui-runtime/0.1.0.js"`,
		`"lodash":"./_deps/lodash@4.17.21.js"`,
		`"lodash/map":"./_deps/lodash@4.17.21/map.js"`,
		`"agent/styles.css":"./_deps/agent@1.0.0/styles.js"`,
		`<link rel="stylesheet" href="./_deps/agent@1.0.0/styles.css">`,
		`<link rel="stylesheet" href="./_deps/agent@1.0.0.css">`,
		`<div id="root"></div>`,
		`<script type="module" src="./entry.js"></script>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("shell missing %q; html:\n%s", want, html)
		}
	}
}

func TestShellRenderEmpty(t *testing.T) {
	html, err := shell.Render(build.Manifest{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(html, `<script type="importmap">`+"\n{\"imports\":{}}\n") {
		t.Errorf("empty import map expected, got:\n%s", html)
	}
	if !strings.Contains(html, `src="./entry.js"`) {
		t.Errorf("entry script missing")
	}
	if strings.Contains(html, "modulepreload") {
		t.Errorf("no home page -> no modulepreload expected:\n%s", html)
	}
}

// TestShellRenderHomePagePreload covers the first-screen contract: the home
// page's code-split chunk is modulepreloaded by the shell.
func TestShellRenderHomePagePreload(t *testing.T) {
	m := build.Manifest{
		HomePage: "page_a",
		Pages: []build.PageRef{
			{PageID: "page_a", Chunk: "pages/page_a.js"},
			{PageID: "page_b", Chunk: "pages/page_b.js"},
		},
	}
	html, err := shell.Render(m)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(html, `<link rel="modulepreload" href="./pages/page_a.js">`) {
		t.Errorf("home chunk must be modulepreloaded:\n%s", html)
	}
	if strings.Contains(html, "page_b.js") {
		t.Errorf("non-home chunks must not be preloaded:\n%s", html)
	}
}

func TestShellWrite(t *testing.T) {
	dir := t.TempDir()
	dist := filepath.Join(dir, "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	m := build.Manifest{Shared: map[string]string{"react": "/build/_shared/react/18.3.1.js"}}
	if err := shell.WriteShell(dist, m); err != nil {
		t.Fatalf("WriteShell: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dist, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if !strings.Contains(string(data), `"react":"/build/_shared/react/18.3.1.js"`) {
		t.Error("shared mapping missing from written shell")
	}
}
