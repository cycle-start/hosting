package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSSHKey_Valid(t *testing.T) {
	// Valid ed25519 key
	key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGKCwmDZb5JjFMYnbPPM6MvxMCEjMltcGacM4AiSuKiP test@localhost"
	fingerprint, err := parseSSHKey(key)
	require.NoError(t, err)
	assert.Contains(t, fingerprint, "SHA256:")
}

func TestParseSSHKey_Invalid(t *testing.T) {
	_, err := parseSSHKey("not-a-valid-ssh-key")
	assert.Error(t, err)
}

func TestGeneratePassword_Length(t *testing.T) {
	pw := generatePassword()
	assert.Len(t, pw, 32)
}

func TestGeneratePassword_Unique(t *testing.T) {
	pw1 := generatePassword()
	pw2 := generatePassword()
	assert.NotEqual(t, pw1, pw2)
}
