package client

import (
	"io"
	"log/slog"
	"net/http"
	"strings"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

type App struct {
	Sites    *domain.SiteService
	Contents *domain.ContentService
	Assets   *domain.AssetService
	Routes   *domain.RouteService
	Forms    *domain.FormService
	Logger   *slog.Logger

	// DefaultSlug is the fallback site when the request Host does not match
	// any site.hosts (LIAPOLDUS_CLIENT_DEFAULT_SLUG).
	DefaultSlug string
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

func serveAssetBytes(w http.ResponseWriter, r *http.Request, asset domain.Asset, reader io.ReadCloser) {
	defer reader.Close()
	w.Header().Set("Content-Type", asset.Mime)
	w.Header().Set("ETag", `"`+asset.ETag+`"`)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
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
	case domain.RouteServeAsset:
		asset, reader, err := e.app.Assets.Open(r.Context(), route.Action.AssetID)
		if err != nil {
			httpapi.RespondError(w, err)
			return
		}
		serveAssetBytes(w, r, asset, reader)
	case domain.RouteRedirect:
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
			status = 301
		}
		http.Redirect(w, r, target, status)
	case domain.RouteRenderPage:
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
