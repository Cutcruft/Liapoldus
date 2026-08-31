package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/liapoldus/liapoldus/backend/domain"
	"github.com/liapoldus/liapoldus/backend/page"
	"github.com/liapoldus/liapoldus/backend/site"
	"github.com/liapoldus/liapoldus/backend/snapshot"
)

type Handler struct {
	sites     *site.Service
	pages     *page.Service
	snapshots *snapshot.Service
	logger    *slog.Logger
}

func NewHandler(sites *site.Service, pages *page.Service, snapshots *snapshot.Service, logger *slog.Logger) http.Handler {
	h := &Handler{sites: sites, pages: pages, snapshots: snapshots, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("POST /api/sites", h.createSite)
	mux.HandleFunc("GET /api/sites/{siteID}", h.getSite)
	mux.HandleFunc("POST /api/sites/{siteID}/pages", h.createPage)
	mux.HandleFunc("GET /api/sites/{siteID}/pages", h.listPages)
	mux.HandleFunc("GET /api/pages/{pageID}", h.getPage)
	mux.HandleFunc("PUT /api/pages/{pageID}/tree", h.updateTree)
	mux.HandleFunc("GET /api/pages/{pageID}/versions", h.listVersions)
	mux.HandleFunc("POST /api/sites/{siteID}/snapshots", h.createSnapshot)
	mux.HandleFunc("GET /api/snapshots/{snapshotID}", h.getSnapshot)
	return withJSON(withCORS(mux))
}

type createSiteRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}
type createPageRequest struct {
	Name string               `json:"name"`
	Slug string               `json:"slug"`
	Root domain.ComponentNode `json:"root"`
}
type updateTreeRequest struct {
	Root domain.ComponentNode `json:"root"`
}
type createSnapshotRequest struct {
	Name string `json:"name"`
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) createSite(w http.ResponseWriter, r *http.Request) {
	var req createSiteRequest
	if !decode(r, &req, w) {
		return
	}
	result, err := h.sites.Create(r.Context(), req.Name, req.Slug)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) getSite(w http.ResponseWriter, r *http.Request) {
	result, err := h.sites.Get(r.Context(), r.PathValue("siteID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) createPage(w http.ResponseWriter, r *http.Request) {
	var req createPageRequest
	if !decode(r, &req, w) {
		return
	}
	result, err := h.pages.Create(r.Context(), r.PathValue("siteID"), req.Name, req.Slug, req.Root)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) listPages(w http.ResponseWriter, r *http.Request) {
	result, err := h.pages.ListBySite(r.Context(), r.PathValue("siteID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getPage(w http.ResponseWriter, r *http.Request) {
	result, err := h.pages.Get(r.Context(), r.PathValue("pageID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) updateTree(w http.ResponseWriter, r *http.Request) {
	var req updateTreeRequest
	if !decode(r, &req, w) {
		return
	}
	result, err := h.pages.UpdateTree(r.Context(), r.PathValue("pageID"), req.Root)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listVersions(w http.ResponseWriter, r *http.Request) {
	result, err := h.pages.Versions(r.Context(), r.PathValue("pageID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) createSnapshot(w http.ResponseWriter, r *http.Request) {
	var req createSnapshotRequest
	if !decode(r, &req, w) {
		return
	}
	result, err := h.snapshots.Create(r.Context(), r.PathValue("siteID"), req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) getSnapshot(w http.ResponseWriter, r *http.Request) {
	result, err := h.snapshots.Get(r.Context(), r.PathValue("snapshotID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func decode(r *http.Request, target any, w http.ResponseWriter) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return false
	}
	return true
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrAlreadyExists):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrInvalidRequest):
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func withJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,OPTIONS")
		next.ServeHTTP(w, r)
	})
}
