package admin

import (
	"net/http"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
)

// SettingsHandler exposes server-side configuration to the admin Settings
// page. The admin token is masked so the full secret never leaves the server.
type SettingsHandler struct {
	adminToken            string
	defaultLocale         string
	redirectDefaultStatus int
}

func NewSettingsHandler(adminToken, defaultLocale string, redirectDefaultStatus int) *SettingsHandler {
	return &SettingsHandler{adminToken: adminToken, defaultLocale: defaultLocale, redirectDefaultStatus: redirectDefaultStatus}
}

type settingsResponse struct {
	AdminToken            string `json:"adminToken"`
	DefaultLocale         string `json:"defaultLocale"`
	RedirectDefaultStatus int    `json:"redirectDefaultStatus"`
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	httpapi.RespondJSON(w, http.StatusOK, settingsResponse{
		AdminToken:            maskToken(h.adminToken),
		DefaultLocale:         h.defaultLocale,
		RedirectDefaultStatus: h.redirectDefaultStatus,
	})
}

// maskToken reveals only the last four characters so an operator can
// recognise the configured value without exposing the secret ("" stays "").
func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 4 {
		return "····"
	}
	return "····" + token[len(token)-4:]
}
