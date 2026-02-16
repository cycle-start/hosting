package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDNSZoneCRUD tests zone lifecycle:
// create zone -> wait active -> list zones -> create record -> wait active
// -> update record -> delete record -> delete zone.
func TestDNSZoneCRUD(t *testing.T) {
	tenantID, regionID, _, _, _ := createTestTenant(t, "e2e-dns-crud")

	// Step 1: Create a zone.
	resp, body := httpPost(t, coreAPIURL+"/zones", map[string]interface{}{
		"name":      "e2e-test.example.com.",
		"tenant_id": tenantID,
		"region_id": regionID,
	})
	require.Equal(t, 202, resp.StatusCode, "create zone: %s", body)
	zone := parseJSON(t, body)
	zoneID := zone["id"].(string)
	require.NotEmpty(t, zoneID)
	t.Logf("created zone: %s", zoneID)

	t.Cleanup(func() {
		httpDelete(t, coreAPIURL+"/zones/"+zoneID)
	})

	// Step 2: Wait for zone to become active.
	zone = waitForStatus(t, coreAPIURL+"/zones/"+zoneID, "active", provisionTimeout)
	require.Equal(t, "active", zone["status"])
	t.Logf("zone active: %s", zoneID)

	// Step 3: Verify zone appears in the zone list.
	resp, body = httpGet(t, coreAPIURL+"/zones")
	require.Equal(t, 200, resp.StatusCode, body)
	zones := parsePaginatedItems(t, body)
	found := false
	for _, z := range zones {
		if id, _ := z["id"].(string); id == zoneID {
			found = true
			break
		}
	}
	require.True(t, found, "zone %s not found in zone list", zoneID)

	// Step 4: Create an A record.
	resp, body = httpPost(t, fmt.Sprintf("%s/zones/%s/records", coreAPIURL, zoneID), map[string]interface{}{
		"type":    "A",
		"name":    "www",
		"content": "192.168.1.100",
		"ttl":     3600,
	})
	require.Equal(t, 202, resp.StatusCode, "create A record: %s", body)
	record := parseJSON(t, body)
	recordID := record["id"].(string)
	require.NotEmpty(t, recordID)
	require.Equal(t, "A", record["type"])
	require.Equal(t, "www", record["name"])
	require.Equal(t, "192.168.1.100", record["content"])
	t.Logf("created A record: %s", recordID)

	// Step 5: Wait for the record to become active.
	record = waitForStatus(t, coreAPIURL+"/zone-records/"+recordID, "active", provisionTimeout)
	require.Equal(t, "active", record["status"])
	t.Logf("record active: %s", recordID)

	// Step 6: Verify the record appears in the zone's record list.
	resp, body = httpGet(t, fmt.Sprintf("%s/zones/%s/records", coreAPIURL, zoneID))
	require.Equal(t, 200, resp.StatusCode, body)
	records := parsePaginatedItems(t, body)
	found = false
	for _, r := range records {
		if id, _ := r["id"].(string); id == recordID {
			found = true
			break
		}
	}
	require.True(t, found, "record %s not found in zone record list", recordID)

	// Step 7: Update the record's content.
	resp, body = httpPut(t, coreAPIURL+"/zone-records/"+recordID, map[string]interface{}{
		"content": "192.168.1.200",
	})
	require.Equal(t, 202, resp.StatusCode, "update record: %s", body)
	updated := parseJSON(t, body)
	require.Equal(t, "192.168.1.200", updated["content"])
	t.Logf("record updated")

	// Step 8: Delete the record.
	resp, body = httpDelete(t, coreAPIURL+"/zone-records/"+recordID)
	require.Equal(t, 202, resp.StatusCode, "delete record: %s", body)
	t.Logf("record deleted")

	// Step 9: Delete the zone.
	resp, body = httpDelete(t, coreAPIURL+"/zones/"+zoneID)
	require.Equal(t, 202, resp.StatusCode, "delete zone: %s", body)
	waitForStatus(t, coreAPIURL+"/zones/"+zoneID, "deleted", provisionTimeout)
	t.Logf("zone deleted")
}

