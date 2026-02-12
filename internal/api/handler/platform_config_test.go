package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPlatformConfigHandler() *PlatformConfig {
	return NewPlatformConfig(nil)
}

// --- Update ---

func TestPlatformConfigUpdate_InvalidJSON(t *testing.T) {
	h := newPlatformConfigHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/platform/config", "{bad json")

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Equal(t, "invalid JSON", body["error"])
}

func TestPlatformConfigUpdate_EmptyBody(t *testing.T) {
	h := newPlatformConfigHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/platform/config", "")

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Equal(t, "invalid JSON", body["error"])
}

func TestPlatformConfigUpdate_NotAStringMap(t *testing.T) {
	// The handler uses json.Decoder to decode into map[string]string.
	// Sending a non-string value should fail decoding.
	h := newPlatformConfigHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/platform/config", `{"key": 123}`)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Equal(t, "invalid JSON", body["error"])
}

func TestPlatformConfigUpdate_ArrayBody(t *testing.T) {
	h := newPlatformConfigHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/platform/config", `["not", "a", "map"]`)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Equal(t, "invalid JSON", body["error"])
}

func TestPlatformConfigUpdate_NestedObjectValue(t *testing.T) {
	h := newPlatformConfigHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/platform/config", `{"key": {"nested": "value"}}`)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Equal(t, "invalid JSON", body["error"])
}

func TestPlatformConfigUpdate_NullBody(t *testing.T) {
	h := newPlatformConfigHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/platform/config", "null")

	// null decodes to nil map, which is fine for JSON decoding -- the for loop
	// just won't execute. But then GetAll is called on the nil service, which
	// panics. We verify the handler doesn't crash before the DB call by
	// recovering from the expected panic.
	assert.Panics(t, func() {
		h.Update(rec, r)
	})
}

// --- Error response format ---

func TestPlatformConfigUpdate_ErrorResponseFormat(t *testing.T) {
	h := newPlatformConfigHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/platform/config", "{bad")

	h.Update(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	_, hasError := body["error"]
	assert.True(t, hasError)
	assert.Equal(t, "invalid JSON", body["error"])
}

// --- JSON Content-Type ---

func TestPlatformConfigUpdate_JSONContentType(t *testing.T) {
	h := newPlatformConfigHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/platform/config", "not json")

	h.Update(rec, r)

	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestPlatformConfigUpdate_BooleanValue(t *testing.T) {
	h := newPlatformConfigHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/platform/config", `{"key": true}`)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Equal(t, "invalid JSON", body["error"])
}
