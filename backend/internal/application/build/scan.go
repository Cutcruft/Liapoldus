package build

import (
	"regexp"
	"strings"
)

// importSpecifierRe matches the module specifier of a static or dynamic
// import statement ("from 'x'", "from \"x\"", "import 'x'", "import('x')").
// The leading [^.\w] guard keeps member call sites like obj.from("x") out.
// It is a heuristic shared by the site-source scanner (materializer) and the
// dependency-source scanner (layout): comment false-positives are accepted,
// they classify against declared/shared sets and are harmless.
var importSpecifierRe = regexp.MustCompile(`(?:^|[^.\w])(?:from|import)(?:\s*\(|\s+)\s*["']([^"']+)["']`)

// ScanImportSpecifiers returns every module specifier found in source —
// bare, relative, absolute or URL. Callers classify the results: relative
// paths and node built-ins are not bare packages, top-level specifiers are
// handled by the top-level bundle loop, and only bare subpath specifiers
// (SplitBareSpecifier returns isSub) enter the subpath pipeline.
func ScanImportSpecifiers(source string) []string {
	matches := importSpecifierRe.FindAllStringSubmatch(source, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

// SplitBareSpecifier splits a bare import specifier into its package name and
// the remaining subpath: "x" → ("x", "", false), "x/y/z" → ("x", "y/z", true),
// "@scope/x/y" → ("@scope/x", "y", true). A specifier that names a plain
// package (isSub == false) is a top-level import, not a subpath.
func SplitBareSpecifier(spec string) (top, rest string, isSub bool) {
	if strings.HasPrefix(spec, "@") {
		parts := strings.Split(spec, "/")
		if len(parts) < 3 {
			return spec, "", false
		}
		return parts[0] + "/" + parts[1], strings.Join(parts[2:], "/"), true
	}
	idx := strings.Index(spec, "/")
	if idx < 0 {
		return spec, "", false
	}
	return spec[:idx], spec[idx+1:], true
}
