package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// AddSSHKeyWorkflow provisions an SSH key by syncing authorized_keys on all shard nodes.
func AddSSHKeyWorkflow(ctx workflow.Context, keyID string) error {
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

	// Set key status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "ssh_keys",
		ID:     keyID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Get the key by ID.
	var key model.SSHKey
	err = workflow.ExecuteActivity(ctx, "GetSSHKeyByID", keyID).Get(ctx, &key)
	if err != nil {
		_ = setResourceFailed(ctx, "ssh_keys", keyID, err)
		return err
	}

	// Get the tenant by ID.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", key.TenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "ssh_keys", keyID, err)
		return err
	}

	if tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", key.TenantID)
		_ = setResourceFailed(ctx, "ssh_keys", keyID, noShardErr)
		return noShardErr
	}

	// Get all active SSH keys for the tenant.
	var activeKeys []model.SSHKey
	err = workflow.ExecuteActivity(ctx, "GetSSHKeysByTenant", key.TenantID).Get(ctx, &activeKeys)
	if err != nil {
		_ = setResourceFailed(ctx, "ssh_keys", keyID, err)
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
		_ = setResourceFailed(ctx, "ssh_keys", keyID, err)
		return err
	}

	// Sync authorized_keys on each node.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "SyncSSHKeys", activity.SyncSSHKeysParams{
			TenantName: tenant.Name,
			PublicKeys: publicKeys,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "ssh_keys", keyID, err)
			return err
		}
	}

	// Set key status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "ssh_keys",
		ID:     keyID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// RemoveSSHKeyWorkflow removes an SSH key by syncing authorized_keys on all shard nodes.
func RemoveSSHKeyWorkflow(ctx workflow.Context, keyID string) error {
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

	// Get the key by ID (before deletion, it's in deleting status).
	var key model.SSHKey
	err := workflow.ExecuteActivity(ctx, "GetSSHKeyByID", keyID).Get(ctx, &key)
	if err != nil {
		_ = setResourceFailed(ctx, "ssh_keys", keyID, err)
		return err
	}

	// Set key status to deleted.
	err = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "ssh_keys",
		ID:     keyID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "ssh_keys", keyID, err)
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

	// Get remaining active SSH keys (deleted key won't be included).
	var remainingKeys []model.SSHKey
	err = workflow.ExecuteActivity(ctx, "GetSSHKeysByTenant", key.TenantID).Get(ctx, &remainingKeys)
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
		err = workflow.ExecuteActivity(nodeCtx, "SyncSSHKeys", activity.SyncSSHKeysParams{
			TenantName: tenant.Name,
			PublicKeys: publicKeys,
		}).Get(ctx, nil)
		if err != nil {
			return err
		}
	}

	return nil
}
