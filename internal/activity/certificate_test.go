package activity

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateSelfSignedCert creates a self-signed certificate and matching RSA private key
// for use in tests. Both are returned as PEM-encoded strings.
func generateSelfSignedCert(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test.example.com"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	return
}

func TestValidateCustomCert_ValidCert(t *testing.T) {
	certPEM, keyPEM := generateSelfSignedCert(t)
	// CertificateActivity with nil coreDB is fine since ValidateCustomCert does not use the DB.
	a := &CertificateActivity{}

	err := a.ValidateCustomCert(context.Background(), certPEM, keyPEM)
	assert.NoError(t, err)
}

func TestValidateCustomCert_MismatchedKey(t *testing.T) {
	certPEM, _ := generateSelfSignedCert(t)
	_, otherKey := generateSelfSignedCert(t) // Generate a different key pair.

	a := &CertificateActivity{}
	err := a.ValidateCustomCert(context.Background(), certPEM, otherKey)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate and key do not match")
}

func TestValidateCustomCert_InvalidCertPEM(t *testing.T) {
	_, keyPEM := generateSelfSignedCert(t)

	a := &CertificateActivity{}
	err := a.ValidateCustomCert(context.Background(), "not-a-pem-cert", keyPEM)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate and key do not match")
}

func TestValidateCustomCert_InvalidKeyPEM(t *testing.T) {
	certPEM, _ := generateSelfSignedCert(t)

	a := &CertificateActivity{}
	err := a.ValidateCustomCert(context.Background(), certPEM, "not-a-pem-key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate and key do not match")
}
