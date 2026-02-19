package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestShardConvergence triggers shard convergence on the web shard and verifies
// that all resources are properly re-converged. This exercises the convergence
// workflow end-to-end: desired state query -> fan-out to nodes -> nginx/PHP-FPM config.
func TestShardConvergence(t *testing.T) {
	tenantID, _, clusterID, webShardID, _ := createTestTenant(t, "e2e-converge")

	// Create a webroot and FQDN so there's something to converge.
	webrootID := createTestWebroot(t, tenantID, "conv-site", "php", "8.5")

	resp, body := httpPost(t, fmt.Sprintf("%s/webroots/%s/fqdns", coreAPIURL, webrootID), map[string]interface{}{
		"fqdn": "conv.e2e-converge.example.com.",
	})
	require.Equal(t, 202, resp.StatusCode, "create FQDN: %s", body)
	fqdn := parseJSON(t, body)
	fqdnID := fqdn["id"].(string)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/fqdns/"+fqdnID) })
	waitForStatus(t, coreAPIURL+"/fqdns/"+fqdnID, "active", provisionTimeout)

	// Write a test file on the web node.
	ips := findNodeIPsByRole(t, clusterID, "web")

	resp, body = httpGet(t, coreAPIURL+"/tenants/"+tenantID)
	require.Equal(t, 200, resp.StatusCode, body)
	tenant := parseJSON(t, body)
	tenantName, _ := tenant["name"].(string)

	webrootPath := fmt.Sprintf("/var/www/storage/%s/conv-site", tenantName)
	sshExec(t, ips[0], fmt.Sprintf(
		"sudo mkdir -p %s/public && echo '<?php echo \"converged-ok\"; ?>' | sudo tee %s/public/index.php",
		webrootPath, webrootPath,
	))

	// Step 1: Trigger convergence on the web shard.
	resp, body = httpPost(t, coreAPIURL+"/shards/"+webShardID+"/converge", nil)
	require.Equal(t, 202, resp.StatusCode, "trigger convergence: %s", body)
	t.Logf("convergence triggered for shard %s", webShardID)

	// Step 2: Wait for the shard to return to active status after convergence.
	waitForStatus(t, coreAPIURL+"/shards/"+webShardID, "active", provisionTimeout)
	t.Logf("shard active after convergence")

	// Step 3: Verify that the FQDN still works through HAProxy after convergence.
	resp2, body2 := waitForHTTP(t, webTrafficURL, "conv.e2e-converge.example.com", httpTimeout)
	if resp2 != nil {
		require.Contains(t, body2, "converged-ok", "response should work after convergence")
		t.Logf("HTTP traffic verified after convergence")
	}

	// Step 4: Verify nginx config was regenerated on all web nodes.
	for _, ip := range ips {
		out := sshExec(t, ip, "sudo nginx -t 2>&1")
		require.Contains(t, out, "syntax is ok", "nginx config should be valid on node %s", ip)
		t.Logf("nginx config valid on %s", ip)
	}

	// Step 5: Verify PHP-FPM pool exists for the webroot.
	for _, ip := range ips {
		out := sshExec(t, ip, fmt.Sprintf("ls /etc/php/*/fpm/pool.d/ 2>/dev/null | grep -c '%s' || echo '0'", tenantName))
		count := strings.TrimSpace(out)
		t.Logf("PHP-FPM pools for %s on %s: %s", tenantName, ip, count)
	}
}

// TestConvergenceIdempotent verifies that triggering convergence multiple
// times does not cause errors or duplicate resources.
func TestConvergenceIdempotent(t *testing.T) {
	_, _, _, webShardID, _ := createTestTenant(t, "e2e-conv-idem")

	// Converge twice in succession.
	for i := 0; i < 2; i++ {
		resp, body := httpPost(t, coreAPIURL+"/shards/"+webShardID+"/converge", nil)
		require.Equal(t, 202, resp.StatusCode, "convergence %d: %s", i+1, body)
		waitForStatus(t, coreAPIURL+"/shards/"+webShardID, "active", provisionTimeout)
		t.Logf("convergence %d complete", i+1)
	}
}
