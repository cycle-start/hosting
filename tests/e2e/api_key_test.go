package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyCRUD(t *testing.T) {
	brandID := findOrCreateBrand(t)

	// Create API key.
	resp, body := httpPost(t, coreAPIURL+"/api-keys", map[string]interface{}{
		"name":   "e2e-test-key",
		"scopes": []string{"tenants:read", "tenants:write"},
		"brands": []string{brandID},
	})
	require.Equal(t, 201, resp.StatusCode, "create API key: %s", body)
	keyData := parseJSON(t, body)
	keyID := keyData["id"].(string)
	rawKey := keyData["key"].(string)
	require.NotEmpty(t, keyID)
	require.NotEmpty(t, rawKey, "key should be returned on creation")
	t.Logf("created API key: %s", keyID)

	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/api-keys/"+keyID) })

	// List API keys — raw_key should NOT be returned.
	resp, body = httpGet(t, coreAPIURL+"/api-keys")
	require.Equal(t, 200, resp.StatusCode, body)
	keys := parsePaginatedItems(t, body)
	found := false
	for _, k := range keys {
		if id, _ := k["id"].(string); id == keyID {
			found = true
			rk, _ := k["raw_key"].(string)
			require.Empty(t, rk, "raw_key should not be returned in list")
			break
		}
	}
	require.True(t, found, "API key %s not in list", keyID)

	// Get API key.
	resp, body = httpGet(t, coreAPIURL+"/api-keys/"+keyID)
	require.Equal(t, 200, resp.StatusCode, body)

	// Update API key.
	resp, body = httpPut(t, coreAPIURL+"/api-keys/"+keyID, map[string]interface{}{
		"name":   "e2e-key-updated",
		"scopes": []string{"*:*"},
		"brands": []string{brandID},
	})
	require.Equal(t, 200, resp.StatusCode, "update API key: %s", body)
	t.Logf("API key updated")

	// Revoke API key.
	resp, body = httpDelete(t, coreAPIURL+"/api-keys/"+keyID)
	require.Equal(t, 204, resp.StatusCode, "revoke API key: %s", body)
	t.Logf("API key revoked")

	// Verify revoked key cannot authenticate.
	resp, _ = httpGetWithKey(t, coreAPIURL+"/tenants", rawKey)
	require.Equal(t, 401, resp.StatusCode, "revoked key should return 401")
	t.Logf("revoked key correctly returns 401")
}

func TestAPIKeyScopeEnforcement(t *testing.T) {
	brandID := findOrCreateBrand(t)

	// Create a key with only tenants:read scope.
	resp, body := httpPost(t, coreAPIURL+"/api-keys", map[string]interface{}{
		"name":   "e2e-readonly-key",
		"scopes": []string{"tenants:read"},
		"brands": []string{brandID},
	})
	require.Equal(t, 201, resp.StatusCode, "create read-only key: %s", body)
	keyData := parseJSON(t, body)
	keyID := keyData["id"].(string)
	rawKey := keyData["key"].(string)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/api-keys/"+keyID) })

	// GET /tenants should succeed (tenants:read).
	resp, _ = httpGetWithKey(t, coreAPIURL+"/tenants", rawKey)
	require.Equal(t, 200, resp.StatusCode, "read-only key should read tenants")

	// POST /tenants should fail (needs tenants:write).
	regionID := findFirstRegionID(t)
	cluster := findFirstCluster(t, regionID)
	clusterID, _ := cluster["id"].(string)
	webShard := findShardByRole(t, clusterID, "web")
	webShardID, _ := webShard["id"].(string)

	resp, _ = httpPostWithKey(t, coreAPIURL+"/tenants", map[string]interface{}{
		"brand_id":   brandID,
		"region_id":  regionID,
		"cluster_id": clusterID,
		"shard_id":   webShardID,
	}, rawKey)
	require.Equal(t, 403, resp.StatusCode, "read-only key should not write tenants")
	t.Logf("scope enforcement working: read allowed, write denied")
}
