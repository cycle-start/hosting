package core

import (
	"context"
	"fmt"
	"time"

	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	temporalclient "go.temporal.io/sdk/client"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// WireGuardPeerCreateResult holds the peer plus the one-time private key and client config.
type WireGuardPeerCreateResult struct {
	Peer         *model.WireGuardPeer `json:"peer"`
	PrivateKey   string               `json:"private_key"`
	ClientConfig string               `json:"client_config"`
}

type WireGuardPeerService struct {
	db               DB
	tc               temporalclient.Client
	wireguardEndpoint string
}

func NewWireGuardPeerService(db DB, tc temporalclient.Client, wireguardEndpoint string) *WireGuardPeerService {
	return &WireGuardPeerService{db: db, tc: tc, wireguardEndpoint: wireguardEndpoint}
}

func (s *WireGuardPeerService) Create(ctx context.Context, peer *model.WireGuardPeer) (*WireGuardPeerCreateResult, error) {
	// Generate keypair.
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("generate wireguard keypair: %w", err)
	}
	publicKey := privateKey.PublicKey()

	// Generate preshared key.
	psk, err := wgtypes.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("generate wireguard psk: %w", err)
	}

	// Allocate peer_index: MAX+1 within the tenant, starting from 1.
	var nextIndex int
	err = s.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(peer_index), 0) + 1 FROM wireguard_peers WHERE tenant_id = $1`,
		peer.TenantID,
	).Scan(&nextIndex)
	if err != nil {
		return nil, fmt.Errorf("allocate peer index: %w", err)
	}

	// Look up cluster ID from tenant.
	var clusterID string
	err = s.db.QueryRow(ctx, "SELECT cluster_id FROM tenants WHERE id = $1", peer.TenantID).Scan(&clusterID)
	if err != nil {
		return nil, fmt.Errorf("get tenant cluster: %w", err)
	}

	now := time.Now()
	peer.ID = platform.NewName("wg")
	peer.PublicKey = publicKey.String()
	peer.PresharedKey = psk.String()
	peer.PeerIndex = nextIndex
	peer.AssignedIP = ComputeWireGuardClientIP(clusterID, nextIndex)
	peer.Endpoint = s.wireguardEndpoint
	peer.Status = model.StatusPending
	peer.CreatedAt = now
	peer.UpdatedAt = now

	_, err = s.db.Exec(ctx,
		`INSERT INTO wireguard_peers (id, tenant_id, subscription_id, name, public_key, preshared_key, assigned_ip, peer_index, endpoint, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		peer.ID, peer.TenantID, peer.SubscriptionID, peer.Name, peer.PublicKey,
		peer.PresharedKey, peer.AssignedIP, peer.PeerIndex, peer.Endpoint,
		peer.Status, peer.CreatedAt, peer.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert wireguard peer: %w", err)
	}

	// Look up gateway shard public key for client config.
	var gatewayPublicKey string
	err = s.db.QueryRow(ctx,
		`SELECT s.config->>'public_key' FROM shards s WHERE s.cluster_id = $1 AND s.role = 'gateway' LIMIT 1`,
		clusterID,
	).Scan(&gatewayPublicKey)
	if err != nil {
		gatewayPublicKey = "GATEWAY_PUBLIC_KEY_NOT_CONFIGURED"
	}

	// Look up tenant UID for ULA computation.
	var tenantUID int
	err = s.db.QueryRow(ctx, "SELECT uid FROM tenants WHERE id = $1", peer.TenantID).Scan(&tenantUID)
	if err != nil {
		return nil, fmt.Errorf("get tenant uid: %w", err)
	}

	// Build service metadata comments for CLI tool.
	var serviceLines string
	type svcRow struct {
		svcType   string
		shardRole string
		shardIdx  int
	}
	var services []svcRow
	rows, err := s.db.Query(ctx, `
		SELECT 'mysql' AS svc_type, s.role, nsa.shard_index
		FROM databases d
		JOIN shards s ON s.id = d.shard_id
		JOIN node_shard_assignments nsa ON nsa.shard_id = d.shard_id AND nsa.shard_index = 1
		WHERE d.tenant_id = $1 AND d.status NOT IN ('deleting', 'deleted', 'failed')
		UNION ALL
		SELECT 'valkey' AS svc_type, s.role, nsa.shard_index
		FROM valkey_instances v
		JOIN shards s ON s.id = v.shard_id
		JOIN node_shard_assignments nsa ON nsa.shard_id = v.shard_id AND nsa.shard_index = 1
		WHERE v.tenant_id = $1 AND v.status NOT IN ('deleting', 'deleted', 'failed')
	`, peer.TenantID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var sr svcRow
			if err := rows.Scan(&sr.svcType, &sr.shardRole, &sr.shardIdx); err == nil {
				services = append(services, sr)
			}
		}
	}
	if len(services) > 0 {
		serviceLines = "\n# hosting-cli:services\n"
		for _, sr := range services {
			ula := ComputeTenantULA(clusterID, TransitIndex(sr.shardRole, sr.shardIdx), tenantUID)
			serviceLines += fmt.Sprintf("# %s=%s\n", sr.svcType, ula)
		}
	}

	// Build client config.
	clientConfig := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/128

