package agent

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	"github.com/edvin/hosting/internal/core"
)

// TenantULAInfo holds the information needed to manage a tenant's ULA address.
type TenantULAInfo struct {
	TenantName   string
	TenantUID    int
	ClusterID    string
	NodeShardIdx int
}

// TenantULAManager manages per-tenant ULA IPv6 addresses on the tenant0 dummy
// interface and nftables binding restrictions.
type TenantULAManager struct {
	logger    zerolog.Logger
	tableOnce sync.Once
	tableErr  error
}

// NewTenantULAManager creates a new TenantULAManager.
func NewTenantULAManager(logger zerolog.Logger) *TenantULAManager {
	return &TenantULAManager{
		logger: logger.With().Str("component", "tenant-ula").Logger(),
	}
}

// EnsureTable creates the base nftables table, set, and chain if not present.
func (m *TenantULAManager) EnsureTable(ctx context.Context) error {
	m.tableOnce.Do(func() {
		m.tableErr = m.ensureTable(ctx)
	})
	return m.tableErr
}

func (m *TenantULAManager) ensureTable(ctx context.Context) error {
	// Each command is idempotent — "add" is a no-op if the object already exists.
	steps := []struct {
		args []string
		desc string
	}{
		{[]string{"add", "table", "ip6", "tenant_binding"}, "create table"},
		{[]string{"add", "set", "ip6", "tenant_binding", "allowed", "{ type ipv6_addr . uid ; }"}, "create set"},
		{[]string{"add", "chain", "ip6", "tenant_binding", "output", "{ type filter hook output priority 0 ; policy accept ; }"}, "create chain"},
	}
	for _, s := range steps {
		if out, err := exec.CommandContext(ctx, "nft", s.args...).CombinedOutput(); err != nil {
			return fmt.Errorf("nft %s: %s: %w", s.desc, string(out), err)
		}
	}

	// Flush and re-add the single rule via nft script (stdin). The "!=" operator only
	// works inside a table definition block in nft v1.0.9, not in "add rule" CLI mode.
	// Flushing first prevents rule accumulation across node-agent restarts.
	// Set elements are preserved because only the chain is flushed.
	nftScript := `flush chain ip6 tenant_binding output
table ip6 tenant_binding {
    chain output {
        ip6 saddr fd00::/16 meta skuid >= 1000 ip6 saddr . meta skuid != @allowed reject
    }
}
`
	cmd := exec.CommandContext(ctx, "nft", "-f", "-")
	cmd.Stdin = strings.NewReader(nftScript)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nft flush+rule: %s: %w", string(out), err)
	}

	m.logger.Info().Msg("nftables tenant_binding table ensured")
	return nil
}

// Configure adds a tenant's ULA address to tenant0 and allows it in nftables.
func (m *TenantULAManager) Configure(ctx context.Context, info *TenantULAInfo) error {
	if err := m.EnsureTable(ctx); err != nil {
		return err
	}

	ula := core.ComputeTenantULA(info.ClusterID, info.NodeShardIdx, info.TenantUID)

	m.logger.Info().
		Str("tenant", info.TenantName).
		Str("ula", ula).
		Int("uid", info.TenantUID).
		Msg("configuring tenant ULA")

	// Add IPv6 address to tenant0 interface — idempotent.
	out, err := exec.CommandContext(ctx, "ip", "-6", "addr", "add", ula+"/128", "dev", "tenant0").CombinedOutput()
	if err != nil && !isAddrAlreadyExists(string(out)) {
		return fmt.Errorf("ip addr add %s: %s: %w", ula, string(out), err)
	}

	// Add nftables element to allow this (address, uid) pair.
	out, err = exec.CommandContext(ctx, "nft", "add", "element", "ip6", "tenant_binding", "allowed",
		fmt.Sprintf("{ %s . %d }", ula, info.TenantUID)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("nft add element %s . %d: %s: %w", ula, info.TenantUID, string(out), err)
	}

	return nil
}

// ULARoutesInfo holds the information needed to configure cross-node ULA routes.
type ULARoutesInfo struct {
	ClusterID        string
	ThisNodeIndex    int
	OtherNodeIndices []int
}

