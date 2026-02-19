package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBrandCRUD(t *testing.T) {
	// Clean up stale brand from previous failed runs.
	deleteBrandByName(t, "E2E CRUD Brand")
	deleteBrandByName(t, "E2E CRUD Brand Updated")

	// Create brand.
	resp, body := httpPost(t, coreAPIURL+"/brands", map[string]interface{}{
		"name":             "E2E CRUD Brand",
		"base_hostname":    "crud.hosting.test",
		"primary_ns":       "ns1.crud.hosting.test",
		"secondary_ns":     "ns2.crud.hosting.test",
		"hostmaster_email": "hostmaster@crud.hosting.test",
	})
	require.Equal(t, 201, resp.StatusCode, "create brand: %s", body)
	brand := parseJSON(t, body)
	brandID := brand["id"].(string)
	t.Logf("created brand: %s", brandID)

	t.Cleanup(func() {
		httpDelete(t, coreAPIURL+"/brands/"+brandID)
	})

	// Get brand.
	resp, body = httpGet(t, coreAPIURL+"/brands/"+brandID)
	require.Equal(t, 200, resp.StatusCode, body)
	brand = parseJSON(t, body)
	require.Equal(t, brandID, brand["id"])
	require.Equal(t, "E2E CRUD Brand", brand["name"])

	// Update brand.
	resp, body = httpPut(t, coreAPIURL+"/brands/"+brandID, map[string]interface{}{
		"name": "E2E CRUD Brand Updated",
	})
	require.Equal(t, 200, resp.StatusCode, "update brand: %s", body)
	updated := parseJSON(t, body)
	require.Equal(t, "E2E CRUD Brand Updated", updated["name"])
	t.Logf("brand updated")

	// List brands.
	resp, body = httpGet(t, coreAPIURL+"/brands")
	require.Equal(t, 200, resp.StatusCode, body)
	brands := parsePaginatedItems(t, body)
	found := false
	for _, b := range brands {
		if id, _ := b["id"].(string); id == brandID {
			found = true
			break
		}
	}
	require.True(t, found, "brand %s not found in list", brandID)

	// Set allowed clusters.
	clusterID := resolveClusterID(t)
	resp, body = httpPut(t, coreAPIURL+"/brands/"+brandID+"/clusters", map[string]interface{}{
		"cluster_ids": []string{clusterID},
	})
	require.Equal(t, 200, resp.StatusCode, "set clusters: %s", body)
	t.Logf("brand clusters set")

	// List allowed clusters.
	resp, body = httpGet(t, coreAPIURL+"/brands/"+brandID+"/clusters")
	require.Equal(t, 200, resp.StatusCode, body)

	// Clear cluster restrictions.
	resp, body = httpPut(t, coreAPIURL+"/brands/"+brandID+"/clusters", map[string]interface{}{
		"cluster_ids": []string{},
	})
	require.Equal(t, 200, resp.StatusCode, "clear clusters: %s", body)
	t.Logf("brand clusters cleared")
}

