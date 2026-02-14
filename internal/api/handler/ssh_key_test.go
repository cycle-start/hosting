package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSSHKeyHandler() *SSHKey {
	return NewSSHKey(nil, nil)
}

// --- ListByTenant ---

func TestSSHKeyListByTenant_EmptyID(t *testing.T) {
	h := newSSHKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/tenants//ssh-keys", nil)
	r = withChiURLParam(r, "tenantID", "")

	h.ListByTenant(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Create ---

func TestSSHKeyCreate_EmptyTenantID(t *testing.T) {
	h := newSSHKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants//ssh-keys", map[string]any{
		"name":       "my-key",
		"public_key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIC...",
	})
	r = withChiURLParam(r, "tenantID", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestSSHKeyCreate_InvalidJSON(t *testing.T) {
	h := newSSHKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants/"+validID+"/ssh-keys", "{bad json")
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestSSHKeyCreate_EmptyBody(t *testing.T) {
	h := newSSHKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants/"+validID+"/ssh-keys", "")
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSSHKeyCreate_MissingRequiredFields(t *testing.T) {
	h := newSSHKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/ssh-keys", map[string]any{})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestSSHKeyCreate_MissingName(t *testing.T) {
	h := newSSHKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/ssh-keys", map[string]any{
		"public_key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIC...",
	})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestSSHKeyCreate_MissingPublicKey(t *testing.T) {
	h := newSSHKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/ssh-keys", map[string]any{
		"name": "my-key",
	})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestSSHKeyCreate_InvalidPublicKey(t *testing.T) {
	h := newSSHKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/ssh-keys", map[string]any{
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

func TestSSHKeyGet_EmptyID(t *testing.T) {
	h := newSSHKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/ssh-keys/", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Delete ---

func TestSSHKeyDelete_EmptyID(t *testing.T) {
	h := newSSHKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/ssh-keys/", nil)
	r = withChiURLParam(r, "id", "")

	h.Delete(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Error response format ---

func TestSSHKeyCreate_ErrorResponseFormat(t *testing.T) {
	h := newSSHKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants/"+validID+"/ssh-keys", "{bad")
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	_, hasError := body["error"]
	assert.True(t, hasError)
}
