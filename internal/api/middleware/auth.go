package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/edvin/hosting/internal/api/response"
)

type contextKey string

const APIKeyIdentityKey contextKey = "api_key_identity"

// APIKeyIdentity holds the authenticated key's ID, scopes, and brand access.
type APIKeyIdentity struct {
	ID     string
	Scopes []string
	Brands []string
}

// APIKeyIDKey is kept for backward compatibility (audit logger).
const APIKeyIDKey contextKey = "api_key_id"

// Auth returns a middleware that validates the X-API-Key header against the api_keys table.
func Auth(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-Key")
			if key == "" {
				response.WriteError(w, http.StatusUnauthorized, "missing API key")
				return
			}

			hash := sha256.Sum256([]byte(key))
			keyHash := hex.EncodeToString(hash[:])

			var identity APIKeyIdentity
			err := pool.QueryRow(r.Context(),
				`SELECT id, scopes, brands FROM api_keys WHERE key_hash = $1 AND revoked_at IS NULL`, keyHash,
			).Scan(&identity.ID, &identity.Scopes, &identity.Brands)
			if err != nil {
				response.WriteError(w, http.StatusUnauthorized, "invalid API key")
				return
			}

			ctx := context.WithValue(r.Context(), APIKeyIdentityKey, &identity)
			ctx = context.WithValue(ctx, APIKeyIDKey, identity.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
