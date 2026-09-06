package admin

import (
	"net/http"
	"strings"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
)

// AuthHandler validates admin tokens against the server-configured secret.
// It is mounted on the public (pre-BearerAuth) mux so the login page can
// authenticate before possessing a valid token. When no token is configured
// the server runs open and every non-empty provider accepts.
type AuthHandler struct {
	adminToken string
}

func NewAuthHandler(adminToken string) *AuthHandler {
	return &AuthHandler{adminToken: adminToken}
}

type authValidateRequest struct {
	Token string `json:"token"`
}

func (h *AuthHandler) Validate(w http.ResponseWriter, r *http.Request) {
	var req authValidateRequest
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	if h.adminToken != "" && strings.TrimSpace(req.Token) != h.adminToken {
		httpapi.RespondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, map[string]bool{"valid": true})
}
