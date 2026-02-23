package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateBackupWorkflow creates a backup of a web storage or MySQL database.
func CreateBackupWorkflow(ctx workflow.Context, backupID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
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
		Table:  "backups",
		ID:     backupID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Get the backup, tenant, and nodes.
	var bctx activity.BackupContext
	err = workflow.ExecuteActivity(ctx, "GetBackupContext", backupID).Get(ctx, &bctx)
	if err != nil {
		_ = setResourceFailed(ctx, "backups", backupID, err)
		return err
	}

	if bctx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", bctx.Backup.TenantID)
		_ = setResourceFailed(ctx, "backups", backupID, noShardErr)
		return noShardErr
	}

	if len(bctx.Nodes) == 0 {
		noNodesErr := fmt.Errorf("no nodes found for shard %s", *bctx.Tenant.ShardID)
		_ = setResourceFailed(ctx, "backups", backupID, noNodesErr)
		return noNodesErr
	}

	// Pick first node for backup.
	node := bctx.Nodes[0]
	nodeCtx := nodeActivityCtx(ctx, node.ID)

	// Record start time.
	startedAt := workflow.Now(ctx)

	var result activity.BackupResult

	switch bctx.Backup.Type {
	case model.BackupTypeWeb:
		// Get webroot by source_id.
		var webroot model.Webroot
		err = workflow.ExecuteActivity(ctx, "GetWebrootByID", bctx.Backup.SourceID).Get(ctx, &webroot)
		if err != nil {
			_ = setResourceFailed(ctx, "backups", backupID, err)
			return err
		}

		backupPath := fmt.Sprintf("/var/backups/hosting/%s/%s.tar.gz", bctx.Tenant.ID, backupID)
		err = workflow.ExecuteActivity(nodeCtx, "CreateWebBackup", activity.CreateWebBackupParams{
			TenantName:  bctx.Tenant.ID,
			WebrootName: webroot.ID,
			BackupPath:  backupPath,
		}).Get(ctx, &result)
		if err != nil {
			_ = setResourceFailed(ctx, "backups", backupID, err)
			return err
		}

	case model.BackupTypeDatabase:
		backupPath := fmt.Sprintf("/var/backups/hosting/%s/%s.sql.gz", bctx.Tenant.ID, backupID)
		err = workflow.ExecuteActivity(nodeCtx, "CreateMySQLBackup", activity.CreateMySQLBackupParams{
			DatabaseName: bctx.Backup.SourceName,
			BackupPath:   backupPath,
		}).Get(ctx, &result)
		if err != nil {
			_ = setResourceFailed(ctx, "backups", backupID, err)
			return err
		}

	default:
		unsupportedErr := fmt.Errorf("unsupported backup type: %s", bctx.Backup.Type)
		_ = setResourceFailed(ctx, "backups", backupID, unsupportedErr)
		return unsupportedErr
	}

	completedAt := workflow.Now(ctx)

	// Update backup result.
	err = workflow.ExecuteActivity(ctx, "UpdateBackupResult", activity.UpdateBackupResultParams{
		ID:          backupID,
		StoragePath: result.StoragePath,
		SizeBytes:   result.SizeBytes,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "backups", backupID, err)
		return err
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "backups",
		ID:     backupID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// RestoreBackupWorkflow restores a backup to the target resource.
func RestoreBackupWorkflow(ctx workflow.Context, backupID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Get the backup, tenant, and nodes.
	var bctx activity.BackupContext
	err := workflow.ExecuteActivity(ctx, "GetBackupContext", backupID).Get(ctx, &bctx)
	if err != nil {
		return err
	}

	// Set status to provisioning (restore in progress).
	err = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "backups",
		ID:     backupID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	if bctx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", bctx.Backup.TenantID)
		_ = setResourceFailed(ctx, "backups", backupID, noShardErr)
		return noShardErr
	}

	if len(bctx.Nodes) == 0 {
		noNodesErr := fmt.Errorf("no nodes found for shard %s", *bctx.Tenant.ShardID)
		_ = setResourceFailed(ctx, "backups", backupID, noNodesErr)
		return noNodesErr
	}

	switch bctx.Backup.Type {
	case model.BackupTypeWeb:
		// Get webroot by source_id.
		var webroot model.Webroot
		err = workflow.ExecuteActivity(ctx, "GetWebrootByID", bctx.Backup.SourceID).Get(ctx, &webroot)
		if err != nil {
			_ = setResourceFailed(ctx, "backups", backupID, err)
			return err
		}

		// Restore on all shard nodes (shared storage means only 1 needed, but be safe).
		for _, node := range bctx.Nodes {
			nodeCtx := nodeActivityCtx(ctx, node.ID)
			err = workflow.ExecuteActivity(nodeCtx, "RestoreWebBackup", activity.RestoreWebBackupParams{
				TenantName:  bctx.Tenant.ID,
				WebrootName: webroot.ID,
				BackupPath:  bctx.Backup.StoragePath,
			}).Get(ctx, nil)
			if err != nil {
				_ = setResourceFailed(ctx, "backups", backupID, err)
				return err
			}
		}

	case model.BackupTypeDatabase:
		// Restore on first node only.
		nodeCtx := nodeActivityCtx(ctx, bctx.Nodes[0].ID)
		err = workflow.ExecuteActivity(nodeCtx, "RestoreMySQLBackup", activity.RestoreMySQLBackupParams{
			DatabaseName: bctx.Backup.SourceName,
			BackupPath:   bctx.Backup.StoragePath,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "backups", backupID, err)
			return err
		}

	default:
		unsupportedErr := fmt.Errorf("unsupported backup type: %s", bctx.Backup.Type)
		_ = setResourceFailed(ctx, "backups", backupID, unsupportedErr)
		return unsupportedErr
	}

	// Set backup status back to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "backups",
		ID:     backupID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteBackupWorkflow deletes a backup file and marks the record as deleted.
func DeleteBackupWorkflow(ctx workflow.Context, backupID string) error {
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

	// Get the backup, tenant, and nodes.
	var bctx activity.BackupContext
	err := workflow.ExecuteActivity(ctx, "GetBackupContext", backupID).Get(ctx, &bctx)
	if err != nil {
		_ = setResourceFailed(ctx, "backups", backupID, err)
		return err
	}

	if bctx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", bctx.Backup.TenantID)
		_ = setResourceFailed(ctx, "backups", backupID, noShardErr)
		return noShardErr
	}

	if len(bctx.Nodes) == 0 {
		noNodesErr := fmt.Errorf("no nodes found for shard %s", *bctx.Tenant.ShardID)
		_ = setResourceFailed(ctx, "backups", backupID, noNodesErr)
		return noNodesErr
	}

	// Delete backup file on the first node (where the backup lives).
	if bctx.Backup.StoragePath != "" {
		nodeCtx := nodeActivityCtx(ctx, bctx.Nodes[0].ID)
		err = workflow.ExecuteActivity(nodeCtx, "DeleteBackupFile", bctx.Backup.StoragePath).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "backups", backupID, err)
			return err
		}
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "backups",
		ID:     backupID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
