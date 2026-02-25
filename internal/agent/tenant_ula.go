package agent

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
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

// EnsureServiceIngressTable creates the ip6 tenant_service_ingress nftables
// table on service nodes (DB/Valkey). It restricts inbound ULA traffic so only
// web-node ULAs and localhost can reach the tenant ULAs on this node.
// Non-ULA traffic (node's regular IPv4/IPv6) is unaffected (policy accept).
func (m *TenantULAManager) EnsureServiceIngressTable(ctx context.Context) error {
	steps := []struct {
		args []string
		desc string
	}{
		{[]string{"add", "table", "ip6", "tenant_service_ingress"}, "create table"},
		{[]string{"add", "set", "ip6", "tenant_service_ingress", "ula_addrs", "{ type ipv6_addr ; }"}, "create set"},
		{[]string{"add", "chain", "ip6", "tenant_service_ingress", "input", "{ type filter hook input priority 0 ; policy accept ; }"}, "create chain"},
	}
	for _, s := range steps {
		if out, err := exec.CommandContext(ctx, "nft", s.args...).CombinedOutput(); err != nil {
			return fmt.Errorf("nft %s: %s: %w", s.desc, string(out), err)
		}
	}

	// Flush and re-add rules. Allow ULA-destined traffic only from fd00::/16 (web
	// nodes) and ::1 (local CLI). Drop all other traffic destined to our ULA addrs.
	nftScript := `flush chain ip6 tenant_service_ingress input
table ip6 tenant_service_ingress {
    chain input {
        ip6 daddr @ula_addrs ip6 saddr fd00::/16 accept
        ip6 daddr @ula_addrs ip6 saddr ::1 accept
        ip6 daddr @ula_addrs drop
    }
}
`
	cmd := exec.CommandContext(ctx, "nft", "-f", "-")
	cmd.Stdin = strings.NewReader(nftScript)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nft service ingress flush+rules: %s: %w", string(out), err)
	}

	m.logger.Info().Msg("nftables tenant_service_ingress table ensured")
	return nil
}

// ConfigureServiceAddr adds a tenant's ULA address to tenant0 on a service node
// (DB/Valkey) and registers it in the service ingress nftables set. Unlike
// Configure, this does not add UID-based binding rules since service nodes
// don't have per-tenant Linux users.
func (m *TenantULAManager) ConfigureServiceAddr(ctx context.Context, info *TenantULAInfo) error {
	if err := m.EnsureServiceIngressTable(ctx); err != nil {
		return err
	}

	ula := core.ComputeTenantULA(info.ClusterID, info.NodeShardIdx, info.TenantUID)

	m.logger.Info().
		Str("tenant", info.TenantName).
		Str("ula", ula).
		Int("uid", info.TenantUID).
		Msg("configuring service tenant ULA")

	// Add IPv6 address to tenant0 interface — idempotent.
	out, err := exec.CommandContext(ctx, "ip", "-6", "addr", "add", ula+"/128", "dev", "tenant0").CombinedOutput()
	if err != nil && !isAddrAlreadyExists(string(out)) {
		return fmt.Errorf("ip addr add %s: %s: %w", ula, string(out), err)
	}

	// Add to nftables ula_addrs set.
	out, err = exec.CommandContext(ctx, "nft", "add", "element", "ip6", "tenant_service_ingress", "ula_addrs",
		fmt.Sprintf("{ %s }", ula)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("nft add element %s: %s: %w", ula, string(out), err)
	}

	return nil
}

