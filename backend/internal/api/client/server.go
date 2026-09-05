package client

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
	"github.com/liapoldus/liapoldus/backend/internal/application/asset"
	"github.com/liapoldus/liapoldus/backend/internal/application/content"
	"github.com/liapoldus/liapoldus/backend/internal/application/form"
	routeapp "github.com/liapoldus/liapoldus/backend/internal/application/route"
	"github.com/liapoldus/liapoldus/backend/internal/application/site"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

type App struct {
	Sites    *site.Service
	Contents *content.Service
	Assets   *asset.Service
	Routes   *routeapp.Service
	Forms    *form.Service
	Logger   *slog.Logger

	// DefaultSlug is the fallback site when the request Host does not match
	// any site.hosts (LIAPOLDUS_CLIENT_DEFAULT_SLUG).
	DefaultSlug string

	// DefaultRedirectStatus is the status used for redirect routes without an
	// explicit one (LIAPOLDUS_REDIRECT_DEFAULT_STATUS).
	DefaultRedirectStatus int

	// AssetCacheMaxAgeSeconds sets the Cache-Control max-age for asset file
	// responses (LIAPOLDUS_ASSET_CACHE_MAX_AGE_SECONDS).
	AssetCacheMaxAgeSeconds int
}

func (a *App) resolveSite(r *http.Request) (domain.Site, error) {
	if siteID := r.PathValue("siteID"); siteID != "" {
		return a.Sites.Get(r.Context(), siteID)
	}
	if siteID := r.URL.Query().Get("siteId"); siteID != "" {
		return a.Sites.Get(r.Context(), siteID)
	}
	host := r.Host
	if index := strings.IndexByte(host, ':'); index >= 0 {
		host = host[:index]
	}
	site, err := a.Sites.ResolveByHost(r.Context(), host, a.DefaultSlug)
	if err != nil {
		return domain.Site{}, err
	}
	return site, nil
}

func NewRouter(a *App) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		httpapi.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	content := NewContentHandler(a)
	asset := NewAssetHandler(a)
	form := NewFormHandler(a)
	contract := NewContractHandler(a)

	mux.HandleFunc("GET /api/contents/{contentID}", content.Get)
	mux.HandleFunc("GET /api/contents", content.List)
	mux.HandleFunc("GET /api/sites/{siteID}/contents", content.List)
	mux.HandleFunc("POST /api/sites/{siteID}/contents/batch", content.Batch)

	mux.HandleFunc("GET /api/assets/{assetID}", asset.Get)
	mux.HandleFunc("GET /api/assets/{assetID}/file", asset.File)
	mux.HandleFunc("GET /api/sites/{siteID}/assets", asset.List)

	mux.HandleFunc("GET /api/forms/{formID}", form.Get)
	mux.HandleFunc("POST /api/forms/{formID}/submissions", form.Submit)

	mux.HandleFunc("GET /runtime/contract", contract.Contract)
	mux.HandleFunc("GET /runtime/routes", contract.Routes)

	edge := &EdgeHandler{app: a}
	mux.HandleFunc("/", edge.Serve)

	return httpapi.WithCORS(mux)
}

func serveAssetBytes(w http.ResponseWriter, r *http.Request, asset domain.Asset, reader io.ReadCloser, cacheMaxAgeSeconds int) {
	defer reader.Close()
	w.Header().Set("Content-Type", asset.Mime)
	w.Header().Set("ETag", `"`+asset.ETag+`"`)
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", cacheMaxAgeSeconds))
	w.Header().Set("Accept-Ranges", "bytes")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader)
}

type EdgeHandler struct{ app *App }

func (e *EdgeHandler) Serve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		httpapi.RespondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	site, err := e.app.resolveSite(r)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	route, groups, matched, err := e.app.Routes.Match(r.Context(), site.ID, r.URL.Path)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	if !matched {
		httpapi.RespondJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	switch route.Action.Type {
	case routeapp.ServeAsset:
		asset, reader, err := e.app.Assets.Open(r.Context(), route.Action.AssetID)
		if err != nil {
			httpapi.RespondError(w, err)
			return
		}
		serveAssetBytes(w, r, asset, reader, e.app.AssetCacheMaxAgeSeconds)
	case routeapp.Redirect:
		target := expandGroups(route.Action.Target, groups)
		if route.Action.KeepQuery && r.URL.RawQuery != "" {
			if strings.Contains(target, "?") {
				target += "&" + r.URL.RawQuery
			} else {
				target += "?" + r.URL.RawQuery
			}
		}
		status := route.Action.Status
		if status == 0 {
			status = e.app.DefaultRedirectStatus
		}
		http.Redirect(w, r, target, status)
	case routeapp.RenderPage:
		httpapi.RespondJSON(w, http.StatusNotFound, map[string]string{"error": "render page not implemented"})
	default:
		httpapi.RespondJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

// expandGroups replaces $1..$9 (also ${1}..${9}) in target with captured groups.
func expandGroups(target string, groups []string) string {
	var sb strings.Builder
	for i := 0; i < len(target); i++ {
		if target[i] == '$' && i+1 < len(target) && target[i+1] >= '1' && target[i+1] <= '9' {
			index := int(target[i+1] - '0')
			value := ""
			if index <= len(groups) {
				value = groups[index-1]
			}
			sb.WriteString(value)
			i++
			continue
		}
		sb.WriteByte(target[i])
	}
	return sb.String()
}
