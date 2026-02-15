package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeTenantULA(t *testing.T) {
	// Basic computation — verify format and determinism.
	ula := ComputeTenantULA("osl-1", 1, 10000)
	assert.Contains(t, ula, "fd00:")
	assert.Contains(t, ula, "::") // tenant UID is in the last segment

	// Same inputs produce same output.
	ula2 := ComputeTenantULA("osl-1", 1, 10000)
	assert.Equal(t, ula, ula2)

	// Different cluster ID produces different address.
	ula3 := ComputeTenantULA("osl-2", 1, 10000)
	assert.NotEqual(t, ula, ula3)

	// Different node shard index produces different address.
	ula4 := ComputeTenantULA("osl-1", 2, 10000)
	assert.NotEqual(t, ula, ula4)

	// Different tenant UID produces different address.
	ula5 := ComputeTenantULA("osl-1", 1, 10001)
	assert.NotEqual(t, ula, ula5)
}

func TestComputeTenantULA_Format(t *testing.T) {
	// Verify the exact format with known values.
	// FNV-32a of "dev" modulo 0xFFFF.
	ula := ComputeTenantULA("dev", 0, 1000)
	// The format should be fd00:{hash}:{index}::{uid_hex}
	assert.Regexp(t, `^fd00:[0-9a-f]+:0::3e8$`, ula)
}

func TestFormatDaemonProxyURL_IPv6(t *testing.T) {
	url := FormatDaemonProxyURL("fd00:1:2::a", 14523)
	assert.Equal(t, "http://[fd00:1:2::a]:14523", url)
}

func TestFormatDaemonProxyURL_IPv4(t *testing.T) {
	url := FormatDaemonProxyURL("127.0.0.1", 14523)
	assert.Equal(t, "http://127.0.0.1:14523", url)
}
