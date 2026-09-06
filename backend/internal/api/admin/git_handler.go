package admin

import (
	"net/http"
	"strings"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
	gitapp "github.com/liapoldus/liapoldus/backend/internal/application/git"
	"github.com/liapoldus/liapoldus/backend/internal/application/gitsnapshot"
)

type GitHandler struct {
	git *gitsnapshot.Service
}

func NewGitHandler(git *gitsnapshot.Service) *GitHandler {
	return &GitHandler{git: git}
}

// gitOverviewResponse bundles the site git status with the dev history,
// produced by the Git page in the admin UI.
type gitOverviewResponse struct {
	Status  gitsnapshot.SiteStatus `json:"status"`
	Commits []gitapp.CommitInfo    `json:"commits"`
}

func (h *GitHandler) Overview(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	if siteID == "" {
		siteID = r.URL.Query().Get("siteId")
	}
	status, err := h.git.Status(r.Context(), siteID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	commits, err := h.git.Commits(r.Context(), siteID, 50)
	if err != nil && !strings.Contains(err.Error(), "not initialized") {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, gitOverviewResponse{Status: *status, Commits: commits})
}

type gitWriteRequest struct {
	Message string `json:"message"`
	SHA     string `json:"sha"`
}

func (h *GitHandler) Commit(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	if siteID == "" {
		siteID = r.URL.Query().Get("siteId")
	}
	var req gitWriteRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.git.Commit(r.Context(), siteID, req.Message)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusCreated, map[string]string{"sha": result})
}

func (h *GitHandler) Publish(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	if siteID == "" {
		siteID = r.URL.Query().Get("siteId")
	}
	var req gitWriteRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.git.Publish(r.Context(), siteID, req.Message)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusCreated, map[string]string{"sha": result})
}

func (h *GitHandler) Restore(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	if siteID == "" {
		siteID = r.URL.Query().Get("siteId")
	}
	var req gitWriteRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	if strings.TrimSpace(req.SHA) == "" {
		httpapi.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "sha is required"})
		return
	}
	result, err := h.git.Restore(r.Context(), siteID, strings.TrimSpace(req.SHA), req.Message)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, map[string]string{"sha": result})
}

func (h *GitHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	if siteID == "" {
		siteID = r.URL.Query().Get("siteId")
	}
	var req gitWriteRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	if strings.TrimSpace(req.SHA) == "" {
		httpapi.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "sha is required"})
		return
	}
	result, err := h.git.Rollback(r.Context(), siteID, strings.TrimSpace(req.SHA), req.Message)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, map[string]string{"sha": result})
}
