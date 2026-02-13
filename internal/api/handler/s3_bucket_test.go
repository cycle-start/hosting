package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newS3BucketHandler() *S3Bucket {
	return &S3Bucket{svc: nil, keySvc: nil}
}

// --- ListByTenant ---

func TestS3BucketListByTenant_EmptyID(t *testing.T) {
	h := newS3BucketHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/tenants//s3-buckets", nil)
	r = withChiURLParam(r, "tenantID", "")

	h.ListByTenant(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Create ---

func TestS3BucketCreate_EmptyTenantID(t *testing.T) {
	h := newS3BucketHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants//s3-buckets", map[string]any{
		"name":     "mybucket",
		"shard_id": validID,
	})
	r = withChiURLParam(r, "tenantID", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestS3BucketCreate_InvalidJSON(t *testing.T) {
	h := newS3BucketHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants/"+validID+"/s3-buckets", "{bad json")
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestS3BucketCreate_EmptyBody(t *testing.T) {
	h := newS3BucketHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants/"+validID+"/s3-buckets", "")
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestS3BucketCreate_MissingRequiredFields(t *testing.T) {
	h := newS3BucketHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/s3-buckets", map[string]any{})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestS3BucketCreate_MissingName(t *testing.T) {
	h := newS3BucketHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/s3-buckets", map[string]any{
		"shard_id": validID,
	})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestS3BucketCreate_MissingShardID(t *testing.T) {
	h := newS3BucketHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/s3-buckets", map[string]any{
		"name": "mybucket",
	})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestS3BucketCreate_InvalidSlugName(t *testing.T) {
	tests := []struct {
		name string
		slug string
	}{
		{"uppercase", "MyBucket"},
		{"spaces", "my bucket"},
		{"special chars", "my@bucket"},
		{"starts with digit", "1bucket"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newS3BucketHandler()
			rec := httptest.NewRecorder()
			r := newRequest(http.MethodPost, "/tenants/"+validID+"/s3-buckets", map[string]any{
				"name":     tt.slug,
				"shard_id": validID,
			})
			r = withChiURLParam(r, "tenantID", validID)

			h.Create(rec, r)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestS3BucketCreate_ValidBody(t *testing.T) {
	h := newS3BucketHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequest(http.MethodPost, "/tenants/"+tid+"/s3-buckets", map[string]any{
		"name":     "mybucket",
		"shard_id": "test-shard-1",
	})
	r = withChiURLParam(r, "tenantID", tid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestS3BucketCreate_WithOptionalFields(t *testing.T) {
	h := newS3BucketHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequest(http.MethodPost, "/tenants/"+tid+"/s3-buckets", map[string]any{
		"name":        "mybucket",
		"shard_id":    "test-shard-1",
		"public":      true,
		"quota_bytes": 1073741824,
	})
	r = withChiURLParam(r, "tenantID", tid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- Get ---

func TestS3BucketGet_EmptyID(t *testing.T) {
	h := newS3BucketHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/s3-buckets/", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Update ---

func TestS3BucketUpdate_EmptyID(t *testing.T) {
	h := newS3BucketHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/s3-buckets/", map[string]any{
		"public": true,
	})
	r = withChiURLParam(r, "id", "")

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestS3BucketUpdate_InvalidJSON(t *testing.T) {
	h := newS3BucketHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/s3-buckets/"+validID, "{bad json")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

// --- Delete ---

func TestS3BucketDelete_EmptyID(t *testing.T) {
	h := newS3BucketHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/s3-buckets/", nil)
	r = withChiURLParam(r, "id", "")

	h.Delete(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}
