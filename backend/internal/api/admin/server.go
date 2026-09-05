package admin

import (
	"log/slog"
	"net/http"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
	"github.com/liapoldus/liapoldus/backend/internal/application/asset"
	"github.com/liapoldus/liapoldus/backend/internal/application/content"
	"github.com/liapoldus/liapoldus/backend/internal/application/form"
	"github.com/liapoldus/liapoldus/backend/internal/application/page"
	"github.com/liapoldus/liapoldus/backend/internal/application/route"
	"github.com/liapoldus/liapoldus/backend/internal/application/site"
	"github.com/liapoldus/liapoldus/backend/internal/application/snapshot"
)

type App struct {
	Sites      *site.Service
	Pages      *page.Service
	Contents   *content.Service
	Assets     *asset.Service
	Routes     *route.Service
	Forms      *form.Service
	Snapshots  *snapshot.Service
	Logger     *slog.Logger
	AdminToken string
}

func NewRouter(app App) http.Handler {
	mux := http.NewServeMux()

	siteHandler := NewSiteHandler(app.Sites)
	pageHandler := NewPageHandler(app.Pages)
	contentHandler := NewContentHandler(app.Contents)
	assetHandler := NewAssetHandler(app.Assets)
	routeHandler := NewRouteHandler(app.Routes)
	formHandler := NewFormHandler(app.Forms)
	snapshotHandler := NewSnapshotHandler(app.Snapshots)

	// Health stays public (pre-auth).
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		httpapi.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Everything else is behind the admin bearer token.
	protected := http.NewServeMux()

	protected.HandleFunc("POST /api/sites", siteHandler.Create)
	protected.HandleFunc("GET /api/sites", siteHandler.List)
	protected.HandleFunc("GET /api/sites/{siteID}", siteHandler.Get)
	protected.HandleFunc("PUT /api/sites/{siteID}", siteHandler.Update)
	protected.HandleFunc("DELETE /api/sites/{siteID}", siteHandler.Delete)

	protected.HandleFunc("POST /api/sites/{siteID}/pages", pageHandler.Create)
	protected.HandleFunc("GET /api/sites/{siteID}/pages", pageHandler.List)
	protected.HandleFunc("GET /api/pages/{pageID}", pageHandler.Get)
	protected.HandleFunc("PUT /api/pages/{pageID}/tree", pageHandler.UpdateTree)
	protected.HandleFunc("DELETE /api/pages/{pageID}", pageHandler.Delete)
	protected.HandleFunc("GET /api/pages/{pageID}/versions", pageHandler.ListVersions)
	protected.HandleFunc("GET /api/pages/{pageID}/versions/{versionID}", pageHandler.GetVersion)

	protected.HandleFunc("POST /api/sites/{siteID}/contents", contentHandler.Create)
	protected.HandleFunc("GET /api/sites/{siteID}/contents", contentHandler.List)
	protected.HandleFunc("GET /api/contents/{contentID}", contentHandler.Get)
	protected.HandleFunc("GET /api/sites/{siteID}/contents/{contentID}", contentHandler.Get)
	protected.HandleFunc("PUT /api/contents/{contentID}/fields", contentHandler.UpdateFields)
	protected.HandleFunc("PUT /api/sites/{siteID}/contents/{contentID}", contentHandler.UpdateFields)
	protected.HandleFunc("PUT /api/contents/{contentID}/translations/{locale}", contentHandler.SetTranslation)
	protected.HandleFunc("PUT /api/sites/{siteID}/contents/{contentID}/translations/{locale}", contentHandler.SetTranslation)
	protected.HandleFunc("DELETE /api/contents/{contentID}/translations/{locale}", contentHandler.DeleteTranslation)
	protected.HandleFunc("DELETE /api/sites/{siteID}/contents/{contentID}/translations/{locale}", contentHandler.DeleteTranslation)
	protected.HandleFunc("GET /api/sites/{siteID}/contents/{contentID}/translations", contentHandler.GetTranslations)
	protected.HandleFunc("DELETE /api/contents/{contentID}", contentHandler.Delete)
	protected.HandleFunc("DELETE /api/sites/{siteID}/contents/{contentID}", contentHandler.Delete)

	protected.HandleFunc("POST /api/sites/{siteID}/assets", assetHandler.Create)
	protected.HandleFunc("GET /api/sites/{siteID}/assets", assetHandler.List)
	protected.HandleFunc("GET /api/assets/{assetID}", assetHandler.GetMetadata)
	protected.HandleFunc("GET /api/assets/{assetID}/file", assetHandler.GetFile)
	protected.HandleFunc("DELETE /api/assets/{assetID}", assetHandler.Delete)

	protected.HandleFunc("POST /api/sites/{siteID}/routes", routeHandler.Create)
	protected.HandleFunc("GET /api/sites/{siteID}/routes", routeHandler.List)
	protected.HandleFunc("GET /api/routes/{routeID}", routeHandler.Get)
	protected.HandleFunc("GET /api/sites/{siteID}/routes/{routeID}", routeHandler.Get)
	protected.HandleFunc("PUT /api/routes/{routeID}", routeHandler.Update)
	protected.HandleFunc("PUT /api/sites/{siteID}/routes/{routeID}", routeHandler.Update)
	protected.HandleFunc("DELETE /api/routes/{routeID}", routeHandler.Delete)
	protected.HandleFunc("DELETE /api/sites/{siteID}/routes/{routeID}", routeHandler.Delete)

	protected.HandleFunc("POST /api/sites/{siteID}/forms", formHandler.Create)
	protected.HandleFunc("GET /api/sites/{siteID}/forms", formHandler.List)
	protected.HandleFunc("GET /api/forms/{formID}", formHandler.Get)
	protected.HandleFunc("GET /api/sites/{siteID}/forms/{formID}", formHandler.Get)
	protected.HandleFunc("PUT /api/forms/{formID}", formHandler.Update)
	protected.HandleFunc("PUT /api/sites/{siteID}/forms/{formID}", formHandler.Update)
	protected.HandleFunc("DELETE /api/forms/{formID}", formHandler.Delete)
	protected.HandleFunc("DELETE /api/sites/{siteID}/forms/{formID}", formHandler.Delete)
	protected.HandleFunc("GET /api/forms/{formID}/submissions", formHandler.ListSubmissions)
	protected.HandleFunc("GET /api/sites/{siteID}/forms/{formID}/submissions", formHandler.ListSubmissions)

	protected.HandleFunc("POST /api/sites/{siteID}/snapshots", snapshotHandler.Create)
	protected.HandleFunc("GET /api/sites/{siteID}/snapshots", snapshotHandler.List)
	protected.HandleFunc("GET /api/snapshots/{snapshotID}", snapshotHandler.Get)
	protected.HandleFunc("DELETE /api/snapshots/{snapshotID}", snapshotHandler.Delete)

	mux.Handle("/", httpapi.BearerAuth(app.AdminToken, httpapi.WithCORS(protected)))

	return httpapi.WithCORS(injectLogger(mux, app.Logger))
}
