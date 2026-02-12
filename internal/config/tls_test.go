package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemporalTLS_NoConfig(t *testing.T) {
	cfg := &Config{}
	tlsCfg, err := cfg.TemporalTLS()
	require.NoError(t, err)
	assert.Nil(t, tlsCfg)
}

func TestTemporalTLS_ValidCertKey(t *testing.T) {
	certPath, keyPath, _ := generateTestCert(t)

	cfg := &Config{
		TemporalTLSCert: certPath,
		TemporalTLSKey:  keyPath,
	}
	tlsCfg, err := cfg.TemporalTLS()
	require.NoError(t, err)
	require.NotNil(t, tlsCfg)
	assert.Len(t, tlsCfg.Certificates, 1)
	assert.Nil(t, tlsCfg.RootCAs)
}

func TestTemporalTLS_MissingCertFile(t *testing.T) {
	cfg := &Config{
		TemporalTLSCert: "/nonexistent/cert.pem",
		TemporalTLSKey:  "/nonexistent/key.pem",
	}
	_, err := cfg.TemporalTLS()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load temporal client cert")
}

func TestTemporalTLS_WithCACert(t *testing.T) {
	certPath, keyPath, caCertPath := generateTestCert(t)

	cfg := &Config{
		TemporalTLSCert:   certPath,
		TemporalTLSKey:    keyPath,
		TemporalTLSCACert: caCertPath,
	}
	tlsCfg, err := cfg.TemporalTLS()
	require.NoError(t, err)
	require.NotNil(t, tlsCfg)
	assert.NotNil(t, tlsCfg.RootCAs)
}

func TestTemporalTLS_ServerName(t *testing.T) {
	certPath, keyPath, _ := generateTestCert(t)

	cfg := &Config{
		TemporalTLSCert:       certPath,
		TemporalTLSKey:        keyPath,
		TemporalTLSServerName: "temporal.example.com",
	}
	tlsCfg, err := cfg.TemporalTLS()
	require.NoError(t, err)
	require.NotNil(t, tlsCfg)
	assert.Equal(t, "temporal.example.com", tlsCfg.ServerName)
}

func TestTemporalTLS_InvalidCACert(t *testing.T) {
	certPath, keyPath, _ := generateTestCert(t)

	badCA := filepath.Join(t.TempDir(), "bad-ca.pem")
	os.WriteFile(badCA, []byte("not a cert"), 0o600)

	cfg := &Config{
		TemporalTLSCert:   certPath,
		TemporalTLSKey:    keyPath,
		TemporalTLSCACert: badCA,
	}
	_, err := cfg.TemporalTLS()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse temporal CA cert")
}

// generateTestCert creates a self-signed CA and client cert for testing.
// Returns paths to (cert.pem, key.pem, ca.pem).
func generateTestCert(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()

	// Generate CA
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)

	caCertPath := filepath.Join(dir, "ca.pem")
	writePEM(t, caCertPath, "CERTIFICATE", caCertDER)

	// Generate client cert signed by CA
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-client"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientCertDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, &clientKey.PublicKey, caKey)
	require.NoError(t, err)

	certPath := filepath.Join(dir, "cert.pem")
	writePEM(t, certPath, "CERTIFICATE", clientCertDER)

	keyDER, err := x509.MarshalECPrivateKey(clientKey)
	require.NoError(t, err)
	keyPath := filepath.Join(dir, "key.pem")
	writePEM(t, keyPath, "EC PRIVATE KEY", keyDER)

	return certPath, keyPath, caCertPath
}

func writePEM(t *testing.T, path, blockType string, data []byte) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, pem.Encode(f, &pem.Block{Type: blockType, Bytes: data}))
}
