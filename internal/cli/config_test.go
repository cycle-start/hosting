package cli

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfigString(t *testing.T) {
	config := `[Interface]
PrivateKey = YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=
Address = fd00:abcd:ffff::1/128

[Peer]
PublicKey = c2VydmVycHVibGlja2V5MTIzNDU2Nzg5MGFiY2RlZmc=
PresharedKey = cHJlc2hhcmVka2V5MTIzNDU2Nzg5MGFiY2RlZmdoaWo=
Endpoint = gw.massive-hosting.com:51820
AllowedIPs = fd00::/16
PersistentKeepalive = 25

# hosting-cli:services
# mysql=fd00:abcd:101::1388
# valkey=fd00:abcd:201::1388
`
	cfg, err := ParseConfigString(config)
	require.NoError(t, err)

	assert.Equal(t, "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=", cfg.PrivateKey)
	assert.Equal(t, netip.MustParsePrefix("fd00:abcd:ffff::1/128"), cfg.Address)
	assert.Equal(t, "c2VydmVycHVibGlja2V5MTIzNDU2Nzg5MGFiY2RlZmc=", cfg.PublicKey)
	assert.Equal(t, "cHJlc2hhcmVka2V5MTIzNDU2Nzg5MGFiY2RlZmdoaWo=", cfg.PresharedKey)
	assert.Equal(t, "gw.massive-hosting.com:51820", cfg.Endpoint)
	assert.Equal(t, 25, cfg.PersistentKeepalive)

	require.Len(t, cfg.AllowedIPs, 1)
	assert.Equal(t, netip.MustParsePrefix("fd00::/16"), cfg.AllowedIPs[0])

	require.Len(t, cfg.Services, 2)
	assert.Equal(t, "mysql", cfg.Services[0].Type)
	assert.Equal(t, "fd00:abcd:101::1388", cfg.Services[0].Address)
	assert.Equal(t, "valkey", cfg.Services[1].Type)
	assert.Equal(t, "fd00:abcd:201::1388", cfg.Services[1].Address)
}

func TestParseConfigString_NoServices(t *testing.T) {
	config := `[Interface]
PrivateKey = YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=
Address = fd00:abcd:ffff::1/128

[Peer]
PublicKey = c2VydmVycHVibGlja2V5MTIzNDU2Nzg5MGFiY2RlZmc=
Endpoint = gw.massive-hosting.com:51820
AllowedIPs = fd00::/16
PersistentKeepalive = 25
`
	cfg, err := ParseConfigString(config)
	require.NoError(t, err)
	assert.Empty(t, cfg.Services)
	assert.Empty(t, cfg.PresharedKey)
}

func TestParseConfigString_MultipleAllowedIPs(t *testing.T) {
	config := `[Interface]
PrivateKey = YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=
Address = fd00:abcd:ffff::1/128

[Peer]
PublicKey = c2VydmVycHVibGlja2V5MTIzNDU2Nzg5MGFiY2RlZmc=
Endpoint = gw.massive-hosting.com:51820
AllowedIPs = fd00::/16, fc00::/7
PersistentKeepalive = 25
`
	cfg, err := ParseConfigString(config)
	require.NoError(t, err)
	require.Len(t, cfg.AllowedIPs, 2)
	assert.Equal(t, netip.MustParsePrefix("fd00::/16"), cfg.AllowedIPs[0])
	assert.Equal(t, netip.MustParsePrefix("fc00::/7"), cfg.AllowedIPs[1])
}

func TestParseConfigString_MissingPrivateKey(t *testing.T) {
	config := `[Interface]
Address = fd00:abcd:ffff::1/128

[Peer]
PublicKey = c2VydmVycHVibGlja2V5MTIzNDU2Nzg5MGFiY2RlZmc=
Endpoint = gw.massive-hosting.com:51820
AllowedIPs = fd00::/16
`
	_, err := ParseConfigString(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing PrivateKey")
}

func TestParseConfigString_MissingPublicKey(t *testing.T) {
	config := `[Interface]
PrivateKey = YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY=
Address = fd00:abcd:ffff::1/128

[Peer]
Endpoint = gw.massive-hosting.com:51820
AllowedIPs = fd00::/16
`
	_, err := ParseConfigString(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing PublicKey")
}

func TestServiceEntry_DefaultPort(t *testing.T) {
	assert.Equal(t, 3306, ServiceEntry{Type: "mysql"}.DefaultPort())
	assert.Equal(t, 6379, ServiceEntry{Type: "valkey"}.DefaultPort())
	assert.Equal(t, 0, ServiceEntry{Type: "custom"}.DefaultPort())
}
