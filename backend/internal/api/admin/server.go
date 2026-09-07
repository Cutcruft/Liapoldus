package admin

import (
	"log/slog"
	"net/http"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
	"github.com/liapoldus/liapoldus/backend/internal/application/asset"
	buildapp "github.com/liapoldus/liapoldus/backend/internal/application/build"
	"github.com/liapoldus/liapoldus/backend/internal/application/component"
	"github.com/liapoldus/liapoldus/backend/internal/application/content"
	"github.com/liapoldus/liapoldus/backend/internal/application/deps"
	"github.com/liapoldus/liapoldus/backend/internal/application/form"
	"github.com/liapoldus/liapoldus/backend/internal/application/gitsnapshot"
	"github.com/liapoldus/liapoldus/backend/internal/application/infra"
	"github.com/liapoldus/liapoldus/backend/internal/application/page"
	"github.com/liapoldus/liapoldus/backend/internal/application/route"
	"github.com/liapoldus/liapoldus/backend/internal/application/site"
	"github.com/liapoldus/liapoldus/backend/internal/application/snapshot"
	"github.com/liapoldus/liapoldus/backend/internal/application/token"
)

type App struct {
	Sites      *site.Service
	Pages      *page.Service
	Contents   *content.Service
	Assets     *asset.Service
	Routes     *route.Service
	Forms      *form.Service
	Snapshots  *snapshot.Service
	Components *component.Service
	Git        *gitsnapshot.Service
	Builds     *buildapp.Service
	Deps       *deps.Service
	Tokens     *token.Service
	Infra      *infra.Service
	Logger     *slog.Logger
	AdminToken string
	// DefaultLocale and RedirectDefaultStatus mirror server configuration so
	// the Settings page can surface them (slice 2.3).
	DefaultLocale         string
	RedirectDefaultStatus int
	// BuildEvents is the hub the /api/builds/ws channel relays (optional).
	BuildEvents BuildEventSource
}

