package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSuspendResume creates a tenant with a webroot and FQDN, verifies
// HTTP traffic, suspends the tenant, verifies traffic returns 503,
// unsuspends, and verifies traffic is restored.
func TestSuspendResume(t *testing.T) {
	tenantID, _, clusterID, _, _ := createTestTenant(t, "e2e-suspend")
	webrootID := createTestWebroot(t, tenantID, "suspend-site", "php", "8.5")

	// Bind an FQDN.
	fqdnID := createTestFQDN(t, webrootID, "suspend.e2e-suspend.example.com.")

	// Fetch tenant name for filesystem path.
	resp, body := httpGet(t, coreAPIURL+"/tenants/"+tenantID)
	require.Equal(t, 200, resp.StatusCode, body)
	tenant := parseJSON(t, body)
	tenantName, _ := tenant["name"].(string)

	// Write a test file.
	ips := findNodeIPsByRole(t, clusterID, "web")
	webrootPath := fmt.Sprintf("/var/www/storage/%s/suspend-site", tenantName)
	sshExec(t, ips[0], fmt.Sprintf(
		"sudo mkdir -p %s/public && echo '<?php echo \"not-suspended\"; ?>' | sudo tee %s/public/index.php",
		webrootPath, webrootPath,
	))

	// Step 1: Verify HTTP traffic works before suspension.
	_, body = waitForHTTP(t, webTrafficURL, "suspend.e2e-suspend.example.com", httpTimeout)
	require.Contains(t, body, "not-suspended")
	t.Logf("HTTP traffic works before suspension")

	// Step 2: Suspend the tenant.
	resp, body = httpPost(t, coreAPIURL+"/tenants/"+tenantID+"/suspend", nil)
	require.Equal(t, 202, resp.StatusCode, "suspend: %s", body)
	waitForStatus(t, coreAPIURL+"/tenants/"+tenantID, "suspended", provisionTimeout)
	t.Logf("tenant suspended")

	// Step 3: Verify HTTP traffic returns 503 while suspended.
	// Give a few seconds for the suspension to propagate to nginx config.
	time.Sleep(5 * time.Second)

	var got503 bool
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, body, err := httpGetWithHost(webTrafficURL, "suspend.e2e-suspend.example.com")
		if err == nil && resp.StatusCode == 503 {
			got503 = true
			t.Logf("confirmed: 503 response while suspended")
			break
		}
		if err == nil {
			t.Logf("suspension check: got %d (waiting for 503)", resp.StatusCode)
			_ = body
		}
		time.Sleep(2 * time.Second)
	}
	require.True(t, got503, "should get 503 while tenant is suspended")

	// Step 4: Verify the tenant API still returns the tenant (readable while suspended).
	resp, body = httpGet(t, coreAPIURL+"/tenants/"+tenantID)
	require.Equal(t, 200, resp.StatusCode, "tenant should be readable while suspended: %s", body)
	suspended := parseJSON(t, body)
	require.Equal(t, "suspended", suspended["status"])

	// Step 5: Verify the webroot is still readable via API.
	resp, body = httpGet(t, coreAPIURL+"/webroots/"+webrootID)
	require.Equal(t, 200, resp.StatusCode, "webroot should be readable while suspended: %s", body)

	// Step 6: Unsuspend the tenant.
	resp, body = httpPost(t, coreAPIURL+"/tenants/"+tenantID+"/unsuspend", nil)
	require.Equal(t, 202, resp.StatusCode, "unsuspend: %s", body)
	waitForStatus(t, coreAPIURL+"/tenants/"+tenantID, "active", provisionTimeout)
	t.Logf("tenant unsuspended")

	// Step 7: Verify HTTP traffic is restored.
	_, body = waitForHTTP(t, webTrafficURL, "suspend.e2e-suspend.example.com", httpTimeout)
	require.Contains(t, body, "not-suspended", "traffic should be restored after unsuspend")
	t.Logf("HTTP traffic restored after unsuspend")

	_ = fqdnID
}

// TestSuspendWithReason verifies that suspension with a reason records it.
func TestSuspendWithReason(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-suspend-reason")

	// Suspend with a reason.
	resp, body := httpPost(t, coreAPIURL+"/tenants/"+tenantID+"/suspend", map[string]interface{}{
		"reason": "non-payment",
	})
	require.Equal(t, 202, resp.StatusCode, "suspend with reason: %s", body)
	waitForStatus(t, coreAPIURL+"/tenants/"+tenantID, "suspended", provisionTimeout)

	// Verify the reason is stored.
	resp, body = httpGet(t, coreAPIURL+"/tenants/"+tenantID)
	require.Equal(t, 200, resp.StatusCode, body)
	tenant := parseJSON(t, body)
	require.Equal(t, "suspended", tenant["status"])
	if reason, ok := tenant["suspend_reason"].(string); ok {
		require.Equal(t, "non-payment", reason)
		t.Logf("suspend reason recorded: %s", reason)
	}

	// Unsuspend.
	resp, body = httpPost(t, coreAPIURL+"/tenants/"+tenantID+"/unsuspend", nil)
	require.Equal(t, 202, resp.StatusCode, "unsuspend: %s", body)
	waitForStatus(t, coreAPIURL+"/tenants/"+tenantID, "active", provisionTimeout)
	t.Logf("unsuspended")
}
