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
	// Create table (idempotent — nft add is a no-op if it exists).
	nftRuleset := `table ip6 tenant_binding {
	set allowed {
		type ipv6_addr . uid
	}
	chain output {
		type filter hook output priority 0; policy accept;
		ip6 saddr fd00::/16 meta skuid != 0 ip6 saddr . meta skuid != @allowed reject
	}
}`
	cmd := exec.CommandContext(ctx, "nft", "-f", "-")
	cmd.Stdin = strings.NewReader(nftRuleset)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nft create table: %s: %w", string(output), err)
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

	// Add IPv6 address to tenant0 interface.
	out, err := exec.CommandContext(ctx, "ip", "-6", "addr", "add", ula+"/128", "dev", "tenant0").CombinedOutput()
	if err != nil {
		// "RTNETLINK answers: File exists" means the address is already assigned — idempotent.
		if !strings.Contains(string(out), "File exists") {
			return fmt.Errorf("ip addr add %s: %s: %w", ula, string(out), err)
		}
	}

	// Add nftables element to allow this (address, uid) pair.
	out, err = exec.CommandContext(ctx, "nft", "add", "element", "ip6", "tenant_binding", "allowed",
		fmt.Sprintf("{ %s . %d }", ula, info.TenantUID)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("nft add element %s . %d: %s: %w", ula, info.TenantUID, string(out), err)
	}

	return nil
}

// Remove removes a tenant's ULA address from tenant0 and the nftables allowed set.
func (m *TenantULAManager) Remove(ctx context.Context, info *TenantULAInfo) error {
	ula := core.ComputeTenantULA(info.ClusterID, info.NodeShardIdx, info.TenantUID)

	m.logger.Info().
		Str("tenant", info.TenantName).
		Str("ula", ula).
		Int("uid", info.TenantUID).
		Msg("removing tenant ULA")

	// Remove IPv6 address from tenant0 — ignore "Cannot assign" errors (already removed).
	out, err := exec.CommandContext(ctx, "ip", "-6", "addr", "del", ula+"/128", "dev", "tenant0").CombinedOutput()
	if err != nil && !strings.Contains(string(out), "Cannot assign") {
		return fmt.Errorf("ip addr del %s: %s: %w", ula, string(out), err)
	}

	// Remove nftables element — ignore errors if not present.
	_, _ = exec.CommandContext(ctx, "nft", "delete", "element", "ip6", "tenant_binding", "allowed",
		fmt.Sprintf("{ %s . %d }", ula, info.TenantUID)).CombinedOutput()

	return nil
}
