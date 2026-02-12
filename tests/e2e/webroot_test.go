//go:build e2e

package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestWebrootLifecycle tests the full webroot lifecycle:
// create tenant -> create webroot -> wait active -> update -> bind FQDN
// -> list FQDNs -> delete FQDN -> delete webroot.
func TestWebrootLifecycle(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-webroot")

	// Step 1: Create a webroot.
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/webroots", coreAPIURL, tenantID), map[string]interface{}{
		"name":            "main-site",
		"runtime":         "php",
		"runtime_version": "8.5",
		"public_folder":   "public",
	})
	require.Equal(t, 202, resp.StatusCode, "create webroot: %s", body)
	webroot := parseJSON(t, body)
	webrootID := webroot["id"].(string)
	require.NotEmpty(t, webrootID)
	require.Equal(t, "php", webroot["runtime"])
	require.Equal(t, "8.5", webroot["runtime_version"])
	require.Equal(t, "public", webroot["public_folder"])
	t.Logf("created webroot: %s", webrootID)

	// Step 2: Wait for webroot to become active.
	webroot = waitForStatus(t, coreAPIURL+"/webroots/"+webrootID, "active", provisionTimeout)
	require.Equal(t, "active", webroot["status"])
	t.Logf("webroot active")

	// Step 3: Verify webroot appears in the tenant's webroot list.
	resp, body = httpGet(t, fmt.Sprintf("%s/tenants/%s/webroots", coreAPIURL, tenantID))
	require.Equal(t, 200, resp.StatusCode, body)
	webroots := parsePaginatedItems(t, body)
	found := false
	for _, w := range webroots {
		if id, _ := w["id"].(string); id == webrootID {
			found = true
			break
		}
	}
	require.True(t, found, "webroot %s not found in tenant webroot list", webrootID)

	// Step 4: Update the webroot (change runtime version).
	resp, body = httpPut(t, coreAPIURL+"/webroots/"+webrootID, map[string]interface{}{
		"runtime_version": "8.4",
	})
	require.Equal(t, 202, resp.StatusCode, "update webroot: %s", body)
	updated := parseJSON(t, body)
	require.Equal(t, "8.4", updated["runtime_version"])
	t.Logf("webroot updated: runtime_version=%s", updated["runtime_version"])

	// Step 5: Update the public folder.
	newFolder := "web"
	resp, body = httpPut(t, coreAPIURL+"/webroots/"+webrootID, map[string]interface{}{
		"public_folder": newFolder,
	})
	require.Equal(t, 202, resp.StatusCode, "update public_folder: %s", body)
	updated = parseJSON(t, body)
	require.Equal(t, newFolder, updated["public_folder"])
	t.Logf("webroot public_folder updated to %q", newFolder)

	// Step 6: Bind an FQDN to the webroot.
	resp, body = httpPost(t, fmt.Sprintf("%s/webroots/%s/fqdns", coreAPIURL, webrootID), map[string]interface{}{
		"fqdn": "site.e2e-webroot.example.com.",
	})
	require.Equal(t, 202, resp.StatusCode, "bind FQDN: %s", body)
	fqdn := parseJSON(t, body)
	fqdnID := fqdn["id"].(string)
	require.NotEmpty(t, fqdnID)
	require.Equal(t, "site.e2e-webroot.example.com.", fqdn["fqdn"])
	t.Logf("bound FQDN: %s (id=%s)", fqdn["fqdn"], fqdnID)

	// Step 7: Wait for the FQDN to become active.
	fqdn = waitForStatus(t, coreAPIURL+"/fqdns/"+fqdnID, "active", provisionTimeout)
	t.Logf("FQDN active")

	// Step 8: Verify the FQDN appears in the webroot's FQDN list.
	resp, body = httpGet(t, fmt.Sprintf("%s/webroots/%s/fqdns", coreAPIURL, webrootID))
	require.Equal(t, 200, resp.StatusCode, body)
	fqdns := parsePaginatedItems(t, body)
	found = false
	for _, f := range fqdns {
		if id, _ := f["id"].(string); id == fqdnID {
			found = true
			break
		}
	}
	require.True(t, found, "FQDN %s not found in webroot FQDN list", fqdnID)

	// Step 9: Get the FQDN by ID to verify all fields.
	resp, body = httpGet(t, coreAPIURL+"/fqdns/"+fqdnID)
	require.Equal(t, 200, resp.StatusCode, body)
	fqdnDetail := parseJSON(t, body)
	require.Equal(t, webrootID, fqdnDetail["webroot_id"])
	require.Equal(t, "site.e2e-webroot.example.com.", fqdnDetail["fqdn"])

	// Step 10: Delete the FQDN.
	resp, body = httpDelete(t, coreAPIURL+"/fqdns/"+fqdnID)
	require.Equal(t, 202, resp.StatusCode, "delete FQDN: %s", body)
	t.Logf("FQDN delete requested")

	// Step 11: Delete the webroot.
	resp, body = httpDelete(t, coreAPIURL+"/webroots/"+webrootID)
	require.Equal(t, 202, resp.StatusCode, "delete webroot: %s", body)
	waitForStatus(t, coreAPIURL+"/webroots/"+webrootID, "deleted", provisionTimeout)
	t.Logf("webroot deleted")
}

// TestWebrootCreateValidation verifies that creating a webroot with missing
// fields returns a 400 error.
func TestWebrootCreateValidation(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-wr-val")

	// Missing runtime and runtime_version.
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/webroots", coreAPIURL, tenantID), map[string]interface{}{
		"name": "bad-webroot",
	})
	require.Equal(t, 400, resp.StatusCode, "expected 400 for missing runtime: %s", body)
}

// TestWebrootMultipleRuntimes verifies that webroots can be created with
// different runtimes.
func TestWebrootMultipleRuntimes(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-wr-runtimes")

	runtimes := []struct {
		runtime string
		version string
	}{
		{"php", "8.5"},
		{"node", "22"},
		{"static", "1"},
	}

	for _, rt := range runtimes {
		t.Run(rt.runtime, func(t *testing.T) {
			resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/webroots", coreAPIURL, tenantID), map[string]interface{}{
				"name":            fmt.Sprintf("site-%s", rt.runtime),
				"runtime":         rt.runtime,
				"runtime_version": rt.version,
			})
			require.Equal(t, 202, resp.StatusCode, "create %s webroot: %s", rt.runtime, body)

			webroot := parseJSON(t, body)
			webrootID := webroot["id"].(string)
			require.Equal(t, rt.runtime, webroot["runtime"])
			require.Equal(t, rt.version, webroot["runtime_version"])
			t.Logf("created %s webroot: %s", rt.runtime, webrootID)

			// Clean up.
			t.Cleanup(func() {
				httpDelete(t, coreAPIURL+"/webroots/"+webrootID)
			})
		})
	}
}