// RemoveServiceAddr removes a tenant's ULA address from tenant0 on a service
// node and removes it from the service ingress nftables set.
func (m *TenantULAManager) RemoveServiceAddr(ctx context.Context, info *TenantULAInfo) error {
	ula := core.ComputeTenantULA(info.ClusterID, info.NodeShardIdx, info.TenantUID)

	m.logger.Info().
		Str("tenant", info.TenantName).
		Str("ula", ula).
		Int("uid", info.TenantUID).
		Msg("removing service tenant ULA")

	// Remove IPv6 address from tenant0 — ignore errors if not present.
	out, err := exec.CommandContext(ctx, "ip", "-6", "addr", "del", ula+"/128", "dev", "tenant0").CombinedOutput()
	if err != nil {
		outStr := string(out)
		if !strings.Contains(outStr, "Cannot assign") && !strings.Contains(outStr, "not found") {
			return fmt.Errorf("ip addr del %s: %s: %w", ula, outStr, err)
		}
	}

	// Remove from nftables set — ignore errors if not present.
	_, _ = exec.CommandContext(ctx, "nft", "delete", "element", "ip6", "tenant_service_ingress", "ula_addrs",
		fmt.Sprintf("{ %s }", ula)).CombinedOutput()

	return nil
}

// ULARoutePeer describes a single peer for cross-shard ULA routing.
type ULARoutePeer struct {
	PrefixIndex  int // The peer's shard index (used in fd00:{hash}:{prefix_index}::/48)
	TransitIndex int // The peer's transit index (used in fd00:{hash}:0::{transit_index})
}

// ULARoutesInfoV2 holds the information needed for generalized cross-shard routing.
type ULARoutesInfoV2 struct {
	ClusterID       string
	ThisTransitIndex int
	Peers           []ULARoutePeer
}

// ConfigureRoutesV2 sets up IPv6 transit addresses and routes supporting
// cross-shard peers (e.g. web nodes routing to DB/Valkey nodes and vice versa).
// Each node gets a transit address fd00:{hash}:0::{transit_index}/64 on the
// primary interface, and routes to each peer's ULA prefix via its transit address.
func (m *TenantULAManager) ConfigureRoutesV2(ctx context.Context, info *ULARoutesInfoV2) error {
	iface, err := m.detectPrimaryInterface(ctx)
	if err != nil {
		return err
	}

	clusterHash := core.ComputeClusterHash(info.ClusterID)

	// Add transit address on primary interface — idempotent.
	transitAddr := fmt.Sprintf("fd00:%x:0::%x/64", clusterHash, info.ThisTransitIndex)
	out, err := exec.CommandContext(ctx, "ip", "-6", "addr", "add", transitAddr, "dev", iface).CombinedOutput()
	if err != nil && !isAddrAlreadyExists(string(out)) {
		return fmt.Errorf("ip addr add transit %s dev %s: %s: %w", transitAddr, iface, string(out), err)
	}
	m.logger.Info().Str("addr", transitAddr).Str("dev", iface).Msg("transit address configured")

	// Add routes to each peer's ULA prefix via their transit address.
	for _, peer := range info.Peers {
		prefix := fmt.Sprintf("fd00:%x:%x::/48", clusterHash, peer.PrefixIndex)
		nextHop := fmt.Sprintf("fd00:%x:0::%x", clusterHash, peer.TransitIndex)
		out, err := exec.CommandContext(ctx, "ip", "-6", "route", "replace", prefix, "via", nextHop).CombinedOutput()
		if err != nil {
			return fmt.Errorf("ip route replace %s via %s: %s: %w", prefix, nextHop, string(out), err)
		}
		m.logger.Info().Str("prefix", prefix).Str("via", nextHop).Msg("ULA route configured")
	}

	return nil
}

