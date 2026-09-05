// Package schema wraps JSON Schema compilation and property validation.
// It is used to validate component definitions (§1 of components-test-spec) and
// instance props during page assembly.
package schema

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

var (
	missingPropRe = regexp.MustCompile(`missing property '([^']+)'`)
	keywordLastRe = regexp.MustCompile(`[^/]+$`)
	compileCache  sync.Map // key: fmt("%v", schema) -> compiledSchema
)

type compiledSchema struct {
	sch *jsonschema.Schema
	err error
}

// ValidateSchema checks that the given schema is a well-formed JSON Schema
// object declaring a root type. nil is valid ("no schema").
func ValidateSchema(schema any) error {
	if schema == nil {
		return nil
	}
	m, ok := schema.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: schema must be a JSON object", domain.ErrSchemaInvalid)
	}
	if _, ok := m["type"]; !ok {
		return fmt.Errorf("%w: schema must declare a root type", domain.ErrSchemaInvalid)
	}
	c := compileCached(m)
	return c.err
}

// ValidateProps validates instance props against a JSON Schema and returns a
// human-readable error list keyed by JSON pointer (e.g. "$.title"). A nil
// schema skips validation entirely.
func ValidateProps(props any, schema any) []error {
	if schema == nil {
		return nil
	}
	m, ok := schema.(map[string]any)
	if !ok {
		return []error{fmt.Errorf("%w: schema must be a JSON object", domain.ErrSchemaInvalid)}
	}
	c := compileCached(m)
	if c.err != nil {
		return []error{c.err}
	}
	return collectErrors(c.sch.Validate(props))
}

func compileCached(m map[string]any) compiledSchema {
	key := fmt.Sprintf("%v", m)
	if v, ok := compileCache.Load(key); ok {
		return v.(compiledSchema)
	}
	sch, err := compile(m)
	c := compiledSchema{sch: sch, err: err}
	compileCache.Store(key, c)
	return c
}

// compile compiles m with jsonschema v6. AddResource takes a decoded JSON
// value; Compile resolves it against its registered URL.
func compile(m map[string]any) (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	uri := "http://schema.local/root.json"
	if err := c.AddResource(uri, m); err != nil {
		return nil, fmt.Errorf("%w: invalid schema: %v", domain.ErrSchemaInvalid, err)
	}
	sch, err := c.Compile(uri)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid schema: %v", domain.ErrSchemaInvalid, err)
	}
	return sch, nil
}

// collectErrors turns the jsonschema.ValidationError tree into a flat list of
// "$path: message" strings. For required-keyword failures the missing property
// is appended to the instance location so callers see "$.title", not "$".
func collectErrors(err error) []error {
	if err == nil {
		return nil
	}
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return []error{fmt.Errorf("schema validation failed: %v", err)}
	}
	var out []error
	walkUnits(ve.DetailedOutput(), &out)
	return out
}

func walkUnits(u *jsonschema.OutputUnit, out *[]error) {
	for i := range u.Errors {
		e := &u.Errors[i]
		if e.Error != nil {
			*out = append(*out, fmt.Errorf("%s: %s", instancePath(e), e.Error.String()))
		}
		walkUnits(e, out)
	}
}

// instancePath converts an OutputUnit into a "$-prefixed" dotted path
// (e.g. "$.meta.published"), resolving missing-property (required) errors to
// the concrete path of the absent property.
func instancePath(e *jsonschema.OutputUnit) string {
	base := "$"
	if loc := e.InstanceLocation; loc != "" {
		segments := strings.Split(strings.TrimPrefix(loc, "/"), "/")
		base = "$." + strings.Join(segments, ".")
	}
	if stringsHasRequiredKeyword(e.KeywordLocation) {
		if m := missingPropRe.FindStringSubmatch(e.Error.String()); len(m) == 2 {
			base = base + "." + m[1]
		}
	}
	return base
}

func stringsHasRequiredKeyword(kw string) bool {
	return keywordLastRe.FindString(kw) == "required"
}
