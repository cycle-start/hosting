package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newZoneRecordHandler() *ZoneRecord {
	return NewZoneRecord(nil)
}

// --- ListByZone ---

func TestZoneRecordListByZone_EmptyID(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/zones//records", nil)
	r = withChiURLParam(r, "zoneID", "")

	h.ListByZone(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Create ---

func TestZoneRecordCreate_EmptyZoneID(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/zones//records", map[string]any{
		"type":    "A",
		"name":    "@",
		"content": "1.2.3.4",
	})
	r = withChiURLParam(r, "zoneID", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestZoneRecordCreate_InvalidJSON(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/zones/"+validID+"/records", "{bad json")
	r = withChiURLParam(r, "zoneID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestZoneRecordCreate_EmptyBody(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/zones/"+validID+"/records", "")
	r = withChiURLParam(r, "zoneID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestZoneRecordCreate_MissingRequiredFields(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/zones/"+validID+"/records", map[string]any{})
	r = withChiURLParam(r, "zoneID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestZoneRecordCreate_MissingType(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/zones/"+validID+"/records", map[string]any{
		"name":    "@",
		"content": "1.2.3.4",
	})
	r = withChiURLParam(r, "zoneID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestZoneRecordCreate_MissingName(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/zones/"+validID+"/records", map[string]any{
		"type":    "A",
		"content": "1.2.3.4",
	})
	r = withChiURLParam(r, "zoneID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestZoneRecordCreate_MissingContent(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/zones/"+validID+"/records", map[string]any{
		"type": "A",
		"name": "@",
	})
	r = withChiURLParam(r, "zoneID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestZoneRecordCreate_InvalidRecordType(t *testing.T) {
	invalidTypes := []string{"INVALID", "X", "http", "a", "mx", ""}
	for _, rt := range invalidTypes {
		t.Run(rt, func(t *testing.T) {
			h := newZoneRecordHandler()
			rec := httptest.NewRecorder()
			r := newRequest(http.MethodPost, "/zones/"+validID+"/records", map[string]any{
				"type":    rt,
				"name":    "@",
				"content": "1.2.3.4",
			})
			r = withChiURLParam(r, "zoneID", validID)

			h.Create(rec, r)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestZoneRecordCreate_ValidRecordTypes(t *testing.T) {
	validTypes := []string{"A", "AAAA", "CNAME", "MX", "TXT", "SRV", "NS", "CAA", "PTR"}
	for _, rt := range validTypes {
		t.Run(rt, func(t *testing.T) {
			h := newZoneRecordHandler()
			rec := httptest.NewRecorder()
			zid := "test-zone-1"
			r := newRequest(http.MethodPost, "/zones/"+zid+"/records", map[string]any{
				"type":    rt,
				"name":    "@",
				"content": "1.2.3.4",
			})
			r = withChiURLParam(r, "zoneID", zid)

			func() {
				defer func() { recover() }()
				h.Create(rec, r)
			}()

			assert.NotEqual(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestZoneRecordCreate_TTLTooLow(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/zones/"+validID+"/records", map[string]any{
		"type":    "A",
		"name":    "@",
		"content": "1.2.3.4",
		"ttl":     10, // min is 60
	})
	r = withChiURLParam(r, "zoneID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestZoneRecordCreate_TTLTooHigh(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/zones/"+validID+"/records", map[string]any{
		"type":    "A",
		"name":    "@",
		"content": "1.2.3.4",
		"ttl":     100000, // max is 86400
	})
	r = withChiURLParam(r, "zoneID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestZoneRecordCreate_ValidTTLBoundaries(t *testing.T) {
	tests := []struct {
		name string
		ttl  int
	}{
		{"minimum TTL", 60},
		{"maximum TTL", 86400},
		{"middle TTL", 3600},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newZoneRecordHandler()
			rec := httptest.NewRecorder()
			zid := "test-zone-2"
			r := newRequest(http.MethodPost, "/zones/"+zid+"/records", map[string]any{
				"type":    "A",
				"name":    "@",
				"content": "1.2.3.4",
				"ttl":     tt.ttl,
			})
			r = withChiURLParam(r, "zoneID", zid)

			func() {
				defer func() { recover() }()
				h.Create(rec, r)
			}()

			assert.NotEqual(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestZoneRecordCreate_OptionalTTLDefaults(t *testing.T) {
	// TTL is optional; if not provided, should default to 0 in the request struct
	// and the handler will set it to 3600
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	zid := "test-zone-3"
	r := newRequest(http.MethodPost, "/zones/"+zid+"/records", map[string]any{
		"type":    "A",
		"name":    "@",
		"content": "1.2.3.4",
	})
	r = withChiURLParam(r, "zoneID", zid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestZoneRecordCreate_WithPriority(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	zid := "test-zone-4"
	r := newRequest(http.MethodPost, "/zones/"+zid+"/records", map[string]any{
		"type":     "MX",
		"name":     "@",
		"content":  "mail.example.com",
		"priority": 10,
	})
	r = withChiURLParam(r, "zoneID", zid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- Get ---

func TestZoneRecordGet_EmptyID(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/zone-records/", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Update ---

func TestZoneRecordUpdate_EmptyID(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/zone-records/", map[string]any{
		"content": "5.6.7.8",
	})
	r = withChiURLParam(r, "id", "")

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestZoneRecordUpdate_InvalidJSON(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/zone-records/"+validID, "{bad json")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestZoneRecordUpdate_EmptyBody(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/zone-records/"+validID, "")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestZoneRecordUpdate_TTLTooLow(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/zone-records/"+validID, map[string]any{
		"ttl": 10,
	})
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestZoneRecordUpdate_TTLTooHigh(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/zone-records/"+validID, map[string]any{
		"ttl": 100000,
	})
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

// --- Delete ---

func TestZoneRecordDelete_EmptyID(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/zone-records/", nil)
	r = withChiURLParam(r, "id", "")

	h.Delete(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Error response format ---

func TestZoneRecordCreate_ErrorResponseFormat(t *testing.T) {
	h := newZoneRecordHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/zones/"+validID+"/records", "{bad")
	r = withChiURLParam(r, "zoneID", validID)

	h.Create(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	_, hasError := body["error"]
	assert.True(t, hasError)
}
