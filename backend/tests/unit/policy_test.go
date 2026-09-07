package unit

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/liapoldus/liapoldus/backend/internal/application/component"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

func nowUTC() time.Time { return time.Now().UTC() }

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestSiteComponentImports(t *testing.T) {
	src := `
import React from "react";
import Card from "@site/components/card";
import Header from "@site/components/header";
import { Box } from "./box";
const Footer = React.lazy(() => import("@site/components/footer"));
`
	imports := component.SiteComponentImports(src)
	want := []string{"card", "header", "footer"}
	if !reflect.DeepEqual(imports, want) {
		t.Fatalf("imports = %v, want %v", imports, want)
	}
}

func TestHasDefaultExport(t *testing.T) {
	cases := []struct {
		source string
		want   bool
	}{
		{"export default function Main() {}", true},
		{"export default () => null", true},
		{"export default class C {}", true},
		{"export function foo() {}", false},
		{"export defaultProps = 1", false},
	}
	for _, c := range cases {
		if got := component.HasDefaultExport(c.source); got != c.want {
			t.Errorf("HasDefaultExport(%q) = %v, want %v", c.source, got, c.want)
		}
	}
}

func TestNormalizeAllowlist(t *testing.T) {
	if got := component.NormalizeAllowlist(nil); got != nil {
		t.Fatalf("nil input should produce nil, got %v", got)
	}
	got := component.NormalizeAllowlist([]string{"b", "", "a", "b", " c "})
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeAllowlist = %v, want %v", got, want)
	}
}

func primDef(id string, mutators ...func(*domain.ComponentDefinition)) domain.ComponentDefinition {
	d := domain.ComponentDefinition{
		ID:        id,
		SiteID:    "site_1",
		Name:      id,
		Kind:      "component",
		Source:    "export default () => null;",
		Schema:    mustJSONMap(`{"type":"object","properties":{}}`),
		Metadata:  map[string]any{},
		CreatedAt: nowUTC(),
	}
	for _, m := range mutators {
		m(&d)
	}
	return d
}

func sectionDef(id string, mutators ...func(*domain.ComponentDefinition)) domain.ComponentDefinition {
	d := primDef(id)
	d.IsSection = true
	for _, m := range mutators {
		m(&d)
	}
	return d
}

func TestValidateGraph(t *testing.T) {
	prim := func(id string) domain.ComponentDefinition { return primDef(id) }
	sect := func(id string, muts ...func(*domain.ComponentDefinition)) domain.ComponentDefinition {
		return sectionDef(id, muts...)
	}
	importing := func(id string) func(*domain.ComponentDefinition) {
		return func(d *domain.ComponentDefinition) {
			d.Source = `import X from "@site/components/` + id + `"; export default () => null;`
		}
	}

	cases := []struct {
		name    string
		defs    []domain.ComponentDefinition
		wantErr string
	}{
		{
			name: "empty set is valid",
			defs: nil,
		},
		{
			name: "valid primitives",
			defs: []domain.ComponentDefinition{prim("text"), prim("button")},
		},
		{
			name: "valid section importing allowlisted primitive",
			defs: []domain.ComponentDefinition{
				prim("text"),
				sect("main", importing("text"), func(d *domain.ComponentDefinition) {
					d.AllowedPrimitiveIDs = []string{"text"}
				}),
			},
		},
		{
			name: "missing default export",
			defs: []domain.ComponentDefinition{primDef("x", func(d *domain.ComponentDefinition) {
				d.Source = "function A() { return null }"
			})},
			wantErr: "must declare a default export",
		},
		{
			name: "acceptsPageContent on a primitive",
			defs: []domain.ComponentDefinition{primDef("x", func(d *domain.ComponentDefinition) {
				d.AcceptsPageContent = true
			})},
			wantErr: "acceptsPageContent requires isSection",
		},
		{
			name: "primitive importing a site component",
			defs: []domain.ComponentDefinition{
				prim("text"),
				primDef("x", importing("text")),
			},
			wantErr: `primitive "x" may not import site components`,
		},
		{
			name: "section importing unknown component",
			defs: []domain.ComponentDefinition{
				sect("main", importing("ghost")),
			},
			wantErr: `section "main" imports "ghost" which is not a registered component`,
		},
		{
			name: "section importing a section",
			defs: []domain.ComponentDefinition{
				sect("a"),
				sect("b", importing("a")),
			},
			wantErr: "may not import section",
		},
		{
			name: "section importing non-allowlisted primitive",
			defs: []domain.ComponentDefinition{
				prim("text"),
				sect("main", importing("text")),
			},
			wantErr: `section "main" imports "text" which is not in its allowlist`,
		},
		{
			name: "allowlist references unknown component",
			defs: []domain.ComponentDefinition{
				sect("main", func(d *domain.ComponentDefinition) {
					d.AllowedPrimitiveIDs = []string{"ghost"}
				}),
			},
			wantErr: `section "main" allowlist references unknown component "ghost"`,
		},
		{
			name: "allowlist references a section",
			defs: []domain.ComponentDefinition{
				sect("a"),
				sect("main", func(d *domain.ComponentDefinition) {
					d.AllowedPrimitiveIDs = []string{"a"}
				}),
			},
			wantErr: "only primitives may be allowed",
		},
		{
			name: "defensive cycle guard",
			defs: []domain.ComponentDefinition{
				sect("a", importing("b")),
				sect("b", importing("a")),
			},
			wantErr: "dependency cycle",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := component.ValidateGraph(c.defs)
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.wantErr)
			}
			if !errors.Is(err, domain.ErrInvalidRequest) {
				t.Fatalf("error %v is not wrapped as ErrInvalidRequest", err)
			}
			if !contains(err.Error(), c.wantErr) {
				t.Fatalf("error %q does not contain %q", err, c.wantErr)
			}
		})
	}
}

