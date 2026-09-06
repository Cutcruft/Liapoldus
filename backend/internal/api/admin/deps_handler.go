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

// ListAllowlist returns the site's allowlist entries (allowlist policy, spec
// §5). An empty array means the site allows every package.
func (h *DepsHandler) ListAllowlist(w http.ResponseWriter, r *http.Request) {
	entries, err := h.deps.ListAllowlist(r.Context(), siteID(r))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, entries)
}

type allowlistRequest struct {
	Entry string `json:"entry"`
}

// AddAllowlist persists one allowlist entry. Entries are validated and
// normalized (trimmed, lowercased) by the service.
func (h *DepsHandler) AddAllowlist(w http.ResponseWriter, r *http.Request) {
	var req allowlistRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	if err := h.deps.AddAllowlist(r.Context(), siteID(r), req.Entry); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusCreated, map[string]string{"entry": req.Entry})
}

// RemoveAllowlist deletes one allowlist entry. Unknown entries are a 404.
func (h *DepsHandler) RemoveAllowlist(w http.ResponseWriter, r *http.Request) {
	entry := r.PathValue("entry")
	if err := h.deps.RemoveAllowlist(r.Context(), siteID(r), entry); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type cacheConfigResponse struct {
	MaxDepsBytes int64 `json:"maxDepsBytes"`
}

type cacheConfigRequest struct {
	MaxDepsBytes int64 `json:"maxDepsBytes"`
}

// GetCacheConfig returns the site's dependency tarball cache limit
// (cache-limits/eviction, spec §11). If the site has none configured, the
// response reports 0, meaning "no limit of its own" (the shared cache then
// falls back to the platform-wide maximum across sites, or unlimited).
func (h *DepsHandler) GetCacheConfig(w http.ResponseWriter, r *http.Request) {
	cfg, _, err := h.deps.GetCacheConfig(r.Context(), siteID(r))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, cacheConfigResponse{MaxDepsBytes: cfg.MaxDepsBytes})
}

// PutCacheConfig sets the site's dependency tarball cache limit. The value
// must be positive and within the platform cap; an invalid value is a 400.
func (h *DepsHandler) PutCacheConfig(w http.ResponseWriter, r *http.Request) {
	var req cacheConfigRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	if err := h.deps.SetCacheConfig(r.Context(), siteID(r), req.MaxDepsBytes); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, cacheConfigResponse{MaxDepsBytes: req.MaxDepsBytes})
}
