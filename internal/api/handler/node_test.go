package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newNodeHandler() *Node {
	return NewNode(nil)
}

// --- ListByCluster ---

func TestNodeListByCluster_EmptyID(t *testing.T) {
	h := newNodeHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/clusters//nodes", nil)
	r = withChiURLParam(r, "clusterID", "")

	h.ListByCluster(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Create ---

func TestNodeCreate_EmptyClusterID(t *testing.T) {
	h := newNodeHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/clusters//nodes", map[string]any{
		"hostname":   "node-01",
		"ip_address": "10.0.0.5",
		"roles":      []string{"web"},
	})
	r = withChiURLParam(r, "clusterID", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestNodeCreate_InvalidJSON(t *testing.T) {
	h := newNodeHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/clusters/"+validID+"/nodes", "{bad json")
	r = withChiURLParam(r, "clusterID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestNodeCreate_EmptyBody(t *testing.T) {
	h := newNodeHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/clusters/"+validID+"/nodes", "")
	r = withChiURLParam(r, "clusterID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNodeCreate_MissingRequiredFields(t *testing.T) {
	h := newNodeHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/clusters/"+validID+"/nodes", map[string]any{})
	r = withChiURLParam(r, "clusterID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestNodeCreate_MissingHostname(t *testing.T) {
	h := newNodeHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/clusters/"+validID+"/nodes", map[string]any{
		"ip_address": "10.0.0.5",
		"roles":      []string{"web"},
	})
	r = withChiURLParam(r, "clusterID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestNodeCreate_MissingRoles(t *testing.T) {
	h := newNodeHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/clusters/"+validID+"/nodes", map[string]any{
		"hostname":   "node-01",
		"ip_address": "10.0.0.5",
	})
	r = withChiURLParam(r, "clusterID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestNodeCreate_EmptyRoles(t *testing.T) {
	h := newNodeHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/clusters/"+validID+"/nodes", map[string]any{
		"hostname":   "node-01",
		"ip_address": "10.0.0.5",
		"roles":      []string{},
	})
	r = withChiURLParam(r, "clusterID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestNodeCreate_ValidBody(t *testing.T) {
	h := newNodeHandler()
	rec := httptest.NewRecorder()
	cid := "test-cluster-1"
	r := newRequest(http.MethodPost, "/clusters/"+cid+"/nodes", map[string]any{
		"hostname":   "node-01",
		"ip_address": "10.0.0.5",
		"roles":      []string{"web", "db"},
	})
	r = withChiURLParam(r, "clusterID", cid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestNodeCreate_ValidBodyWithOptionals(t *testing.T) {
	h := newNodeHandler()
	rec := httptest.NewRecorder()
	cid := "test-cluster-2"
	r := newRequest(http.MethodPost, "/clusters/"+cid+"/nodes", map[string]any{
		"hostname":    "node-01",
		"ip_address":  "10.0.0.5",
		"ip6_address": "::1",
		"roles":       []string{"web"},
	})
	r = withChiURLParam(r, "clusterID", cid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- Get ---

func TestNodeGet_EmptyID(t *testing.T) {
	h := newNodeHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/nodes/", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Update ---

func TestNodeUpdate_EmptyID(t *testing.T) {
	h := newNodeHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/nodes/", map[string]any{
		"hostname": "new-node-01",
	})
	r = withChiURLParam(r, "id", "")

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestNodeUpdate_InvalidJSON(t *testing.T) {
	h := newNodeHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/nodes/"+validID, "{bad json")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestNodeUpdate_EmptyBody(t *testing.T) {
	h := newNodeHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/nodes/"+validID, "")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Delete ---

func TestNodeDelete_EmptyID(t *testing.T) {
	h := newNodeHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/nodes/", nil)
	r = withChiURLParam(r, "id", "")

	h.Delete(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Error response format ---

func TestNodeCreate_ErrorResponseFormat(t *testing.T) {
	h := newNodeHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/clusters/"+validID+"/nodes", "{bad")
	r = withChiURLParam(r, "clusterID", validID)

	h.Create(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	_, hasError := body["error"]
	assert.True(t, hasError)
}
