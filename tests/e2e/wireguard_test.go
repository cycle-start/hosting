package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createWireGuardTestTenant creates a tenant with a subscription for WireGuard tests.
func createWireGuardTestTenant(t *testing.T, name string) (tenantID, subID string) {
	t.Helper()
	tid, _, _, _, _ := createTestTenant(t, name)
	subID = createTestSubscription(t, tid, name)
	return tid, subID
}

func TestWireGuardPeerCRUD(t *testing.T) {
	tenantID, subID := createWireGuardTestTenant(t, "e2e-wg-crud")

	// Create WireGuard peer.
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/wireguard-peers", coreAPIURL, tenantID), map[string]interface{}{
		"name":            "e2e-test-peer",
		"subscription_id": subID,
	})
	require.Equal(t, 202, resp.StatusCode, "create wireguard peer: %s", body)
	result := parseJSON(t, body)

	// Create response wraps peer in a "peer" key and includes private_key + client_config.
	peerObj, ok := result["peer"].(map[string]interface{})
	require.True(t, ok, "response should have peer object: %s", body)
	peerID := peerObj["id"].(string)
	require.NotEmpty(t, peerID)
	t.Logf("created WireGuard peer: %s", peerID)

	privateKey, _ := result["private_key"].(string)
	require.NotEmpty(t, privateKey, "private_key should be returned on create")
	clientConfig, _ := result["client_config"].(string)
	require.NotEmpty(t, clientConfig, "client_config should be returned on create")
	t.Logf("got client config (%d bytes)", len(clientConfig))

	// Verify peer fields.
	require.Equal(t, "e2e-test-peer", peerObj["name"])
	require.NotEmpty(t, peerObj["public_key"], "public_key should be set")
	require.NotEmpty(t, peerObj["assigned_ip"], "assigned_ip should be set")

	// Wait for peer to become active.
	peer := waitForStatus(t, coreAPIURL+"/wireguard-peers/"+peerID, "active", provisionTimeout)
	require.Equal(t, "active", peer["status"])
	t.Logf("WireGuard peer active: ip=%s", peer["assigned_ip"])

	// List peers for tenant.
	resp, body = httpGet(t, fmt.Sprintf("%s/tenants/%s/wireguard-peers", coreAPIURL, tenantID))
	require.Equal(t, 200, resp.StatusCode, body)
	peers := parsePaginatedItems(t, body)
	found := false
	for _, p := range peers {
		if id, _ := p["id"].(string); id == peerID {
			found = true
			break
		}
	}
	require.True(t, found, "peer %s not in tenant list", peerID)

	// Get peer by ID.
	resp, body = httpGet(t, coreAPIURL+"/wireguard-peers/"+peerID)
	require.Equal(t, 200, resp.StatusCode, body)
	detail := parseJSON(t, body)
	require.Equal(t, "e2e-test-peer", detail["name"])
	require.Equal(t, "active", detail["status"])

	// Delete peer.
	resp, body = httpDelete(t, coreAPIURL+"/wireguard-peers/"+peerID)
	require.Equal(t, 202, resp.StatusCode, "delete wireguard peer: %s", body)

	// Wait for deletion (hard-delete returns 404).
	waitForStatus(t, coreAPIURL+"/wireguard-peers/"+peerID, "deleted", provisionTimeout)
	t.Logf("WireGuard peer deleted")
}

func TestWireGuardPeerMultiple(t *testing.T) {
	tenantID, subID := createWireGuardTestTenant(t, "e2e-wg-multi")

	// Create 3 peers.
	var peerIDs []string
	for i := 0; i < 3; i++ {
		resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/wireguard-peers", coreAPIURL, tenantID), map[string]interface{}{
			"name":            fmt.Sprintf("e2e-peer-%d", i),
			"subscription_id": subID,
		})
		require.Equal(t, 202, resp.StatusCode, "create peer %d: %s", i, body)
		result := parseJSON(t, body)
		peerObj := result["peer"].(map[string]interface{})
		peerID := peerObj["id"].(string)
		peerIDs = append(peerIDs, peerID)
		waitForStatus(t, coreAPIURL+"/wireguard-peers/"+peerID, "active", provisionTimeout)
		t.Cleanup(func() { httpDelete(t, coreAPIURL+"/wireguard-peers/"+peerID) })
	}

	// List should show all 3.
	resp, body := httpGet(t, fmt.Sprintf("%s/tenants/%s/wireguard-peers", coreAPIURL, tenantID))
	require.Equal(t, 200, resp.StatusCode, body)
	peers := parsePaginatedItems(t, body)
	require.GreaterOrEqual(t, len(peers), 3, "should have at least 3 peers")
	t.Logf("found %d WireGuard peers", len(peers))

	// Each peer should have a unique assigned_ip.
	ips := make(map[string]bool)
	for _, p := range peers {
		ip, _ := p["assigned_ip"].(string)
		require.NotEmpty(t, ip)
		require.False(t, ips[ip], "duplicate assigned_ip: %s", ip)
		ips[ip] = true
	}

	// Delete one.
	resp, body = httpDelete(t, coreAPIURL+"/wireguard-peers/"+peerIDs[0])
	require.Equal(t, 202, resp.StatusCode, "delete peer: %s", body)
	waitForStatus(t, coreAPIURL+"/wireguard-peers/"+peerIDs[0], "deleted", provisionTimeout)
	t.Logf("deleted peer %s", peerIDs[0])
}

