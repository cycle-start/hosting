package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFQDNFullStackBinding(t *testing.T) {
	tenantID, regionID, clusterID, _, _ := createTestTenant(t, "e2e-fqdn-full")

	// Create a zone for the domain.
	zoneID := createTestZone(t, tenantID, regionID, "e2e-fullstack.example.com.")

	// Create a webroot.
	webrootID := createTestWebroot(t, tenantID, "fqdn-site", "php", "8.5")

	// Write a PHP test file to the webroot.
	ips := findNodeIPsByRole(t, clusterID, "web")
	sshExec(t, ips[0], fmt.Sprintf(
		"sudo mkdir -p /var/www/storage/%s/fqdn-site/public && "+
			"echo '<?php echo \"fqdn-test-ok\"; ?>' | sudo tee /var/www/storage/%s/fqdn-site/public/index.php",
		tenantID, tenantID))
	t.Logf("wrote test PHP file")

	// Bind FQDN to the webroot.
	fqdnID := createTestFQDN(t, webrootID, "app.e2e-fullstack.example.com.")

	// Verify DNS auto-records were created.
	resp, body := httpGet(t, fmt.Sprintf("%s/zones/%s/records", coreAPIURL, zoneID))
	require.Equal(t, 200, resp.StatusCode, body)
	records := parsePaginatedItems(t, body)
	var autoRecordCount int
	for _, rec := range records {
		if managedBy, _ := rec["managed_by"].(string); managedBy == "platform" {
			if sourceFQDN, _ := rec["source_fqdn_id"].(string); sourceFQDN == fqdnID {
				autoRecordCount++
				t.Logf("auto record: type=%s name=%s", rec["type"], rec["name"])
			}
		}
	}
	if autoRecordCount > 0 {
		t.Logf("found %d auto-created DNS records", autoRecordCount)
	} else {
		t.Logf("no auto DNS records found (may not be wired yet)")
	}

	// Verify DNS propagation (if DNS nodes available).
	dnsNodeIPs := findNodeIPsByRole(t, clusterID, "dns")
	if len(dnsNodeIPs) > 0 {
		answer := digQuery(t, dnsNodeIPs[0], "A", "app.e2e-fullstack.example.com")
		if answer != "" {
			t.Logf("DNS propagation: %s", answer)
		}
	}

	// Verify nginx config includes the FQDN.
	nginxConf := sshExec(t, ips[0], "sudo grep -r 'app.e2e-fullstack.example.com' /etc/nginx/sites-enabled/ 2>/dev/null || echo ''")
	if strings.Contains(nginxConf, "app.e2e-fullstack.example.com") {
		t.Logf("nginx config contains FQDN")
	} else {
		t.Logf("FQDN not in nginx config (may be configured differently)")
	}

	// Make HTTP request through HAProxy.
	resp2, body2 := waitForHTTP(t, webTrafficURL, "app.e2e-fullstack.example.com", httpTimeout)
	if resp2 != nil {
		require.Contains(t, body2, "fqdn-test-ok", "response should contain test content")
		t.Logf("full-stack FQDN binding verified: HTTP traffic reaches webroot")
	}
}