[Peer]
PublicKey = %s
PresharedKey = %s
Endpoint = %s
AllowedIPs = fd00::/16
PersistentKeepalive = 25
`, privateKey.String(), peer.AssignedIP, gatewayPublicKey, psk.String(), peer.Endpoint)
	clientConfig += serviceLines

	if err := signalProvision(ctx, s.tc, s.db, peer.TenantID, model.ProvisionTask{
		WorkflowName: "CreateWireGuardPeerWorkflow",
		WorkflowID:   workflowID("wireguard-peer", peer.ID),
		Arg:          peer.ID,
	}); err != nil {
		return nil, fmt.Errorf("signal CreateWireGuardPeerWorkflow: %w", err)
	}

	return &WireGuardPeerCreateResult{
		Peer:         peer,
		PrivateKey:   privateKey.String(),
		ClientConfig: clientConfig,
	}, nil
}

func (s *WireGuardPeerService) GetByID(ctx context.Context, id string) (*model.WireGuardPeer, error) {
	var p model.WireGuardPeer
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, subscription_id, name, public_key, preshared_key, assigned_ip, peer_index, endpoint, status, status_message, created_at, updated_at
		 FROM wireguard_peers WHERE id = $1`, id,
	).Scan(&p.ID, &p.TenantID, &p.SubscriptionID, &p.Name, &p.PublicKey,
		&p.PresharedKey, &p.AssignedIP, &p.PeerIndex, &p.Endpoint,
		&p.Status, &p.StatusMessage, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get wireguard peer %s: %w", id, err)
	}
	return &p, nil
}

func (s *WireGuardPeerService) ListByTenant(ctx context.Context, tenantID string, limit int, cursor string) ([]model.WireGuardPeer, bool, error) {
	query := `SELECT id, tenant_id, subscription_id, name, public_key, preshared_key, assigned_ip, peer_index, endpoint, status, status_message, created_at, updated_at FROM wireguard_peers WHERE tenant_id = $1`
	args := []any{tenantID}
	argIdx := 2

	if cursor != "" {
		query += fmt.Sprintf(` AND id > $%d`, argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query += ` ORDER BY id`
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list wireguard peers for tenant %s: %w", tenantID, err)
	}
	defer rows.Close()

	var peers []model.WireGuardPeer
	for rows.Next() {
		var p model.WireGuardPeer
		if err := rows.Scan(&p.ID, &p.TenantID, &p.SubscriptionID, &p.Name, &p.PublicKey,
			&p.PresharedKey, &p.AssignedIP, &p.PeerIndex, &p.Endpoint,
			&p.Status, &p.StatusMessage, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan wireguard peer: %w", err)
		}
		peers = append(peers, p)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate wireguard peers: %w", err)
	}

	hasMore := len(peers) > limit
	if hasMore {
		peers = peers[:limit]
	}
	return peers, hasMore, nil
}

func (s *WireGuardPeerService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE wireguard_peers SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusDeleting, id,
	)
	if err != nil {
		return fmt.Errorf("set wireguard peer %s status to deleting: %w", id, err)
	}

	var tenantID string
	err = s.db.QueryRow(ctx, "SELECT tenant_id FROM wireguard_peers WHERE id = $1", id).Scan(&tenantID)
	if err != nil {
		return fmt.Errorf("resolve tenant for wireguard peer %s: %w", id, err)
	}

	if err := signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteWireGuardPeerWorkflow",
		WorkflowID:   workflowID("wireguard-peer", id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("signal DeleteWireGuardPeerWorkflow: %w", err)
	}

	return nil
}

func (s *WireGuardPeerService) Retry(ctx context.Context, id string) error {
	var status string
	err := s.db.QueryRow(ctx, "SELECT status FROM wireguard_peers WHERE id = $1", id).Scan(&status)
	if err != nil {
		return fmt.Errorf("get wireguard peer status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("wireguard peer %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE wireguard_peers SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set wireguard peer %s status to provisioning: %w", id, err)
	}
	var tenantID string
	err = s.db.QueryRow(ctx, "SELECT tenant_id FROM wireguard_peers WHERE id = $1", id).Scan(&tenantID)
	if err != nil {
		return fmt.Errorf("resolve tenant for wireguard peer %s: %w", id, err)
	}
	return signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "CreateWireGuardPeerWorkflow",
		WorkflowID:   workflowID("wireguard-peer", id),
		Arg:          id,
	})
}