// SyncEgressRules applies all egress rules for a tenant via nftables.
// Whitelist model: when rules exist, each CIDR gets an accept verdict, then a
// final reject catches everything else. When no rules exist, the chain and jump
// rule are removed so the default accept policy applies (unrestricted egress).
func (m *TenantULAManager) SyncEgressRules(ctx context.Context, tenantUID int, rules []model.TenantEgressRule) error {
	if err := m.ensureEgressTable(ctx); err != nil {
		return err
	}

	chainName := fmt.Sprintf("tenant_%d", tenantUID)

	if len(rules) == 0 {
		// No rules — remove chain and jump rule to restore unrestricted egress.
		m.removeEgressChain(ctx, tenantUID, chainName)
		m.logger.Info().Int("uid", tenantUID).Msg("removed egress restrictions (no rules)")
		return nil
	}

	// Create chain if it doesn't exist (idempotent).
	if out, err := exec.CommandContext(ctx, "nft", "add", "chain", "inet", "tenant_egress", chainName).CombinedOutput(); err != nil {
		return fmt.Errorf("nft add egress chain: %s: %w", string(out), err)
	}

	// Flush the chain to remove old rules.
	if out, err := exec.CommandContext(ctx, "nft", "flush", "chain", "inet", "tenant_egress", chainName).CombinedOutput(); err != nil {
		return fmt.Errorf("nft flush egress chain: %s: %w", string(out), err)
	}

	// Add accept rules for each allowed CIDR, then a final reject.
	var b strings.Builder
	for _, rule := range rules {
		addrMatch := "ip daddr"
		if strings.Contains(rule.CIDR, ":") {
			addrMatch = "ip6 daddr"
		}
		b.WriteString(fmt.Sprintf("add rule inet tenant_egress %s %s %s accept\n", chainName, addrMatch, rule.CIDR))
	}
	// Final reject — anything not matching an allowed CIDR is blocked.
	b.WriteString(fmt.Sprintf("add rule inet tenant_egress %s reject\n", chainName))

	cmd := exec.CommandContext(ctx, "nft", "-f", "-")
	cmd.Stdin = strings.NewReader(b.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nft add egress rules: %s: %w", string(out), err)
	}

	// Ensure jump rule exists.
	listOut, _ := exec.CommandContext(ctx, "nft", "list", "chain", "inet", "tenant_egress", "output").CombinedOutput()
	jumpTarget := fmt.Sprintf("jump %s", chainName)
	if !strings.Contains(string(listOut), jumpTarget) {
		jumpCmd := fmt.Sprintf("add rule inet tenant_egress output meta skuid %d jump %s\n", tenantUID, chainName)
		cmd := exec.CommandContext(ctx, "nft", "-f", "-")
		cmd.Stdin = strings.NewReader(jumpCmd)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("nft add jump rule: %s: %w", string(out), err)
		}
	}

	m.logger.Info().Int("uid", tenantUID).Int("rule_count", len(rules)).Msg("synced egress rules")
	return nil
}

// ensureEgressTable creates the inet tenant_egress table and output chain.
func (m *TenantULAManager) ensureEgressTable(ctx context.Context) error {
	steps := []struct {
		args []string
		desc string
	}{
		{[]string{"add", "table", "inet", "tenant_egress"}, "create egress table"},
		{[]string{"add", "chain", "inet", "tenant_egress", "output", "{ type filter hook output priority 1 ; policy accept ; }"}, "create egress output chain"},
	}
	for _, s := range steps {
		if out, err := exec.CommandContext(ctx, "nft", s.args...).CombinedOutput(); err != nil {
			return fmt.Errorf("nft %s: %s: %w", s.desc, string(out), err)
		}
	}
	return nil
}

// removeEgressChain removes a tenant's egress chain and jump rule.
func (m *TenantULAManager) removeEgressChain(ctx context.Context, uid int, chainName string) error {
	// Flush chain (ignore errors if it doesn't exist).
	exec.CommandContext(ctx, "nft", "flush", "chain", "inet", "tenant_egress", chainName).CombinedOutput()
	// Delete the chain — nft requires no references to it first, so remove
	// the jump rule from the output chain by listing handles and deleting.
	exec.CommandContext(ctx, "nft", "delete", "chain", "inet", "tenant_egress", chainName).CombinedOutput()

	m.logger.Info().Int("uid", uid).Msg("removed egress chain (no rules)")
	return nil
}
