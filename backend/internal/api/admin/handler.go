package admin

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
	"github.com/liapoldus/liapoldus/backend/internal/application/asset"
	"github.com/liapoldus/liapoldus/backend/internal/application/content"
	"github.com/liapoldus/liapoldus/backend/internal/application/form"
	"github.com/liapoldus/liapoldus/backend/internal/application/page"
	"github.com/liapoldus/liapoldus/backend/internal/application/route"
	"github.com/liapoldus/liapoldus/backend/internal/application/site"
	"github.com/liapoldus/liapoldus/backend/internal/application/snapshot"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

type SiteHandler struct {
	sites *site.Service
}

func NewSiteHandler(sites *site.Service) *SiteHandler {
	return &SiteHandler{sites: sites}
}

type createSiteRequest struct {
	Name          string   `json:"name"`
	Slug          string   `json:"slug"`
	DefaultLocale string   `json:"defaultLocale"`
	Hosts         []string `json:"hosts"`
}

type updateSiteRequest struct {
	Name          *string   `json:"name"`
	Slug          *string   `json:"slug"`
	DefaultLocale *string   `json:"defaultLocale"`
	Hosts         *[]string `json:"hosts"`
}

func (h *SiteHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createSiteRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.sites.Create(r.Context(), req.Name, req.Slug, req.DefaultLocale, req.Hosts)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusCreated, result)
}

