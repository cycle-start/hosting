package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDashboardStats(t *testing.T) {
	resp, body := httpGet(t, coreAPIURL+"/dashboard/stats")
	require.Equal(t, 200, resp.StatusCode, "dashboard stats: %s", body)
	stats := parseJSON(t, body)
	// Verify response has some expected structure.
	require.NotNil(t, stats, "stats should be a valid JSON object")
	t.Logf("dashboard stats: %+v", stats)
}

func TestSearch(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-search")

	resp, body := httpGet(t, coreAPIURL+"/search?q="+tenantID)
	require.Equal(t, 200, resp.StatusCode, "search: %s", body)
	t.Logf("search results: %s", body)
}

func TestAuditLogs(t *testing.T) {
	resp, body := httpGet(t, coreAPIURL+"/audit-logs")
	require.Equal(t, 200, resp.StatusCode, "audit logs: %s", body)
	result := parseJSON(t, body)
	_, hasItems := result["items"]
	require.True(t, hasItems, "audit logs missing 'items' key")
	t.Logf("audit logs: %d items", len(parsePaginatedItems(t, body)))
}
