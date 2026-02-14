package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDatabaseCRUD tests the full database lifecycle:
// create tenant -> create database -> wait active -> create user -> wait active
// -> update user -> delete user -> delete database -> cleanup.
func TestDatabaseCRUD(t *testing.T) {
	tenantID, _, clusterID, _, dbShardID := createTestTenant(t, "e2e-db-crud")
	if dbShardID == "" {
		t.Skip("no database shard found in cluster; skipping database tests")
	}

	// Step 1: Create a database for the tenant.
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/databases", coreAPIURL, tenantID), map[string]interface{}{
		"name":     "e2e-testdb",
		"shard_id": dbShardID,
	})
	require.Equal(t, 202, resp.StatusCode, "create database: %s", body)
	db := parseJSON(t, body)
	dbID := db["id"].(string)
	require.NotEmpty(t, dbID)
	t.Logf("created database: %s", dbID)

	// Step 2: Wait for the database to become active.
	db = waitForStatus(t, coreAPIURL+"/databases/"+dbID, "active", provisionTimeout)
	require.Equal(t, "active", db["status"])
	t.Logf("database active: %s", dbID)

	// Step 3: Verify the database appears in the tenant's database list.
	resp, body = httpGet(t, fmt.Sprintf("%s/tenants/%s/databases", coreAPIURL, tenantID))
	require.Equal(t, 200, resp.StatusCode, body)
	databases := parsePaginatedItems(t, body)
	found := false
	for _, d := range databases {
		if id, _ := d["id"].(string); id == dbID {
			found = true
			break
		}
	}
	require.True(t, found, "database %s not found in tenant database list", dbID)

	// Step 4: Create a database user.
	resp, body = httpPost(t, fmt.Sprintf("%s/databases/%s/users", coreAPIURL, dbID), map[string]interface{}{
		"username":   "e2e-user",
		"password":   "Str0ngP@ssw0rd!",
		"privileges": []string{"ALL"},
	})
	require.Equal(t, 202, resp.StatusCode, "create database user: %s", body)
	user := parseJSON(t, body)
	userID := user["id"].(string)
	require.NotEmpty(t, userID)
	// Password should be stripped from the response.
	require.Empty(t, user["password"], "password should not be returned")
	t.Logf("created database user: %s", userID)

	// Step 5: Wait for the user to become active.
	user = waitForStatus(t, coreAPIURL+"/database-users/"+userID, "active", provisionTimeout)
	require.Equal(t, "active", user["status"])
	t.Logf("database user active: %s", userID)

	// Step 6: Update the database user (change password).
	resp, body = httpPut(t, coreAPIURL+"/database-users/"+userID, map[string]interface{}{
		"password": "N3wStr0ngP@ss!",
	})
	require.Equal(t, 202, resp.StatusCode, "update database user: %s", body)
	t.Logf("database user updated")

	// Step 7: Verify user appears in the database's user list.
	resp, body = httpGet(t, fmt.Sprintf("%s/databases/%s/users", coreAPIURL, dbID))
	require.Equal(t, 200, resp.StatusCode, body)
	users := parsePaginatedItems(t, body)
	found = false
	for _, u := range users {
		if id, _ := u["id"].(string); id == userID {
			found = true
			// Verify password is not returned in list either.
			require.Empty(t, u["password"], "password should not be in list response")
			break
		}
	}
	require.True(t, found, "user %s not found in database user list", userID)

	// Step 8: Delete the database user.
	resp, body = httpDelete(t, coreAPIURL+"/database-users/"+userID)
	require.Equal(t, 202, resp.StatusCode, "delete database user: %s", body)
	t.Logf("database user delete requested")

	// Step 9: Delete the database.
	resp, body = httpDelete(t, coreAPIURL+"/databases/"+dbID)
	require.Equal(t, 202, resp.StatusCode, "delete database: %s", body)
	t.Logf("database delete requested")

	// Step 10: Wait for the database to be deleted (or 404).
	waitForStatus(t, coreAPIURL+"/databases/"+dbID, "deleted", provisionTimeout)
	t.Logf("database deleted")

	// The tenant cleanup is handled by createTestTenant's t.Cleanup.
	_ = clusterID
}

// TestDatabaseGetNotFound verifies that fetching a non-existent database returns 404.
func TestDatabaseGetNotFound(t *testing.T) {
	resp, _ := httpGet(t, coreAPIURL+"/databases/00000000-0000-0000-0000-000000000000")
	require.Equal(t, 404, resp.StatusCode)
}

// TestDatabaseUserCreateValidation verifies that creating a user with a
// too-short password returns 400.
func TestDatabaseUserCreateValidation(t *testing.T) {
	tenantID, _, _, _, dbShardID := createTestTenant(t, "e2e-db-valuser")
	if dbShardID == "" {
		t.Skip("no database shard found")
	}

	// Create a database first.
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/databases", coreAPIURL, tenantID), map[string]interface{}{
		"name":     "e2e-valdb",
		"shard_id": dbShardID,
	})
	require.Equal(t, 202, resp.StatusCode, body)
	db := parseJSON(t, body)
	dbID := db["id"].(string)
	waitForStatus(t, coreAPIURL+"/databases/"+dbID, "active", provisionTimeout)

	// Try to create a user with a short password.
	resp, body = httpPost(t, fmt.Sprintf("%s/databases/%s/users", coreAPIURL, dbID), map[string]interface{}{
		"username":   "baduser",
		"password":   "short",
		"privileges": []string{"ALL"},
	})
	require.Equal(t, 400, resp.StatusCode, "expected 400 for short password: %s", body)
}
