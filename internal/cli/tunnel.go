package cli

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

// Tunnel represents an active userspace WireGuard tunnel.
type Tunnel struct {
	dev  *device.Device
	tnet *netstack.Net
	cfg  *WireGuardConfig
}

// CreateTunnel establishes a userspace WireGuard tunnel from the given config.
// Returns a Tunnel that can be used to dial services through the tunnel.
func CreateTunnel(cfg *WireGuardConfig) (*Tunnel, error) {
	// Create the netstack TUN device.
	localAddrs := []netip.Addr{cfg.Address.Addr()}
	tun, tnet, err := netstack.CreateNetTUN(localAddrs, nil, device.DefaultMTU)
	if err != nil {
		return nil, fmt.Errorf("create netstack tun: %w", err)
	}

	// Create the WireGuard device.
	dev := device.NewDevice(tun, conn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, ""))

	// Build the UAPI config string.
	uapi, err := buildUAPIConfig(cfg)
	if err != nil {
		tun.Close()
		return nil, fmt.Errorf("build uapi config: %w", err)
	}

	if err := dev.IpcSet(uapi); err != nil {
		dev.Close()
		return nil, fmt.Errorf("configure wireguard device: %w", err)
	}

	if err := dev.Up(); err != nil {
		dev.Close()
		return nil, fmt.Errorf("bring up wireguard device: %w", err)
	}

	return &Tunnel{
		dev:  dev,
		tnet: tnet,
		cfg:  cfg,
	}, nil
}

// DialTCP connects to a TCP address through the tunnel.
func (t *Tunnel) DialTCP(addr string) (net.Conn, error) {
	return t.tnet.DialContextTCPAddrPort(nil, netip.MustParseAddrPort(addr))
}

// Net returns the netstack Net for direct use.
func (t *Tunnel) Net() *netstack.Net {
	return t.tnet
}

// Close tears down the tunnel.
func (t *Tunnel) Close() {
	if t.dev != nil {
		t.dev.Close()
	}
}

// buildUAPIConfig builds the UAPI configuration string for the WireGuard device.
func buildUAPIConfig(cfg *WireGuardConfig) (string, error) {
	privKey, err := base64.StdEncoding.DecodeString(cfg.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("decode private key: %w", err)
	}

	pubKey, err := base64.StdEncoding.DecodeString(cfg.PublicKey)
	if err != nil {
		return "", fmt.Errorf("decode public key: %w", err)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "private_key=%s\n", hex.EncodeToString(privKey))
	fmt.Fprintf(&sb, "public_key=%s\n", hex.EncodeToString(pubKey))

	if cfg.PresharedKey != "" {
		psk, err := base64.StdEncoding.DecodeString(cfg.PresharedKey)
		if err != nil {
			return "", fmt.Errorf("decode preshared key: %w", err)
		}
		fmt.Fprintf(&sb, "preshared_key=%s\n", hex.EncodeToString(psk))
	}

	// Resolve endpoint.
	host, portStr, err := net.SplitHostPort(cfg.Endpoint)
	if err != nil {
		return "", fmt.Errorf("parse endpoint %q: %w", cfg.Endpoint, err)
	}

	// Resolve hostname to IP.
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", fmt.Errorf("resolve endpoint %q: %w", host, err)
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("no IPs found for endpoint %q", host)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", fmt.Errorf("parse port %q: %w", portStr, err)
	}

	fmt.Fprintf(&sb, "endpoint=%s:%d\n", ips[0].String(), port)

	if cfg.PersistentKeepalive > 0 {
		fmt.Fprintf(&sb, "persistent_keepalive_interval=%d\n", cfg.PersistentKeepalive)
	}

	for _, prefix := range cfg.AllowedIPs {
		fmt.Fprintf(&sb, "allowed_ip=%s\n", prefix.String())
	}

	return sb.String(), nil
}
