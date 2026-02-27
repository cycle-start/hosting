package middleware

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/model"
)

const partnerKey contextKey = "partner"

// Partner returns middleware that resolves the partner from the request hostname
// and injects it into the context.
//
// Resolution order:
//  1. X-Dev-Partner header (dev mode only — allows switching partners locally)
//  2. Origin header (cross-origin requests in production)
//  3. Host header (same-origin / proxied requests in dev)
func Partner(partnerService *core.PartnerService, devMode bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var hostname string

			if devMode {
				hostname = r.Header.Get("X-Dev-Partner")
			}
			if hostname == "" {
				hostname = hostnameFromRequest(r)
			}

			if hostname == "" {
				response.WriteError(w, http.StatusBadRequest, "unable to determine hostname")
				return
			}

			partner, err := partnerService.GetByHostname(r.Context(), hostname)
			if err != nil {
				response.WriteError(w, http.StatusNotFound, "unknown partner")
				return
			}

			ctx := context.WithValue(r.Context(), partnerKey, partner)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetPartner extracts the resolved partner from the request context.
func GetPartner(ctx context.Context) *model.Partner {
	partner, _ := ctx.Value(partnerKey).(*model.Partner)
	return partner
}

// hostnameFromRequest extracts the bare hostname (no port) from
// the Origin header first, then falls back to the Host header.
func hostnameFromRequest(r *http.Request) string {
	if origin := r.Header.Get("Origin"); origin != "" {
		if u, err := url.Parse(origin); err == nil && u.Hostname() != "" {
			return u.Hostname()
		}
	}

	host := r.Host
	if host == "" {
		host = r.Header.Get("Host")
	}

	// Strip port
	if i := strings.LastIndex(host, ":"); i != -1 {
		return host[:i]
	}
	return host
}
