package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRegionHandler() *Region {
	return NewRegion(nil)
}

// --- Create ---

func TestRegionCreate_InvalidJSON(t *testing.T) {
	h := newRegionHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/regions", "{bad json")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestRegionCreate_EmptyBody(t *testing.T) {
	h := newRegionHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/regions", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRegionCreate_MissingRequiredFields(t *testing.T) {
	h := newRegionHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/regions", map[string]any{})

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestRegionCreate_MissingName(t *testing.T) {
	h := newRegionHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/regions", map[string]any{})

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestRegionCreate_InvalidSlugName(t *testing.T) {
	tests := []struct {
		name string
		slug string
	}{
		{"uppercase", "US-East"},
		{"spaces", "us east"},
		{"special chars", "us@east"},
		{"starts with digit", "1region"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newRegionHandler()
			rec := httptest.NewRecorder()
			r := newRequest(http.MethodPost, "/regions", map[string]any{
				"name": tt.slug,
			})

			h.Create(rec, r)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestRegionCreate_ValidBody(t *testing.T) {
	h := newRegionHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/regions", map[string]any{
		"name": "us-east-1",
	})

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestRegionCreate_ValidBodyWithConfig(t *testing.T) {
	h := newRegionHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/regions", map[string]any{
		"name":   "us-east-1",
		"config": map[string]any{"dns_provider": "cloudflare"},
	})

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestRegionCreate_OptionalConfig(t *testing.T) {
	h := newRegionHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/regions", map[string]any{
		"name": "us-east-2",
	})

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- Get ---

func TestRegionGet_EmptyID(t *testing.T) {
	h := newRegionHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/regions/", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Update ---

func TestRegionUpdate_EmptyID(t *testing.T) {
	h := newRegionHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/regions/", map[string]any{
		"name": "us-west-1",
	})
	r = withChiURLParam(r, "id", "")

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestRegionUpdate_InvalidJSON(t *testing.T) {
	h := newRegionHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/regions/"+validID, "{bad json")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestRegionUpdate_EmptyBody(t *testing.T) {
	h := newRegionHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/regions/"+validID, "")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Delete ---

func TestRegionDelete_EmptyID(t *testing.T) {
	h := newRegionHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/regions/", nil)
	r = withChiURLParam(r, "id", "")

	h.Delete(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Error response format ---

func TestRegionCreate_ErrorResponseFormat(t *testing.T) {
	h := newRegionHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/regions", "{bad")

	h.Create(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	_, hasError := body["error"]
	assert.True(t, hasError)
}
