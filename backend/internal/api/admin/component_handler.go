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

func (h *ComponentHandler) List(w http.ResponseWriter, r *http.Request) {
	result, err := h.components.List(r.Context(), siteID(r))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
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

func (h *ComponentHandler) Versions(w http.ResponseWriter, r *http.Request) {
	result, err := h.components.Versions(r.Context(), siteID(r), r.PathValue("componentID"))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *ComponentHandler) CheckoutVersion(w http.ResponseWriter, r *http.Request) {
	files, err := h.components.CheckoutVersion(r.Context(), siteID(r), r.PathValue("sha"))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, files)
}

func (h *ComponentHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SHA string `json:"sha"`
	}
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	if err := h.components.Rollback(r.Context(), siteID(r), r.PathValue("componentID"), req.SHA); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
