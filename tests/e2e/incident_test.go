package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIncidentCRUD tests the full incident lifecycle:
// create incident -> get -> add event -> list events -> update ->
// escalate -> resolve -> verify status transitions.
func TestIncidentCRUD(t *testing.T) {
	// Step 1: Create an incident.
	resp, body := httpPost(t, coreAPIURL+"/incidents", map[string]interface{}{
		"dedupe_key": "e2e-test-incident-1",
		"type":       "health_check_failed",
		"severity":   "warning",
		"title":      "E2E test incident",
		"detail":     "Created by E2E test suite",
		"source":     "e2e-tests",
	})
	require.Equal(t, 201, resp.StatusCode, "create incident: %s", body)
	incident := parseJSON(t, body)
	incidentID := incident["id"].(string)
	require.NotEmpty(t, incidentID)
	require.Equal(t, "warning", incident["severity"])
	require.Equal(t, "open", incident["status"])
	t.Logf("created incident: %s", incidentID)

	t.Cleanup(func() {
		// Cancel the incident to clean up.
		httpPost(t, coreAPIURL+"/incidents/"+incidentID+"/cancel", nil)
	})

	// Step 2: Get the incident by ID.
	resp, body = httpGet(t, coreAPIURL+"/incidents/"+incidentID)
	require.Equal(t, 200, resp.StatusCode, "get incident: %s", body)
	fetched := parseJSON(t, body)
	require.Equal(t, incidentID, fetched["id"])
	require.Equal(t, "E2E test incident", fetched["title"])

	// Step 3: Dedupe — creating with the same dedupe_key returns the existing incident.
	resp, body = httpPost(t, coreAPIURL+"/incidents", map[string]interface{}{
		"dedupe_key": "e2e-test-incident-1",
		"type":       "health_check_failed",
		"severity":   "warning",
		"title":      "Duplicate incident",
		"source":     "e2e-tests",
	})
	require.Equal(t, 200, resp.StatusCode, "dedupe should return 200: %s", body)
	deduped := parseJSON(t, body)
	require.Equal(t, incidentID, deduped["id"], "dedupe should return same incident")

	// Step 4: Add a timeline event.
	resp, body = httpPost(t, fmt.Sprintf("%s/incidents/%s/events", coreAPIURL, incidentID), map[string]interface{}{
		"actor":  "e2e-test",
		"action": "investigated",
		"detail": "Checked system metrics, found elevated error rate",
	})
	require.Equal(t, 201, resp.StatusCode, "add event: %s", body)
	t.Logf("added timeline event")

	// Step 5: List events.
	resp, body = httpGet(t, fmt.Sprintf("%s/incidents/%s/events", coreAPIURL, incidentID))
	require.Equal(t, 200, resp.StatusCode, "list events: %s", body)
	events := parsePaginatedItems(t, body)
	require.GreaterOrEqual(t, len(events), 1, "should have at least 1 event")
	t.Logf("found %d events", len(events))

	// Step 6: Update incident severity.
	resp, body = httpPatch(t, coreAPIURL+"/incidents/"+incidentID, map[string]interface{}{
		"severity": "critical",
	})
	require.Equal(t, 204, resp.StatusCode, "update incident: %s", body)

	// Verify update.
	resp, body = httpGet(t, coreAPIURL+"/incidents/"+incidentID)
	require.Equal(t, 200, resp.StatusCode, body)
	updated := parseJSON(t, body)
	require.Equal(t, "critical", updated["severity"])
	t.Logf("incident updated to critical")

	// Step 7: Escalate.
	resp, body = httpPost(t, coreAPIURL+"/incidents/"+incidentID+"/escalate", map[string]interface{}{
		"reason": "E2E test escalation",
	})
	require.Equal(t, 204, resp.StatusCode, "escalate: %s", body)

	resp, body = httpGet(t, coreAPIURL+"/incidents/"+incidentID)
	require.Equal(t, 200, resp.StatusCode, body)
	escalated := parseJSON(t, body)
	require.Equal(t, "escalated", escalated["status"])
	t.Logf("incident escalated")

	// Step 8: Resolve.
	resp, body = httpPost(t, coreAPIURL+"/incidents/"+incidentID+"/resolve", map[string]interface{}{
		"resolution": "Fixed by E2E test",
	})
	require.Equal(t, 204, resp.StatusCode, "resolve: %s", body)

	resp, body = httpGet(t, coreAPIURL+"/incidents/"+incidentID)
	require.Equal(t, 200, resp.StatusCode, body)
	resolved := parseJSON(t, body)
	require.Equal(t, "resolved", resolved["status"])
	t.Logf("incident resolved")
}

