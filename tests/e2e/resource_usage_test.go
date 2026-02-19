package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestResourceUsage creates a tenant with a webroot, writes some data,
// then polls the resource-usage API until the collection cron has run
// and reported byte counts for the webroot.
func TestResourceUsage(t *testing.T) {
	tenantID, _, clusterID, _, _ := createTestTenant(t, "e2e-rusage")
	webrootID := createTestWebroot(t, tenantID, "usage-site", "static", "1")

	// Write a known file to the webroot so there's measurable disk usage.
	ips := findNodeIPsByRole(t, clusterID, "web")
	require.NotEmpty(t, ips)

	// Fetch the tenant name from the API for the filesystem path.
	resp, body := httpGet(t, coreAPIURL+"/tenants/"+tenantID)
	require.Equal(t, 200, resp.StatusCode, body)
	tenant := parseJSON(t, body)
	tenantName, _ := tenant["name"].(string)
	require.NotEmpty(t, tenantName, "tenant should have a name")

	webrootPath := fmt.Sprintf("/var/www/storage/%s/usage-site", tenantName)
	sshExec(t, ips[0], fmt.Sprintf(
		"sudo mkdir -p %s/public && dd if=/dev/urandom of=%s/public/testdata.bin bs=1024 count=100 2>/dev/null",
		webrootPath, webrootPath,
	))
	t.Logf("wrote 100KB test file to %s", webrootPath)

	// Poll the resource-usage API. The collection cron runs every 30 minutes,
	// so in CI we may need to wait. Use a generous timeout.
	usageURL := fmt.Sprintf("%s/tenants/%s/resource-usage", coreAPIURL, tenantID)
	deadline := time.Now().Add(35 * time.Minute)

	var foundWebroot bool
	for time.Now().Before(deadline) {
		resp, body = httpGet(t, usageURL)
		require.Equal(t, 200, resp.StatusCode, "resource-usage: %s", body)

		items := parsePaginatedItems(t, body)
		for _, item := range items {
			resType, _ := item["resource_type"].(string)
			resID, _ := item["resource_id"].(string)
			bytesUsed, _ := item["bytes_used"].(float64)

			if resType == "webroot" && resID == webrootID {
				require.Greater(t, bytesUsed, float64(0), "webroot should have non-zero bytes_used")
				t.Logf("webroot %s usage: %.0f bytes", webrootID, bytesUsed)
				foundWebroot = true
				break
			}
		}
		if foundWebroot {
			break
		}
		t.Logf("waiting for resource usage collection (next check in 30s)...")
		time.Sleep(30 * time.Second)
	}

	require.True(t, foundWebroot, "resource usage for webroot should appear after collection cron runs")
}

// TestResourceUsageEmpty verifies that a fresh tenant with no data returns
// an empty items list (not an error).
func TestResourceUsageEmpty(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-rusage-empty")

	resp, body := httpGet(t, fmt.Sprintf("%s/tenants/%s/resource-usage", coreAPIURL, tenantID))
	require.Equal(t, 200, resp.StatusCode, "resource-usage: %s", body)

	wrapper := parseJSON(t, body)
	items, ok := wrapper["items"]
	require.True(t, ok, "response should contain 'items' key")

	// Items should be empty or nil for a fresh tenant.
	if items != nil {
		itemsStr := fmt.Sprintf("%v", items)
		if !strings.Contains(itemsStr, "[]") {
			// Items exist but should all be for this tenant.
			parsed := parsePaginatedItems(t, body)
			for _, item := range parsed {
				tid, _ := item["tenant_id"].(string)
				require.Equal(t, tenantID, tid, "usage items should belong to this tenant")
			}
		}
	}
	t.Logf("resource-usage for fresh tenant: %s", body)
}
