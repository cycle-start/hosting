package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
)

type OIDCLogin struct {
	oidcSvc *core.OIDCService
}

func NewOIDCLogin(oidcSvc *core.OIDCService) *OIDCLogin {
	return &OIDCLogin{oidcSvc: oidcSvc}
}

// CreateLoginSession creates a short-lived OIDC login session for a tenant.
func (h *OIDCLogin) CreateLoginSession(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.oidcSvc.EnsureSigningKey(r.Context()); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	session, err := h.oidcSvc.CreateLoginSession(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusCreated, map[string]any{
		"session_id": session.ID,
		"expires_at": session.ExpiresAt,
	})
}
