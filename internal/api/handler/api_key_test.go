package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newAPIKeyHandler() *APIKey {
	return NewAPIKey(nil)
}

// --- Create ---

func TestAPIKeyCreate_InvalidJSON(t *testing.T) {
	h := newAPIKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/api-keys", "{bad json")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestAPIKeyCreate_EmptyBody(t *testing.T) {
	h := newAPIKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/api-keys", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPIKeyCreate_MissingName(t *testing.T) {
	h := newAPIKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/api-keys", map[string]any{})

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestAPIKeyCreate_MissingRequiredFields(t *testing.T) {
	h := newAPIKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/api-keys", map[string]any{
		"scopes": []string{"read"},
	})

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

// --- Get ---

func TestAPIKeyGet_EmptyID(t *testing.T) {
	h := newAPIKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/api-keys/", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Revoke ---

func TestAPIKeyRevoke_EmptyID(t *testing.T) {
	h := newAPIKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/api-keys/", nil)
	r = withChiURLParam(r, "id", "")

	h.Revoke(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Error response format ---

func TestAPIKeyCreate_ErrorResponseFormat(t *testing.T) {
	h := newAPIKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/api-keys", "{bad")

	h.Create(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	_, hasError := body["error"]
	assert.True(t, hasError)
}
