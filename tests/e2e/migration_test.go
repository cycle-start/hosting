package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTenantMigration(t *testing.T) {
	tenantID, _, clusterID, webShardID, _ := createTestTenant(t, "e2e-migrate-tenant")

	// Find all web shards — need at least 2 for migration.
	resp, body := httpGet(t, fmt.Sprintf("%s/clusters/%s/shards", coreAPIURL, clusterID))
	require.Equal(t, 200, resp.StatusCode, body)
	shards := parsePaginatedItems(t, body)

	var targetShardID string
	for _, s := range shards {
		if r, _ := s["role"].(string); r == "web" {
			if id, _ := s["id"].(string); id != webShardID {
				targetShardID = id
				break
			}
		}
	}
	if targetShardID == "" {
		t.Skip("only one web shard available; skipping migration test")
	}

	// Trigger migration.
	resp, body = httpPost(t, coreAPIURL+"/tenants/"+tenantID+"/migrate", map[string]interface{}{
		"target_shard_id": targetShardID,
	})
	require.Equal(t, 202, resp.StatusCode, "migrate tenant: %s", body)
	t.Logf("migration started to shard %s", targetShardID)

	// Wait for tenant to return to active.
	tenant := waitForStatus(t, coreAPIURL+"/tenants/"+tenantID, "active", migrationTimeout)
	require.Equal(t, "active", tenant["status"])
	t.Logf("tenant active after migration")
}

func TestDatabaseMigration(t *testing.T) {
	tenantID, _, clusterID, _, dbShardID := createTestTenant(t, "e2e-migrate-db")
	if dbShardID == "" {
		t.Skip("no database shard found")
	}

	dbID, _ := createTestDatabase(t, tenantID, dbShardID, "e2e_migratedb")

	// Find a second database shard.
	resp, body := httpGet(t, fmt.Sprintf("%s/clusters/%s/shards", coreAPIURL, clusterID))
	require.Equal(t, 200, resp.StatusCode, body)
	shards := parsePaginatedItems(t, body)

	var targetShardID string
	for _, s := range shards {
		if r, _ := s["role"].(string); r == "database" {
			if id, _ := s["id"].(string); id != dbShardID {
				targetShardID = id
				break
			}
		}
	}
	if targetShardID == "" {
		t.Skip("only one database shard available; skipping migration test")
	}

	resp, body = httpPost(t, coreAPIURL+"/databases/"+dbID+"/migrate", map[string]interface{}{
		"target_shard_id": targetShardID,
	})
	require.Equal(t, 202, resp.StatusCode, "migrate database: %s", body)
	t.Logf("database migration started")

	db := waitForStatus(t, coreAPIURL+"/databases/"+dbID, "active", migrationTimeout)
	require.Equal(t, "active", db["status"])
	t.Logf("database active after migration")
}

func TestValkeyMigration(t *testing.T) {
	tenantID, _, clusterID, _, _ := createTestTenant(t, "e2e-migrate-valkey")
	valkeyShardID := findValkeyShardID(t, clusterID)
	if valkeyShardID == "" {
		t.Skip("no valkey shard found")
	}

	instanceID := createTestValkeyInstance(t, tenantID, valkeyShardID, "e2e-migrate-cache")

	// Find a second valkey shard.
	resp, body := httpGet(t, fmt.Sprintf("%s/clusters/%s/shards", coreAPIURL, clusterID))
	require.Equal(t, 200, resp.StatusCode, body)
	shards := parsePaginatedItems(t, body)

	var targetShardID string
	for _, s := range shards {
		if r, _ := s["role"].(string); r == "valkey" {
			if id, _ := s["id"].(string); id != valkeyShardID {
				targetShardID = id
				break
			}
		}
	}
	if targetShardID == "" {
		t.Skip("only one valkey shard available; skipping migration test")
	}

	resp, body = httpPost(t, coreAPIURL+"/valkey-instances/"+instanceID+"/migrate", map[string]interface{}{
		"target_shard_id": targetShardID,
	})
	require.Equal(t, 202, resp.StatusCode, "migrate valkey: %s", body)
	t.Logf("valkey migration started")

	inst := waitForStatus(t, coreAPIURL+"/valkey-instances/"+instanceID, "active", migrationTimeout)
	require.Equal(t, "active", inst["status"])
	t.Logf("valkey active after migration")
}
