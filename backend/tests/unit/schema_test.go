package unit

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
	"github.com/liapoldus/liapoldus/backend/internal/schema"
)

func decodeSchema(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	return m
}

// cardSchema is the shared fixture for a component whose required prop is a
// string title and whose optional meta.published is a boolean.
func cardSchema(t *testing.T) map[string]any {
	return decodeSchema(t, `{
		"type": "object",
		"required": ["title"],
		"properties": {
			"title": {"type": "string"},
			"meta": {
				"type": "object",
				"properties": {"published": {"type": "boolean"}}
			}
		}
	}`)
}

func TestValidateSchemaValid(t *testing.T) {
	if err := schema.ValidateSchema(cardSchema(t)); err != nil {
		t.Fatalf("valid schema rejected: %v", err)
	}
}

func TestValidateSchemaNil(t *testing.T) {
	if err := schema.ValidateSchema(nil); err != nil {
		t.Fatalf("nil schema should be valid, got %v", err)
	}
}

func TestValidateSchemaMissingRootType(t *testing.T) {
	s := decodeSchema(t, `{"properties": {"title": {"type": "string"}}}`)
	if err := schema.ValidateSchema(s); !errors.Is(err, domain.ErrSchemaInvalid) {
		t.Fatalf("missing root type: want ErrSchemaInvalid, got %v", err)
	}
}

func TestValidateSchemaNonObject(t *testing.T) {
	cases := []any{
		[]any{map[string]any{"type": "object"}},
		"type: object",
		42,
	}
	for _, c := range cases {
		if err := schema.ValidateSchema(c); !errors.Is(err, domain.ErrSchemaInvalid) {
			t.Fatalf("non-object schema %v: want ErrSchemaInvalid, got %v", c, err)
		}
	}
}

func TestValidateSchemaUnknownKeyword(t *testing.T) {
	s := decodeSchema(t, `{"type": "object", "properties": {"title": {"type": "unknown"}}}`)
	if err := schema.ValidateSchema(s); !errors.Is(err, domain.ErrSchemaInvalid) {
		t.Fatalf("unknown type keyword: want ErrSchemaInvalid, got %v", err)
	}
}

func TestValidatePropsValid(t *testing.T) {
	props := map[string]any{"title": "Карточка", "meta": map[string]any{"published": true}}
	if errs := schema.ValidateProps(props, cardSchema(t)); len(errs) != 0 {
		t.Fatalf("valid props rejected: %v", errs)
	}
}

func TestValidatePropsMissingRequired(t *testing.T) {
	errs := schema.ValidateProps(map[string]any{}, cardSchema(t))
	if len(errs) == 0 {
		t.Fatal("missing required title: want error")
	}
	if !strings.Contains(errs[0].Error(), "$.title") {
		t.Fatalf("expected path $.title, got %v", errs[0])
	}
}

func TestValidatePropsWrongType(t *testing.T) {
	errs := schema.ValidateProps(map[string]any{"title": 42}, cardSchema(t))
	if len(errs) == 0 {
		t.Fatal("wrong title type: want error")
	}
	if !strings.Contains(errs[0].Error(), "$.title") {
		t.Fatalf("expected path $.title, got %v", errs[0])
	}
}

func TestValidatePropsNested(t *testing.T) {
	props := map[string]any{"title": "x", "meta": map[string]any{"published": "no"}}
	errs := schema.ValidateProps(props, cardSchema(t))
	if len(errs) == 0 {
		t.Fatal("wrong nested type: want error")
	}
	if !strings.Contains(errs[0].Error(), "$.meta.published") {
		t.Fatalf("expected path $.meta.published, got %v", errs[0])
	}
}

func TestValidatePropsNilSchema(t *testing.T) {
	if errs := schema.ValidateProps(map[string]any{"anything": true}, nil); len(errs) != 0 {
		t.Fatalf("nil schema must skip validation, got %v", errs)
	}
}

func TestValidatePropsNullVsAbsent(t *testing.T) {
	s := cardSchema(t)
	// Null value does not satisfy type: string.
	if errs := schema.ValidateProps(map[string]any{"title": nil}, s); len(errs) == 0 {
		t.Fatal("null title should fail against type string")
	}
	// Absent optional property is fine (only title is required).
	optional := decodeSchema(t, `{"type": "object", "properties": {"note": {"type": "string"}}}`)
	if errs := schema.ValidateProps(map[string]any{}, optional); len(errs) != 0 {
		t.Fatalf("absent optional property rejected: %v", errs)
	}
}

func TestValidatePropsInvalidSchemaValue(t *testing.T) {
	errs := schema.ValidateProps(map[string]any{"a": 1}, "not a schema")
	if len(errs) == 0 {
		t.Fatal("non-object schema: want error")
	}
	if !errors.Is(errs[0], domain.ErrSchemaInvalid) {
		t.Fatalf("want ErrSchemaInvalid, got %v", errs[0])
	}
}
