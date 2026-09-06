package admin

import (
	"fmt"
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
// The entry is read from the ?entry= query parameter (not a path segment):
// entries like "@scope/*" and "*" are natural to type in a query value, and
// path-based encoding of "/" and "*" is awkward and error-prone.
func (h *DepsHandler) RemoveAllowlist(w http.ResponseWriter, r *http.Request) {
	entry := r.URL.Query().Get("entry")
	if entry == "" {
		httpapi.RespondError(w, fmt.Errorf("missing ?entry= query parameter"))
		return
	}
	if err := h.deps.RemoveAllowlist(r.Context(), siteID(r), entry); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Resolve runs a full dependency resolution of the site's declared range set
// and returns the frozen instance graph (spec §5, nested-versioned layout).
// The response mirrors a snapshot's deps-lock: one entry per (name, version)
// in deterministic BFS order, with the Hoisted flag, RequestedBy parent edges,
// and each instance's peer metadata, so the admin page can render the graph as
// an expandable tree. The feed is "what a snapshot would freeze right now" —
// same allowlist and peer policy as lock creation, so an unsatisfiable graph
// fails with the same status mapping (e.g. 422 on ErrUnresolvableSpec /
// ErrUnsatisfiedPeer / ErrDepNotAllowed).
func (h *DepsHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	lock, err := h.deps.ResolveLock(r.Context(), siteID(r))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, lock)
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

type evictCacheRequest struct {
	TargetDepsBytes int64 `json:"targetDepsBytes"`
}

type evictCacheResponse struct {
	Evicted      int   `json:"evicted"`
	EvictedBytes int64 `json:"evictedBytes"`
}

// EvictCacheConfig runs a manual LRU-eviction pass of the site's dependency
// tarball cache down to the requested retained size (cache-limits/eviction,
// spec §11). A target of 0 (or omitted) evicts down to the effective limit.
// Only on-disk .tgz blobs are removed; DB metadata stays intact. The request
// targets the shared cache, so the siteID path parameter is informational.
func (h *DepsHandler) EvictCacheConfig(w http.ResponseWriter, r *http.Request) {
	var req evictCacheRequest
	if r.Body != nil && r.ContentLength != 0 {
		if !httpapi.DecodeJSON(r, &req, w) {
			return
		}
	}
	evictedBytes, evicted, err := h.deps.ManualEvictTarballs(r.Context(), req.TargetDepsBytes)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, evictCacheResponse{Evicted: evicted, EvictedBytes: evictedBytes})
}
