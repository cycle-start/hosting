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

// CreateLoginSession godoc
//
//	@Summary		Create an OIDC login session
//	@Description	Creates a short-lived login session for a tenant. The session ID is used as the login_hint in the OIDC authorize request, allowing passwordless authentication. Sessions expire after 5 minutes and can only be used once. This is how the hosting platform initiates OIDC-based login on behalf of a tenant.
//	@Tags			OIDC
//	@Security		ApiKeyAuth
//	@Param			id path string true "Tenant ID"
//	@Success		201 {object} map[string]any
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{id}/login-sessions [post]
func (h *OIDCLogin) CreateLoginSession(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.oidcSvc.EnsureSigningKey(r.Context()); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	session, err := h.oidcSvc.CreateLoginSession(r.Context(), id)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusCreated, map[string]any{
		"session_id": session.ID,
		"expires_at": session.ExpiresAt,
	})
}
