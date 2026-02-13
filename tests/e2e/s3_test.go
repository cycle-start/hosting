//go:build e2e

package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestS3BucketCRUD tests the full S3 bucket lifecycle:
// create tenant -> create S3 bucket -> wait active -> create access key ->
// verify credentials -> update bucket (toggle public) -> list access keys ->
// delete access key -> delete bucket -> cleanup.
func TestS3BucketCRUD(t *testing.T) {
	tenantID, _, clusterID, _, _ := createTestTenant(t, "e2e-s3-crud")

	// Find an S3 shard.
	s3ShardID := findS3Shard(t, clusterID)
	if s3ShardID == "" {
		t.Skip("no S3 shard found in cluster; skipping S3 tests")
	}

	// Step 1: Create an S3 bucket.
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/s3-buckets", coreAPIURL, tenantID), map[string]interface{}{
		"name":     "e2e-test-bucket",
		"shard_id": s3ShardID,
	})
	require.Equal(t, 202, resp.StatusCode, "create S3 bucket: %s", body)
	bucket := parseJSON(t, body)
	bucketID := bucket["id"].(string)
	require.NotEmpty(t, bucketID)
	require.Equal(t, false, bucket["public"])
	t.Logf("created S3 bucket: %s", bucketID)

	// Step 2: Wait for the bucket to become active.
	bucket = waitForStatus(t, coreAPIURL+"/s3-buckets/"+bucketID, "active", provisionTimeout)
	require.Equal(t, "active", bucket["status"])
	t.Logf("S3 bucket active: %s", bucketID)

	// Step 3: Verify the bucket appears in the tenant's S3 bucket list.
	resp, body = httpGet(t, fmt.Sprintf("%s/tenants/%s/s3-buckets", coreAPIURL, tenantID))
	require.Equal(t, 200, resp.StatusCode, body)
	buckets := parsePaginatedItems(t, body)
	found := false
	for _, b := range buckets {
		if id, _ := b["id"].(string); id == bucketID {
			found = true
			break
		}
	}
	require.True(t, found, "bucket %s not found in tenant S3 bucket list", bucketID)

	// Step 4: Create an access key.
	resp, body = httpPost(t, fmt.Sprintf("%s/s3-buckets/%s/access-keys", coreAPIURL, bucketID), map[string]interface{}{
		"permissions": "read-write",
	})
	require.Equal(t, 201, resp.StatusCode, "create access key: %s", body)
	key := parseJSON(t, body)
	keyID := key["id"].(string)
	require.NotEmpty(t, keyID)

	// Verify access key credentials are returned on creation.
	accessKeyID, _ := key["access_key_id"].(string)
	secretAccessKey, _ := key["secret_access_key"].(string)
	require.NotEmpty(t, accessKeyID, "access_key_id should be returned on creation")
	require.NotEmpty(t, secretAccessKey, "secret_access_key should be returned on creation")
	require.Len(t, accessKeyID, 20, "access_key_id should be 20 characters")
	require.Len(t, secretAccessKey, 40, "secret_access_key should be 40 characters")
	t.Logf("created access key: %s (access_key_id=%s)", keyID, accessKeyID)

	// Step 5: List access keys — secret should be redacted.
	resp, body = httpGet(t, fmt.Sprintf("%s/s3-buckets/%s/access-keys", coreAPIURL, bucketID))
	require.Equal(t, 200, resp.StatusCode, body)
	keys := parsePaginatedItems(t, body)
	found = false
	for _, k := range keys {
		if id, _ := k["id"].(string); id == keyID {
			found = true
			// Secret should be empty/missing in list response.
			secret, _ := k["secret_access_key"].(string)
			require.Empty(t, secret, "secret_access_key should be redacted in list response")
			break
		}
	}
	require.True(t, found, "access key %s not found in bucket access key list", keyID)

	// Step 6: Update bucket to public.
	resp, body = httpPut(t, coreAPIURL+"/s3-buckets/"+bucketID, map[string]interface{}{
		"public": true,
	})
	require.Equal(t, 200, resp.StatusCode, "update S3 bucket: %s", body)
	updated := parseJSON(t, body)
	require.Equal(t, true, updated["public"])
	t.Logf("S3 bucket updated to public")

	// Step 7: Update bucket back to private.
	resp, body = httpPut(t, coreAPIURL+"/s3-buckets/"+bucketID, map[string]interface{}{
		"public": false,
	})
	require.Equal(t, 200, resp.StatusCode, "update S3 bucket: %s", body)
	updated = parseJSON(t, body)
	require.Equal(t, false, updated["public"])
	t.Logf("S3 bucket updated to private")

	// Step 8: Delete the access key.
	resp, body = httpDelete(t, coreAPIURL+"/s3-access-keys/"+keyID)
	require.Equal(t, 202, resp.StatusCode, "delete access key: %s", body)
	t.Logf("access key delete requested")

	// Step 9: Delete the S3 bucket.
	resp, body = httpDelete(t, coreAPIURL+"/s3-buckets/"+bucketID)
	require.Equal(t, 202, resp.StatusCode, "delete S3 bucket: %s", body)
	t.Logf("S3 bucket delete requested")

	// Step 10: Wait for the bucket to be deleted (or 404).
	waitForStatus(t, coreAPIURL+"/s3-buckets/"+bucketID, "deleted", provisionTimeout)
	t.Logf("S3 bucket deleted")
}

