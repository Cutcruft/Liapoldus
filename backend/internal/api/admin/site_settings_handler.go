package admin

import (
	"net/http"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
	"github.com/liapoldus/liapoldus/backend/internal/application/sitesettings"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// SiteSettingsHandler owns presentation defaults for one site. It is separate
// from SettingsHandler, which exposes read-only server process configuration.
type SiteSettingsHandler struct{ settings *sitesettings.Service }

func NewSiteSettingsHandler(settings *sitesettings.Service) *SiteSettingsHandler {
	return &SiteSettingsHandler{settings: settings}
}

func (h *SiteSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	result, err := h.settings.Get(r.Context(), siteID(r))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *SiteSettingsHandler) Put(w http.ResponseWriter, r *http.Request) {
	var req domain.SiteSettings
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.settings.Update(r.Context(), siteID(r), &req)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}
