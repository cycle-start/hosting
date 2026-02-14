package middleware

import (
	"net/http"

	"github.com/edvin/hosting/internal/core"
)

// CallbackURL is a middleware that extracts the X-Callback-URL header
// and injects it into the request context for downstream services.
func CallbackURL(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if url := r.Header.Get("X-Callback-URL"); url != "" {
			r = r.WithContext(core.WithCallbackURL(r.Context(), url))
		}
		next.ServeHTTP(w, r)
	})
}
