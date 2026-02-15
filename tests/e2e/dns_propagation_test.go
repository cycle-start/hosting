package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDNSRecordPropagation(t *testing.T) {
	tenantID, regionID, clusterID, _, _ := createTestTenant(t, "e2e-dns-prop")

	dnsNodeIPs := findNodeIPsByRole(t, clusterID, "dns")
	if len(dnsNodeIPs) == 0 {
		t.Skip("no dns nodes found")
	}

	zoneID := createTestZone(t, tenantID, regionID, "e2e-prop.example.com.")

	// Create an A record.
	resp, body := httpPost(t, fmt.Sprintf("%s/zones/%s/records", coreAPIURL, zoneID), map[string]interface{}{
		"type":    "A",
		"name":    "www",
		"content": "203.0.113.42",
		"ttl":     300,
	})
	require.Equal(t, 202, resp.StatusCode, "create A record: %s", body)
	record := parseJSON(t, body)
	recordID := record["id"].(string)

	waitForStatus(t, coreAPIURL+"/zone-records/"+recordID, "active", provisionTimeout)
	t.Logf("A record active")

	// Query PowerDNS directly.
	answer := digQuery(t, dnsNodeIPs[0], "A", "www.e2e-prop.example.com")
	require.Contains(t, answer, "203.0.113.42", "DNS should resolve to 203.0.113.42, got: %s", answer)
	t.Logf("DNS propagation verified: %s", answer)

	// Update the record.
	resp, body = httpPut(t, coreAPIURL+"/zone-records/"+recordID, map[string]interface{}{
		"content": "203.0.113.100",
	})
	require.Equal(t, 202, resp.StatusCode, "update record: %s", body)
	waitForStatus(t, coreAPIURL+"/zone-records/"+recordID, "active", provisionTimeout)

	// Re-query and verify new value.
	answer = digQuery(t, dnsNodeIPs[0], "A", "www.e2e-prop.example.com")
	require.Contains(t, answer, "203.0.113.100", "DNS should resolve to updated value, got: %s", answer)
	t.Logf("DNS update propagation verified: %s", answer)

	// Delete the record.
	resp, body = httpDelete(t, coreAPIURL+"/zone-records/"+recordID)
	require.Equal(t, 202, resp.StatusCode, "delete record: %s", body)
	waitForStatus(t, coreAPIURL+"/zone-records/"+recordID, "deleted", provisionTimeout)

	// Verify record no longer resolves.
	answer = digQuery(t, dnsNodeIPs[0], "A", "www.e2e-prop.example.com")
	require.Empty(t, answer, "record should no longer resolve after delete, got: %s", answer)
	t.Logf("DNS deletion propagation verified")
}

func TestDNSMultipleRecordTypePropagation(t *testing.T) {
	tenantID, regionID, clusterID, _, _ := createTestTenant(t, "e2e-dns-multi-prop")

	dnsNodeIPs := findNodeIPsByRole(t, clusterID, "dns")
	if len(dnsNodeIPs) == 0 {
		t.Skip("no dns nodes found")
	}

	zoneID := createTestZone(t, tenantID, regionID, "e2e-multiprop.example.com.")

	records := []struct {
		recType string
		name    string
		content string
		query   string
	}{
		{"A", "test-a", "10.0.0.1", "test-a.e2e-multiprop.example.com"},
		{"AAAA", "test-aaaa", "2001:db8::1", "test-aaaa.e2e-multiprop.example.com"},
		{"TXT", "test-txt", "v=spf1 -all", "test-txt.e2e-multiprop.example.com"},
	}

	for _, r := range records {
		t.Run(r.recType, func(t *testing.T) {
			resp, body := httpPost(t, fmt.Sprintf("%s/zones/%s/records", coreAPIURL, zoneID), map[string]interface{}{
				"type":    r.recType,
				"name":    r.name,
				"content": r.content,
				"ttl":     300,
			})
			require.Equal(t, 202, resp.StatusCode, "create %s: %s", r.recType, body)
			rec := parseJSON(t, body)
			recID := rec["id"].(string)
			waitForStatus(t, coreAPIURL+"/zone-records/"+recID, "active", provisionTimeout)

			answer := digQuery(t, dnsNodeIPs[0], r.recType, r.query)
			require.NotEmpty(t, answer, "%s record should resolve, got empty", r.recType)
			t.Logf("%s propagation: %s", r.recType, answer)

			httpDelete(t, coreAPIURL+"/zone-records/"+recID)
		})
	}
}
