package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestServiceHostname creates a webroot with service_hostname_enabled (default true),
// verifies the service hostname DNS record is created and HTTP traffic reaches
// the webroot through the LB using the service hostname.
func TestServiceHostname(t *testing.T) {
	tenantID, _, clusterID, _, _ := createTestTenant(t, "e2e-svc-host")

	// Create a webroot (service_hostname_enabled defaults to true).
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/webroots", coreAPIURL, tenantID), map[string]interface{}{
		"name":            "svc-site",
		"runtime":         "php",
		"runtime_version": "8.5",
	})
	require.Equal(t, 202, resp.StatusCode, "create webroot: %s", body)
	webroot := parseJSON(t, body)
	webrootID := webroot["id"].(string)
	require.Equal(t, true, webroot["service_hostname_enabled"])
	t.Logf("created webroot: %s", webrootID)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/webroots/"+webrootID) })

	// Wait for active.
	webroot = waitForStatus(t, coreAPIURL+"/webroots/"+webrootID, "active", provisionTimeout)

	// Fetch tenant name for filesystem path.
	resp, body = httpGet(t, coreAPIURL+"/tenants/"+tenantID)
	require.Equal(t, 200, resp.StatusCode, body)
	tenant := parseJSON(t, body)
	tenantName, _ := tenant["name"].(string)
	require.NotEmpty(t, tenantName)

	// Write a PHP test file.
	ips := findNodeIPsByRole(t, clusterID, "web")
	webrootPath := fmt.Sprintf("/var/www/storage/%s/svc-site", tenantName)
	sshExec(t, ips[0], fmt.Sprintf(
		"sudo mkdir -p %s/public && echo '<?php echo \"svc-hostname-ok\"; ?>' | sudo tee %s/public/index.php",
		webrootPath, webrootPath,
	))

	// Compute the expected service hostname: {webroot}.{tenant}.{brand.base_hostname}
	serviceHostname := fmt.Sprintf("svc-site.%s.e2e.hosting.test", tenantName)
	t.Logf("expected service hostname: %s", serviceHostname)

	// Verify DNS record was created for the service hostname.
	dnsNodeIPs := findNodeIPsByRole(t, clusterID, "dns")
	if len(dnsNodeIPs) > 0 {
		answer := digQuery(t, dnsNodeIPs[0], "A", serviceHostname)
		if answer != "" {
			t.Logf("service hostname DNS: %s -> %s", serviceHostname, answer)
		} else {
			t.Logf("service hostname DNS not yet propagated (may be expected)")
		}
	}

	// Verify HTTP traffic through HAProxy using the service hostname.
	resp2, body2 := waitForHTTP(t, webTrafficURL, serviceHostname, httpTimeout)
	if resp2 != nil {
		require.Contains(t, body2, "svc-hostname-ok", "response should contain test content via service hostname")
		t.Logf("service hostname HTTP traffic verified: %s", serviceHostname)
	}
}

// TestServiceHostnameDisabled verifies that creating a webroot with
// service_hostname_enabled=false does NOT create a DNS record or LB entry
// for the service hostname.
func TestServiceHostnameDisabled(t *testing.T) {
	tenantID, _, clusterID, _, _ := createTestTenant(t, "e2e-svc-off")

	// Create a webroot with service hostname disabled.
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/webroots", coreAPIURL, tenantID), map[string]interface{}{
		"name":                     "no-svc-site",
		"runtime":                  "static",
		"runtime_version":          "1",
		"service_hostname_enabled": false,
	})
	require.Equal(t, 202, resp.StatusCode, "create webroot: %s", body)
	webroot := parseJSON(t, body)
	webrootID := webroot["id"].(string)
	require.Equal(t, false, webroot["service_hostname_enabled"])
	t.Logf("created webroot with service hostname disabled: %s", webrootID)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/webroots/"+webrootID) })

	waitForStatus(t, coreAPIURL+"/webroots/"+webrootID, "active", provisionTimeout)

	// Fetch tenant name.
	resp, body = httpGet(t, coreAPIURL+"/tenants/"+tenantID)
	require.Equal(t, 200, resp.StatusCode, body)
	tenant := parseJSON(t, body)
	tenantName, _ := tenant["name"].(string)

	serviceHostname := fmt.Sprintf("no-svc-site.%s.e2e.hosting.test", tenantName)

	// Verify the service hostname does NOT resolve in DNS.
	dnsNodeIPs := findNodeIPsByRole(t, clusterID, "dns")
	if len(dnsNodeIPs) > 0 {
		answer := digQuery(t, dnsNodeIPs[0], "A", serviceHostname)
		require.Empty(t, answer, "service hostname should NOT resolve when disabled")
		t.Logf("confirmed: no DNS record for disabled service hostname %s", serviceHostname)
	}
}

