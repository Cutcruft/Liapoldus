package admin

import (
	"net/http"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
	"github.com/liapoldus/liapoldus/backend/internal/application/component"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

type ComponentHandler struct {
	components *component.Service
}

func NewComponentHandler(components *component.Service) *ComponentHandler {
	return &ComponentHandler{components: components}
}

// componentRequest mirrors ComponentDefinition but keeps Source explicit (the
// domain field is json:"-" because the source lives in the git repo, R5).
type componentRequest struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Kind     string         `json:"kind"`
	Source   string         `json:"source"`
	Schema   map[string]any `json:"schema"`
	Metadata map[string]any `json:"metadata"`
}

func (req componentRequest) toDomain(siteID string) domain.ComponentDefinition {
	def := domain.ComponentDefinition{
		SiteID:   siteID,
		ID:       req.ID,
		Name:     req.Name,
		Kind:     req.Kind,
		Source:   req.Source,
		Schema:   req.Schema,
		Metadata: req.Metadata,
	}
	if def.Kind == "" {
		def.Kind = "component"
	}
	return def
}

func (h *ComponentHandler) Define(w http.ResponseWriter, r *http.Request) {
	var req componentRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.components.Define(r.Context(), req.toDomain(siteID(r)))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusCreated, result)
}

// editorComponent is one entry of the editor catalog returned by
// GET /api/sites/{id}/components: builtin platform components plus the site's
// defined components, each shaped for the editor palette/inspector.
type editorComponent struct {
	Type      string         `json:"type"`
	Label     string         `json:"label"`
	Container bool           `json:"container"`
	Schema    map[string]any `json:"schema"`
}

// List returns the unified editor catalog for a site: platform builtin
// components first, then the site's component definitions mapped to the same
// shape. The admin editor loads this once when it mounts (schemas.ts).
func (h *ComponentHandler) List(w http.ResponseWriter, r *http.Request) {
	defs, err := h.components.List(r.Context(), siteID(r))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	builtins := component.Builtins()
	catalog := make([]editorComponent, 0, len(builtins)+len(defs))
	for _, b := range builtins {
		catalog = append(catalog, editorComponent{Type: b.Type, Label: b.Label, Container: b.Container, Schema: b.Schema})
	}
	for _, d := range defs {
		label := d.Name
		if l, ok := stringMeta(d.Metadata, "label"); ok && l != "" {
			label = l
		}
		catalog = append(catalog, editorComponent{Type: d.ID, Label: label, Container: false, Schema: d.Schema})
	}
	httpapi.RespondJSON(w, http.StatusOK, catalog)
}

func stringMeta(m map[string]any, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[key].(string)
	return v, ok
}

func (h *ComponentHandler) Get(w http.ResponseWriter, r *http.Request) {
	result, err := h.components.Get(r.Context(), siteID(r), r.PathValue("componentID"))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *ComponentHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req componentRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	def := req.toDomain(siteID(r))
	def.ID = r.PathValue("componentID")
	result, err := h.components.Update(r.Context(), def)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *ComponentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.components.Delete(r.Context(), siteID(r), r.PathValue("componentID")); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
