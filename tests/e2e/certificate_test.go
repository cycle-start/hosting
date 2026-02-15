package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCertificateUpload(t *testing.T) {
	tenantID, _, _, _, _ := createTestTenant(t, "e2e-cert")
	webrootID := createTestWebroot(t, tenantID, "cert-site", "static", "1")
	fqdnID := createTestFQDN(t, webrootID, "secure.e2e-cert.example.com.")

	// Generate self-signed certificate.
	certPEM, keyPEM := generateSelfSignedCert(t, "secure.e2e-cert.example.com")

	// Upload certificate.
	resp, body := httpPost(t, fmt.Sprintf("%s/fqdns/%s/certificates", coreAPIURL, fqdnID), map[string]interface{}{
		"cert_pem": certPEM,
		"key_pem":  keyPEM,
	})
	require.Equal(t, 202, resp.StatusCode, "upload certificate: %s", body)
	cert := parseJSON(t, body)
	certID := cert["id"].(string)
	require.NotEmpty(t, certID)
	t.Logf("uploaded certificate: %s", certID)

	// Wait for certificate to become active.
	cert = waitForStatus(t, coreAPIURL+"/certificates/"+certID, "active", provisionTimeout)
	require.Equal(t, "active", cert["status"])
	t.Logf("certificate active")

	// List certificates for the FQDN.
	resp, body = httpGet(t, fmt.Sprintf("%s/fqdns/%s/certificates", coreAPIURL, fqdnID))
	require.Equal(t, 200, resp.StatusCode, body)
	certs := parsePaginatedItems(t, body)
	found := false
	for _, c := range certs {
		if id, _ := c["id"].(string); id == certID {
			found = true
			// key_pem should be redacted.
			kp, _ := c["key_pem"].(string)
			require.Empty(t, kp, "key_pem should be redacted in list response")
			break
		}
	}
	require.True(t, found, "certificate %s not in FQDN cert list", certID)
}
