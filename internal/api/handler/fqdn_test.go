package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFQDNHandler() *FQDN {
	return &FQDN{svc: nil, services: nil}
}

// --- ListByTenant ---

func TestFQDNListByTenant_EmptyID(t *testing.T) {
	h := newFQDNHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/tenants//fqdns", nil)
	r = withChiURLParam(r, "tenantID", "")

	h.ListByTenant(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- ListByWebroot ---

func TestFQDNListByWebroot_EmptyID(t *testing.T) {
	h := newFQDNHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/webroots//fqdns", nil)
	r = withChiURLParam(r, "webrootID", "")

	h.ListByWebroot(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Create ---

func TestFQDNCreate_EmptyTenantID(t *testing.T) {
	h := newFQDNHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants//fqdns", map[string]any{
		"fqdn": "example.com",
	})
	r = withChiURLParam(r, "tenantID", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestFQDNCreate_InvalidJSON(t *testing.T) {
	h := newFQDNHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequestRaw(http.MethodPost, "/tenants/"+tid+"/fqdns", "{bad json")
	r = withChiURLParam(r, "tenantID", tid)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestFQDNCreate_EmptyBody(t *testing.T) {
	h := newFQDNHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequestRaw(http.MethodPost, "/tenants/"+tid+"/fqdns", "")
	r = withChiURLParam(r, "tenantID", tid)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestFQDNCreate_MissingFQDN(t *testing.T) {
	h := newFQDNHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequest(http.MethodPost, "/tenants/"+tid+"/fqdns", map[string]any{})
	r = withChiURLParam(r, "tenantID", tid)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestFQDNCreate_InvalidFQDNFormat(t *testing.T) {
	tests := []struct {
		name string
		fqdn string
	}{
		{"empty string in fqdn field", ""},
		{"just spaces", "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newFQDNHandler()
			rec := httptest.NewRecorder()
			tid := "test-tenant-1"
			r := newRequest(http.MethodPost, "/tenants/"+tid+"/fqdns", map[string]any{
				"fqdn": tt.fqdn,
			})
			r = withChiURLParam(r, "tenantID", tid)

			h.Create(rec, r)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestFQDNCreate_ValidFQDN(t *testing.T) {
	tests := []string{
		"example.com",
		"sub.example.com",
		"deep.sub.example.com",
	}
	for _, fqdn := range tests {
		t.Run(fqdn, func(t *testing.T) {
			h := newFQDNHandler()
			rec := httptest.NewRecorder()
			tid := "test-tenant-1"
			r := newRequest(http.MethodPost, "/tenants/"+tid+"/fqdns", map[string]any{
				"fqdn": fqdn,
			})
			r = withChiURLParam(r, "tenantID", tid)

			func() {
				defer func() { recover() }()
				h.Create(rec, r)
			}()

			assert.NotEqual(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestFQDNCreate_OptionalSSLEnabled(t *testing.T) {
	h := newFQDNHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequest(http.MethodPost, "/tenants/"+tid+"/fqdns", map[string]any{
		"fqdn":        "example.com",
		"ssl_enabled": true,
	})
	r = withChiURLParam(r, "tenantID", tid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestFQDNCreate_WithoutSSLEnabled(t *testing.T) {
	h := newFQDNHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequest(http.MethodPost, "/tenants/"+tid+"/fqdns", map[string]any{
		"fqdn": "example.com",
	})
	r = withChiURLParam(r, "tenantID", tid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- Nested resource validation ---

func TestFQDNCreate_WithNestedEmailAccounts_ValidationPasses(t *testing.T) {
	h := newFQDNHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequest(http.MethodPost, "/tenants/"+tid+"/fqdns", map[string]any{
		"fqdn": "example.com",
		"email_accounts": []map[string]any{
			{
				"subscription_id": "sub-1",
				"address":         "admin@example.com",
				"display_name":    "Admin",
				"quota_bytes":     1073741824,
			},
			{
				"subscription_id": "sub-1",
				"address":         "info@example.com",
			},
		},
	})
	r = withChiURLParam(r, "tenantID", tid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestFQDNCreate_WithInvalidNestedEmail_ValidationFails(t *testing.T) {
	h := newFQDNHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequest(http.MethodPost, "/tenants/"+tid+"/fqdns", map[string]any{
		"fqdn": "example.com",
		"email_accounts": []map[string]any{
			{"subscription_id": "sub-1", "address": "not-an-email"}, // invalid email
		},
	})
	r = withChiURLParam(r, "tenantID", tid)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

// --- Get ---

func TestFQDNGet_EmptyID(t *testing.T) {
	h := newFQDNHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/fqdns/", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Delete ---

func TestFQDNDelete_EmptyID(t *testing.T) {
	h := newFQDNHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/fqdns/", nil)
	r = withChiURLParam(r, "id", "")

	h.Delete(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Error format ---

func TestFQDNCreate_ErrorResponseFormat(t *testing.T) {
	h := newFQDNHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequestRaw(http.MethodPost, "/tenants/"+tid+"/fqdns", "{bad")
	r = withChiURLParam(r, "tenantID", tid)

	h.Create(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	_, hasError := body["error"]
	assert.True(t, hasError)
}
