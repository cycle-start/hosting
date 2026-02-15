package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebrootRetry(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-wr-retry")
	webrootID := createTestWebroot(t, tenantID, "retry-site", "php", "8.5")

	resp, body := httpPost(t, coreAPIURL+"/webroots/"+webrootID+"/retry", nil)
	require.True(t, resp.StatusCode == 202 || resp.StatusCode == 200,
		"webroot retry: status %d body=%s", resp.StatusCode, body)
	t.Logf("webroot retry accepted: %d", resp.StatusCode)

	waitForStatus(t, coreAPIURL+"/webroots/"+webrootID, "active", provisionTimeout)
	t.Logf("webroot active after retry")
}

func TestDatabaseRetry(t *testing.T) {
	tenantID, _, _, _, dbShardID := createTestTenant(t, "e2e-db-retry")
	if dbShardID == "" {
		t.Skip("no database shard found")
	}

	dbID := createTestDatabase(t, tenantID, dbShardID, "e2e_retrydb")

	resp, body := httpPost(t, coreAPIURL+"/databases/"+dbID+"/retry", nil)
	require.True(t, resp.StatusCode == 202 || resp.StatusCode == 200,
		"database retry: status %d body=%s", resp.StatusCode, body)
	t.Logf("database retry accepted: %d", resp.StatusCode)

	waitForStatus(t, coreAPIURL+"/databases/"+dbID, "active", provisionTimeout)
	t.Logf("database active after retry")
}

func TestZoneRecordRetry(t *testing.T) {
	tenantID, regionID, _, _, _ := createTestTenant(t, "e2e-zr-retry")
	zoneID := createTestZone(t, tenantID, regionID, "e2e-retry.example.com.")

	// Create record.
	resp, body := httpPost(t, fmt.Sprintf("%s/zones/%s/records", coreAPIURL, zoneID), map[string]interface{}{
		"type":    "A",
		"name":    "retry-test",
		"content": "10.0.0.1",
		"ttl":     300,
	})
	require.Equal(t, 202, resp.StatusCode, "create record: %s", body)
	record := parseJSON(t, body)
	recordID := record["id"].(string)
	waitForStatus(t, coreAPIURL+"/zone-records/"+recordID, "active", provisionTimeout)

	// Retry.
	resp, body = httpPost(t, coreAPIURL+"/zone-records/"+recordID+"/retry", nil)
	require.True(t, resp.StatusCode == 202 || resp.StatusCode == 200,
		"record retry: status %d body=%s", resp.StatusCode, body)
	waitForStatus(t, coreAPIURL+"/zone-records/"+recordID, "active", provisionTimeout)
	t.Logf("zone record retry ok")

	httpDelete(t, coreAPIURL+"/zone-records/"+recordID)
}

func TestFQDNRetry(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-fqdn-retry")
	webrootID := createTestWebroot(t, tenantID, "retry-fqdn-site", "static", "1")
	fqdnID := createTestFQDN(t, webrootID, "retry.e2e-fqdn.example.com.")

	resp, body := httpPost(t, coreAPIURL+"/fqdns/"+fqdnID+"/retry", nil)
	require.True(t, resp.StatusCode == 202 || resp.StatusCode == 200,
		"FQDN retry: status %d body=%s", resp.StatusCode, body)
	waitForStatus(t, coreAPIURL+"/fqdns/"+fqdnID, "active", provisionTimeout)
	t.Logf("FQDN retry ok")
}

func TestSSHKeyRetry(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-ssh-retry")
	pubKey := generateSSHKeyPair(t)

	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/ssh-keys", coreAPIURL, tenantID), map[string]interface{}{
		"name":       "retry-key",
		"public_key": pubKey,
	})
	require.Equal(t, 202, resp.StatusCode, "create SSH key: %s", body)
	sshKey := parseJSON(t, body)
	keyID := sshKey["id"].(string)
	waitForStatus(t, coreAPIURL+"/ssh-keys/"+keyID, "active", provisionTimeout)

	resp, body = httpPost(t, coreAPIURL+"/ssh-keys/"+keyID+"/retry", nil)
	require.True(t, resp.StatusCode == 202 || resp.StatusCode == 200,
		"SSH key retry: status %d body=%s", resp.StatusCode, body)
	waitForStatus(t, coreAPIURL+"/ssh-keys/"+keyID, "active", provisionTimeout)
	t.Logf("SSH key retry ok")

	httpDelete(t, coreAPIURL+"/ssh-keys/"+keyID)
}

func TestS3BucketRetry(t *testing.T) {
	tenantID, _, clusterID, _, _ := createTestTenant(t, "e2e-s3-retry")
	s3ShardID := findStorageShard(t, clusterID)
	if s3ShardID == "" {
		t.Skip("no S3 shard found")
	}

	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/s3-buckets", coreAPIURL, tenantID), map[string]interface{}{
		"name":     "e2e-retry-bucket",
		"shard_id": s3ShardID,
	})
	require.Equal(t, 202, resp.StatusCode, "create bucket: %s", body)
	bucket := parseJSON(t, body)
	bucketID := bucket["id"].(string)
	waitForStatus(t, coreAPIURL+"/s3-buckets/"+bucketID, "active", provisionTimeout)

	resp, body = httpPost(t, coreAPIURL+"/s3-buckets/"+bucketID+"/retry", nil)
	require.True(t, resp.StatusCode == 202 || resp.StatusCode == 200,
		"S3 bucket retry: status %d body=%s", resp.StatusCode, body)
	waitForStatus(t, coreAPIURL+"/s3-buckets/"+bucketID, "active", provisionTimeout)
	t.Logf("S3 bucket retry ok")

	httpDelete(t, coreAPIURL+"/s3-buckets/"+bucketID)
}

func TestValkeyRetry(t *testing.T) {
	tenantID, _, clusterID, _, _ := createTestTenant(t, "e2e-valkey-retry")
	valkeyShardID := findValkeyShardID(t, clusterID)
	if valkeyShardID == "" {
		t.Skip("no valkey shard found")
	}

	instanceID := createTestValkeyInstance(t, tenantID, valkeyShardID, "e2e-retry-cache")

	resp, body := httpPost(t, coreAPIURL+"/valkey-instances/"+instanceID+"/retry", nil)
	require.True(t, resp.StatusCode == 202 || resp.StatusCode == 200,
		"valkey retry: status %d body=%s", resp.StatusCode, body)
	waitForStatus(t, coreAPIURL+"/valkey-instances/"+instanceID, "active", provisionTimeout)
	t.Logf("valkey instance retry ok")
}
