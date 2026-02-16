package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuth_MissingKey(t *testing.T) {
	// Auth checks the header before any DB lookup, so nil pool is safe here.
	handler := Auth(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/v1/tenants", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var body map[string]string
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	assert.NoError(t, err)
	assert.Equal(t, "missing API key", body["error"])
}

func TestExtractAPIKey(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"bearer token", "Bearer hst_abc123", "hst_abc123"},
		{"empty", "", ""},
		{"no prefix", "hst_abc123", ""},
		{"basic auth ignored", "Basic dXNlcjpwYXNz", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			assert.Equal(t, tt.want, extractAPIKey(req))
		})
	}
}

func TestHashConsistency(t *testing.T) {
	key := "test-api-key-12345"
	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])
	assert.Len(t, keyHash, 64) // SHA-256 = 64 hex chars
}
