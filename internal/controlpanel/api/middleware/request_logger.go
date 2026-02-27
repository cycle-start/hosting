package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

// RequestLogger returns a middleware that logs each request.
func RequestLogger(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			reqID := middleware.GetReqID(r.Context())
			reqLogger := logger.With().Str("request_id", reqID).Logger()
			r = r.WithContext(reqLogger.WithContext(r.Context()))

			ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(ww, r)

			reqLogger.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.status).
				Dur("duration", time.Since(start)).
				Msg("request")
		})
	}
}
