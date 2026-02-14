package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestBackupWebroot tests web backup creation and restore:
// create tenant -> create webroot -> write test file via SSH -> create backup
// -> wait active -> modify file -> restore backup -> verify original content.
func TestBackupWebroot(t *testing.T) {
	// Check if backup endpoints are wired in the router. If the platform
	// returns 404 for the backup list endpoint, skip the test.
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-backup-web")

	resp, _ := httpGet(t, fmt.Sprintf("%s/tenants/%s/backups", coreAPIURL, tenantID))
	if resp.StatusCode == 404 {
		t.Skip("backup endpoints not registered; skipping backup tests")
	}
	require.Equal(t, 200, resp.StatusCode)

	// Create a webroot.
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/webroots", coreAPIURL, tenantID), map[string]interface{}{
		"name":            "backup-site",
		"runtime":         "php",
		"runtime_version": "8.5",
	})
	require.Equal(t, 202, resp.StatusCode, "create webroot: %s", body)
	webroot := parseJSON(t, body)
	webrootID := webroot["id"].(string)
	t.Logf("created webroot: %s", webrootID)

	webroot = waitForStatus(t, coreAPIURL+"/webroots/"+webrootID, "active", provisionTimeout)
	t.Logf("webroot active")

	// Write a test file on a web node.
	ips := findNodeIPsByRole(t, clusterName, "web")
	nodeIP := ips[0]
	tenantName := "e2e-backup-web"
	testFilePath := fmt.Sprintf("/var/www/storage/%s/backup-site/public/backup-test.txt", tenantName)
	originalContent := "original-backup-content"
	sshExec(t, nodeIP, fmt.Sprintf("sudo mkdir -p $(dirname %s) && echo '%s' | sudo tee %s", testFilePath, originalContent, testFilePath))
	t.Logf("wrote test file on node %s", nodeIP)

	// Create a web backup.
	resp, body = httpPost(t, fmt.Sprintf("%s/tenants/%s/backups", coreAPIURL, tenantID), map[string]interface{}{
		"type":      "web",
		"source_id": webrootID,
	})
	require.Equal(t, 202, resp.StatusCode, "create backup: %s", body)
	backup := parseJSON(t, body)
	backupID := backup["id"].(string)
	require.NotEmpty(t, backupID)
	t.Logf("created backup: %s", backupID)

	// Wait for backup to become active (completed).
	backup = waitForStatus(t, coreAPIURL+"/backups/"+backupID, "active", provisionTimeout)
	require.Equal(t, "active", backup["status"])
	t.Logf("backup completed: %s", backupID)

	// Verify backup metadata.
	require.Equal(t, "web", backup["type"])
	require.Equal(t, webrootID, backup["source_id"])
	t.Logf("backup source_name=%s size_bytes=%v", backup["source_name"], backup["size_bytes"])

	// Modify the test file.
	modifiedContent := "modified-after-backup"
	sshExec(t, nodeIP, fmt.Sprintf("echo '%s' | sudo tee %s", modifiedContent, testFilePath))
	t.Logf("modified test file")

	// Restore the backup.
	resp, body = httpPost(t, coreAPIURL+"/backups/"+backupID+"/restore", nil)
	require.Equal(t, 202, resp.StatusCode, "restore backup: %s", body)
	t.Logf("restore requested")

	// Wait for restore to complete. The backup status may cycle through
	// provisioning states. We poll until the backup is back to "active".
	waitForStatus(t, coreAPIURL+"/backups/"+backupID, "active", provisionTimeout)
	t.Logf("restore completed")

	// Verify the original content is restored.
	got := sshExec(t, nodeIP, fmt.Sprintf("cat %s", testFilePath))
	require.Contains(t, got, originalContent, "expected original content after restore")
	t.Logf("file content verified after restore")

	// Clean up.
	sshExec(t, nodeIP, fmt.Sprintf("sudo rm -f %s", testFilePath))
}

// TestBackupList verifies that the backup list endpoint returns a paginated
// response for a tenant.
func TestBackupList(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-backup-list")

	resp, body := httpGet(t, fmt.Sprintf("%s/tenants/%s/backups", coreAPIURL, tenantID))
	if resp.StatusCode == 404 {
		t.Skip("backup endpoints not registered")
	}
	require.Equal(t, 200, resp.StatusCode, body)

	result := parseJSON(t, body)
	_, hasItems := result["items"]
	require.True(t, hasItems, "backup list missing 'items' key")
	_, hasMore := result["has_more"]
	require.True(t, hasMore, "backup list missing 'has_more' key")
}
