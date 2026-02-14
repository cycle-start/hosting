package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
)

// s3NodePort is the RGW port on S3 node VMs.
const s3NodePort = 7480

// TestS3BucketCRUD tests the full S3 bucket lifecycle:
// create tenant -> create S3 bucket -> wait active -> create access key ->
// verify credentials -> update bucket (toggle public) -> list access keys ->
// delete access key -> delete bucket -> cleanup.
func TestS3BucketCRUD(t *testing.T) {
	tenantID, _, clusterID, _, _ := createTestTenant(t, "e2e-s3-crud")

	// Find an S3 shard.
	s3ShardID := findStorageShard(t, clusterID)
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
	require.True(t, resp.StatusCode == 200 || resp.StatusCode == 202, "update S3 bucket: status %d: %s", resp.StatusCode, body)
	updated := parseJSON(t, body)
	require.Equal(t, true, updated["public"])
	t.Logf("S3 bucket updated to public")

	// Step 7: Update bucket back to private.
	resp, body = httpPut(t, coreAPIURL+"/s3-buckets/"+bucketID, map[string]interface{}{
		"public": false,
	})
	require.True(t, resp.StatusCode == 200 || resp.StatusCode == 202, "update S3 bucket: status %d: %s", resp.StatusCode, body)
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

// TestS3ObjectOperations creates a bucket, uploads objects via the S3 API,
// downloads them back, tests public access, and cleans up. This verifies that
// RGW is properly serving S3 requests on the node agent VMs.
func TestS3ObjectOperations(t *testing.T) {
	tenantID, _, clusterID, _, _ := createTestTenant(t, "e2e-s3-objects")

	s3ShardID := findStorageShard(t, clusterID)
	if s3ShardID == "" {
		t.Skip("no S3 shard found in cluster; skipping S3 object tests")
	}

	// Discover the storage node IP for direct RGW access.
	storageNodeIPs := findNodeIPsByRole(t, clusterID, "storage")
	require.NotEmpty(t, storageNodeIPs, "no storage node IPs found")
	rgwEndpoint := fmt.Sprintf("http://%s:%d", storageNodeIPs[0], s3NodePort)
	t.Logf("RGW endpoint: %s", rgwEndpoint)

	// Create a bucket.
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/s3-buckets", coreAPIURL, tenantID), map[string]interface{}{
		"name":     "e2e-objects",
		"shard_id": s3ShardID,
	})
	require.Equal(t, 202, resp.StatusCode, "create S3 bucket: %s", body)
	bucket := parseJSON(t, body)
	bucketID := bucket["id"].(string)

	bucket = waitForStatus(t, coreAPIURL+"/s3-buckets/"+bucketID, "active", provisionTimeout)
	require.Equal(t, "active", bucket["status"])
	t.Logf("S3 bucket active: %s", bucketID)

	// The internal RGW bucket name is "{tenantID}--{bucketName}".
	internalBucket := tenantID + "--e2e-objects"

	// Create an access key and wait for it to be provisioned in RGW.
	resp, body = httpPost(t, fmt.Sprintf("%s/s3-buckets/%s/access-keys", coreAPIURL, bucketID), map[string]interface{}{
		"permissions": "read-write",
	})
	require.Equal(t, 201, resp.StatusCode, "create access key: %s", body)
	key := parseJSON(t, body)
	keyID := key["id"].(string)
	accessKeyID := key["access_key_id"].(string)
	secretAccessKey := key["secret_access_key"].(string)
	t.Logf("access key created: %s, waiting for provisioning...", accessKeyID)

	// Wait for the async workflow to register the key in RGW.
	time.Sleep(5 * time.Second)

	// Create an S3 client pointing to the RGW endpoint.
	s3Client := s3.New(s3.Options{
		BaseEndpoint: aws.String(rgwEndpoint),
		Region:       "us-east-1",
		Credentials:  credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		UsePathStyle: true, // RGW uses path-style addressing.
	})

	ctx := context.Background()

	// --- Test 1: Upload an object. ---
	testContent := "Hello from S3 e2e test! This verifies that Ceph RGW is operational."
	_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(internalBucket),
		Key:         aws.String("test-file.txt"),
		Body:        strings.NewReader(testContent),
		ContentType: aws.String("text/plain"),
	})
	require.NoError(t, err, "PutObject should succeed")
	t.Logf("uploaded test-file.txt to bucket %s", internalBucket)

	// --- Test 2: Download and verify the object. ---
	getResp, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(internalBucket),
		Key:    aws.String("test-file.txt"),
	})
	require.NoError(t, err, "GetObject should succeed")
	defer getResp.Body.Close()

	downloaded, err := io.ReadAll(getResp.Body)
	require.NoError(t, err)
	require.Equal(t, testContent, string(downloaded), "downloaded content should match uploaded")
	require.Equal(t, "text/plain", aws.ToString(getResp.ContentType))
	t.Logf("downloaded and verified test-file.txt")

	// --- Test 3: List objects in the bucket. ---
	listResp, err := s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(internalBucket),
	})
	require.NoError(t, err, "ListObjects should succeed")
	require.Equal(t, int32(1), aws.ToInt32(listResp.KeyCount))
	require.Equal(t, "test-file.txt", aws.ToString(listResp.Contents[0].Key))
	t.Logf("listed objects: found %d", aws.ToInt32(listResp.KeyCount))

	// --- Test 4: Upload a second object (binary data). ---
	binaryData := make([]byte, 1024)
	for i := range binaryData {
		binaryData[i] = byte(i % 256)
	}
	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(internalBucket),
		Key:         aws.String("subdir/binary.bin"),
		Body:        bytes.NewReader(binaryData),
		ContentType: aws.String("application/octet-stream"),
	})
	require.NoError(t, err, "PutObject binary should succeed")
	t.Logf("uploaded subdir/binary.bin (1024 bytes)")

	// Verify list now shows 2 objects.
	listResp, err = s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(internalBucket),
	})
	require.NoError(t, err)
	require.Equal(t, int32(2), aws.ToInt32(listResp.KeyCount))
	t.Logf("listed objects: found %d", aws.ToInt32(listResp.KeyCount))

	// --- Test 5: Set bucket to public and verify anonymous read. ---
	resp, body = httpPut(t, coreAPIURL+"/s3-buckets/"+bucketID, map[string]interface{}{
		"public": true,
	})
	require.True(t, resp.StatusCode == 200 || resp.StatusCode == 202, "set public: status %d: %s", resp.StatusCode, body)
	t.Logf("bucket set to public")

	// Wait for the async workflow to apply the public policy in RGW,
	// then verify anonymous GET works.
	anonURL := fmt.Sprintf("%s/%s/test-file.txt", rgwEndpoint, internalBucket)
	var anonBody []byte
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		anonResp, err := http.Get(anonURL)
		if err == nil {
			body, _ := io.ReadAll(anonResp.Body)
			anonResp.Body.Close()
			if anonResp.StatusCode == 200 {
				anonBody = body
				break
			}
		}
		time.Sleep(2 * time.Second)
	}
	require.NotNil(t, anonBody, "anonymous GET should eventually return 200 for public bucket")
	require.Equal(t, testContent, string(anonBody), "anonymous download content should match")
	t.Logf("anonymous public read OK")

	// --- Test 6: Set bucket back to private and verify anonymous read fails. ---
	resp, body = httpPut(t, coreAPIURL+"/s3-buckets/"+bucketID, map[string]interface{}{
		"public": false,
	})
	require.True(t, resp.StatusCode == 200 || resp.StatusCode == 202, "set private: status %d: %s", resp.StatusCode, body)
	t.Logf("bucket set to private")

	// Wait for the async workflow to remove the public policy.
	deadline = time.Now().Add(30 * time.Second)
	var gotForbidden bool
	for time.Now().Before(deadline) {
		anonResp2, err := http.Get(anonURL)
		if err == nil {
			anonResp2.Body.Close()
			if anonResp2.StatusCode == 403 {
				gotForbidden = true
				break
			}
		}
		time.Sleep(2 * time.Second)
	}
	require.True(t, gotForbidden, "anonymous GET should eventually return 403 for private bucket")
	t.Logf("anonymous private read correctly denied (403)")

	// --- Test 7: Authenticated read should still work after setting private. ---
	getResp2, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(internalBucket),
		Key:    aws.String("test-file.txt"),
	})
	require.NoError(t, err, "authenticated GetObject should still work on private bucket")
	defer getResp2.Body.Close()
	downloaded2, _ := io.ReadAll(getResp2.Body)
	require.Equal(t, testContent, string(downloaded2))
	t.Logf("authenticated read on private bucket OK")

	// --- Test 8: Delete an object. ---
	_, err = s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(internalBucket),
		Key:    aws.String("test-file.txt"),
	})
	require.NoError(t, err, "DeleteObject should succeed")
	t.Logf("deleted test-file.txt")

	// Verify it's gone.
	listResp, err = s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(internalBucket),
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), aws.ToInt32(listResp.KeyCount), "should have 1 object left")
	t.Logf("verified object deleted, %d remaining", aws.ToInt32(listResp.KeyCount))

	// --- Cleanup ---
	httpDelete(t, coreAPIURL+"/s3-access-keys/"+keyID)
	httpDelete(t, coreAPIURL+"/s3-buckets/"+bucketID)
	waitForStatus(t, coreAPIURL+"/s3-buckets/"+bucketID, "deleted", provisionTimeout)
	t.Logf("cleanup complete")
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
	s3ShardID := findStorageShard(t, clusterID)
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
	s3ShardID := findStorageShard(t, clusterID)
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
	require.True(t, resp.StatusCode == 200 || resp.StatusCode == 202, "update quota: status %d: %s", resp.StatusCode, body)
	updated := parseJSON(t, body)
	require.Equal(t, float64(newQuota), updated["quota_bytes"])
	t.Logf("S3 bucket quota updated to %d", newQuota)
}

// findStorageShard returns the ID of the first storage shard in the cluster, or empty string if none.
func findStorageShard(t *testing.T, clusterID string) string {
	t.Helper()
	resp, body := httpGet(t, fmt.Sprintf("%s/clusters/%s/shards", coreAPIURL, clusterID))
	if resp.StatusCode != 200 {
		return ""
	}
	shards := parsePaginatedItems(t, body)
	for _, s := range shards {
		if r, _ := s["role"].(string); r == "storage" {
			id, _ := s["id"].(string)
			return id
		}
	}
	return ""
}