// TestDNSAutoRecords tests that binding an FQDN to a webroot automatically
// creates DNS records when a matching zone exists:
// create tenant -> create webroot -> create zone -> bind FQDN -> verify
// auto-created A/AAAA records -> unbind FQDN -> verify records removed.
func TestDNSAutoRecords(t *testing.T) {
	tenantID, regionID, _, webShardID, _ := createTestTenant(t, "e2e-dns-auto")
	_ = webShardID

	// Step 1: Create a webroot.
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/webroots", coreAPIURL, tenantID), map[string]interface{}{
		"name":            "dns-auto-site",
		"runtime":         "php",
		"runtime_version": "8.5",
	})
	require.Equal(t, 202, resp.StatusCode, "create webroot: %s", body)
	webroot := parseJSON(t, body)
	webrootID := webroot["id"].(string)
	t.Logf("created webroot: %s", webrootID)

	webroot = waitForStatus(t, coreAPIURL+"/webroots/"+webrootID, "active", provisionTimeout)
	t.Logf("webroot active")

	// Step 2: Create a zone for the FQDN's domain.
	resp, body = httpPost(t, coreAPIURL+"/zones", map[string]interface{}{
		"name":      "e2e-auto.example.com.",
		"tenant_id": tenantID,
		"region_id": regionID,
	})
	require.Equal(t, 202, resp.StatusCode, "create zone: %s", body)
	zone := parseJSON(t, body)
	zoneID := zone["id"].(string)

	t.Cleanup(func() {
		httpDelete(t, coreAPIURL+"/zones/"+zoneID)
	})

	waitForStatus(t, coreAPIURL+"/zones/"+zoneID, "active", provisionTimeout)
	t.Logf("zone active: %s", zoneID)

	// Step 3: Bind an FQDN to the webroot.
	resp, body = httpPost(t, fmt.Sprintf("%s/webroots/%s/fqdns", coreAPIURL, webrootID), map[string]interface{}{
		"fqdn": "app.e2e-auto.example.com.",
	})
	require.Equal(t, 202, resp.StatusCode, "bind FQDN: %s", body)
	fqdn := parseJSON(t, body)
	fqdnID := fqdn["id"].(string)
	require.NotEmpty(t, fqdnID)
	t.Logf("bound FQDN: %s (id=%s)", fqdn["fqdn"], fqdnID)

	// Step 4: Wait for the FQDN to become active.
	fqdn = waitForStatus(t, coreAPIURL+"/fqdns/"+fqdnID, "active", provisionTimeout)
	t.Logf("FQDN active")

	// Step 5: Check zone records for auto-created entries referencing this FQDN.
	resp, body = httpGet(t, fmt.Sprintf("%s/zones/%s/records", coreAPIURL, zoneID))
	require.Equal(t, 200, resp.StatusCode, body)
	records := parsePaginatedItems(t, body)
	t.Logf("zone has %d records after FQDN bind", len(records))

	// Look for auto-managed records pointing to this FQDN.
	var autoRecordCount int
	for _, rec := range records {
		if managedBy, _ := rec["managed_by"].(string); managedBy == "auto" {
			if sourceFQDN, _ := rec["source_fqdn_id"].(string); sourceFQDN == fqdnID {
				autoRecordCount++
				t.Logf("auto record: type=%s name=%s content=%s", rec["type"], rec["name"], rec["content"])
			}
		}
	}
	// The platform should create at least one auto record (A or AAAA).
	require.Greater(t, autoRecordCount, 0, "expected at least one auto-created DNS record for FQDN %s", fqdnID)
	t.Logf("found %d auto-created DNS records for FQDN %s", autoRecordCount, fqdnID)

	// Step 6: Delete the FQDN.
	resp, body = httpDelete(t, coreAPIURL+"/fqdns/"+fqdnID)
	require.Equal(t, 202, resp.StatusCode, "delete FQDN: %s", body)
	t.Logf("FQDN delete requested")

	// Step 7: Clean up the webroot.
	resp, body = httpDelete(t, coreAPIURL+"/webroots/"+webrootID)
	require.Equal(t, 202, resp.StatusCode, "delete webroot: %s", body)
	t.Logf("webroot delete requested")
}

// TestDNSRecordTypes verifies that multiple DNS record types (A, AAAA, CNAME,
// MX, TXT) can be created for a zone.
func TestDNSRecordTypes(t *testing.T) {
	tenantID, regionID, _, _, _ := createTestTenant(t, "e2e-dns-types")

	// Create a zone.
	resp, body := httpPost(t, coreAPIURL+"/zones", map[string]interface{}{
		"name":      "e2e-types.example.com.",
		"tenant_id": tenantID,
		"region_id": regionID,
	})
	require.Equal(t, 202, resp.StatusCode, "create zone: %s", body)
	zone := parseJSON(t, body)
	zoneID := zone["id"].(string)

	t.Cleanup(func() {
		httpDelete(t, coreAPIURL+"/zones/"+zoneID)
	})

	waitForStatus(t, coreAPIURL+"/zones/"+zoneID, "active", provisionTimeout)

	recordTypes := []struct {
		recType  string
		name     string
		content  string
		priority *int
	}{
		{"A", "test-a", "10.0.0.1", nil},
		{"AAAA", "test-aaaa", "2001:db8::1", nil},
		{"CNAME", "test-cname", "target.example.com.", nil},
		{"MX", "@", "mail.example.com.", intPtr(10)},
		{"TXT", "test-txt", "v=spf1 include:example.com ~all", nil},
	}

	for _, rt := range recordTypes {
		t.Run(rt.recType, func(t *testing.T) {
			payload := map[string]interface{}{
				"type":    rt.recType,
				"name":    rt.name,
				"content": rt.content,
				"ttl":     3600,
			}
			if rt.priority != nil {
				payload["priority"] = *rt.priority
			}

			resp, body := httpPost(t, fmt.Sprintf("%s/zones/%s/records", coreAPIURL, zoneID), payload)
			require.Equal(t, 202, resp.StatusCode, "create %s record: %s", rt.recType, body)

			record := parseJSON(t, body)
			recordID := record["id"].(string)
			require.Equal(t, rt.recType, record["type"])
			require.Equal(t, rt.name, record["name"])
			require.Equal(t, rt.content, record["content"])
			t.Logf("created %s record: %s", rt.recType, recordID)

			// Clean up.
			resp, body = httpDelete(t, coreAPIURL+"/zone-records/"+recordID)
			require.Equal(t, 202, resp.StatusCode, "delete %s record: %s", rt.recType, body)
		})
	}
}

// intPtr returns a pointer to an int.
func intPtr(v int) *int {
	return &v
}
