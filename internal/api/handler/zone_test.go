package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newZoneHandler() *Zone {
	return &Zone{svc: nil, services: nil}
}

// --- Create ---

func TestZoneCreate_InvalidJSON(t *testing.T) {
	h := newZoneHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/zones", "{bad json")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestZoneCreate_EmptyBody(t *testing.T) {
	h := newZoneHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/zones", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestZoneCreate_MissingRequiredFields(t *testing.T) {
	h := newZoneHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/zones", map[string]any{})

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestZoneCreate_MissingName(t *testing.T) {
	h := newZoneHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/zones", map[string]any{
		"region_id": validID,
	})

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestZoneCreate_MissingRegionID(t *testing.T) {
	h := newZoneHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/zones", map[string]any{
		"name": "example.com",
	})

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestZoneCreate_ValidBody(t *testing.T) {
	h := newZoneHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/zones", map[string]any{
		"name":            "example.com",
		"brand_id":        "test-brand",
		"subscription_id": "sub-1",
		"region_id":       "test-region-1",
	})

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestZoneCreate_ValidBodyWithTenantID(t *testing.T) {
	h := newZoneHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/zones", map[string]any{
		"name":            "example.com",
		"subscription_id": "sub-1",
		"region_id":       "test-region-1",
		"tenant_id":       "test-tenant-1",
	})

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestZoneCreate_OptionalTenantID(t *testing.T) {
	h := newZoneHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/zones", map[string]any{
		"name":            "example.com",
		"brand_id":        "test-brand",
		"subscription_id": "sub-1",
		"region_id":       "test-region-2",
	})

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- Get ---

func TestZoneGet_EmptyID(t *testing.T) {
	h := newZoneHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/zones/", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Update ---

func TestZoneUpdate_EmptyID(t *testing.T) {
	h := newZoneHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/zones/", map[string]any{
		"tenant_id": validID,
	})
	r = withChiURLParam(r, "id", "")

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestZoneUpdate_InvalidJSON(t *testing.T) {
	h := newZoneHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/zones/"+validID, "{bad json")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestZoneUpdate_EmptyBody(t *testing.T) {
	h := newZoneHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/zones/"+validID, "")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Delete ---

func TestZoneDelete_EmptyID(t *testing.T) {
	h := newZoneHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/zones/", nil)
	r = withChiURLParam(r, "id", "")

	h.Delete(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Error response format ---

func TestZoneCreate_ErrorResponseFormat(t *testing.T) {
	h := newZoneHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/zones", "{bad")

	h.Create(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	_, hasError := body["error"]
	assert.True(t, hasError)
}
