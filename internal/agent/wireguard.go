package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rs/zerolog"
)

const (
	wgInterface  = "wg0"
	wgListenPort = 51820
	wgKeyFile    = "/etc/wireguard/server.key"
)

// WireGuardManager manages WireGuard interface and peer configuration on gateway nodes.
type WireGuardManager struct {
	logger zerolog.Logger
}

// NewWireGuardManager creates a new WireGuardManager.
func NewWireGuardManager(logger zerolog.Logger) *WireGuardManager {
	return &WireGuardManager{
		logger: logger.With().Str("component", "wireguard").Logger(),
	}
}

// ensureInterface creates the wg0 interface if not present and configures it,
// reading the private key from /etc/wireguard/server.key.
func (m *WireGuardManager) ensureInterface(ctx context.Context) error {
	// Check if wg0 already exists and is up.
	if _, err := exec.CommandContext(ctx, "ip", "link", "show", wgInterface).CombinedOutput(); err == nil {
		return nil // Already exists.
	}

	// Create the interface.
	if out, err := exec.CommandContext(ctx, "ip", "link", "add", wgInterface, "type", "wireguard").CombinedOutput(); err != nil {
		return fmt.Errorf("create %s: %s: %w", wgInterface, string(out), err)
	}

	if out, err := exec.CommandContext(ctx, "wg", "set", wgInterface,
		"listen-port", fmt.Sprintf("%d", wgListenPort),
		"private-key", wgKeyFile,
	).CombinedOutput(); err != nil {
		return fmt.Errorf("wg set %s: %s: %w", wgInterface, string(out), err)
	}

	// Bring interface up.
	if out, err := exec.CommandContext(ctx, "ip", "link", "set", wgInterface, "up").CombinedOutput(); err != nil {
		return fmt.Errorf("ip link set %s up: %s: %w", wgInterface, string(out), err)
	}

	m.logger.Info().Int("listen_port", wgListenPort).Msg("WireGuard interface created")
	return nil
}

// AddPeerParams holds parameters for adding a WireGuard peer.
type AddPeerParams struct {
	PublicKey    string
	PresharedKey string
	AssignedIP   string
	AllowedIPs   []string
}

// AddPeer adds a WireGuard peer to the wg0 interface and sets up nftables FORWARD rules.
func (m *WireGuardManager) AddPeer(ctx context.Context, params AddPeerParams) error {
	if err := m.ensureInterface(ctx); err != nil {
		return fmt.Errorf("ensure wg interface: %w", err)
	}

	// Write PSK to temp file.
	pskFile, err := os.CreateTemp("", "wg-psk-*")
	if err != nil {
		return fmt.Errorf("create temp psk file: %w", err)
	}
	defer os.Remove(pskFile.Name())
	if _, err := pskFile.WriteString(params.PresharedKey); err != nil {
		pskFile.Close()
		return fmt.Errorf("write psk: %w", err)
	}
	pskFile.Close()

	allowedIPs := params.AssignedIP + "/128"

	if out, err := exec.CommandContext(ctx, "wg", "set", wgInterface,
		"peer", params.PublicKey,
		"preshared-key", pskFile.Name(),
		"allowed-ips", allowedIPs,
	).CombinedOutput(); err != nil {
		return fmt.Errorf("wg set peer %s: %s: %w", params.PublicKey, string(out), err)
	}

	// Add route for the peer's assigned IP.
	if out, err := exec.CommandContext(ctx, "ip", "-6", "route", "replace",
		params.AssignedIP+"/128", "dev", wgInterface).CombinedOutput(); err != nil {
		return fmt.Errorf("ip route add %s: %s: %w", params.AssignedIP, string(out), err)
	}

	// Add nftables FORWARD rules for this peer.
	m.addForwardRules(ctx, params.AssignedIP, params.AllowedIPs)

	m.logger.Info().Str("public_key", params.PublicKey[:8]+"...").Str("assigned_ip", params.AssignedIP).Msg("WireGuard peer added")
	return nil
}

