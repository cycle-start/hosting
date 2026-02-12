package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCertificateHandler() *Certificate {
	return NewCertificate(nil)
}

// --- ListByFQDN ---

func TestCertificateListByFQDN_EmptyID(t *testing.T) {
	h := newCertificateHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/fqdns//certificates", nil)
	r = withChiURLParam(r, "fqdnID", "")

	h.ListByFQDN(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Upload ---

func TestCertificateUpload_EmptyFQDNID(t *testing.T) {
	h := newCertificateHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/fqdns//certificates", map[string]any{
		"cert_pem": "-----BEGIN CERTIFICATE-----\nMIIB...",
		"key_pem":  "-----BEGIN PRIVATE KEY-----\nMIIE...",
	})
	r = withChiURLParam(r, "fqdnID", "")

	h.Upload(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestCertificateUpload_InvalidJSON(t *testing.T) {
	h := newCertificateHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/fqdns/"+validID+"/certificates", "{bad json")
	r = withChiURLParam(r, "fqdnID", validID)

	h.Upload(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestCertificateUpload_EmptyBody(t *testing.T) {
	h := newCertificateHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/fqdns/"+validID+"/certificates", "")
	r = withChiURLParam(r, "fqdnID", validID)

	h.Upload(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCertificateUpload_MissingRequiredFields(t *testing.T) {
	h := newCertificateHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/fqdns/"+validID+"/certificates", map[string]any{})
	r = withChiURLParam(r, "fqdnID", validID)

	h.Upload(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestCertificateUpload_MissingCertPEM(t *testing.T) {
	h := newCertificateHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/fqdns/"+validID+"/certificates", map[string]any{
		"key_pem": "-----BEGIN PRIVATE KEY-----\nMIIE...",
	})
	r = withChiURLParam(r, "fqdnID", validID)

	h.Upload(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestCertificateUpload_MissingKeyPEM(t *testing.T) {
	h := newCertificateHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/fqdns/"+validID+"/certificates", map[string]any{
		"cert_pem": "-----BEGIN CERTIFICATE-----\nMIIB...",
	})
	r = withChiURLParam(r, "fqdnID", validID)

	h.Upload(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestCertificateUpload_ValidBody(t *testing.T) {
	h := newCertificateHandler()
	rec := httptest.NewRecorder()
	fid := "test-fqdn-1"
	r := newRequest(http.MethodPost, "/fqdns/"+fid+"/certificates", map[string]any{
		"cert_pem": "-----BEGIN CERTIFICATE-----\nMIIB...",
		"key_pem":  "-----BEGIN PRIVATE KEY-----\nMIIE...",
	})
	r = withChiURLParam(r, "fqdnID", fid)

	func() {
		defer func() { recover() }()
		h.Upload(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestCertificateUpload_WithOptionalChainPEM(t *testing.T) {
	h := newCertificateHandler()
	rec := httptest.NewRecorder()
	fid := "test-fqdn-2"
	r := newRequest(http.MethodPost, "/fqdns/"+fid+"/certificates", map[string]any{
		"cert_pem":  "-----BEGIN CERTIFICATE-----\nMIIB...",
		"key_pem":   "-----BEGIN PRIVATE KEY-----\nMIIE...",
		"chain_pem": "-----BEGIN CERTIFICATE-----\nMIIC...",
	})
	r = withChiURLParam(r, "fqdnID", fid)

	func() {
		defer func() { recover() }()
		h.Upload(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestCertificateUpload_WithoutChainPEM(t *testing.T) {
	h := newCertificateHandler()
	rec := httptest.NewRecorder()
	fid := "test-fqdn-3"
	r := newRequest(http.MethodPost, "/fqdns/"+fid+"/certificates", map[string]any{
		"cert_pem": "-----BEGIN CERTIFICATE-----\nMIIB...",
		"key_pem":  "-----BEGIN PRIVATE KEY-----\nMIIE...",
	})
	r = withChiURLParam(r, "fqdnID", fid)

	func() {
		defer func() { recover() }()
		h.Upload(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- Error response format ---

func TestCertificateUpload_ErrorResponseFormat(t *testing.T) {
	h := newCertificateHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/fqdns/"+validID+"/certificates", "{bad")
	r = withChiURLParam(r, "fqdnID", validID)

	h.Upload(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	_, hasError := body["error"]
	assert.True(t, hasError)
}
