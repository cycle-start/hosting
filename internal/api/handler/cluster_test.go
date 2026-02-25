package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newClusterHandler() *Cluster {
	return NewCluster(nil)
}

// --- ListByRegion ---

func TestClusterListByRegion_EmptyID(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/regions//clusters", nil)
	r = withChiURLParam(r, "regionID", "")

	h.ListByRegion(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Create ---

func TestClusterCreate_EmptyRegionID(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/regions//clusters", map[string]any{
		"name": "cluster-01",
	})
	r = withChiURLParam(r, "regionID", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestClusterCreate_InvalidJSON(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/regions/"+validID+"/clusters", "{bad json")
	r = withChiURLParam(r, "regionID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestClusterCreate_EmptyBody(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/regions/"+validID+"/clusters", "")
	r = withChiURLParam(r, "regionID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestClusterCreate_MissingRequiredFields(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/regions/"+validID+"/clusters", map[string]any{})
	r = withChiURLParam(r, "regionID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestClusterCreate_MissingName(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/regions/"+validID+"/clusters", map[string]any{
		"config": map[string]any{"max_nodes": 10},
	})
	r = withChiURLParam(r, "regionID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestClusterCreate_InvalidSlugName(t *testing.T) {
	tests := []struct {
		name string
		slug string
	}{
		{"uppercase", "MyCluster"},
		{"spaces", "my cluster"},
		{"special chars", "cluster@01"},
		{"starts with digit", "1cluster"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newClusterHandler()
			rec := httptest.NewRecorder()
			r := newRequest(http.MethodPost, "/regions/"+validID+"/clusters", map[string]any{
				"name": tt.slug,
			})
			r = withChiURLParam(r, "regionID", validID)

			h.Create(rec, r)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestClusterCreate_ValidBody(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	rid := "test-region-1"
	r := newRequest(http.MethodPost, "/regions/"+rid+"/clusters", map[string]any{
		"name": "cluster-01",
	})
	r = withChiURLParam(r, "regionID", rid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestClusterCreate_ValidBodyWithOptionals(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	rid := "test-region-2"
	r := newRequest(http.MethodPost, "/regions/"+rid+"/clusters", map[string]any{
		"name":   "cluster-01",
		"config": map[string]any{"max_nodes": 10},
		"spec":   map[string]any{"cpu": 4},
	})
	r = withChiURLParam(r, "regionID", rid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- Get ---

func TestClusterGet_EmptyID(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/clusters/", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Update ---

func TestClusterUpdate_EmptyID(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/clusters/", map[string]any{
		"name": "new-name",
	})
	r = withChiURLParam(r, "id", "")

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestClusterUpdate_InvalidJSON(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/clusters/"+validID, "{bad json")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestClusterUpdate_EmptyBody(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/clusters/"+validID, "")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Delete ---

func TestClusterDelete_EmptyID(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/clusters/", nil)
	r = withChiURLParam(r, "id", "")

	h.Delete(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- ListRuntimes ---

func TestClusterListRuntimes_EmptyID(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/clusters//runtimes", nil)
	r = withChiURLParam(r, "id", "")

	h.ListRuntimes(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- AddRuntime ---

func TestClusterAddRuntime_EmptyID(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/clusters//runtimes", map[string]any{
		"runtime": "php",
		"version": "8.3",
	})
	r = withChiURLParam(r, "id", "")

	h.AddRuntime(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestClusterAddRuntime_InvalidJSON(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/clusters/"+validID+"/runtimes", "{bad json")
	r = withChiURLParam(r, "id", validID)

	h.AddRuntime(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestClusterAddRuntime_EmptyBody(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/clusters/"+validID+"/runtimes", "")
	r = withChiURLParam(r, "id", validID)

	h.AddRuntime(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestClusterAddRuntime_MissingRuntime(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/clusters/"+validID+"/runtimes", map[string]any{
		"version": "8.3",
	})
	r = withChiURLParam(r, "id", validID)

	h.AddRuntime(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestClusterAddRuntime_MissingVersion(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/clusters/"+validID+"/runtimes", map[string]any{
		"runtime": "php",
	})
	r = withChiURLParam(r, "id", validID)

	h.AddRuntime(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestClusterAddRuntime_ValidBody(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	cid := "test-cluster-1"
	r := newRequest(http.MethodPost, "/clusters/"+cid+"/runtimes", map[string]any{
		"runtime": "php",
		"version": "8.3",
	})
	r = withChiURLParam(r, "id", cid)

	func() {
		defer func() { recover() }()
		h.AddRuntime(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- RemoveRuntime ---

func TestClusterRemoveRuntime_EmptyID(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/clusters//runtimes", map[string]any{
		"runtime": "php",
		"version": "8.3",
	})
	r = withChiURLParam(r, "id", "")

	h.RemoveRuntime(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestClusterRemoveRuntime_InvalidJSON(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodDelete, "/clusters/"+validID+"/runtimes", "{bad json")
	r = withChiURLParam(r, "id", validID)

	h.RemoveRuntime(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestClusterRemoveRuntime_EmptyBody(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodDelete, "/clusters/"+validID+"/runtimes", "")
	r = withChiURLParam(r, "id", validID)

	h.RemoveRuntime(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestClusterRemoveRuntime_MissingRuntime(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/clusters/"+validID+"/runtimes", map[string]any{
		"version": "8.3",
	})
	r = withChiURLParam(r, "id", validID)

	h.RemoveRuntime(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestClusterRemoveRuntime_MissingVersion(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/clusters/"+validID+"/runtimes", map[string]any{
		"runtime": "php",
	})
	r = withChiURLParam(r, "id", validID)

	h.RemoveRuntime(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

// --- Error response format ---

func TestClusterCreate_ErrorResponseFormat(t *testing.T) {
	h := newClusterHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/regions/"+validID+"/clusters", "{bad")
	r = withChiURLParam(r, "regionID", validID)

	h.Create(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	_, hasError := body["error"]
	assert.True(t, hasError)
}
