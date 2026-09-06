package component

import (
	"encoding/json"
	"errors"
	"slices"
)

// BuiltinComponent is an editor-usable component shipped with the platform.
// The editor palette and inspector form generator consume these via
// GET /api/sites/{id}/components (unified catalog: builtins + site definitions).
type BuiltinComponent struct {
	Type      string         `json:"type"`
	Label     string         `json:"label"`
	Container bool           `json:"container"`
	Schema    map[string]any `json:"schema"`
}

// builtinSchema parses a JSON literal into a schema map without allocation
// churn at first unmarshal; the result is frozen per the raw constant.
func builtinSchema(raw string) map[string]any {
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		panic("bad builtin schema: " + err.Error())
	}
	return m
}

// builtinComponents is the platform-level editor palette. These mirror the
// schemas the frontend used to hardcode; the backend is now the single source
// of truth and the editor loads them at runtime.
var builtinComponents = [...]BuiltinComponent{
	{
		Type:      "Container",
		Label:     "Контейнер",
		Container: true,
		Schema: builtinSchema(`{
			"type": "object",
			"title": "Контейнер",
			"default": {"layout": "stack", "gap": 8},
			"properties": {
				"layout": {"type": "string", "title": "Раскладка", "enum": ["stack", "row", "grid"], "default": "stack"},
				"gap": {"type": "number", "title": "Отступ", "default": 8, "min": 0, "max": 64}
			}
		}`),
	},
	{
		Type:      "Text",
		Label:     "Текст",
		Container: false,
		Schema: builtinSchema(`{
			"type": "object",
			"title": "Текст",
			"default": {"text": "Текст", "size": "md", "align": "left"},
			"required": ["text"],
			"properties": {
				"text": {"type": "string", "title": "Текст", "default": "Текст", "required": true, "format": "richtext"},
				"size": {"type": "string", "title": "Размер", "enum": ["sm", "md", "lg", "xl"], "default": "md"},
				"align": {"type": "string", "title": "Выравнивание", "enum": ["left", "center", "right"], "default": "left"},
				"color": {"type": "string", "title": "Цвет", "default": "#111827"}
			}
		}`),
	},
	{
		Type:      "Image",
		Label:     "Картинка",
		Container: false,
		Schema: builtinSchema(`{
			"type": "object",
			"title": "Картинка",
			"default": {"alt": "", "width": 320},
			"properties": {
				"assetId": {"type": "string", "title": "Ассет", "format": "asset", "default": ""},
				"alt": {"type": "string", "title": "Alt", "default": ""},
				"width": {"type": "number", "title": "Ширина", "default": 320, "min": 1, "max": 1920}
			}
		}`),
	},
	{
		Type:      "Button",
		Label:     "Кнопка",
		Container: false,
		Schema: builtinSchema(`{
			"type": "object",
			"title": "Кнопка",
			"default": {"label": "Кнопка", "variant": "primary"},
			"required": ["label"],
			"properties": {
				"label": {"type": "string", "title": "Надпись", "default": "Кнопка", "required": true},
				"variant": {"type": "string", "title": "Вариант", "enum": ["primary", "secondary", "ghost"], "default": "primary"}
			}
		}`),
	},
}

// ErrUnknownBuiltin guards catalog lookups; callers should treat it as absent.
var ErrUnknownBuiltin = errors.New("unknown builtin component")

// Builtin returns the builtin component for a type, if it exists.
func Builtin(typeName string) (BuiltinComponent, bool) {
	for _, b := range builtinComponents {
		if b.Type == typeName {
			return b, true
		}
	}
	return BuiltinComponent{}, false
}

// Builtins returns an independent copy of the platform builtin palette.
// The schema maps are shallow copies — callers must not mutate them.
func Builtins() []BuiltinComponent {
	return slices.Clone(builtinComponents[:])
}
