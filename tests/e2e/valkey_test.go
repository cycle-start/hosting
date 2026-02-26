package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValkeyInstanceCRUD(t *testing.T) {
	tenantID, _, clusterID, _, _ := createTestTenant(t, "e2e-valkey-crud")
	valkeyShardID := findValkeyShardID(t, clusterID)
	if valkeyShardID == "" {
		t.Skip("no valkey shard found in cluster; skipping valkey tests")
	}

	// Create a subscription (required for Valkey instance creation).
	subID := createTestSubscription(t, tenantID, "e2e-valkey-crud")

	// Create a Valkey instance.
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/valkey-instances", coreAPIURL, tenantID), map[string]interface{}{
		"shard_id":        valkeyShardID,
		"max_memory_mb":   64,
		"subscription_id": subID,
	})
	require.Equal(t, 202, resp.StatusCode, "create valkey instance: %s", body)
	inst := parseJSON(t, body)
	instID := inst["id"].(string)
	require.NotEmpty(t, instID)
	t.Logf("created valkey instance: %s", instID)

	// Wait for instance to become active.
	inst = waitForStatus(t, coreAPIURL+"/valkey-instances/"+instID, "active", provisionTimeout)
	require.Equal(t, "active", inst["status"])
	t.Logf("valkey instance active")

	// Verify port was assigned.
	port, _ := inst["port"].(float64)
	require.Greater(t, port, float64(0), "port should be assigned")
	t.Logf("valkey port: %d", int(port))

	// Verify instance in tenant list.
	resp, body = httpGet(t, fmt.Sprintf("%s/tenants/%s/valkey-instances", coreAPIURL, tenantID))
	require.Equal(t, 200, resp.StatusCode, body)
	instances := parsePaginatedItems(t, body)
	found := false
	for _, i := range instances {
		if id, _ := i["id"].(string); id == instID {
			found = true
			break
		}
	}
	require.True(t, found, "instance %s not found in tenant valkey list", instID)

	// Create a Valkey user.
	resp, body = httpPost(t, fmt.Sprintf("%s/valkey-instances/%s/users", coreAPIURL, instID), map[string]interface{}{
		"username":    instID + "_app",
		"password":    "TestP@ssw0rd!123",
		"privileges":  []string{"+@all"},
		"key_pattern": "~*",
	})
	require.Equal(t, 202, resp.StatusCode, "create valkey user: %s", body)
	user := parseJSON(t, body)
	userID := user["id"].(string)
	require.NotEmpty(t, userID)
	t.Logf("created valkey user: %s", userID)

	// Wait for user to become active.
	user = waitForStatus(t, coreAPIURL+"/valkey-users/"+userID, "active", provisionTimeout)
	require.Equal(t, "active", user["status"])

	// Update the user (change password).
	resp, body = httpPut(t, coreAPIURL+"/valkey-users/"+userID, map[string]interface{}{
		"password": "NewP@ssw0rd!456",
	})
	require.Equal(t, 202, resp.StatusCode, "update valkey user: %s", body)
	t.Logf("valkey user updated")

	// List users for the instance.
	resp, body = httpGet(t, fmt.Sprintf("%s/valkey-instances/%s/users", coreAPIURL, instID))
	require.Equal(t, 200, resp.StatusCode, body)
	users := parsePaginatedItems(t, body)
	found = false
	for _, u := range users {
		if id, _ := u["id"].(string); id == userID {
			found = true
			break
		}
	}
	require.True(t, found, "user %s not found in valkey user list", userID)

	// Delete the user.
	resp, body = httpDelete(t, coreAPIURL+"/valkey-users/"+userID)
	require.Equal(t, 202, resp.StatusCode, "delete valkey user: %s", body)
	t.Logf("valkey user delete requested")

	// Delete the instance.
	resp, body = httpDelete(t, coreAPIURL+"/valkey-instances/"+instID)
	require.Equal(t, 202, resp.StatusCode, "delete valkey instance: %s", body)
	waitForStatus(t, coreAPIURL+"/valkey-instances/"+instID, "deleted", provisionTimeout)
	t.Logf("valkey instance deleted")
}

func TestValkeyInstanceGetNotFound(t *testing.T) {
	resp, _ := httpGet(t, coreAPIURL+"/valkey-instances/00000000-0000-0000-0000-000000000000")
	require.Equal(t, 404, resp.StatusCode)
}