func NewRouter(app App) http.Handler {
	mux := http.NewServeMux()

	siteHandler := NewSiteHandler(app.Sites)
	pageHandler := NewPageHandler(app.Pages)
	contentHandler := NewContentHandler(app.Contents)
	localesHandler := NewLocalesHandler(app.Sites, app.Contents)
	assetHandler := NewAssetHandler(app.Assets)
	assetUsageHandler := NewAssetUsageHandler(app.Assets, app.Contents, app.Forms)
	routeHandler := NewRouteHandler(app.Routes)
	formHandler := NewFormHandler(app.Forms)
	snapshotHandler := NewSnapshotHandler(app.Snapshots)
	componentHandler := NewComponentHandler(app.Components, app.Pages, app.Git)
	buildHandler := NewBuildHandler(app.Builds)
	depsHandler := NewDepsHandler(app.Deps)
	dashboardHandler := NewDashboardHandler(app.Sites, app.Builds, app.Snapshots, app.Git)
	settingsHandler := NewSettingsHandler(app.AdminToken, app.DefaultLocale, app.RedirectDefaultStatus)
	authHandler := NewAuthHandler(app.AdminToken)
	infraHandler := NewInfraHandler(app.Infra)

	var gitHandler *GitHandler
	if app.Git != nil {
		gitHandler = NewGitHandler(app.Git)
	}
	var tokensHandler *TokensHandler
	if app.Tokens != nil {
		tokensHandler = NewTokensHandler(app.Tokens)
	}

	// Health stays public (pre-auth).
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		httpapi.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Live build events for the admin Builds page. Registered on the outer
	// (pre-BearerAuth) mux because browsers can't send an Authorization header
	// on WebSocket; the handler validates the token from the query instead.
	if app.BuildEvents != nil {
		mux.Handle("GET /api/builds/ws", http.HandlerFunc(NewBuildsWsHandler(app.BuildEvents, app.AdminToken, app.Logger).Serve))
	}

	// Token validation is public (pre-BearerAuth) so the admin login page can
	// authenticate before holding a valid token.
	mux.HandleFunc("POST /api/auth/validate", authHandler.Validate)

	// Everything else is behind the admin bearer token.
	protected := http.NewServeMux()

	protected.HandleFunc("GET /api/dashboard", dashboardHandler.Get)
	protected.HandleFunc("GET /api/settings", settingsHandler.Get)

	protected.HandleFunc("POST /api/sites", siteHandler.Create)
	protected.HandleFunc("GET /api/sites", siteHandler.List)
	protected.HandleFunc("GET /api/sites/{siteID}", siteHandler.Get)
	protected.HandleFunc("PUT /api/sites/{siteID}", siteHandler.Update)
	protected.HandleFunc("DELETE /api/sites/{siteID}", siteHandler.Delete)

	protected.HandleFunc("POST /api/sites/{siteID}/pages", pageHandler.Create)
	protected.HandleFunc("GET /api/sites/{siteID}/pages", pageHandler.List)
	protected.HandleFunc("GET /api/pages/{pageID}", pageHandler.Get)
	protected.HandleFunc("PUT /api/pages/{pageID}", pageHandler.Update)
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
	protected.HandleFunc("GET /api/sites/{siteID}/locales", localesHandler.Get)

	protected.HandleFunc("POST /api/sites/{siteID}/assets", assetHandler.Create)
	protected.HandleFunc("GET /api/sites/{siteID}/assets", assetHandler.List)
	protected.HandleFunc("GET /api/assets/{assetID}", assetHandler.GetMetadata)
	protected.HandleFunc("GET /api/assets/{assetID}/file", assetHandler.GetFile)
	protected.HandleFunc("DELETE /api/assets/{assetID}", assetHandler.Delete)
	protected.HandleFunc("GET /api/sites/{siteID}/assets/{assetID}/usage", assetUsageHandler.Get)

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
	protected.HandleFunc("DELETE /api/forms/{formID}/submissions/{submissionID}", formHandler.DeleteSubmission)
	protected.HandleFunc("DELETE /api/sites/{siteID}/forms/{formID}/submissions/{submissionID}", formHandler.DeleteSubmission)

	protected.HandleFunc("POST /api/sites/{siteID}/snapshots", snapshotHandler.Create)
	protected.HandleFunc("GET /api/sites/{siteID}/snapshots", snapshotHandler.List)
	protected.HandleFunc("GET /api/snapshots/{snapshotID}", snapshotHandler.Get)
	protected.HandleFunc("DELETE /api/snapshots/{snapshotID}", snapshotHandler.Delete)

	protected.HandleFunc("POST /api/sites/{siteID}/components", componentHandler.Define)
	protected.HandleFunc("GET /api/sites/{siteID}/components", componentHandler.List)
	protected.HandleFunc("GET /api/sites/{siteID}/components/registry", componentHandler.Registry)
	protected.HandleFunc("GET /api/sites/{siteID}/components/{componentID}", componentHandler.Get)
	protected.HandleFunc("PUT /api/sites/{siteID}/components/{componentID}", componentHandler.Update)
	protected.HandleFunc("DELETE /api/sites/{siteID}/components/{componentID}", componentHandler.Delete)
	protected.HandleFunc("GET /api/sites/{siteID}/components/{componentID}/history", componentHandler.History)
	protected.HandleFunc("GET /api/sites/{siteID}/components/{componentID}/usage", componentHandler.Usage)
	protected.HandleFunc("POST /api/sites/{siteID}/components/{componentID}/commit", componentHandler.Commit)

	protected.HandleFunc("POST /api/sites/{siteID}/operations", infraHandler.CreateOperation)
	protected.HandleFunc("GET /api/sites/{siteID}/operations", infraHandler.ListOperations)
	protected.HandleFunc("GET /api/sites/{siteID}/operations/{operationID}", infraHandler.GetOperation)
	protected.HandleFunc("PUT /api/sites/{siteID}/operations/{operationID}", infraHandler.UpdateOperation)
	protected.HandleFunc("DELETE /api/sites/{siteID}/operations/{operationID}", infraHandler.DeleteOperation)

	protected.HandleFunc("POST /api/sites/{siteID}/endpoints", infraHandler.CreateEndpoint)
	protected.HandleFunc("GET /api/sites/{siteID}/endpoints", infraHandler.ListEndpoints)
	protected.HandleFunc("GET /api/sites/{siteID}/endpoints/{endpointID}", infraHandler.GetEndpoint)
	protected.HandleFunc("PUT /api/sites/{siteID}/endpoints/{endpointID}", infraHandler.UpdateEndpoint)
	protected.HandleFunc("DELETE /api/sites/{siteID}/endpoints/{endpointID}", infraHandler.DeleteEndpoint)

	protected.HandleFunc("POST /api/sites/{siteID}/builds", buildHandler.Create)
	protected.HandleFunc("GET /api/sites/{siteID}/builds", buildHandler.List)
	protected.HandleFunc("GET /api/builds/{buildID}", buildHandler.Get)

	if gitHandler != nil {
		protected.HandleFunc("GET /api/sites/{siteID}/git", gitHandler.Overview)
		protected.HandleFunc("POST /api/sites/{siteID}/git/commit", gitHandler.Commit)
		protected.HandleFunc("POST /api/sites/{siteID}/git/publish", gitHandler.Publish)
		protected.HandleFunc("POST /api/sites/{siteID}/git/restore", gitHandler.Restore)
		protected.HandleFunc("POST /api/sites/{siteID}/git/rollback", gitHandler.Rollback)
	}

	protected.HandleFunc("GET /api/sites/{siteID}/dependencies", depsHandler.List)
	protected.HandleFunc("POST /api/sites/{siteID}/dependencies", depsHandler.Create)
	protected.HandleFunc("DELETE /api/sites/{siteID}/dependencies/{name}", depsHandler.Delete)
	protected.HandleFunc("POST /api/sites/{siteID}/dependencies/resolve", depsHandler.Resolve)
	protected.HandleFunc("GET /api/sites/{siteID}/dependencies/allowlist", depsHandler.ListAllowlist)
	protected.HandleFunc("POST /api/sites/{siteID}/dependencies/allowlist", depsHandler.AddAllowlist)
	protected.HandleFunc("DELETE /api/sites/{siteID}/dependencies/allowlist", depsHandler.RemoveAllowlist)
	protected.HandleFunc("GET /api/sites/{siteID}/cache-config", depsHandler.GetCacheConfig)
	protected.HandleFunc("PUT /api/sites/{siteID}/cache-config", depsHandler.PutCacheConfig)
	protected.HandleFunc("POST /api/sites/{siteID}/cache-config/evict", depsHandler.EvictCacheConfig)

	if tokensHandler != nil {
		protected.HandleFunc("GET /api/sites/{siteID}/tokens", tokensHandler.Get)
		protected.HandleFunc("PUT /api/sites/{siteID}/tokens", tokensHandler.Put)
	}

	mux.Handle("/", httpapi.BearerAuth(app.AdminToken, httpapi.WithCORS(protected)))

	return httpapi.WithCORS(injectLogger(mux, app.Logger))
}
