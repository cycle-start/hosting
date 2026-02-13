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

// Discovery serves the OpenID Connect discovery document.
func (h *OIDC) Discovery(w http.ResponseWriter, _ *http.Request) {
	response.WriteJSON(w, http.StatusOK, h.svc.Discovery())
}

// JWKS serves the JSON Web Key Set.
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

// Authorize handles the OIDC authorization endpoint.
// It validates the login_hint (login session ID), creates an auth code, and redirects.
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

// Token handles the OIDC token endpoint.
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