// TestIncidentList verifies incident list with filtering.
func TestIncidentList(t *testing.T) {
	// Create two incidents with different severities.
	resp, body := httpPost(t, coreAPIURL+"/incidents", map[string]interface{}{
		"dedupe_key": "e2e-list-warning",
		"type":       "health_check_failed",
		"severity":   "warning",
		"title":      "Warning incident for list test",
		"source":     "e2e-tests",
	})
	require.Equal(t, 201, resp.StatusCode, "create warning incident: %s", body)
	warn := parseJSON(t, body)
	warnID := warn["id"].(string)
	t.Cleanup(func() { httpPost(t, coreAPIURL+"/incidents/"+warnID+"/cancel", nil) })

	resp, body = httpPost(t, coreAPIURL+"/incidents", map[string]interface{}{
		"dedupe_key": "e2e-list-info",
		"type":       "health_check_failed",
		"severity":   "info",
		"title":      "Info incident for list test",
		"source":     "e2e-tests",
	})
	require.Equal(t, 201, resp.StatusCode, "create info incident: %s", body)
	info := parseJSON(t, body)
	infoID := info["id"].(string)
	t.Cleanup(func() { httpPost(t, coreAPIURL+"/incidents/"+infoID+"/cancel", nil) })

	// List all incidents.
	resp, body = httpGet(t, coreAPIURL+"/incidents")
	require.Equal(t, 200, resp.StatusCode, "list incidents: %s", body)
	all := parsePaginatedItems(t, body)
	require.GreaterOrEqual(t, len(all), 2, "should have at least 2 incidents")
	t.Logf("total incidents: %d", len(all))

	// Filter by severity.
	resp, body = httpGet(t, coreAPIURL+"/incidents?severity=warning")
	require.Equal(t, 200, resp.StatusCode, "filter by severity: %s", body)
	warnings := parsePaginatedItems(t, body)
	for _, inc := range warnings {
		require.Equal(t, "warning", inc["severity"], "filtered incidents should all be warning")
	}
	t.Logf("warning incidents: %d", len(warnings))

	// Filter by status.
	resp, body = httpGet(t, coreAPIURL+"/incidents?status=open")
	require.Equal(t, 200, resp.StatusCode, "filter by status: %s", body)
	open := parsePaginatedItems(t, body)
	for _, inc := range open {
		require.Equal(t, "open", inc["status"], "filtered incidents should all be open")
	}
	t.Logf("open incidents: %d", len(open))
}

// TestIncidentCancel verifies the cancel (false positive) flow.
func TestIncidentCancel(t *testing.T) {
	resp, body := httpPost(t, coreAPIURL+"/incidents", map[string]interface{}{
		"dedupe_key": "e2e-cancel-test",
		"type":       "health_check_failed",
		"severity":   "info",
		"title":      "Incident to cancel",
		"source":     "e2e-tests",
	})
	require.Equal(t, 201, resp.StatusCode, "create incident: %s", body)
	incident := parseJSON(t, body)
	incidentID := incident["id"].(string)

	// Cancel it.
	resp, body = httpPost(t, coreAPIURL+"/incidents/"+incidentID+"/cancel", nil)
	require.Equal(t, 204, resp.StatusCode, "cancel: %s", body)

	// Verify status.
	resp, body = httpGet(t, coreAPIURL+"/incidents/"+incidentID)
	require.Equal(t, 200, resp.StatusCode, body)
	cancelled := parseJSON(t, body)
	require.Equal(t, "cancelled", cancelled["status"])
	t.Logf("incident cancelled")
}