// RemovePeer removes a WireGuard peer from the wg0 interface.
func (m *WireGuardManager) RemovePeer(ctx context.Context, publicKey string, assignedIP string) error {
	if err := m.ensureInterface(ctx); err != nil {
		return fmt.Errorf("ensure wg interface: %w", err)
	}

	if out, err := exec.CommandContext(ctx, "wg", "set", wgInterface,
		"peer", publicKey, "remove",
	).CombinedOutput(); err != nil {
		return fmt.Errorf("wg remove peer %s: %s: %w", publicKey, string(out), err)
	}

	// Remove route.
	exec.CommandContext(ctx, "ip", "-6", "route", "del", assignedIP+"/128", "dev", wgInterface).CombinedOutput()

	// Remove nftables FORWARD rules.
	m.removeForwardRules(ctx, assignedIP)

	m.logger.Info().Str("public_key", publicKey[:8]+"...").Msg("WireGuard peer removed")
	return nil
}

// SyncPeers performs full convergence: rebuilds all peers from desired state.
func (m *WireGuardManager) SyncPeers(ctx context.Context, peers []AddPeerParams) error {
	if err := m.ensureInterface(ctx); err != nil {
		return fmt.Errorf("ensure wg interface: %w", err)
	}

	// Get current peers from wg show.
	currentPeers, err := m.listCurrentPeers(ctx)
	if err != nil {
		return fmt.Errorf("list current peers: %w", err)
	}

	// Build desired set.
	desired := make(map[string]AddPeerParams)
	for _, p := range peers {
		desired[p.PublicKey] = p
	}

	// Remove peers not in desired state.
	for _, pubkey := range currentPeers {
		if _, ok := desired[pubkey]; !ok {
			exec.CommandContext(ctx, "wg", "set", wgInterface, "peer", pubkey, "remove").CombinedOutput()
		}
	}

	// Add/update all desired peers.
	for _, p := range peers {
		if err := m.AddPeer(ctx, p); err != nil {
			return fmt.Errorf("add peer %s: %w", p.PublicKey[:8], err)
		}
	}

	// Rebuild nftables forward table completely.
	m.rebuildForwardTable(ctx, peers)

	m.logger.Info().Int("peer_count", len(peers)).Msg("WireGuard peers synced")
	return nil
}

func (m *WireGuardManager) listCurrentPeers(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, "wg", "show", wgInterface, "peers").CombinedOutput()
	if err != nil {
		// Interface may not exist yet.
		return nil, nil
	}
	var peers []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			peers = append(peers, line)
		}
	}
	return peers, nil
}

func (m *WireGuardManager) addForwardRules(ctx context.Context, srcIP string, allowedDstIPs []string) {
	// Ensure the wg_forward table and chain exist.
	exec.CommandContext(ctx, "nft", "add", "table", "ip6", "wg_forward").CombinedOutput()
	exec.CommandContext(ctx, "nft", "add", "chain", "ip6", "wg_forward", "forward",
		"{ type filter hook forward priority 0 ; policy drop ; }").CombinedOutput()

	for _, dst := range allowedDstIPs {
		exec.CommandContext(ctx, "nft", "add", "rule", "ip6", "wg_forward", "forward",
			"ip6", "saddr", srcIP, "ip6", "daddr", dst, "accept").CombinedOutput()
	}
}

func (m *WireGuardManager) removeForwardRules(ctx context.Context, srcIP string) {
	// List rules with handles and delete matching ones.
	out, err := exec.CommandContext(ctx, "nft", "-a", "list", "chain", "ip6", "wg_forward", "forward").CombinedOutput()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, srcIP) && strings.Contains(line, "handle") {
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "handle" && i+1 < len(parts) {
					exec.CommandContext(ctx, "nft", "delete", "rule", "ip6", "wg_forward", "forward", "handle", parts[i+1]).CombinedOutput()
				}
			}
		}
	}
}

func (m *WireGuardManager) rebuildForwardTable(ctx context.Context, peers []AddPeerParams) {
	var b strings.Builder
	b.WriteString("flush table ip6 wg_forward\n")
	b.WriteString("table ip6 wg_forward {\n")
	b.WriteString("    chain forward {\n")
	b.WriteString("        type filter hook forward priority 0; policy drop;\n")

	for _, p := range peers {
		for _, dst := range p.AllowedIPs {
			b.WriteString(fmt.Sprintf("        ip6 saddr %s ip6 daddr %s accept\n", p.AssignedIP, dst))
		}
	}

	b.WriteString("    }\n")
	b.WriteString("}\n")

	cmd := exec.CommandContext(ctx, "nft", "-f", "-")
	cmd.Stdin = strings.NewReader(b.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		m.logger.Warn().Err(err).Str("output", string(out)).Msg("failed to rebuild wg_forward table")
	}
}
