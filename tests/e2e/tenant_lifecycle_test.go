package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTenantLifecycle tests the full tenant lifecycle:
// create -> wait for active -> update -> suspend -> unsuspend -> delete -> verify gone.
func TestTenantLifecycle(t *testing.T) {
	regionID := findFirstRegionID(t)
	cluster := findFirstCluster(t, regionID)
	clusterID, _ := cluster["id"].(string)
	webShard := findShardByRole(t, clusterID, "web")
	webShardID, _ := webShard["id"].(string)

	// Step 1: Create a tenant.
	brandID := findOrCreateBrand(t)
	resp, body := httpPost(t, coreAPIURL+"/tenants", map[string]interface{}{
		"brand_id":   brandID,
		"region_id":  regionID,
		"cluster_id": clusterID,
		"shard_id":   webShardID,
	})
	require.Equal(t, 202, resp.StatusCode, "create tenant: %s", body)
	tenant := parseJSON(t, body)
	tenantID := tenant["id"].(string)
	require.NotEmpty(t, tenantID)
	t.Logf("created tenant: %s", tenantID)

	// Step 2: Wait for tenant to become active.
	tenant = waitForStatus(t, coreAPIURL+"/tenants/"+tenantID, "active", provisionTimeout)
	require.Equal(t, "active", tenant["status"])
	t.Logf("tenant active: %s", tenantID)

	// Step 3: Update tenant (enable SFTP).
	sftpEnabled := true
	resp, body = httpPut(t, coreAPIURL+"/tenants/"+tenantID, map[string]interface{}{
		"sftp_enabled": sftpEnabled,
	})
	require.Equal(t, 202, resp.StatusCode, "update tenant: %s", body)
	updated := parseJSON(t, body)
	require.Equal(t, true, updated["sftp_enabled"])
	t.Logf("tenant updated: sftp_enabled=%v", updated["sftp_enabled"])

	// Step 4: Suspend the tenant.
	resp, body = httpPost(t, coreAPIURL+"/tenants/"+tenantID+"/suspend", nil)
	require.Equal(t, 202, resp.StatusCode, "suspend tenant: %s", body)
	tenant = waitForStatus(t, coreAPIURL+"/tenants/"+tenantID, "suspended", provisionTimeout)
	require.Equal(t, "suspended", tenant["status"])
	t.Logf("tenant suspended")

	// Step 5: Unsuspend the tenant.
	resp, body = httpPost(t, coreAPIURL+"/tenants/"+tenantID+"/unsuspend", nil)
	require.Equal(t, 202, resp.StatusCode, "unsuspend tenant: %s", body)
	tenant = waitForStatus(t, coreAPIURL+"/tenants/"+tenantID, "active", provisionTimeout)
	require.Equal(t, "active", tenant["status"])
	t.Logf("tenant unsuspended")

	// Step 6: Delete the tenant.
	resp, body = httpDelete(t, coreAPIURL+"/tenants/"+tenantID)
	require.Equal(t, 202, resp.StatusCode, "delete tenant: %s", body)

	// Step 7: Wait for the tenant to reach deleted state (or 404).
	tenant = waitForStatus(t, coreAPIURL+"/tenants/"+tenantID, "deleted", provisionTimeout)
	t.Logf("tenant deleted")
}

// TestTenantListPagination verifies that the tenant list endpoint returns a
// paginated response with the expected structure.
func TestTenantListPagination(t *testing.T) {
	resp, body := httpGet(t, coreAPIURL+"/tenants")
	require.Equal(t, 200, resp.StatusCode, body)

	result := parseJSON(t, body)
	// The response must contain "items" and "has_more" keys.
	_, hasItems := result["items"]
	require.True(t, hasItems, "response missing 'items' key")
	_, hasMore := result["has_more"]
	require.True(t, hasMore, "response missing 'has_more' key")
}

// TestTenantCreateValidation verifies that creating a tenant with missing
// required fields returns a 400 error.
func TestTenantCreateValidation(t *testing.T) {
	// Missing all required fields.
	resp, body := httpPost(t, coreAPIURL+"/tenants", map[string]interface{}{})
	require.Equal(t, 400, resp.StatusCode, "expected 400 for empty body: %s", body)
	errResp := parseJSON(t, body)
	_, hasError := errResp["error"]
	require.True(t, hasError, "error response missing 'error' key")
}

// TestTenantGetNotFound verifies that fetching a non-existent tenant returns 404.
func TestTenantGetNotFound(t *testing.T) {
	resp, _ := httpGet(t, coreAPIURL+"/tenants/nonexistent-id-12345")
	require.Equal(t, 404, resp.StatusCode)
}
