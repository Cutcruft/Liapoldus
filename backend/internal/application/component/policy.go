package component

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// siteComponentsPrefix is the bare import scope site component sources use to
// reference each other: `import X from "@site/components/<id>"`. It mirrors the
// platform alias a page chunk resolves at build time.
const siteComponentsPrefix = "@site/components/"

// defaultExportRe matches the ES default-export shorthand every component file
// must declare: `export default …`. The `export { X as default }` long form is
// intentionally not accepted — the registry convention is the shorthand form.
var defaultExportRe = regexp.MustCompile(`(?m)\bexport\s+default\b`)

// SiteComponentImports returns every definition id referenced through the
// @site/components/<id> bare scope in source. Static (`import X from …`) and
// dynamic (`import("…")`) forms are both recognized (shared scanner).
func SiteComponentImports(source string) []string {
	var out []string
	for _, spec := range build.ScanImportSpecifiers(source) {
		if !strings.HasPrefix(spec, siteComponentsPrefix) {
			continue
		}
		if id := strings.TrimPrefix(spec, siteComponentsPrefix); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// HasDefaultExport reports whether source declares `export default`.
func HasDefaultExport(source string) bool {
	return defaultExportRe.MatchString(source)
}

// NormalizeAllowlist deduplicates and sorts a section's allowed primitive ids
// so persisted allowlists are deterministic and comparison-friendly.
func NormalizeAllowlist(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	slices.Sort(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

// ValidateGraph enforces the component hierarchy invariants (R10 decisions):
//
//   - every file declares `export default`
//   - acceptsPageContent requires isSection (only sections wrap page content)
//   - primitives import no site components (`@site/components/*` is a ban)
//   - sections import only existing primitives that their allowlist admits;
//     an empty allowlist is deny-by-default
//   - allowlist entries reference existing primitives only
//   - the import graph is acyclic (structurally impossible under the rules
//     above — sections reach only primitives — kept as a defensive guard)
//
// All checks run over the whole set, so a single inconsistent definition
// blocks the save that would introduce it.
func ValidateGraph(defs []domain.ComponentDefinition) error {
	byID := make(map[string]domain.ComponentDefinition, len(defs))
	for _, d := range defs {
		if _, dup := byID[d.ID]; dup {
			return policyErrorf("duplicate component id %q in validation set", d.ID)
		}
		byID[d.ID] = d
	}

	// Pass 1: per-definition shape.
	for _, d := range defs {
		if !HasDefaultExport(d.Source) {
			return policyErrorf("component %q must declare a default export", d.ID)
		}
		if d.AcceptsPageContent && !d.IsSection {
			return policyErrorf("component %q: acceptsPageContent requires isSection", d.ID)
		}
	}

	// Pass 2: resolve edges and reference integrity.
	type sectionGraph struct {
		id    string
		edges []string
	}
	var sections []sectionGraph
	for _, d := range defs {
		imports := SiteComponentImports(d.Source)
		if !d.IsSection {
			if len(imports) > 0 {
				return policyErrorf("primitive %q may not import site components (found %q)", d.ID, imports[0])
			}
			continue
		}
		for _, id := range imports {
			if _, ok := byID[id]; !ok {
				return policyErrorf("section %q imports %q which is not a registered component", d.ID, id)
			}
		}
		sections = append(sections, sectionGraph{id: d.ID, edges: imports})
		for _, allowed := range NormalizeAllowlist(d.AllowedPrimitiveIDs) {
			target, ok := byID[allowed]
			if !ok {
				return policyErrorf("section %q allowlist references unknown component %q", d.ID, allowed)
			}
			if target.IsSection {
				return policyErrorf("section %q allowlist references section %q; only primitives may be allowed", d.ID, allowed)
			}
		}
	}

	// Pass 3: defensive cycle detection over section imports.
	index := make(map[string]int, len(sections))
	for i, sg := range sections {
		index[sg.id] = i
	}
	state := make([]int, len(sections)) // 0 = unvisited, 1 = visiting, 2 = done
	var visit func(int) error
	visit = func(i int) error {
		switch state[i] {
		case 1:
			return policyErrorf("component dependency cycle involving %q", sections[i].id)
		case 2:
			return nil
		}
		state[i] = 1
		for _, edge := range sections[i].edges {
			j, ok := index[edge]
			if !ok {
				continue
			}
			if err := visit(j); err != nil {
				return err
			}
		}
		state[i] = 2
		return nil
	}
	for i := range sections {
		if err := visit(i); err != nil {
			return err
		}
	}

	// Pass 4: edge typing and allowlist enforcement for sections.
	for _, d := range defs {
		if !d.IsSection {
			continue
		}
		allowed := make(map[string]bool, len(d.AllowedPrimitiveIDs))
		for _, id := range d.AllowedPrimitiveIDs {
			allowed[id] = true
		}
		for _, id := range SiteComponentImports(d.Source) {
			target := byID[id]
			if target.IsSection {
				return policyErrorf("section %q may not import section %q", d.ID, id)
			}
			if !allowed[id] {
				return policyErrorf("section %q imports %q which is not in its allowlist", d.ID, id)
			}
		}
	}
	return nil
}

func policyErrorf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", domain.ErrInvalidRequest, fmt.Sprintf(format, args...))
}
