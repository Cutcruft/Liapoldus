package client

import (
	"errors"
	"net/http"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

type ContentHandler struct{ app *App }

func NewContentHandler(a *App) *ContentHandler { return &ContentHandler{app: a} }

func (h *ContentHandler) Get(w http.ResponseWriter, r *http.Request) {
	contentID := r.PathValue("contentID")
	if contentID == "" {
		contentID = r.URL.Query().Get("contentId")
	}
	site, err := h.app.resolveSite(r)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	locale := r.URL.Query().Get("locale")
	result, err := h.app.Contents.GetMerged(r.Context(), site.ID, contentID, locale)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *ContentHandler) List(w http.ResponseWriter, r *http.Request) {
	site, err := h.app.resolveSite(r)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	collectionID := r.URL.Query().Get("collectionId")
	locale := r.URL.Query().Get("locale")
	all, err := h.app.Contents.List(r.Context(), site.ID, collectionID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	locale = normalizeLocale(locale, site.DefaultLocale)
	result := make([]domain.ContentData, 0, len(all))
	for _, content := range all {
		merged, err := h.app.Contents.GetMerged(r.Context(), site.ID, content.ID, locale)
		if err != nil {
			httpapi.RespondError(w, err)
			return
		}
		if collectionID == "" || merged.CollectionID == collectionID {
			result = append(result, merged)
		}
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

type batchRequest struct {
	IDs []string `json:"ids"`
}

func (h *ContentHandler) Batch(w http.ResponseWriter, r *http.Request) {
	site, err := h.app.resolveSite(r)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	var req batchRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	locale := r.URL.Query().Get("locale")
	if locale == "" {
		locale = site.DefaultLocale
	}
	result, err := h.app.Contents.Batch(r.Context(), site.ID, req.IDs, locale)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func normalizeLocale(locale, fallback string) string {
	if locale == "" {
		return fallback
	}
	return locale
}

type AssetHandler struct{ app *App }

func NewAssetHandler(a *App) *AssetHandler { return &AssetHandler{app: a} }

func (h *AssetHandler) Get(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("assetID")
	if assetID == "" {
		assetID = r.URL.Query().Get("assetId")
	}
	asset, err := h.app.Assets.Get(r.Context(), assetID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, h.app.Assets.Metadata(asset))
}

func (h *AssetHandler) List(w http.ResponseWriter, r *http.Request) {
	site, err := h.app.resolveSite(r)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	assets, err := h.app.Assets.List(r.Context(), site.ID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, h.app.Assets.MetadataList(assets))
}

func (h *AssetHandler) File(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("assetID")
	if assetID == "" {
		assetID = r.URL.Query().Get("assetId")
	}
	if variant := r.URL.Query().Get("variant"); variant != "" && !h.app.Assets.IsMaster(variant) {
		httpapi.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown variant"})
		return
	}
	asset, reader, err := h.app.Assets.Open(r.Context(), assetID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	serveAssetBytes(w, r, asset, reader, h.app.AssetCacheMaxAgeSeconds)
}

type FormHandler struct{ app *App }

func NewFormHandler(a *App) *FormHandler { return &FormHandler{app: a} }

func (h *FormHandler) Get(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("formID")
	if formID == "" {
		formID = r.URL.Query().Get("formId")
	}
	site, err := h.app.resolveSite(r)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	form, err := h.app.Forms.Get(r.Context(), site.ID, formID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, form.Definition)
}

type submitRequest struct {
	FormID      string         `json:"formId"`
	Locale      string         `json:"locale,omitempty"`
	SubmittedAt string         `json:"submittedAt,omitempty"`
	Values      map[string]any `json:"values"`
}

func (h *FormHandler) Submit(w http.ResponseWriter, r *http.Request) {
	formID := r.PathValue("formID")
	if formID == "" {
		formID = r.URL.Query().Get("formId")
	}
	site, err := h.app.resolveSite(r)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	var req submitRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	if req.FormID != "" && req.FormID != formID {
		httpapi.RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "formId mismatch"})
		return
	}
	payload := map[string]any{
		"formId":      formID,
		"locale":      req.Locale,
		"submittedAt": req.SubmittedAt,
		"values":      req.Values,
	}
	sub, err := h.app.Forms.Submit(r.Context(), site.ID, formID, payload)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			httpapi.RespondJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusCreated, map[string]string{"submissionId": sub.ID, "status": "ok"})
}

type ContractHandler struct{ app *App }

func NewContractHandler(a *App) *ContractHandler { return &ContractHandler{app: a} }

func (h *ContractHandler) Contract(w http.ResponseWriter, r *http.Request) {
	site, err := h.app.resolveSite(r)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	routes, err := h.app.Routes.List(r.Context(), site.ID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	forms, err := h.app.Forms.List(r.Context(), site.ID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	formDescriptors := make([]map[string]any, 0, len(forms))
	for _, form := range forms {
		formDescriptors = append(formDescriptors, form.Definition)
	}
	httpapi.RespondJSON(w, http.StatusOK, map[string]any{
		"siteId":        site.ID,
		"defaultLocale": site.DefaultLocale,
		"routes":        routes,
		"forms":         formDescriptors,
		"operations":    []any{},
		"endpoints":     []any{},
		"environments":  []any{},
		"theme":         nil,
	})
}

func (h *ContractHandler) Routes(w http.ResponseWriter, r *http.Request) {
	site, err := h.app.resolveSite(r)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	routes, err := h.app.Routes.List(r.Context(), site.ID)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, map[string]any{
		"routes": routes,
	})
}
