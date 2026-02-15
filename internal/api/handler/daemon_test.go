package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDaemonHandler() *Daemon {
	return &Daemon{}
}

// --- ListByWebroot ---

func TestDaemonListByWebroot_EmptyID(t *testing.T) {
	h := newDaemonHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/webroots//daemons", nil)
	r = withChiURLParam(r, "webrootID", "")

	h.ListByWebroot(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Create ---

func TestDaemonCreate_EmptyWebrootID(t *testing.T) {
	h := newDaemonHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/webroots//daemons", map[string]any{
		"command": "php artisan queue:work",
	})
	r = withChiURLParam(r, "webrootID", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestDaemonCreate_InvalidJSON(t *testing.T) {
	h := newDaemonHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/webroots/"+validID+"/daemons", "{bad json")
	r = withChiURLParam(r, "webrootID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestDaemonCreate_EmptyBody(t *testing.T) {
	h := newDaemonHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/webroots/"+validID+"/daemons", "")
	r = withChiURLParam(r, "webrootID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDaemonCreate_MissingCommand(t *testing.T) {
	h := newDaemonHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/webroots/"+validID+"/daemons", map[string]any{})
	r = withChiURLParam(r, "webrootID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

// --- Get ---

func TestDaemonGet_EmptyID(t *testing.T) {
	h := newDaemonHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/daemons/", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Update ---

func TestDaemonUpdate_EmptyID(t *testing.T) {
	h := newDaemonHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/daemons/", map[string]any{
		"command": "updated",
	})
	r = withChiURLParam(r, "id", "")

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestDaemonUpdate_InvalidJSON(t *testing.T) {
	h := newDaemonHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/daemons/"+validID, "{bad")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

// --- Delete ---

func TestDaemonDelete_EmptyID(t *testing.T) {
	h := newDaemonHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/daemons/", nil)
	r = withChiURLParam(r, "id", "")

	h.Delete(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Enable ---

func TestDaemonEnable_EmptyID(t *testing.T) {
	h := newDaemonHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/daemons//enable", nil)
	r = withChiURLParam(r, "id", "")

	h.Enable(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Disable ---

func TestDaemonDisable_EmptyID(t *testing.T) {
	h := newDaemonHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/daemons//disable", nil)
	r = withChiURLParam(r, "id", "")

	h.Disable(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Retry ---

func TestDaemonRetry_EmptyID(t *testing.T) {
	h := newDaemonHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/daemons//retry", nil)
	r = withChiURLParam(r, "id", "")

	h.Retry(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Error response format ---

func TestDaemonCreate_ErrorResponseFormat(t *testing.T) {
	h := newDaemonHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/webroots/"+validID+"/daemons", "{bad")
	r = withChiURLParam(r, "webrootID", validID)

	h.Create(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	_, hasError := body["error"]
	assert.True(t, hasError)
}
