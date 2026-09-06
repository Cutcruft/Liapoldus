package shell

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/liapoldus/liapoldus/backend/internal/application/build"
)

// relToPage converts an artifact-relative path ("dist/_deps/x.js") to the path
// the browser resolves relative to dist/index.html ("./_deps/x.js").
func relToPage(path string) string {
	return "./" + strings.TrimPrefix(path, "dist/")
}

// Render builds the runtime shell HTML (spec §12): an import-map wiring every
// shared library and dep bundle to its public URL, <link> tags for the dep
// stylesheets, and the module entry script for the site bundle. The page
// element mount(siteId, env) renders into appears as #root.
func Render(m build.Manifest) (string, error) {
	imports := make(map[string]string, len(m.Shared)+len(m.Deps))
	for spec, url := range m.Shared {
		imports[spec] = url
	}
	for spec, ref := range m.Deps {
		imports[spec] = relToPage(ref.PublicArtifact)
	}
	importMap, err := json.Marshal(map[string]any{"imports": imports})
	if err != nil {
		return "", fmt.Errorf("shell: marshal import map: %w", err)
	}
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"ru\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1">` + "\n")
	b.WriteString("<title>Liapoldus</title>\n")
	for _, css := range m.Styles {
		fmt.Fprintf(&b, "<link rel=\"stylesheet\" href=\"%s\">\n", relToPage(css))
	}
	fmt.Fprintf(&b, "<script type=\"importmap\">\n%s\n</script>\n", importMap)
	b.WriteString("</head>\n<body>\n<div id=\"root\"></div>\n")
	b.WriteString(`<script type="module" src="./entry.js"></script>` + "\n</body>\n</html>\n")
	return b.String(), nil
}

// WriteShell writes dist/index.html into an esbuild dist directory so the
// published artifact is directly openable in a browser.
func WriteShell(distDir string, m build.Manifest) error {
	html, err := Render(m)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(distDir, "index.html"), []byte(html), 0o644); err != nil {
		return fmt.Errorf("write dist/index.html: %w", err)
	}
	return nil
}
