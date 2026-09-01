package httpapi

import "net/http"

// NewRouter registers all HTTP routes in one place. Handlers contain only
// request processing; route composition and middleware stay here.
func NewRouter(h *Handler) http.Handler {
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
