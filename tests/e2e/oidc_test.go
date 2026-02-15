package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOIDCDiscovery(t *testing.T) {
	// The OIDC discovery endpoint is public (no auth required).
	// Use the base URL without /api/v1.
	baseURL := "https://api.hosting.test"
	resp, body := httpGet(t, baseURL+"/.well-known/openid-configuration")
	if resp.StatusCode == 404 {
		t.Skip("OIDC not configured")
	}
	require.Equal(t, 200, resp.StatusCode, "OIDC discovery: %s", body)
	config := parseJSON(t, body)
	require.NotEmpty(t, config["issuer"], "issuer should be set")
	require.NotEmpty(t, config["jwks_uri"], "jwks_uri should be set")
	t.Logf("OIDC issuer: %s", config["issuer"])
}

func TestOIDCJWKS(t *testing.T) {
	baseURL := "https://api.hosting.test"
	resp, body := httpGet(t, baseURL+"/oidc/jwks")
	if resp.StatusCode == 404 {
		t.Skip("OIDC not configured")
	}
	require.Equal(t, 200, resp.StatusCode, "OIDC JWKS: %s", body)
	jwks := parseJSON(t, body)
	_, hasKeys := jwks["keys"]
	require.True(t, hasKeys, "JWKS response missing 'keys'")
	t.Logf("JWKS: %s", body)
}
