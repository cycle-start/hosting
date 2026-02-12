package activity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseECKey(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	der, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})

	parsed, err := parseECKey(pemBytes)
	require.NoError(t, err)
	assert.True(t, key.Equal(parsed))
}

func TestParseECKey_InvalidPEM(t *testing.T) {
	_, err := parseECKey([]byte("not-pem"))
	assert.Error(t, err)
}

func TestNewACMEActivity(t *testing.T) {
	a := NewACMEActivity("test@example.com", "https://acme-staging-v02.api.letsencrypt.org/directory")
	assert.Equal(t, "test@example.com", a.email)
	assert.Equal(t, "https://acme-staging-v02.api.letsencrypt.org/directory", a.directoryURL)
}
