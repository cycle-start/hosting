package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTenantResourceSummary(t *testing.T) {
	tenantID, regionID, clusterID, _, dbShardID := createTestTenant(t, "e2e-summary")

	// Create some resources.
	createTestWebroot(t, tenantID, "summary-site", "php", "8.5")

	if dbShardID != "" {
		subID := createTestSubscription(t, tenantID, "e2e-summary")
		_ = createTestDatabase(t, tenantID, dbShardID, subID)
	}

	createTestZone(t, tenantID, regionID, "e2e-summary.example.com.")

	// Get resource summary.
	resp, body := httpGet(t, fmt.Sprintf("%s/tenants/%s/resource-summary", coreAPIURL, tenantID))
	require.Equal(t, 200, resp.StatusCode, "resource summary: %s", body)
	summary := parseJSON(t, body)
	t.Logf("resource summary: %+v", summary)

	_ = clusterID
}

func TestTenantRetryEndpoints(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-retry-ep")

	// POST /tenants/{id}/retry -> should accept.
	resp, body := httpPost(t, coreAPIURL+"/tenants/"+tenantID+"/retry", nil)
	require.True(t, resp.StatusCode == 202 || resp.StatusCode == 200,
		"retry: status %d body=%s", resp.StatusCode, body)
	t.Logf("tenant retry accepted: %d", resp.StatusCode)

	// POST /tenants/{id}/retry-failed -> should accept.
	resp, body = httpPost(t, coreAPIURL+"/tenants/"+tenantID+"/retry-failed", nil)
	require.True(t, resp.StatusCode == 202 || resp.StatusCode == 200,
		"retry-failed: status %d body=%s", resp.StatusCode, body)
	t.Logf("tenant retry-failed accepted: %d", resp.StatusCode)
}

func TestTenantSuspendedResourceAccess(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-suspend-access")
	webrootID := createTestWebroot(t, tenantID, "suspend-site", "static", "1")

	// Suspend the tenant.
	resp, body := httpPost(t, coreAPIURL+"/tenants/"+tenantID+"/suspend", nil)
	require.Equal(t, 202, resp.StatusCode, "suspend: %s", body)
	waitForStatus(t, coreAPIURL+"/tenants/"+tenantID, "suspended", provisionTimeout)
	t.Logf("tenant suspended")

	// Webroot should still be readable.
	resp, body = httpGet(t, coreAPIURL+"/webroots/"+webrootID)
	require.Equal(t, 200, resp.StatusCode, "webroot should be readable while suspended: %s", body)
	t.Logf("webroot readable while tenant suspended")

	// Unsuspend.
	resp, body = httpPost(t, coreAPIURL+"/tenants/"+tenantID+"/unsuspend", nil)
	require.Equal(t, 202, resp.StatusCode, "unsuspend: %s", body)
	waitForStatus(t, coreAPIURL+"/tenants/"+tenantID, "active", provisionTimeout)
	t.Logf("tenant unsuspended")
}