func TestDefineGraphBlocked(t *testing.T) {
	svc, _ := newComponentService(t)
	ctx := context.Background()

	// Defining a section that imports an unknown primitive is a hard block.
	_, err := svc.Define(ctx, sectionDef("main", func(d *domain.ComponentDefinition) {
		d.Source = `import T from "@site/components/text"; export default () => null;`
	}))
	if !errors.Is(err, domain.ErrInvalidRequest) {
		t.Fatalf("expected graph block, got %v", err)
	}
}

func TestDefineOrderSensitiveGraph(t *testing.T) {
	svc, _ := newComponentService(t)
	ctx := context.Background()

	// Register the primitive first, then the section that imports it.
	if _, err := svc.Define(ctx, primDef("text")); err != nil {
		t.Fatalf("define text: %v", err)
	}
	if _, err := svc.Define(ctx, sectionDef("main", func(d *domain.ComponentDefinition) {
		d.Source = `import T from "@site/components/text"; export default () => null;`
		d.AllowedPrimitiveIDs = []string{"text"}
	})); err != nil {
		t.Fatalf("define main after text: %v", err)
	}
}

func TestUpdateRevalidatesWholeGraph(t *testing.T) {
	svc, fd := newComponentService(t)
	ctx := context.Background()

	// A section allowlists primitive "text".
	text := primDef("text")
	if _, err := svc.Define(ctx, text); err != nil {
		t.Fatalf("define text: %v", err)
	}
	if _, err := svc.Define(ctx, sectionDef("main", func(d *domain.ComponentDefinition) {
		d.Source = `import T from "@site/components/text"; export default () => null;`
		d.AllowedPrimitiveIDs = []string{"text"}
	})); err != nil {
		t.Fatalf("define main: %v", err)
	}

	// Turning "text" into a section must fail: the section's allowlist would
	// then reference a section.
	td := text
	td.IsSection = true
	if _, err := svc.Update(ctx, td); err == nil {
		t.Fatal("update of text into a section should be blocked by the graph")
	}

	// The previous failed update must leave the stored definition intact.
	kept, err := fd.Get(ctx, text.SiteID, "text")
	if err != nil {
		t.Fatalf("get text: %v", err)
	}
	if kept.IsSection {
		t.Fatal("stored text definition should still be a primitive after blocked update")
	}
}

func TestNormalizeAllowlistOnSave(t *testing.T) {
	svc, fd := newComponentService(t)
	ctx := context.Background()
	for _, id := range []string{"a", "b"} {
		if _, err := svc.Define(ctx, primDef(id)); err != nil {
			t.Fatalf("define %s: %v", id, err)
		}
	}
	if _, err := svc.Define(ctx, sectionDef("main", func(d *domain.ComponentDefinition) {
		d.AllowedPrimitiveIDs = []string{"b", "a", "b"}
	})); err != nil {
		t.Fatalf("define: %v", err)
	}
	got, err := fd.Get(ctx, "site_1", "main")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !reflect.DeepEqual(got.AllowedPrimitiveIDs, []string{"a", "b"}) {
		t.Fatalf("allowlist not normalized on save: %v", got.AllowedPrimitiveIDs)
	}
}