func TestBrandIsolation(t *testing.T) {
	// Clean up stale brands from previous failed runs.
	brandNames := []string{"E2E Iso Brand A", "E2E Iso Brand B"}
	for _, name := range brandNames {
		deleteBrandByName(t, name)
	}

	// Create two brands.
	brandIDs := make([]string, 2)
	brandHostnames := []string{"iso-a.hosting.test", "iso-b.hosting.test"}

	for i, name := range brandNames {
		resp, body := httpPost(t, coreAPIURL+"/brands", map[string]interface{}{
			"name":             name,
			"base_hostname":    brandHostnames[i],
			"primary_ns":       "ns1." + brandHostnames[i],
			"secondary_ns":     "ns2." + brandHostnames[i],
			"hostmaster_email": "hostmaster@" + brandHostnames[i],
		})
		require.Equal(t, 201, resp.StatusCode, "create brand %s: %s", name, body)
		brand := parseJSON(t, body)
		brandIDs[i] = brand["id"].(string)
		bid := brandIDs[i]
		t.Cleanup(func() {
			httpDelete(t, coreAPIURL+"/brands/"+bid)
		})
	}
	brandAID := brandIDs[0]
	brandBID := brandIDs[1]

	// Set cluster access for both brands.
	regionID := findFirstRegionID(t)
	cluster := findFirstCluster(t, regionID)
	clusterID, _ := cluster["id"].(string)

	for _, bid := range []string{brandAID, brandBID} {
		resp, body := httpPut(t, coreAPIURL+"/brands/"+bid+"/clusters", map[string]interface{}{
			"cluster_ids": []string{clusterID},
		})
		require.Equal(t, 200, resp.StatusCode, "set clusters for %s: %s", bid, body)
	}

	// Create API keys scoped to each brand.
	resp, body := httpPost(t, coreAPIURL+"/api-keys", map[string]interface{}{
		"name":   "e2e-key-brand-a",
		"scopes": []string{"*:*"},
		"brands": []string{brandAID},
	})
	require.Equal(t, 201, resp.StatusCode, "create key A: %s", body)
	keyAData := parseJSON(t, body)
	keyA := keyAData["key"].(string)
	keyAID := keyAData["id"].(string)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/api-keys/"+keyAID) })

	resp, body = httpPost(t, coreAPIURL+"/api-keys", map[string]interface{}{
		"name":   "e2e-key-brand-b",
		"scopes": []string{"*:*"},
		"brands": []string{brandBID},
	})
	require.Equal(t, 201, resp.StatusCode, "create key B: %s", body)
	keyBData := parseJSON(t, body)
	keyB := keyBData["key"].(string)
	keyBID := keyBData["id"].(string)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/api-keys/"+keyBID) })

	webShard := findShardByRole(t, clusterID, "web")
	webShardID, _ := webShard["id"].(string)

	// Create tenant under brand A using key A.
	resp, body = httpPostWithKey(t, coreAPIURL+"/tenants", map[string]interface{}{
		"brand_id":   brandAID,
		"region_id":  regionID,
		"cluster_id": clusterID,
		"shard_id":   webShardID,
	}, keyA)
	require.Equal(t, 202, resp.StatusCode, "create tenant A: %s", body)
	tenantA := parseJSON(t, body)
	tenantAID := tenantA["id"].(string)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/tenants/"+tenantAID) })
	t.Logf("created tenant A: %s", tenantAID)

	// Create tenant under brand B using key B.
	resp, body = httpPostWithKey(t, coreAPIURL+"/tenants", map[string]interface{}{
		"brand_id":   brandBID,
		"region_id":  regionID,
		"cluster_id": clusterID,
		"shard_id":   webShardID,
	}, keyB)
	require.Equal(t, 202, resp.StatusCode, "create tenant B: %s", body)
	tenantB := parseJSON(t, body)
	tenantBID := tenantB["id"].(string)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/tenants/"+tenantBID) })
	t.Logf("created tenant B: %s", tenantBID)

	// Test: Key A cannot see Brand B's tenant.
	resp, _ = httpGetWithKey(t, coreAPIURL+"/tenants/"+tenantBID, keyA)
	require.True(t, resp.StatusCode == 403 || resp.StatusCode == 404,
		"key A should not see tenant B, got %d", resp.StatusCode)
	t.Logf("key A correctly cannot see tenant B (status %d)", resp.StatusCode)

	// Test: Key B cannot see Brand A's tenant.
	resp, _ = httpGetWithKey(t, coreAPIURL+"/tenants/"+tenantAID, keyB)
	require.True(t, resp.StatusCode == 403 || resp.StatusCode == 404,
		"key B should not see tenant A, got %d", resp.StatusCode)
	t.Logf("key B correctly cannot see tenant A (status %d)", resp.StatusCode)

	// Test: Platform admin key can see both.
	resp, body = httpGet(t, coreAPIURL+"/tenants/"+tenantAID)
	require.Equal(t, 200, resp.StatusCode, "admin should see tenant A")
	resp, body = httpGet(t, coreAPIURL+"/tenants/"+tenantBID)
	require.Equal(t, 200, resp.StatusCode, "admin should see tenant B")
	t.Logf("platform admin can see both tenants")
}
