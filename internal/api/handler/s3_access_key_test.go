package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newS3AccessKeyHandler() *S3AccessKey {
	return &S3AccessKey{svc: nil}
}

// --- ListByBucket ---

func TestS3AccessKeyListByBucket_EmptyID(t *testing.T) {
	h := newS3AccessKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/s3-buckets//access-keys", nil)
	r = withChiURLParam(r, "bucketID", "")

	h.ListByBucket(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Create ---

func TestS3AccessKeyCreate_EmptyBucketID(t *testing.T) {
	h := newS3AccessKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/s3-buckets//access-keys", map[string]any{
		"permissions": "read-write",
	})
	r = withChiURLParam(r, "bucketID", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestS3AccessKeyCreate_InvalidJSON(t *testing.T) {
	h := newS3AccessKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/s3-buckets/"+validID+"/access-keys", "{bad json")
	r = withChiURLParam(r, "bucketID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestS3AccessKeyCreate_ValidBody(t *testing.T) {
	h := newS3AccessKeyHandler()
	rec := httptest.NewRecorder()
	bid := "test-bucket-1"
	r := newRequest(http.MethodPost, "/s3-buckets/"+bid+"/access-keys", map[string]any{
		"permissions": "read-write",
	})
	r = withChiURLParam(r, "bucketID", bid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestS3AccessKeyCreate_ValidEmptyBody(t *testing.T) {
	h := newS3AccessKeyHandler()
	rec := httptest.NewRecorder()
	bid := "test-bucket-1"
	r := newRequest(http.MethodPost, "/s3-buckets/"+bid+"/access-keys", map[string]any{})
	r = withChiURLParam(r, "bucketID", bid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestS3AccessKeyCreate_InvalidPermissions(t *testing.T) {
	h := newS3AccessKeyHandler()
	rec := httptest.NewRecorder()
	bid := "test-bucket-1"
	r := newRequest(http.MethodPost, "/s3-buckets/"+bid+"/access-keys", map[string]any{
		"permissions": "admin",
	})
	r = withChiURLParam(r, "bucketID", bid)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

// --- Delete ---

func TestS3AccessKeyDelete_EmptyID(t *testing.T) {
	h := newS3AccessKeyHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/s3-access-keys/", nil)
	r = withChiURLParam(r, "id", "")

	h.Delete(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}
