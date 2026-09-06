package admin

import (
	"net/http"

	httpapi "github.com/liapoldus/liapoldus/backend/internal/api/http"
	"github.com/liapoldus/liapoldus/backend/internal/application/token"
	"github.com/liapoldus/liapoldus/backend/internal/domain"
)

// TokensHandler exposes the per-site design-token set. The editor always reads
// and writes the whole set (GET returns the empty set when none is stored).
type TokensHandler struct {
	tokens *token.Service
}

func NewTokensHandler(tokens *token.Service) *TokensHandler {
	return &TokensHandler{tokens: tokens}
}

func (h *TokensHandler) Get(w http.ResponseWriter, r *http.Request) {
	result, err := h.tokens.Get(r.Context(), siteID(r))
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}

func (h *TokensHandler) Put(w http.ResponseWriter, r *http.Request) {
	var req domain.TokenSet
	if !httpapi.DecodeJSON(r, &req, w) {
		return
	}
	result, err := h.tokens.Update(r.Context(), siteID(r), &req)
	if err != nil {
		httpapi.RespondError(w, err)
		return
	}
	httpapi.RespondJSON(w, http.StatusOK, result)
}
