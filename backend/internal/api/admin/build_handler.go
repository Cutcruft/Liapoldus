package admin

import (
	"net/http"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
	buildapp "github.com/liapoldus/liapoldus/backend/internal/application/build"
)

type BuildHandler struct {
	builds *buildapp.Service
}

func NewBuildHandler(builds *buildapp.Service) *BuildHandler {
	return &BuildHandler{builds: builds}
}

type createBuildRequest struct {
	SnapshotID  string `json:"snapshotId"`
	Environment string `json:"environment"`
}

// Create queues/publishes a snapshot for an environment (Этап 3: synchronous).
// Publication is idempotent: repeating the same snapshot returns the same
// build id.
func (h *BuildHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createBuildRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.builds.Create(r.Context(), siteID(r), req.SnapshotID, req.Environment)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusCreated, result)
}

// List returns every build of the site, oldest first.
func (h *BuildHandler) List(w http.ResponseWriter, r *http.Request) {
	result, err := h.builds.ListBySite(r.Context(), siteID(r))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *BuildHandler) Get(w http.ResponseWriter, r *http.Request) {
	buildID := r.PathValue("buildID")
	if buildID == "" {
		buildID = r.URL.Query().Get("buildId")
	}
	result, err := h.builds.Get(r.Context(), buildID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}
