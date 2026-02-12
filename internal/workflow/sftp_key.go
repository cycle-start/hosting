package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// AddSFTPKeyWorkflow provisions an SFTP key by syncing authorized_keys on all shard nodes.
func AddSFTPKeyWorkflow(ctx workflow.Context, keyID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set key status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "sftp_keys",
		ID:     keyID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Get the key by ID.
	var key model.SFTPKey
	err = workflow.ExecuteActivity(ctx, "GetSFTPKeyByID", keyID).Get(ctx, &key)
	if err != nil {
		_ = setResourceFailed(ctx, "sftp_keys", keyID)
		return err
	}

	// Get the tenant by ID.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", key.TenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "sftp_keys", keyID)
		return err
	}

	if tenant.ShardID == nil {
		_ = setResourceFailed(ctx, "sftp_keys", keyID)
		return fmt.Errorf("tenant %s has no shard assigned", key.TenantID)
	}

	// Get all active SFTP keys for the tenant.
	var activeKeys []model.SFTPKey
	err = workflow.ExecuteActivity(ctx, "GetSFTPKeysByTenant", key.TenantID).Get(ctx, &activeKeys)
	if err != nil {
		_ = setResourceFailed(ctx, "sftp_keys", keyID)
		return err
	}

	// Collect all public keys (include the new key which is not yet active).
	publicKeys := make([]string, 0, len(activeKeys)+1)
	for _, k := range activeKeys {
		publicKeys = append(publicKeys, k.PublicKey)
	}
	// The new key is still in provisioning status, so it won't be in activeKeys.
	publicKeys = append(publicKeys, key.PublicKey)

	// Get shard nodes.
	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "sftp_keys", keyID)
		return err
	}

	// Sync authorized_keys on each node.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "SyncSFTPKeys", activity.SyncSFTPKeysParams{
			TenantName: tenant.Name,
			PublicKeys: publicKeys,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "sftp_keys", keyID)
			return err
		}
	}

	// Set key status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "sftp_keys",
		ID:     keyID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// RemoveSFTPKeyWorkflow removes an SFTP key by syncing authorized_keys on all shard nodes.
func RemoveSFTPKeyWorkflow(ctx workflow.Context, keyID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Get the key by ID (before deletion, it's in deleting status).
	var key model.SFTPKey
	err := workflow.ExecuteActivity(ctx, "GetSFTPKeyByID", keyID).Get(ctx, &key)
	if err != nil {
		_ = setResourceFailed(ctx, "sftp_keys", keyID)
		return err
	}

	// Set key status to deleted.
	err = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "sftp_keys",
		ID:     keyID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "sftp_keys", keyID)
		return err
	}

	// Get the tenant by ID.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", key.TenantID).Get(ctx, &tenant)
	if err != nil {
		return err
	}

	if tenant.ShardID == nil {
		return fmt.Errorf("tenant %s has no shard assigned", key.TenantID)
	}

	// Get remaining active SFTP keys (deleted key won't be included).
	var remainingKeys []model.SFTPKey
	err = workflow.ExecuteActivity(ctx, "GetSFTPKeysByTenant", key.TenantID).Get(ctx, &remainingKeys)
	if err != nil {
		return err
	}

	// Collect remaining public keys.
	publicKeys := make([]string, 0, len(remainingKeys))
	for _, k := range remainingKeys {
		publicKeys = append(publicKeys, k.PublicKey)
	}

	// Get shard nodes.
	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		return err
	}

	// Sync authorized_keys on each node.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "SyncSFTPKeys", activity.SyncSFTPKeysParams{
			TenantName: tenant.Name,
			PublicKeys: publicKeys,
		}).Get(ctx, nil)
		if err != nil {
			return err
		}
	}

	return nil
}
