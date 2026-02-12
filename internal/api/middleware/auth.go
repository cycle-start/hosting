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

			var id string
			err := pool.QueryRow(r.Context(),
				`SELECT id FROM api_keys WHERE key_hash = $1 AND revoked_at IS NULL`, keyHash,
			).Scan(&id)
			if err != nil {
				response.WriteError(w, http.StatusUnauthorized, "invalid API key")
				return
			}

			ctx := context.WithValue(r.Context(), APIKeyIDKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