// ConfigureRoutes sets up IPv6 transit addresses and routes so nodes in a shard
// can reach each other's tenant ULA addresses. Each node gets a transit address
// (fd00:{hash}:0::{index}/64) on the primary interface, and routes to other
// nodes' ULA prefixes via their transit addresses.
func (m *TenantULAManager) ConfigureRoutes(ctx context.Context, info *ULARoutesInfo) error {
	iface, err := m.detectPrimaryInterface(ctx)
	if err != nil {
		return err
	}

	clusterHash := core.ComputeClusterHash(info.ClusterID)

	// Add transit address on primary interface — idempotent.
	transitAddr := fmt.Sprintf("fd00:%x:0::%x/64", clusterHash, info.ThisNodeIndex)
	out, err := exec.CommandContext(ctx, "ip", "-6", "addr", "add", transitAddr, "dev", iface).CombinedOutput()
	if err != nil && !isAddrAlreadyExists(string(out)) {
		return fmt.Errorf("ip addr add transit %s dev %s: %s: %w", transitAddr, iface, string(out), err)
	}
	m.logger.Info().Str("addr", transitAddr).Str("dev", iface).Msg("transit address configured")

	// Add routes to other nodes' ULA prefixes via their transit addresses.
	for _, otherIdx := range info.OtherNodeIndices {
		prefix := fmt.Sprintf("fd00:%x:%x::/48", clusterHash, otherIdx)
		nextHop := fmt.Sprintf("fd00:%x:0::%x", clusterHash, otherIdx)
		out, err := exec.CommandContext(ctx, "ip", "-6", "route", "replace", prefix, "via", nextHop).CombinedOutput()
		if err != nil {
			return fmt.Errorf("ip route replace %s via %s: %s: %w", prefix, nextHop, string(out), err)
		}
		m.logger.Info().Str("prefix", prefix).Str("via", nextHop).Msg("ULA route configured")
	}

	return nil
}

// detectPrimaryInterface finds the network interface used for the default IPv4 route.
func (m *TenantULAManager) detectPrimaryInterface(ctx context.Context) (string, error) {
	// Output: "default via 10.10.10.1 dev enp0s2 proto ..."
	out, err := exec.CommandContext(ctx, "ip", "-4", "route", "show", "default").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("detect default interface: %s: %w", string(out), err)
	}
	parts := strings.Fields(string(out))
	for i, p := range parts {
		if p == "dev" && i+1 < len(parts) {
			return parts[i+1], nil
		}
	}
	return "", fmt.Errorf("could not find dev in default route: %s", string(out))
}

// Remove removes a tenant's ULA address from tenant0 and the nftables allowed set.
func (m *TenantULAManager) Remove(ctx context.Context, info *TenantULAInfo) error {
	ula := core.ComputeTenantULA(info.ClusterID, info.NodeShardIdx, info.TenantUID)

	m.logger.Info().
		Str("tenant", info.TenantName).
		Str("ula", ula).
		Int("uid", info.TenantUID).
		Msg("removing tenant ULA")

	// Remove IPv6 address from tenant0 — ignore errors if address not present.
	out, err := exec.CommandContext(ctx, "ip", "-6", "addr", "del", ula+"/128", "dev", "tenant0").CombinedOutput()
	if err != nil {
		outStr := string(out)
		if !strings.Contains(outStr, "Cannot assign") && !strings.Contains(outStr, "not found") {
			return fmt.Errorf("ip addr del %s: %s: %w", ula, outStr, err)
		}
	}

	// Remove nftables element — ignore errors if not present.
	_, _ = exec.CommandContext(ctx, "nft", "delete", "element", "ip6", "tenant_binding", "allowed",
		fmt.Sprintf("{ %s . %d }", ula, info.TenantUID)).CombinedOutput()

	return nil
}

// isAddrAlreadyExists checks if an `ip addr add` error indicates the address
// is already assigned. Different kernel/iproute2 versions use different messages.
func isAddrAlreadyExists(output string) bool {
	return strings.Contains(output, "File exists") || strings.Contains(output, "already assigned")
}
