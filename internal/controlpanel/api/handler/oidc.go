package handler

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"

	"github.com/edvin/hosting/internal/controlpanel/api/middleware"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
)

type OIDC struct {
	svc *core.OIDCService
}

func NewOIDC(svc *core.OIDCService) *OIDC {
	return &OIDC{svc: svc}
}

// ListProviders returns the list of enabled OIDC providers.
func (h *OIDC) ListProviders(w http.ResponseWriter, r *http.Request) {
	providers := h.svc.Providers()
	response.WriteJSON(w, http.StatusOK, map[string]any{"items": providers})
}

// Authorize redirects the user to the OIDC provider for login.
func (h *OIDC) Authorize(w http.ResponseWriter, r *http.Request) {
	partner := middleware.GetPartner(r.Context())
	if partner == nil {
		response.WriteError(w, http.StatusBadRequest, "unable to resolve partner")
		return
	}

	providerID := r.URL.Query().Get("provider")
	provider := h.svc.GetProvider(providerID)
	if provider == nil {
		response.WriteError(w, http.StatusBadRequest, "unknown provider")
		return
	}

	callbackURL := callbackURLFromRequest(r)
	redirectURL, err := h.svc.AuthorizeURL(provider, partner.ID, "login", "", callbackURL)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to build authorize URL")
		return
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// Callback handles the OIDC provider callback for both login and connect flows.
func (h *OIDC) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		oidcError := r.URL.Query().Get("error")
		if oidcError != "" {
			http.Redirect(w, r, "/?oidc_error="+url.QueryEscape(oidcError), http.StatusFound)
			return
		}
		response.WriteError(w, http.StatusBadRequest, "missing code or state")
		return
	}

	callbackURL := callbackURLFromRequest(r)
	result, err := h.svc.HandleCallback(code, state, callbackURL)
	if err != nil {
		http.Redirect(w, r, "/?oidc_error=invalid_callback", http.StatusFound)
		return
	}

	switch result.Mode {
	case "login":
		token, _, err := h.svc.LoginByOIDC(r.Context(), result.PartnerID, result.Provider, result.Subject)
		if err != nil {
			http.Redirect(w, r, "/?oidc_error=no_account", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/?token="+url.QueryEscape(token), http.StatusFound)

	case "connect":
		err := h.svc.Connect(r.Context(), result.UserID, result.PartnerID, result.Provider, result.Subject, result.Email)
		if err != nil {
			http.Redirect(w, r, "/profile?oidc_error="+url.QueryEscape("connection failed"), http.StatusFound)
			return
		}
		http.Redirect(w, r, "/profile?oidc=connected&provider="+url.QueryEscape(result.Provider), http.StatusFound)

	default:
		response.WriteError(w, http.StatusBadRequest, "invalid state mode")
	}
}

// AuthorizeConnect returns a redirect URL for connecting an OIDC provider (authenticated).
func (h *OIDC) AuthorizeConnect(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		response.WriteError(w, http.StatusUnauthorized, "missing claims")
		return
	}

	providerID := r.URL.Query().Get("provider")
	provider := h.svc.GetProvider(providerID)
	if provider == nil {
		response.WriteError(w, http.StatusBadRequest, "unknown provider")
		return
	}

	callbackURL := callbackURLFromRequest(r)
	redirectURL, err := h.svc.AuthorizeURL(provider, claims.PartnerID, "connect", claims.Sub, callbackURL)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to build authorize URL")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]string{"redirect_url": redirectURL})
}

// ListConnections returns the user's OIDC connections.
func (h *OIDC) ListConnections(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		response.WriteError(w, http.StatusUnauthorized, "missing claims")
		return
	}

	conns, err := h.svc.ListConnections(r.Context(), claims.Sub)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if conns == nil {
		conns = []core.OIDCConnection{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": conns})
}

// Disconnect removes an OIDC connection for the current user.
func (h *OIDC) Disconnect(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		response.WriteError(w, http.StatusUnauthorized, "missing claims")
		return
	}

	provider := chi.URLParam(r, "provider")
	if provider == "" {
		response.WriteError(w, http.StatusBadRequest, "missing provider")
		return
	}

	if err := h.svc.Disconnect(r.Context(), claims.Sub, provider); err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// callbackURLFromRequest builds the callback URL from the current request.
func callbackURLFromRequest(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil {
		if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
			scheme = fwd
		} else {
			scheme = "http"
		}
	}
	return fmt.Sprintf("%s://%s/auth/oidc/callback", scheme, r.Host)
}
