package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newShardHandler() *Shard {
	return NewShard(nil)
}

// --- ListByCluster ---

func TestShardListByCluster_EmptyID(t *testing.T) {
	h := newShardHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/clusters//shards", nil)
	r = withChiURLParam(r, "clusterID", "")

	h.ListByCluster(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Create ---

func TestShardCreate_EmptyClusterID(t *testing.T) {
	h := newShardHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/clusters//shards", map[string]any{
		"name": "web-shard-01",
		"role": "web",
	})
	r = withChiURLParam(r, "clusterID", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestShardCreate_InvalidJSON(t *testing.T) {
	h := newShardHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/clusters/"+validID+"/shards", "{bad json")
	r = withChiURLParam(r, "clusterID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestShardCreate_EmptyBody(t *testing.T) {
	h := newShardHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/clusters/"+validID+"/shards", "")
	r = withChiURLParam(r, "clusterID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestShardCreate_MissingRequiredFields(t *testing.T) {
	h := newShardHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/clusters/"+validID+"/shards", map[string]any{})
	r = withChiURLParam(r, "clusterID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestShardCreate_MissingName(t *testing.T) {
	h := newShardHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/clusters/"+validID+"/shards", map[string]any{
		"role": "web",
	})
	r = withChiURLParam(r, "clusterID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestShardCreate_MissingRole(t *testing.T) {
	h := newShardHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/clusters/"+validID+"/shards", map[string]any{
		"name": "web-shard-01",
	})
	r = withChiURLParam(r, "clusterID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestShardCreate_InvalidRole(t *testing.T) {
	h := newShardHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/clusters/"+validID+"/shards", map[string]any{
		"name": "web-shard-01",
		"role": "invalid-role",
	})
	r = withChiURLParam(r, "clusterID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestShardCreate_ValidBody(t *testing.T) {
	h := newShardHandler()
	rec := httptest.NewRecorder()
	cid := "test-cluster-1"
	r := newRequest(http.MethodPost, "/clusters/"+cid+"/shards", map[string]any{
		"name": "web-shard-01",
		"role": "web",
	})
	r = withChiURLParam(r, "clusterID", cid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestShardCreate_ValidBodyWithOptionals(t *testing.T) {
	h := newShardHandler()
	rec := httptest.NewRecorder()
	cid := "test-cluster-2"
	r := newRequest(http.MethodPost, "/clusters/"+cid+"/shards", map[string]any{
		"name":       "db-shard-01",
		"role":       "database",
		"lb_backend": "backend-db",
		"config":     map[string]any{"replicas": 3},
	})
	r = withChiURLParam(r, "clusterID", cid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- Get ---

func TestShardGet_EmptyID(t *testing.T) {
	h := newShardHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/shards/", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Update ---

func TestShardUpdate_EmptyID(t *testing.T) {
	h := newShardHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/shards/", map[string]any{
		"lb_backend": "new-backend",
	})
	r = withChiURLParam(r, "id", "")

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestShardUpdate_InvalidJSON(t *testing.T) {
	h := newShardHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/shards/"+validID, "{bad json")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestShardUpdate_EmptyBody(t *testing.T) {
	h := newShardHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/shards/"+validID, "")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Delete ---

func TestShardDelete_EmptyID(t *testing.T) {
	h := newShardHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/shards/", nil)
	r = withChiURLParam(r, "id", "")

	h.Delete(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Error response format ---

func TestShardCreate_ErrorResponseFormat(t *testing.T) {
	h := newShardHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/clusters/"+validID+"/shards", "{bad")
	r = withChiURLParam(r, "clusterID", validID)

	h.Create(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	_, hasError := body["error"]
	assert.True(t, hasError)
}
