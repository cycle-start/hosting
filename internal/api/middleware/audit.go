package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

// AuditLogger is an async audit log writer.
type AuditLogger struct {
	pool   *pgxpool.Pool
	logger zerolog.Logger
	ch     chan auditEntry
}

type auditEntry struct {
	APIKeyID     *string
	Method       string
	Path         string
	ResourceType *string
	ResourceID   *string
	StatusCode   int
	RequestBody  json.RawMessage
}

func NewAuditLogger(pool *pgxpool.Pool, logger zerolog.Logger) *AuditLogger {
	al := &AuditLogger{
		pool:   pool,
		logger: logger,
		ch:     make(chan auditEntry, 1024),
	}
	go al.drain()
	return al
}

func (al *AuditLogger) drain() {
	for entry := range al.ch {
		_, err := al.pool.Exec(
			// use context.Background since this is async
			context.Background(),
			`INSERT INTO audit_logs (api_key_id, method, path, resource_type, resource_id, status_code, request_body, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, now())`,
			entry.APIKeyID, entry.Method, entry.Path, entry.ResourceType, entry.ResourceID, entry.StatusCode, entry.RequestBody,
		)
		if err != nil {
			al.logger.Error().Err(err).Msg("failed to write audit log")
		}
	}
}

// Close drains remaining entries and closes the channel.
func (al *AuditLogger) Close() {
	close(al.ch)
}

// Middleware returns a chi middleware that logs mutating API requests.
func (al *AuditLogger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only audit mutating operations.
		if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodDelete {
			next.ServeHTTP(w, r)
			return
		}

		// Read and re-buffer the request body.
		var bodyBytes []byte
		if r.Body != nil {
			bodyBytes, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// Wrap response writer to capture status code.
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		// Extract resource info from path.
		resourceType, resourceID := extractResource(r.URL.Path)

		// Get API key ID from context.
		var apiKeyID *string
		if id, ok := r.Context().Value(APIKeyIDKey).(string); ok {
			apiKeyID = &id
		}

		// Sanitize body - don't log passwords or keys.
		var sanitizedBody json.RawMessage
		if len(bodyBytes) > 0 && json.Valid(bodyBytes) {
			sanitizedBody = sanitizeBody(bodyBytes)
		}

		// Send to async writer.
		select {
		case al.ch <- auditEntry{
			APIKeyID:     apiKeyID,
			Method:       r.Method,
			Path:         r.URL.Path,
			ResourceType: resourceType,
			ResourceID:   resourceID,
			StatusCode:   sw.status,
			RequestBody:  sanitizedBody,
		}:
		default:
			al.logger.Warn().Msg("audit log buffer full, dropping entry")
		}
	})
}

func extractResource(path string) (*string, *string) {
	// Extract the last resource type and optional ID from the path.
	// e.g., /api/v1/tenants -> type=tenants
	//       /api/v1/tenants/abc -> type=tenants, id=abc
	//       /api/v1/tenants/abc/webroots -> type=webroots
	//       /api/v1/tenants/abc/webroots/def -> type=webroots, id=def
	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/"), "/")
	if len(parts) == 0 {
		return nil, nil
	}

	// Walk the parts: resource types are at even indices, IDs at odd indices
	var resourceType, resourceID *string
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i%2 == 0 {
			p := part
			resourceType = &p
			resourceID = nil
		} else {
			p := part
			resourceID = &p
		}
	}

	return resourceType, resourceID
}

// sensitiveFields are fields that should be redacted from audit logs.
var sensitiveFields = map[string]bool{
	"password": true, "key_pem": true, "cert_pem": true, "chain_pem": true,
	"api_key": true, "secret": true, "token": true,
}

func sanitizeBody(body []byte) json.RawMessage {
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return body
	}
	for k := range data {
		if sensitiveFields[k] {
			data[k] = "[REDACTED]"
		}
	}
	sanitized, _ := json.Marshal(data)
	return sanitized
}