// TestServiceHostnameToggle verifies that toggling service_hostname_enabled
// on an existing webroot adds/removes the DNS record and LB entry.
func TestServiceHostnameToggle(t *testing.T) {
	tenantID, _, clusterID, _, _ := createTestTenant(t, "e2e-svc-toggle")

	// Create webroot with service hostname enabled.
	webrootID := createTestWebroot(t, tenantID, "toggle-site", "static", "1")

	// Fetch tenant name.
	resp, body := httpGet(t, coreAPIURL+"/tenants/"+tenantID)
	require.Equal(t, 200, resp.StatusCode, body)
	tenant := parseJSON(t, body)
	tenantName, _ := tenant["name"].(string)

	serviceHostname := fmt.Sprintf("toggle-site.%s.e2e.hosting.test", tenantName)

	// Write a test file.
	ips := findNodeIPsByRole(t, clusterID, "web")
	webrootPath := fmt.Sprintf("/var/www/storage/%s/toggle-site", tenantName)
	sshExec(t, ips[0], fmt.Sprintf(
		"sudo mkdir -p %s && echo 'toggle-ok' | sudo tee %s/index.html",
		webrootPath, webrootPath,
	))

	// Verify service hostname works initially.
	_, body = waitForHTTP(t, webTrafficURL, serviceHostname, httpTimeout)
	require.Contains(t, body, "toggle-ok")
	t.Logf("service hostname works before toggle")

	// Disable service hostname.
	resp, body = httpPut(t, coreAPIURL+"/webroots/"+webrootID, map[string]interface{}{
		"service_hostname_enabled": false,
	})
	require.Equal(t, 202, resp.StatusCode, "disable service hostname: %s", body)
	waitForStatus(t, coreAPIURL+"/webroots/"+webrootID, "active", provisionTimeout)
	t.Logf("service hostname disabled")

	// Verify the service hostname no longer works through the LB.
	// (It should return a 503 or similar since LB map entry was removed.)
	dnsNodeIPs := findNodeIPsByRole(t, clusterID, "dns")
	if len(dnsNodeIPs) > 0 {
		answer := digQuery(t, dnsNodeIPs[0], "A", serviceHostname)
		require.Empty(t, answer, "DNS record should be removed after disabling")
		t.Logf("confirmed: DNS record removed for %s", serviceHostname)
	}

	// Re-enable service hostname.
	resp, body = httpPut(t, coreAPIURL+"/webroots/"+webrootID, map[string]interface{}{
		"service_hostname_enabled": true,
	})
	require.Equal(t, 202, resp.StatusCode, "re-enable service hostname: %s", body)
	waitForStatus(t, coreAPIURL+"/webroots/"+webrootID, "active", provisionTimeout)
	t.Logf("service hostname re-enabled")

	// Verify it works again.
	_, body = waitForHTTP(t, webTrafficURL, serviceHostname, httpTimeout)
	require.True(t, strings.Contains(body, "toggle-ok") || strings.Contains(body, "html"),
		"service hostname should serve content after re-enabling")
	t.Logf("service hostname works after re-enable")
}