func (h *SiteHandler) Get(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	if siteID == "" {
		siteID = r.URL.Query().Get("siteId")
	}
	result, err := h.sites.Get(r.Context(), siteID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *SiteHandler) List(w http.ResponseWriter, r *http.Request) {
	result, err := h.sites.List(r.Context())
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *SiteHandler) Update(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	if siteID == "" {
		siteID = r.URL.Query().Get("siteId")
	}
	var req updateSiteRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.sites.Update(r.Context(), siteID, func(site domain.Site) domain.Site {
		if req.Name != nil {
			site.Name = *req.Name
		}
		if req.Slug != nil {
			site.Slug = *req.Slug
		}
		if req.DefaultLocale != nil {
			site.DefaultLocale = *req.DefaultLocale
		}
		if req.Hosts != nil {
			site.Hosts = *req.Hosts
		}
		return site
	})
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *SiteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	if siteID == "" {
		siteID = r.URL.Query().Get("siteId")
	}
	if err := h.sites.Delete(r.Context(), siteID); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type PageHandler struct{ pages *page.Service }

func NewPageHandler(pages *page.Service) *PageHandler {
	return &PageHandler{pages: pages}
}

type createPageRequest struct {
	Name string               `json:"name"`
	Slug string               `json:"slug"`
	Root domain.ComponentNode `json:"root"`
}

type updateTreeRequest struct {
	Root domain.ComponentNode `json:"root"`
}

func (h *PageHandler) Create(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	if siteID == "" {
		siteID = r.URL.Query().Get("siteId")
	}
	var req createPageRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.pages.Create(r.Context(), siteID, req.Name, req.Slug, req.Root)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusCreated, result)
}

func (h *PageHandler) List(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	if siteID == "" {
		siteID = r.URL.Query().Get("siteId")
	}
	result, err := h.pages.ListBySite(r.Context(), siteID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *PageHandler) Get(w http.ResponseWriter, r *http.Request) {
	pageID := r.PathValue("pageID")
	if pageID == "" {
		pageID = r.URL.Query().Get("pageId")
	}
	result, err := h.pages.Get(r.Context(), pageID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *PageHandler) UpdateTree(w http.ResponseWriter, r *http.Request) {
	pageID := r.PathValue("pageID")
	if pageID == "" {
		pageID = r.URL.Query().Get("pageId")
	}
	var req updateTreeRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.pages.UpdateTree(r.Context(), pageID, req.Root)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *PageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	pageID := r.PathValue("pageID")
	if pageID == "" {
		pageID = r.URL.Query().Get("pageId")
	}
	if err := h.pages.Delete(r.Context(), pageID); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PageHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	pageID := r.PathValue("pageID")
	if pageID == "" {
		pageID = r.URL.Query().Get("pageId")
	}
	result, err := h.pages.Versions(r.Context(), pageID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *PageHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	pageID := r.PathValue("pageID")
	if pageID == "" {
		pageID = r.URL.Query().Get("pageId")
	}
	versionID := r.PathValue("versionID")
	if versionID == "" {
		versionID = r.URL.Query().Get("versionId")
	}
	result, err := h.pages.Version(r.Context(), pageID, versionID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

type ContentHandler struct{ contents *content.Service }

func NewContentHandler(contents *content.Service) *ContentHandler {
	return &ContentHandler{contents: contents}
}

type createContentRequest struct {
	CollectionID string         `json:"collectionId"`
	ID           string         `json:"id,omitempty"`
	Fields       map[string]any `json:"fields"`
}

type updateFieldsRequest struct {
	Fields map[string]any `json:"fields"`
}

type setTranslationRequest struct {
	Fields map[string]any `json:"fields"`
}

func (h *ContentHandler) Create(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	if siteID == "" {
		siteID = r.URL.Query().Get("siteId")
	}
	var req createContentRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.contents.Create(r.Context(), siteID, req.CollectionID, req.ID, req.Fields)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusCreated, result)
}

func (h *ContentHandler) List(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	if siteID == "" {
		siteID = r.URL.Query().Get("siteId")
	}
	collectionID := r.URL.Query().Get("collection")
	result, err := h.contents.List(r.Context(), siteID, collectionID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func siteID(r *http.Request) string {
	if s := r.PathValue("siteID"); s != "" {
		return s
	}
	return r.URL.Query().Get("siteId")
}

func (h *ContentHandler) Get(w http.ResponseWriter, r *http.Request) {
	contentID := r.PathValue("contentID")
	if contentID == "" {
		contentID = r.URL.Query().Get("contentId")
	}
	result, err := h.contents.Get(r.Context(), siteID(r), contentID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *ContentHandler) UpdateFields(w http.ResponseWriter, r *http.Request) {
	contentID := r.PathValue("contentID")
	if contentID == "" {
		contentID = r.URL.Query().Get("contentId")
	}
	var req updateFieldsRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.contents.UpdateFields(r.Context(), siteID(r), contentID, req.Fields)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *ContentHandler) SetTranslation(w http.ResponseWriter, r *http.Request) {
	contentID := r.PathValue("contentID")
	if contentID == "" {
		contentID = r.URL.Query().Get("contentId")
	}
	locale := r.PathValue("locale")
	if locale == "" {
		locale = r.URL.Query().Get("locale")
	}
	var req setTranslationRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.contents.SetTranslation(r.Context(), siteID(r), contentID, locale, req.Fields)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *ContentHandler) DeleteTranslation(w http.ResponseWriter, r *http.Request) {
	contentID := r.PathValue("contentID")
	if contentID == "" {
		contentID = r.URL.Query().Get("contentId")
	}
	locale := r.PathValue("locale")
	if locale == "" {
		locale = r.URL.Query().Get("locale")
	}
	if err := h.contents.DeleteTranslation(r.Context(), siteID(r), contentID, locale); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ContentHandler) GetTranslations(w http.ResponseWriter, r *http.Request) {
	contentID := r.PathValue("contentID")
	if contentID == "" {
		contentID = r.URL.Query().Get("contentId")
	}
	result, err := h.contents.Get(r.Context(), siteID(r), contentID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result.Translations)
}

func (h *ContentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	contentID := r.PathValue("contentID")
	if contentID == "" {
		contentID = r.URL.Query().Get("contentId")
	}
	if err := h.contents.Delete(r.Context(), siteID(r), contentID); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type AssetHandler struct {
	assets *asset.Service
}

func NewAssetHandler(assets *asset.Service) *AssetHandler {
	return &AssetHandler{assets: assets}
}

func (h *AssetHandler) Create(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	if siteID == "" {
		siteID = r.URL.Query().Get("siteId")
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		httpapi.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "multipart form parse error: " + err.Error()})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpapi.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required"})
		return
	}
	defer file.Close()
	mime := header.Header.Get("Content-Type")
	if strings.TrimSpace(mime) == "" {
		mime = "application/octet-stream"
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = header.Filename
	}
	asset, err := h.assets.Create(r.Context(), siteID, name, mime, file)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusCreated, h.assets.Metadata(asset))
}

func (h *AssetHandler) List(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	if siteID == "" {
		siteID = r.URL.Query().Get("siteId")
	}
	result, err := h.assets.List(r.Context(), siteID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, h.assets.MetadataList(result))
}

func (h *AssetHandler) GetMetadata(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("assetID")
	if assetID == "" {
		assetID = r.URL.Query().Get("assetId")
	}
	result, err := h.assets.Get(r.Context(), assetID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, h.assets.Metadata(result))
}

func (h *AssetHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("assetID")
	if assetID == "" {
		assetID = r.URL.Query().Get("assetId")
	}
	asset, reader, err := h.assets.Open(r.Context(), assetID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	defer reader.Close()
	w.Header().Set("Content-Type", asset.Mime)
	w.Header().Set("ETag", `"`+asset.ETag+`"`)
	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader)
}

func (h *AssetHandler) Delete(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("assetID")
	if assetID == "" {
		assetID = r.URL.Query().Get("assetId")
	}
	if err := h.assets.Delete(r.Context(), assetID); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type RouteHandler struct{ routes *route.Service }

func NewRouteHandler(routes *route.Service) *RouteHandler {
	return &RouteHandler{routes: routes}
}

type createRouteRequest struct {
	Matcher  string             `json:"matcher"`
	Priority int                `json:"priority"`
	Action   domain.RouteAction `json:"action"`
}

func (h *RouteHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRouteRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.routes.Create(r.Context(), siteID(r), req.Matcher, req.Priority, req.Action)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusCreated, result)
}

func (h *RouteHandler) List(w http.ResponseWriter, r *http.Request) {
	result, err := h.routes.List(r.Context(), siteID(r))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *RouteHandler) Get(w http.ResponseWriter, r *http.Request) {
	routeID := r.PathValue("routeID")
	if routeID == "" {
		routeID = r.URL.Query().Get("routeId")
	}
	result, err := h.routes.Get(r.Context(), siteID(r), routeID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *RouteHandler) Update(w http.ResponseWriter, r *http.Request) {
	routeID := r.PathValue("routeID")
	if routeID == "" {
		routeID = r.URL.Query().Get("routeId")
	}
	var req createRouteRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.routes.Update(r.Context(), siteID(r), routeID, req.Matcher, req.Priority, req.Action)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *RouteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	routeID := r.PathValue("routeID")
	if routeID == "" {
		routeID = r.URL.Query().Get("routeId")
	}
	if err := h.routes.Delete(r.Context(), siteID(r), routeID); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type FormHandler struct{ forms *form.Service }

func NewFormHandler(forms *form.Service) *FormHandler {
	return &FormHandler{forms: forms}
}

type createFormRequest struct {
	Name       string         `json:"name"`
	Definition map[string]any `json:"definition"`
}

func (h *FormHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createFormRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.forms.Create(r.Context(), siteID(r), req.Name, req.Definition)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusCreated, result)
}

func (h *FormHandler) List(w http.ResponseWriter, r *http.Request) {
	result, err := h.forms.List(r.Context(), siteID(r))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *FormHandler) Get(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("formID")
	if formID == "" {
		formID = r.URL.Query().Get("formId")
	}
	result, err := h.forms.Get(r.Context(), siteID(r), formID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *FormHandler) Update(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("formID")
	if formID == "" {
		formID = r.URL.Query().Get("formId")
	}
	var req createFormRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.forms.Update(r.Context(), siteID(r), formID, req.Name, req.Definition)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *FormHandler) Delete(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("formID")
	if formID == "" {
		formID = r.URL.Query().Get("formId")
	}
	if err := h.forms.Delete(r.Context(), siteID(r), formID); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *FormHandler) ListSubmissions(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("formID")
	if formID == "" {
		formID = r.URL.Query().Get("formId")
	}
	result, err := h.forms.ListSubmissions(r.Context(), siteID(r), formID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

type SnapshotHandler struct{ snapshots *snapshot.Service }

func NewSnapshotHandler(snapshots *snapshot.Service) *SnapshotHandler {
	return &SnapshotHandler{snapshots: snapshots}
}

type createSnapshotRequest struct {
	Name string `json:"name"`
}

func (h *SnapshotHandler) Create(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	if siteID == "" {
		siteID = r.URL.Query().Get("siteId")
	}
	var req createSnapshotRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.snapshots.Create(r.Context(), siteID, req.Name)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusCreated, result)
}

func (h *SnapshotHandler) List(w http.ResponseWriter, r *http.Request) {
	siteID := r.PathValue("siteID")
	if siteID == "" {
		siteID = r.URL.Query().Get("siteId")
	}
	result, err := h.snapshots.ListBySite(r.Context(), siteID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *SnapshotHandler) Get(w http.ResponseWriter, r *http.Request) {
	snapshotID := r.PathValue("snapshotID")
	if snapshotID == "" {
		snapshotID = r.URL.Query().Get("snapshotId")
	}
	result, err := h.snapshots.Get(r.Context(), snapshotID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *SnapshotHandler) Delete(w http.ResponseWriter, r *http.Request) {
	snapshotID := r.PathValue("snapshotID")
	if snapshotID == "" {
		snapshotID = r.URL.Query().Get("snapshotId")
	}
	if err := h.snapshots.Delete(r.Context(), snapshotID); err != nil {
		httpapi.RespondError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func NotFound(w http.ResponseWriter, r *http.Request) {
	httpapi.RespondJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func injectLogger(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("admin request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start).String())
	})
}
