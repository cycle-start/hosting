package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/model"
)

type contextKey string

const claimsKey contextKey = "claims"

// Auth returns middleware that validates JWT Bearer tokens and injects claims into context.
func Auth(authService *core.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.WriteError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				response.WriteError(w, http.StatusUnauthorized, "invalid authorization format")
				return
			}

			claims, err := authService.ValidateToken(token)
			if err != nil {
				response.WriteError(w, http.StatusUnauthorized, err.Error())
				return
			}

			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetClaims extracts JWT claims from the request context.
func GetClaims(ctx context.Context) *model.JWTClaims {
	claims, _ := ctx.Value(claimsKey).(*model.JWTClaims)
	return claims
}
