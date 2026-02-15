package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSSHKeyCRUD(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-ssh-crud")
	pubKey := generateSSHKeyPair(t)

	// Create SSH key.
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/ssh-keys", coreAPIURL, tenantID), map[string]interface{}{
		"name":       "e2e-deploy-key",
		"public_key": pubKey,
	})
	require.Equal(t, 202, resp.StatusCode, "create SSH key: %s", body)
	sshKey := parseJSON(t, body)
	keyID := sshKey["id"].(string)
	require.NotEmpty(t, keyID)
	t.Logf("created SSH key: %s", keyID)

	// Wait for key to become active.
	sshKey = waitForStatus(t, coreAPIURL+"/ssh-keys/"+keyID, "active", provisionTimeout)
	require.Equal(t, "active", sshKey["status"])
	t.Logf("SSH key active")

	// Verify fingerprint.
	fingerprint, _ := sshKey["fingerprint"].(string)
	require.NotEmpty(t, fingerprint, "fingerprint should be computed")
	t.Logf("fingerprint: %s", fingerprint)

	// List SSH keys for tenant.
	resp, body = httpGet(t, fmt.Sprintf("%s/tenants/%s/ssh-keys", coreAPIURL, tenantID))
	require.Equal(t, 200, resp.StatusCode, body)
	keys := parsePaginatedItems(t, body)
	found := false
	for _, k := range keys {
		if id, _ := k["id"].(string); id == keyID {
			found = true
			break
		}
	}
	require.True(t, found, "SSH key %s not in tenant list", keyID)

	// Get SSH key by ID.
	resp, body = httpGet(t, coreAPIURL+"/ssh-keys/"+keyID)
	require.Equal(t, 200, resp.StatusCode, body)
	detail := parseJSON(t, body)
	require.Equal(t, pubKey, detail["public_key"])

	// Delete SSH key.
	resp, body = httpDelete(t, coreAPIURL+"/ssh-keys/"+keyID)
	require.Equal(t, 202, resp.StatusCode, "delete SSH key: %s", body)
	t.Logf("SSH key deleted")
}

func TestSSHKeyMultipleKeys(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-ssh-multi")

	var keyIDs []string
	for i := 0; i < 3; i++ {
		pubKey := generateSSHKeyPair(t)
		resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/ssh-keys", coreAPIURL, tenantID), map[string]interface{}{
			"name":       fmt.Sprintf("e2e-key-%d", i),
			"public_key": pubKey,
		})
		require.Equal(t, 202, resp.StatusCode, "create SSH key %d: %s", i, body)
		sshKey := parseJSON(t, body)
		keyID := sshKey["id"].(string)
		keyIDs = append(keyIDs, keyID)
		waitForStatus(t, coreAPIURL+"/ssh-keys/"+keyID, "active", provisionTimeout)
		t.Cleanup(func() { httpDelete(t, coreAPIURL+"/ssh-keys/"+keyID) })
	}

	// List should show all 3.
	resp, body := httpGet(t, fmt.Sprintf("%s/tenants/%s/ssh-keys", coreAPIURL, tenantID))
	require.Equal(t, 200, resp.StatusCode, body)
	keys := parsePaginatedItems(t, body)
	require.GreaterOrEqual(t, len(keys), 3, "should have at least 3 SSH keys")
	t.Logf("found %d SSH keys", len(keys))

	// Delete one.
	resp, body = httpDelete(t, coreAPIURL+"/ssh-keys/"+keyIDs[0])
	require.Equal(t, 202, resp.StatusCode, "delete key: %s", body)
	t.Logf("deleted key %s", keyIDs[0])
}