// TestWireGuardPeerServiceMetadata verifies that creating a WireGuard peer
// for a tenant with databases and Valkey instances includes service metadata
// comments in the client_config (used by hosting-cli for auto-proxy).
func TestWireGuardPeerServiceMetadata(t *testing.T) {
	// Create tenant with full infrastructure: web shard + DB shard + Valkey shard.
	tenantID, _, clusterID, _, dbShardID := createTestTenant(t, "e2e-wg-svcmeta")
	if dbShardID == "" {
		t.Skip("no database shard found in cluster; skipping service metadata test")
	}
	valkeyShardID := findValkeyShardID(t, clusterID)
	if valkeyShardID == "" {
		t.Skip("no valkey shard found in cluster; skipping service metadata test")
	}

	// Create a subscription (required for database, Valkey, and WireGuard).
	subID := createTestSubscription(t, tenantID, "e2e-wg-svcmeta")

	// Create a database and Valkey instance for the tenant.
	dbID := createTestDatabase(t, tenantID, dbShardID, subID)
	t.Logf("created database: %s", dbID)

	valkeyID := createTestValkeyInstance(t, tenantID, valkeyShardID, subID)
	t.Logf("created valkey instance: %s", valkeyID)

	// Create WireGuard peer.
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/wireguard-peers", coreAPIURL, tenantID), map[string]interface{}{
		"name":            "e2e-svcmeta-peer",
		"subscription_id": subID,
	})
	require.Equal(t, 202, resp.StatusCode, "create wireguard peer: %s", body)
	result := parseJSON(t, body)
	peerObj := result["peer"].(map[string]interface{})
	peerID := peerObj["id"].(string)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/wireguard-peers/"+peerID) })

	clientConfig, _ := result["client_config"].(string)
	require.NotEmpty(t, clientConfig, "client_config should be returned on create")
	t.Logf("client_config:\n%s", clientConfig)

	// Verify the client_config contains service metadata comments.
	assert.Contains(t, clientConfig, "# hosting-cli:services", "client_config should contain service metadata header")

	// Parse service lines from the config.
	var mysqlAddr, valkeyAddr string
	inServices := false
	for _, line := range strings.Split(clientConfig, "\n") {
		line = strings.TrimSpace(line)
		if line == "# hosting-cli:services" {
			inServices = true
			continue
		}
		if inServices && strings.HasPrefix(line, "# ") {
			parts := strings.SplitN(strings.TrimPrefix(line, "# "), "=", 2)
			if len(parts) == 2 {
				switch strings.TrimSpace(parts[0]) {
				case "mysql":
					mysqlAddr = strings.TrimSpace(parts[1])
				case "valkey":
					valkeyAddr = strings.TrimSpace(parts[1])
				}
			}
		}
	}

	// Both services should have ULA addresses.
	assert.NotEmpty(t, mysqlAddr, "client_config should contain mysql service address")
	assert.NotEmpty(t, valkeyAddr, "client_config should contain valkey service address")
	t.Logf("service addresses: mysql=%s valkey=%s", mysqlAddr, valkeyAddr)

	// Addresses should be ULA (fd00: prefix).
	if mysqlAddr != "" {
		assert.True(t, strings.HasPrefix(mysqlAddr, "fd00:"), "mysql address should be ULA: %s", mysqlAddr)
	}
	if valkeyAddr != "" {
		assert.True(t, strings.HasPrefix(valkeyAddr, "fd00:"), "valkey address should be ULA: %s", valkeyAddr)
	}

	// MySQL and Valkey should have different addresses (different shard indices).
	if mysqlAddr != "" && valkeyAddr != "" {
		assert.NotEqual(t, mysqlAddr, valkeyAddr, "mysql and valkey should have different ULA addresses")
	}

	// Wait for peer to become active (validates the full workflow still works).
	waitForStatus(t, coreAPIURL+"/wireguard-peers/"+peerID, "active", provisionTimeout)
	t.Logf("WireGuard peer with service metadata active")
}