// TestS3BucketGetNotFound verifies that fetching a non-existent S3 bucket returns 404.
func TestS3BucketGetNotFound(t *testing.T) {
	resp, _ := httpGet(t, coreAPIURL+"/s3-buckets/00000000-0000-0000-0000-000000000000")
	require.Equal(t, 404, resp.StatusCode)
}

// TestS3BucketCreateValidation verifies that creating a bucket with missing
// required fields returns 400.
func TestS3BucketCreateValidation(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-s3-val")

	// Missing name.
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/s3-buckets", coreAPIURL, tenantID), map[string]interface{}{
		"shard_id": "some-shard",
	})
	require.Equal(t, 400, resp.StatusCode, "expected 400 for missing name: %s", body)

	// Missing shard_id.
	resp, body = httpPost(t, fmt.Sprintf("%s/tenants/%s/s3-buckets", coreAPIURL, tenantID), map[string]interface{}{
		"name": "my-bucket",
	})
	require.Equal(t, 400, resp.StatusCode, "expected 400 for missing shard_id: %s", body)

	// Invalid name (uppercase).
	resp, body = httpPost(t, fmt.Sprintf("%s/tenants/%s/s3-buckets", coreAPIURL, tenantID), map[string]interface{}{
		"name":     "MyBucket",
		"shard_id": "some-shard",
	})
	require.Equal(t, 400, resp.StatusCode, "expected 400 for invalid name: %s", body)
}

// TestS3AccessKeyCreateValidation verifies that creating an access key with
// invalid permissions returns 400.
func TestS3AccessKeyCreateValidation(t *testing.T) {
	tenantID, _, clusterID, _, _ := createTestTenant(t, "e2e-s3-keyval")
	s3ShardID := findS3Shard(t, clusterID)
	if s3ShardID == "" {
		t.Skip("no S3 shard found")
	}

	// Create a bucket first.
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/s3-buckets", coreAPIURL, tenantID), map[string]interface{}{
		"name":     "e2e-keyval-bucket",
		"shard_id": s3ShardID,
	})
	require.Equal(t, 202, resp.StatusCode, body)
	bucket := parseJSON(t, body)
	bucketID := bucket["id"].(string)
	waitForStatus(t, coreAPIURL+"/s3-buckets/"+bucketID, "active", provisionTimeout)

	// Try to create a key with invalid permissions.
	resp, body = httpPost(t, fmt.Sprintf("%s/s3-buckets/%s/access-keys", coreAPIURL, bucketID), map[string]interface{}{
		"permissions": "admin",
	})
	require.Equal(t, 400, resp.StatusCode, "expected 400 for invalid permissions: %s", body)
}

// TestS3BucketWithQuota tests creating a bucket with a quota.
func TestS3BucketWithQuota(t *testing.T) {
	tenantID, _, clusterID, _, _ := createTestTenant(t, "e2e-s3-quota")
	s3ShardID := findS3Shard(t, clusterID)
	if s3ShardID == "" {
		t.Skip("no S3 shard found")
	}

	var quotaBytes int64 = 1073741824 // 1GB

	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/s3-buckets", coreAPIURL, tenantID), map[string]interface{}{
		"name":        "e2e-quota-bucket",
		"shard_id":    s3ShardID,
		"public":      true,
		"quota_bytes": quotaBytes,
	})
	require.Equal(t, 202, resp.StatusCode, "create S3 bucket with quota: %s", body)
	bucket := parseJSON(t, body)
	bucketID := bucket["id"].(string)
	require.Equal(t, true, bucket["public"])
	require.Equal(t, float64(quotaBytes), bucket["quota_bytes"])

	bucket = waitForStatus(t, coreAPIURL+"/s3-buckets/"+bucketID, "active", provisionTimeout)
	require.Equal(t, "active", bucket["status"])
	t.Logf("S3 bucket with quota active: %s", bucketID)

	// Update quota.
	var newQuota int64 = 2147483648 // 2GB
	resp, body = httpPut(t, coreAPIURL+"/s3-buckets/"+bucketID, map[string]interface{}{
		"quota_bytes": newQuota,
	})
	require.Equal(t, 200, resp.StatusCode, "update quota: %s", body)
	updated := parseJSON(t, body)
	require.Equal(t, float64(newQuota), updated["quota_bytes"])
	t.Logf("S3 bucket quota updated to %d", newQuota)
}

// findS3Shard returns the ID of the first S3 shard in the cluster, or empty string if none.
func findS3Shard(t *testing.T, clusterID string) string {
	t.Helper()
	resp, body := httpGet(t, fmt.Sprintf("%s/clusters/%s/shards", coreAPIURL, clusterID))
	if resp.StatusCode != 200 {
		return ""
	}
	shards := parsePaginatedItems(t, body)
	for _, s := range shards {
		if r, _ := s["role"].(string); r == "s3" {
			id, _ := s["id"].(string)
			return id
		}
	}
	return ""
}
