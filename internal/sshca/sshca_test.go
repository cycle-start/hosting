package sshca

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func generateTestCAKey(t *testing.T) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKey(priv, "test-ca")
	require.NoError(t, err)
	return pem.EncodeToMemory(block)
}

func TestNew(t *testing.T) {
	pem := generateTestCAKey(t)
	ca, err := New(pem)
	require.NoError(t, err)
	assert.NotNil(t, ca)
}

func TestNew_InvalidPEM(t *testing.T) {
	_, err := New([]byte("not a valid key"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse private key")
}

func TestSign(t *testing.T) {
	pem := generateTestCAKey(t)
	ca, err := New(pem)
	require.NoError(t, err)

	signer, err := ca.Sign("testuser", 60*time.Second)
	require.NoError(t, err)
	require.NotNil(t, signer)

	// The signer's public key should be a certificate.
	cert, ok := signer.PublicKey().(*ssh.Certificate)
	require.True(t, ok, "public key should be a certificate")

	assert.Equal(t, uint32(ssh.UserCert), cert.CertType)
	assert.Equal(t, []string{"testuser"}, cert.ValidPrincipals)

	// Check time bounds.
	now := uint64(time.Now().Unix())
	assert.Less(t, cert.ValidAfter, now, "ValidAfter should be in the past (clock skew allowance)")
	assert.Greater(t, cert.ValidBefore, now, "ValidBefore should be in the future")
}

func TestSign_DifferentPrincipals(t *testing.T) {
	pem := generateTestCAKey(t)
	ca, err := New(pem)
	require.NoError(t, err)

	s1, err := ca.Sign("alice", time.Minute)
	require.NoError(t, err)
	s2, err := ca.Sign("bob", time.Minute)
	require.NoError(t, err)

	cert1 := s1.PublicKey().(*ssh.Certificate)
	cert2 := s2.PublicKey().(*ssh.Certificate)

	assert.Equal(t, []string{"alice"}, cert1.ValidPrincipals)
	assert.Equal(t, []string{"bob"}, cert2.ValidPrincipals)
}

func TestSign_TTL(t *testing.T) {
	pem := generateTestCAKey(t)
	ca, err := New(pem)
	require.NoError(t, err)

	ttl := 5 * time.Minute
	signer, err := ca.Sign("user", ttl)
	require.NoError(t, err)

	cert := signer.PublicKey().(*ssh.Certificate)
	validDuration := time.Unix(int64(cert.ValidBefore), 0).Sub(time.Unix(int64(cert.ValidAfter), 0))

	// Should be approximately ttl + 30s clock skew.
	expected := ttl + 30*time.Second
	assert.InDelta(t, expected.Seconds(), validDuration.Seconds(), 2)
}

func TestSign_EphemeralKeysAreUnique(t *testing.T) {
	pem := generateTestCAKey(t)
	ca, err := New(pem)
	require.NoError(t, err)

	s1, err := ca.Sign("user", time.Minute)
	require.NoError(t, err)
	s2, err := ca.Sign("user", time.Minute)
	require.NoError(t, err)

	// Each call generates a new ephemeral key, so certificates should differ.
	cert1 := s1.PublicKey().(*ssh.Certificate)
	cert2 := s2.PublicKey().(*ssh.Certificate)
	assert.NotEqual(t, cert1.Key.Marshal(), cert2.Key.Marshal())
}
