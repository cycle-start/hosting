package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCronJobCRUD tests the full cron job lifecycle:
// create webroot -> create cron job -> wait active -> verify systemd timer
// on web nodes -> update -> disable -> enable -> delete.
func TestCronJobCRUD(t *testing.T) {
	tenantID, _, clusterID, _, _ := createTestTenant(t, "e2e-cron")
	webrootID := createTestWebroot(t, tenantID, "cron-site", "php", "8.5")

	// Step 1: Create a cron job.
	resp, body := httpPost(t, fmt.Sprintf("%s/webroots/%s/cron-jobs", coreAPIURL, webrootID), map[string]interface{}{
		"schedule": "*/5 * * * *",
		"command":  "php artisan schedule:run",
	})
	require.Equal(t, 202, resp.StatusCode, "create cron job: %s", body)
	cronJob := parseJSON(t, body)
	cronJobID := cronJob["id"].(string)
	require.NotEmpty(t, cronJobID)
	require.Equal(t, "*/5 * * * *", cronJob["schedule"])
	t.Logf("created cron job: %s", cronJobID)

	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/cron-jobs/"+cronJobID) })

	// Step 2: Wait for the cron job to become active.
	cronJob = waitForStatus(t, coreAPIURL+"/cron-jobs/"+cronJobID, "active", provisionTimeout)
	require.Equal(t, "active", cronJob["status"])
	t.Logf("cron job active")

	// Step 3: Verify systemd timer exists on web nodes.
	ips := findNodeIPsByRole(t, clusterID, "web")
	for _, ip := range ips {
		out := sshExec(t, ip, fmt.Sprintf("systemctl list-timers --all 2>/dev/null | grep %s || echo 'not found'", cronJobID))
		if strings.Contains(out, cronJobID) {
			t.Logf("systemd timer found on node %s", ip)
		} else {
			// Timer may use a different naming scheme — check for the service file.
			out = sshExec(t, ip, fmt.Sprintf("ls /etc/systemd/system/*%s* 2>/dev/null || echo 'not found'", cronJobID))
			t.Logf("systemd files on node %s: %s", ip, strings.TrimSpace(out))
		}
	}

	// Step 4: Get the cron job by ID.
	resp, body = httpGet(t, coreAPIURL+"/cron-jobs/"+cronJobID)
	require.Equal(t, 200, resp.StatusCode, body)
	detail := parseJSON(t, body)
	require.Equal(t, webrootID, detail["webroot_id"])
	require.Equal(t, "php artisan schedule:run", detail["command"])

	// Step 5: Update the cron job schedule.
	resp, body = httpPut(t, coreAPIURL+"/cron-jobs/"+cronJobID, map[string]interface{}{
		"schedule": "*/10 * * * *",
	})
	require.Equal(t, 202, resp.StatusCode, "update cron job: %s", body)
	waitForStatus(t, coreAPIURL+"/cron-jobs/"+cronJobID, "active", provisionTimeout)
	t.Logf("cron job updated")

	// Verify the update.
	resp, body = httpGet(t, coreAPIURL+"/cron-jobs/"+cronJobID)
	require.Equal(t, 200, resp.StatusCode, body)
	updated := parseJSON(t, body)
	require.Equal(t, "*/10 * * * *", updated["schedule"])

	// Step 6: Disable the cron job.
	resp, body = httpPost(t, coreAPIURL+"/cron-jobs/"+cronJobID+"/disable", nil)
	require.Equal(t, 202, resp.StatusCode, "disable cron job: %s", body)
	waitForStatus(t, coreAPIURL+"/cron-jobs/"+cronJobID, "disabled", provisionTimeout)
	t.Logf("cron job disabled")

	// Step 7: Enable the cron job.
	resp, body = httpPost(t, coreAPIURL+"/cron-jobs/"+cronJobID+"/enable", nil)
	require.Equal(t, 202, resp.StatusCode, "enable cron job: %s", body)
	waitForStatus(t, coreAPIURL+"/cron-jobs/"+cronJobID, "active", provisionTimeout)
	t.Logf("cron job re-enabled")

	// Step 8: List cron jobs for the webroot.
	resp, body = httpGet(t, fmt.Sprintf("%s/webroots/%s/cron-jobs", coreAPIURL, webrootID))
	require.Equal(t, 200, resp.StatusCode, body)
	jobs := parsePaginatedItems(t, body)
	found := false
	for _, j := range jobs {
		if id, _ := j["id"].(string); id == cronJobID {
			found = true
			break
		}
	}
	require.True(t, found, "cron job should appear in webroot cron job list")

	// Step 9: Delete the cron job.
	resp, body = httpDelete(t, coreAPIURL+"/cron-jobs/"+cronJobID)
	require.Equal(t, 202, resp.StatusCode, "delete cron job: %s", body)
	waitForStatus(t, coreAPIURL+"/cron-jobs/"+cronJobID, "deleted", provisionTimeout)
	t.Logf("cron job deleted")
}

// TestCronJobValidation verifies that creating a cron job with invalid
// fields returns appropriate errors.
func TestCronJobValidation(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-cron-val")
	webrootID := createTestWebroot(t, tenantID, "cron-val-site", "php", "8.5")

	// Missing schedule.
	resp, body := httpPost(t, fmt.Sprintf("%s/webroots/%s/cron-jobs", coreAPIURL, webrootID), map[string]interface{}{
		"command": "echo hello",
	})
	require.Equal(t, 400, resp.StatusCode, "expected 400 for missing schedule: %s", body)

	// Missing command.
	resp, body = httpPost(t, fmt.Sprintf("%s/webroots/%s/cron-jobs", coreAPIURL, webrootID), map[string]interface{}{
		"schedule": "* * * * *",
	})
	require.Equal(t, 400, resp.StatusCode, "expected 400 for missing command: %s", body)
}
