package admin

import (
	"net/http"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
	"github.com/liapoldus/liapoldus/backend/internal/application/deps"
)

type DepsHandler struct {
	deps *deps.Service
}

func NewDepsHandler(deps *deps.Service) *DepsHandler {
	return &DepsHandler{deps: deps}
}

type createDependencyRequest struct {
	Name string `json:"name"`
	Spec string `json:"spec"`
}

type dependencyResponse struct {
	Name            string `json:"name"`
	Spec            string `json:"spec"`
	ResolvedVersion string `json:"resolvedVersion,omitempty"`
}

// Create declares a top-level dependency for a site. The declaration is
// validated and uploaded immediately; the range is resolved to an exact
// version on snapshot creation (spec §8, решение 3). The registry is probed
// best-effort only, so resolvedVersion is informational.
func (h *DepsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createDependencyRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	resolved, err := h.deps.Add(r.Context(), siteID(r), req.Name, req.Spec)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusCreated, dependencyResponse{
		Name:            req.Name,
		Spec:            req.Spec,
		ResolvedVersion: resolved.Version,
	})
}

func (h *DepsHandler) List(w http.ResponseWriter, r *http.Request) {
	deps, err := h.deps.List(r.Context(), siteID(r))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	result := make([]dependencyResponse, 0, len(deps))
	for _, dep := range deps {
		result = append(result, dependencyResponse{Name: dep.Name, Spec: dep.Spec})
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *DepsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := h.deps.Remove(r.Context(), siteID(r), name); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
