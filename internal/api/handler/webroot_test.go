package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newWebrootHandler() *Webroot {
	return &Webroot{svc: nil, services: nil}
}

// --- ListByTenant ---

func TestWebrootListByTenant_EmptyID(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/tenants//webroots", nil)
	r = withChiURLParam(r, "tenantID", "")

	h.ListByTenant(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Create ---

func TestWebrootCreate_EmptyTenantID(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants//webroots", map[string]any{
		"runtime":         "php",
		"runtime_version": "8.3",
	})
	r = withChiURLParam(r, "tenantID", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestWebrootCreate_InvalidJSON(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants/"+validID+"/webroots", "{bad json")
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestWebrootCreate_EmptyBody(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants/"+validID+"/webroots", "")
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWebrootCreate_MissingRequiredFields(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/webroots", map[string]any{})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestWebrootCreate_MissingRuntime(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/webroots", map[string]any{
		"runtime_version": "8.3",
	})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestWebrootCreate_MissingRuntimeVersion(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/webroots", map[string]any{
		"runtime": "php",
	})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestWebrootCreate_InvalidRuntime(t *testing.T) {
	tests := []string{"java", "go", "rust", "perl", ""}
	for _, runtime := range tests {
		t.Run(runtime, func(t *testing.T) {
			h := newWebrootHandler()
			rec := httptest.NewRecorder()
			r := newRequest(http.MethodPost, "/tenants/"+validID+"/webroots", map[string]any{
					"runtime":         runtime,
				"runtime_version": "1.0",
			})
			r = withChiURLParam(r, "tenantID", validID)

			h.Create(rec, r)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestWebrootCreate_ValidRuntimes(t *testing.T) {
	runtimes := []string{"php", "node", "python", "ruby", "static"}
	for _, runtime := range runtimes {
		t.Run(runtime, func(t *testing.T) {
			h := newWebrootHandler()
			rec := httptest.NewRecorder()
			r := newRequest(http.MethodPost, "/tenants/"+validID+"/webroots", map[string]any{
				"subscription_id": "sub-1",
				"runtime":         runtime,
				"runtime_version": "1.0",
			})
			r = withChiURLParam(r, "tenantID", validID)

			func() {
				defer func() { recover() }()
				h.Create(rec, r)
			}()

			// Should pass validation (not 400)
			assert.NotEqual(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestWebrootCreate_ValidBody(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/webroots", map[string]any{
		"subscription_id": "sub-1",
		"runtime":         "php",
		"runtime_version": "8.3",
		"public_folder":   "public",
		"runtime_config":  map[string]any{"max_workers": 4},
	})
	r = withChiURLParam(r, "tenantID", validID)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- Nested resource validation ---

func TestWebrootCreate_WithNestedFQDNs_ValidationPasses(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/webroots", map[string]any{
		"subscription_id": "sub-1",
		"runtime":         "php",
		"runtime_version": "8.5",
		"fqdns": []map[string]any{
			{"fqdn": "example.com"},
			{"fqdn": "www.example.com", "ssl_enabled": true},
		},
	})
	r = withChiURLParam(r, "tenantID", validID)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestWebrootCreate_WithInvalidNestedFQDN_ValidationFails(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/webroots", map[string]any{
		"runtime":         "php",
		"runtime_version": "8.5",
		"fqdns": []map[string]any{
			{}, // missing fqdn
		},
	})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestWebrootCreate_WithNestedFQDNAndEmail_ValidationPasses(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/webroots", map[string]any{
		"subscription_id": "sub-1",
		"runtime":         "php",
		"runtime_version": "8.5",
		"fqdns": []map[string]any{
			{
				"fqdn": "example.com",
				"email_accounts": []map[string]any{
					{
						"subscription_id": "sub-1",
						"address":         "admin@example.com",
						"display_name":    "Admin",
					},
				},
			},
		},
	})
	r = withChiURLParam(r, "tenantID", validID)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- Get ---

func TestWebrootGet_EmptyID(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/webroots/", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Update ---

func TestWebrootUpdate_EmptyID(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/webroots/", map[string]any{
		"runtime": "node",
	})
	r = withChiURLParam(r, "id", "")

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestWebrootUpdate_InvalidJSON(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/webroots/"+validID, "not json")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestWebrootUpdate_InvalidRuntimeValue(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/webroots/"+validID, map[string]any{
		"runtime": "java",
	})
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

// --- Delete ---

func TestWebrootDelete_EmptyID(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/webroots/", nil)
	r = withChiURLParam(r, "id", "")

	h.Delete(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Error response format ---

func TestWebrootCreate_ErrorResponseFormat(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants/"+validID+"/webroots", "{bad")
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	_, hasError := body["error"]
	assert.True(t, hasError, "error response should contain 'error' key")
}

// --- Create with optional fields ---

func TestWebrootCreate_OptionalFieldsOmitted(t *testing.T) {
	h := newWebrootHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequest(http.MethodPost, "/tenants/"+tid+"/webroots", map[string]any{
		"subscription_id": "sub-1",
		"runtime":         "php",
		"runtime_version": "8.3",
	})
	r = withChiURLParam(r, "tenantID", tid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}
