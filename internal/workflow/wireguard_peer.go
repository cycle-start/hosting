package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
)

// CreateWireGuardPeerWorkflow provisions a WireGuard peer on gateway nodes.
func CreateWireGuardPeerWorkflow(ctx workflow.Context, peerID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "wireguard_peers",
		ID:     peerID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the peer.
	var peer model.WireGuardPeer
	err = workflow.ExecuteActivity(ctx, "GetWireGuardPeerByID", peerID).Get(ctx, &peer)
	if err != nil {
		_ = setResourceFailed(ctx, "wireguard_peers", peerID, err)
		return err
	}

	// Look up the tenant.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", peer.TenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "wireguard_peers", peerID, err)
		return err
	}

	// Find gateway shard nodes for this cluster.
	var shards []model.Shard
	err = workflow.ExecuteActivity(ctx, "ListShardsByClusterAndRole", tenant.ClusterID, model.ShardRoleGateway).Get(ctx, &shards)
	if err != nil {
		_ = setResourceFailed(ctx, "wireguard_peers", peerID, err)
		return err
	}

	if len(shards) == 0 {
		noGatewayErr := fmt.Errorf("no gateway shard found for cluster %s", tenant.ClusterID)
		_ = setResourceFailed(ctx, "wireguard_peers", peerID, noGatewayErr)
		return noGatewayErr
	}

	// Compute allowed IPs for this peer: all DB and Valkey ULAs for the tenant.
	clusterHash := core.ComputeClusterHash(tenant.ClusterID)
	var allowedIPs []string
	for _, role := range []string{model.ShardRoleDatabase, model.ShardRoleValkey} {
		var roleShards []model.Shard
		err = workflow.ExecuteActivity(ctx, "ListShardsByClusterAndRole", tenant.ClusterID, role).Get(ctx, &roleShards)
		if err != nil {
			continue
		}
		for _, s := range roleShards {
			var nodes []model.Node
			if nErr := workflow.ExecuteActivity(ctx, "ListNodesByShard", s.ID).Get(ctx, &nodes); nErr != nil {
				continue
			}
			for _, n := range nodes {
				if n.ShardIndex != nil {
					ula := fmt.Sprintf("fd00:%x:%x::%x", clusterHash, *n.ShardIndex, tenant.UID)
					allowedIPs = append(allowedIPs, ula+"/128")
				}
			}
		}
	}

	// Fan out ConfigureWireGuardPeer to all gateway nodes.
	for _, shard := range shards {
		var nodes []model.Node
		err = workflow.ExecuteActivity(ctx, "ListNodesByShard", shard.ID).Get(ctx, &nodes)
		if err != nil {
			_ = setResourceFailed(ctx, "wireguard_peers", peerID, err)
			return err
		}

		errs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
			nodeCtx := nodeActivityCtx(gCtx, node.ID)
			return workflow.ExecuteActivity(nodeCtx, "ConfigureWireGuardPeer", activity.ConfigureWireGuardPeerParams{
				PublicKey:    peer.PublicKey,
				PresharedKey: peer.PresharedKey,
				AssignedIP:   peer.AssignedIP,
				AllowedIPs:   allowedIPs,
			}).Get(gCtx, nil)
		})
		if len(errs) > 0 {
			_ = setResourceFailed(ctx, "wireguard_peers", peerID, fmt.Errorf("configure peer on gateway: %s", joinErrors(errs)))
			return fmt.Errorf("configure peer on gateway: %s", joinErrors(errs))
		}
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "wireguard_peers",
		ID:     peerID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteWireGuardPeerWorkflow removes a WireGuard peer from gateway nodes.
func DeleteWireGuardPeerWorkflow(ctx workflow.Context, peerID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to deleting.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "wireguard_peers",
		ID:     peerID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the peer.
	var peer model.WireGuardPeer
	err = workflow.ExecuteActivity(ctx, "GetWireGuardPeerByID", peerID).Get(ctx, &peer)
	if err != nil {
		_ = setResourceFailed(ctx, "wireguard_peers", peerID, err)
		return err
	}

	// Look up tenant for cluster ID.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", peer.TenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "wireguard_peers", peerID, err)
		return err
	}

	// Find gateway shard nodes.
	var shards []model.Shard
	err = workflow.ExecuteActivity(ctx, "ListShardsByClusterAndRole", tenant.ClusterID, model.ShardRoleGateway).Get(ctx, &shards)
	if err != nil {
		_ = setResourceFailed(ctx, "wireguard_peers", peerID, err)
		return err
	}

	// Fan out RemoveWireGuardPeer to all gateway nodes.
	for _, shard := range shards {
		var nodes []model.Node
		err = workflow.ExecuteActivity(ctx, "ListNodesByShard", shard.ID).Get(ctx, &nodes)
		if err != nil {
			_ = setResourceFailed(ctx, "wireguard_peers", peerID, err)
			return err
		}

		errs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
			nodeCtx := nodeActivityCtx(gCtx, node.ID)
			return workflow.ExecuteActivity(nodeCtx, "RemoveWireGuardPeer", activity.RemoveWireGuardPeerParams{
				PublicKey:  peer.PublicKey,
				AssignedIP: peer.AssignedIP,
			}).Get(gCtx, nil)
		})
		if len(errs) > 0 {
			_ = setResourceFailed(ctx, "wireguard_peers", peerID, fmt.Errorf("remove peer from gateway: %s", joinErrors(errs)))
			return fmt.Errorf("remove peer from gateway: %s", joinErrors(errs))
		}
	}

	// Hard delete.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "wireguard_peers",
		ID:     peerID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
