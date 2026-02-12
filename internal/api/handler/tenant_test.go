package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTenantHandler() *Tenant {
	return NewTenant(nil)
}

// --- Create ---

func TestTenantCreate_InvalidJSON(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants", "{bad json")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestTenantCreate_EmptyBody(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestTenantCreate_MissingRequiredFields(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants", map[string]any{})

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestTenantCreate_MissingName(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants", map[string]any{
		"region_id":  validID,
		"cluster_id": validID2,
		"shard_id":   "test-shard-1",
	})

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestTenantCreate_InvalidSlugName(t *testing.T) {
	tests := []struct {
		name string
		slug string
	}{
		{"uppercase", "MyTenant"},
		{"spaces", "my tenant"},
		{"special chars", "my@tenant"},
		{"starts with digit", "1tenant"},
		{"starts with dash", "-tenant"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTenantHandler()
			rec := httptest.NewRecorder()
			r := newRequest(http.MethodPost, "/tenants", map[string]any{
				"name":       tt.slug,
				"region_id":  validID,
				"cluster_id": validID2,
				"shard_id":   "test-shard-1",
			})

			h.Create(rec, r)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			body := decodeErrorResponse(rec)
			assert.Contains(t, body["error"], "validation error")
		})
	}
}

func TestTenantCreate_MissingRegionID(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants", map[string]any{
		"name":       "my-tenant",
		"cluster_id": validID2,
		"shard_id":   "test-shard-1",
	})

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestTenantCreate_MissingClusterID(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants", map[string]any{
		"name":      "my-tenant",
		"region_id": validID,
		"shard_id":  "test-shard-1",
	})

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

// --- Get ---

func TestTenantGet_EmptyID(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/tenants/", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestTenantGet_MissingURLParam(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	// No chi context set, so URLParam returns ""
	r := newRequest(http.MethodGet, "/tenants/", nil)

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Update ---

func TestTenantUpdate_EmptyID(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/tenants/", map[string]any{
		"sftp_enabled": true,
	})
	r = withChiURLParam(r, "id", "")

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestTenantUpdate_InvalidJSON(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/tenants/"+validID, "{bad json")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestTenantUpdate_EmptyBody(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/tenants/"+validID, "")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

// --- Delete ---

func TestTenantDelete_EmptyID(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/tenants/", nil)
	r = withChiURLParam(r, "id", "")

	h.Delete(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Suspend ---

func TestTenantSuspend_EmptyID(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants//suspend", nil)
	r = withChiURLParam(r, "id", "")

	h.Suspend(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Unsuspend ---

func TestTenantUnsuspend_EmptyID(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants//unsuspend", nil)
	r = withChiURLParam(r, "id", "")

	h.Unsuspend(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- JSON content-type verification ---

func TestTenantCreate_ResponseHasJSONContentType(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	// Send invalid body so we get a 400 without hitting the DB
	r := newRequestRaw(http.MethodPost, "/tenants", "not json")

	h.Create(rec, r)

	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestTenantGet_ResponseHasJSONContentType(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/tenants/test-id", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

// --- Various ID format tests ---

func TestTenantGet_ValidIDFormats(t *testing.T) {
	// These are all valid ID formats that should pass the RequireID check
	// but will fail at the service layer (nil service). We just verify they pass
	// the ID validation step.
	tests := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"my-tenant-1",
		"simple",
	}
	for _, id := range tests {
		t.Run(id, func(t *testing.T) {
			h := newTenantHandler()
			rec := httptest.NewRecorder()
			r := newRequest(http.MethodGet, "/tenants/"+id, nil)
			r = withChiURLParam(r, "id", id)

			// This will panic or return 500/404 because svc is nil, but NOT 400.
			// We use recover to catch nil pointer dereference.
			func() {
				defer func() { recover() }()
				h.Get(rec, r)
			}()

			// If we got a response code, it should not be 400
			if rec.Code != 0 && rec.Code != 200 {
				assert.NotEqual(t, http.StatusBadRequest, rec.Code,
					"valid ID %s should not produce 400", id)
			}
		})
	}
}

func TestTenantGet_InvalidIDFormats(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"empty string", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTenantHandler()
			rec := httptest.NewRecorder()
			r := newRequest(http.MethodGet, "/tenants/test-id", nil)
			r = withChiURLParam(r, "id", tt.id)

			h.Get(rec, r)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

// --- Create body validation permutations ---

func TestTenantCreate_ValidBodyParsing(t *testing.T) {
	// Verify that a well-formed body gets past the validation/decode step.
	// It will fail at the service layer (nil svc) rather than at validation.
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants", map[string]any{
		"name":         "my-tenant",
		"region_id":    "test-region-1",
		"cluster_id":   "test-cluster-1",
		"shard_id":     "test-shard-1",
		"sftp_enabled": true,
	})

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	// Should NOT be 400 (validation passed)
	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestTenantCreate_OptionalSFTPEnabled(t *testing.T) {
	// sftp_enabled is optional, so body without it should pass validation.
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants", map[string]any{
		"name":       "my-tenant",
		"region_id":  "test-region-1",
		"cluster_id": "test-cluster-1",
		"shard_id":   "test-shard-1",
	})

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestTenantCreate_ExtraFieldsIgnored(t *testing.T) {
	// Extra fields in JSON should not cause validation errors.
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants", map[string]any{
		"name":        "my-tenant",
		"region_id":   "test-region-1",
		"cluster_id":  "test-cluster-1",
		"shard_id":    "test-shard-1",
		"extra_field": "should be ignored",
	})

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- Error response format ---

func TestTenantCreate_ErrorResponseFormat(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants", "{bad")

	h.Create(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)

	// Error response should have an "error" key
	_, hasError := body["error"]
	assert.True(t, hasError, "error response should contain 'error' key")
}
