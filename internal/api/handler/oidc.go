package handler

import (
	"net/http"
	"net/url"

	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
)

type OIDC struct {
	svc *core.OIDCService
}

func NewOIDC(svc *core.OIDCService) *OIDC {
	return &OIDC{svc: svc}
}

// Discovery godoc
//
//	@Summary		OpenID Connect discovery
//	@Description	Returns the OpenID Connect discovery document (RFC 8414). Contains issuer URL, authorization/token endpoints, JWKS URI, and supported scopes. No authentication required.
//	@Tags			OIDC
//	@Success		200 {object} map[string]any
//	@Router			/.well-known/openid-configuration [get]
func (h *OIDC) Discovery(w http.ResponseWriter, _ *http.Request) {
	response.WriteJSON(w, http.StatusOK, h.svc.Discovery())
}

// JWKS godoc
//
//	@Summary		JSON Web Key Set
//	@Description	Returns the JWKS (JSON Web Key Set) containing the public keys used to verify ID tokens. Clients use this to validate token signatures. The signing key is auto-generated on first access. No authentication required.
//	@Tags			OIDC
//	@Success		200 {object} map[string]any
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/oidc/jwks [get]
func (h *OIDC) JWKS(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.EnsureSigningKey(r.Context()); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jwks, err := h.svc.JWKS()
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jwks)
}

// Authorize godoc
//
//	@Summary		OIDC authorization endpoint
//	@Description	Handles the OIDC authorization code flow. Validates the login_hint (a short-lived login session ID created via the API), creates an authorization code, and redirects to the client's redirect_uri with the code. The login_hint is consumed on use. No authentication required — the login session acts as proof of identity.
//	@Tags			OIDC
//	@Param			client_id query string true "OIDC client ID"
//	@Param			redirect_uri query string true "Client redirect URI (must match registered URI)"
//	@Param			login_hint query string true "Login session ID (from POST /tenants/{id}/login-sessions)"
//	@Param			scope query string false "Requested scopes" default(openid)
//	@Param			state query string false "CSRF state parameter (returned in redirect)"
//	@Param			nonce query string false "Nonce for ID token replay protection"
//	@Success		302
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		401 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/oidc/authorize [get]
func (h *OIDC) Authorize(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.EnsureSigningKey(r.Context()); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	q := r.URL.Query()
	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	scope := q.Get("scope")
	state := q.Get("state")
	nonce := q.Get("nonce")
	loginHint := q.Get("login_hint")

	if clientID == "" || redirectURI == "" || loginHint == "" {
		response.WriteError(w, http.StatusBadRequest, "missing required parameters: client_id, redirect_uri, login_hint")
		return
	}

	if scope == "" {
		scope = "openid"
	}

	// Validate login session.
	session, err := h.svc.ValidateLoginSession(r.Context(), loginHint)
	if err != nil {
		response.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Create auth code.
	code, err := h.svc.CreateAuthCode(r.Context(), clientID, session.TenantID, redirectURI, scope, nonce)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Redirect back with code.
	u, err := url.Parse(redirectURI)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid redirect_uri")
		return
	}
	params := u.Query()
	params.Set("code", code)
	if state != "" {
		params.Set("state", state)
	}
	u.RawQuery = params.Encode()

	http.Redirect(w, r, u.String(), http.StatusFound)
}

// Token godoc
//
//	@Summary		OIDC token endpoint
//	@Description	Exchanges an authorization code for an ID token. Supports only the authorization_code grant type. Validates the client credentials and returns a signed JWT ID token containing the tenant ID as the subject claim. The authorization code is consumed on use.
//	@Tags			OIDC
//	@Accept			application/x-www-form-urlencoded
//	@Param			grant_type formData string true "Must be 'authorization_code'"
//	@Param			code formData string true "Authorization code from the authorize redirect"
//	@Param			client_id formData string true "OIDC client ID"
//	@Param			client_secret formData string true "OIDC client secret"
//	@Param			redirect_uri formData string false "Must match the URI used in the authorize request"
//	@Success		200 {object} map[string]string
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		401 {object} response.ErrorResponse
//	@Router			/oidc/token [post]
func (h *OIDC) Token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid form data")
		return
	}

	grantType := r.FormValue("grant_type")
	if grantType != "authorization_code" {
		response.WriteError(w, http.StatusBadRequest, "unsupported grant_type")
		return
	}

	code := r.FormValue("code")
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	redirectURI := r.FormValue("redirect_uri")

	if code == "" || clientID == "" || clientSecret == "" {
		response.WriteError(w, http.StatusBadRequest, "missing required parameters")
		return
	}

	idToken, err := h.svc.ExchangeCode(r.Context(), code, clientID, clientSecret, redirectURI)
	if err != nil {
		response.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"access_token":"` + idToken + `","token_type":"Bearer","id_token":"` + idToken + `"}`))
}
