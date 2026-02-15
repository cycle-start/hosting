package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmailAccountCRUD(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-email-crud")
	webrootID := createTestWebroot(t, tenantID, "email-site", "static", "1")
	fqdnID := createTestFQDN(t, webrootID, "mail.e2e-email.example.com.")

	// Create email account.
	resp, body := httpPost(t, fmt.Sprintf("%s/fqdns/%s/email-accounts", coreAPIURL, fqdnID), map[string]interface{}{
		"address":      "test@mail.e2e-email.example.com",
		"display_name": "Test User",
		"quota_bytes":  1073741824,
	})
	if resp.StatusCode == 404 || resp.StatusCode == 500 {
		t.Skip("email endpoints not available (Stalwart may not be deployed)")
	}
	require.Equal(t, 202, resp.StatusCode, "create email account: %s", body)
	acct := parseJSON(t, body)
	acctID := acct["id"].(string)
	require.NotEmpty(t, acctID)
	t.Logf("created email account: %s", acctID)

	// Wait for account to become active.
	acct = waitForStatus(t, coreAPIURL+"/email-accounts/"+acctID, "active", provisionTimeout)
	require.Equal(t, "active", acct["status"])
	t.Logf("email account active")

	// List email accounts for the FQDN.
	resp, body = httpGet(t, fmt.Sprintf("%s/fqdns/%s/email-accounts", coreAPIURL, fqdnID))
	require.Equal(t, 200, resp.StatusCode, body)
	accounts := parsePaginatedItems(t, body)
	found := false
	for _, a := range accounts {
		if id, _ := a["id"].(string); id == acctID {
			found = true
			break
		}
	}
	require.True(t, found, "email account %s not found in FQDN list", acctID)

	// Create alias.
	resp, body = httpPost(t, fmt.Sprintf("%s/email-accounts/%s/aliases", coreAPIURL, acctID), map[string]interface{}{
		"address": "contact@mail.e2e-email.example.com",
	})
	require.Equal(t, 202, resp.StatusCode, "create alias: %s", body)
	alias := parseJSON(t, body)
	aliasID := alias["id"].(string)
	t.Logf("created alias: %s", aliasID)

	// Wait for alias active.
	waitForStatus(t, coreAPIURL+"/email-aliases/"+aliasID, "active", provisionTimeout)

	// Create forward.
	resp, body = httpPost(t, fmt.Sprintf("%s/email-accounts/%s/forwards", coreAPIURL, acctID), map[string]interface{}{
		"destination": "external@example.com",
		"keep_copy":   true,
	})
	require.Equal(t, 202, resp.StatusCode, "create forward: %s", body)
	fwd := parseJSON(t, body)
	fwdID := fwd["id"].(string)
	t.Logf("created forward: %s", fwdID)

	waitForStatus(t, coreAPIURL+"/email-forwards/"+fwdID, "active", provisionTimeout)

	// Set auto-reply.
	resp, body = httpPut(t, fmt.Sprintf("%s/email-accounts/%s/autoreply", coreAPIURL, acctID), map[string]interface{}{
		"subject": "Out of office",
		"body":    "I am away.",
		"enabled": true,
	})
	require.True(t, resp.StatusCode == 200 || resp.StatusCode == 202, "set auto-reply: %d %s", resp.StatusCode, body)
	t.Logf("auto-reply set")

	// List aliases.
	resp, body = httpGet(t, fmt.Sprintf("%s/email-accounts/%s/aliases", coreAPIURL, acctID))
	require.Equal(t, 200, resp.StatusCode, body)

	// List forwards.
	resp, body = httpGet(t, fmt.Sprintf("%s/email-accounts/%s/forwards", coreAPIURL, acctID))
	require.Equal(t, 200, resp.StatusCode, body)

	// Get auto-reply.
	resp, body = httpGet(t, fmt.Sprintf("%s/email-accounts/%s/autoreply", coreAPIURL, acctID))
	require.Equal(t, 200, resp.StatusCode, body)

	// Delete alias.
	resp, body = httpDelete(t, coreAPIURL+"/email-aliases/"+aliasID)
	require.Equal(t, 202, resp.StatusCode, "delete alias: %s", body)

	// Delete forward.
	resp, body = httpDelete(t, coreAPIURL+"/email-forwards/"+fwdID)
	require.Equal(t, 202, resp.StatusCode, "delete forward: %s", body)

	// Delete auto-reply.
	resp, body = httpDelete(t, fmt.Sprintf("%s/email-accounts/%s/autoreply", coreAPIURL, acctID))
	require.True(t, resp.StatusCode == 200 || resp.StatusCode == 202, "delete auto-reply: %d %s", resp.StatusCode, body)

	// Delete account.
	resp, body = httpDelete(t, coreAPIURL+"/email-accounts/"+acctID)
	require.Equal(t, 202, resp.StatusCode, "delete email account: %s", body)
	t.Logf("email account deleted")
}
