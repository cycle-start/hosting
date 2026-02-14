package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSFTPKeyHandler() *SFTPKey {
	return NewSFTPKey(nil, nil)
}

// --- ListByTenant ---

func TestSFTPKeyListByTenant_EmptyID(t *testing.T) {
	h := newSFTPKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/tenants//sftp-keys", nil)
	r = withChiURLParam(r, "tenantID", "")

	h.ListByTenant(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Create ---

func TestSFTPKeyCreate_EmptyTenantID(t *testing.T) {
	h := newSFTPKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants//sftp-keys", map[string]any{
		"name":       "my-key",
		"public_key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIC...",
	})
	r = withChiURLParam(r, "tenantID", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestSFTPKeyCreate_InvalidJSON(t *testing.T) {
	h := newSFTPKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants/"+validID+"/sftp-keys", "{bad json")
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestSFTPKeyCreate_EmptyBody(t *testing.T) {
	h := newSFTPKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants/"+validID+"/sftp-keys", "")
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSFTPKeyCreate_MissingRequiredFields(t *testing.T) {
	h := newSFTPKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/sftp-keys", map[string]any{})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestSFTPKeyCreate_MissingName(t *testing.T) {
	h := newSFTPKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/sftp-keys", map[string]any{
		"public_key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIC...",
	})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestSFTPKeyCreate_MissingPublicKey(t *testing.T) {
	h := newSFTPKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/sftp-keys", map[string]any{
		"name": "my-key",
	})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestSFTPKeyCreate_InvalidPublicKey(t *testing.T) {
	h := newSFTPKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/sftp-keys", map[string]any{
		"name":       "my-key",
		"public_key": "not-a-valid-ssh-key",
	})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid SSH public key")
}

// --- Get ---

func TestSFTPKeyGet_EmptyID(t *testing.T) {
	h := newSFTPKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/sftp-keys/", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Delete ---

func TestSFTPKeyDelete_EmptyID(t *testing.T) {
	h := newSFTPKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/sftp-keys/", nil)
	r = withChiURLParam(r, "id", "")

	h.Delete(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Error response format ---

func TestSFTPKeyCreate_ErrorResponseFormat(t *testing.T) {
	h := newSFTPKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants/"+validID+"/sftp-keys", "{bad")
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	_, hasError := body["error"]
	assert.True(t, hasError)
}
