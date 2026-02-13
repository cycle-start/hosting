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
			MaximumAttempts: 3,
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

	// Get the backup record.
	var backup model.Backup
	err = workflow.ExecuteActivity(ctx, "GetBackupByID", backupID).Get(ctx, &backup)
	if err != nil {
		_ = setResourceFailed(ctx, "backups", backupID)
		return err
	}

	// Get the tenant to find shard.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", backup.TenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "backups", backupID)
		return err
	}

	if tenant.ShardID == nil {
		_ = setResourceFailed(ctx, "backups", backupID)
		return fmt.Errorf("tenant %s has no shard assigned", backup.TenantID)
	}

	// Get shard nodes.
	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "backups", backupID)
		return err
	}

	if len(nodes) == 0 {
		_ = setResourceFailed(ctx, "backups", backupID)
		return fmt.Errorf("no nodes found for shard %s", *tenant.ShardID)
	}

	// Pick first node for backup.
	node := nodes[0]
	nodeCtx := nodeActivityCtx(ctx, node.ID)

	// Record start time.
	startedAt := workflow.Now(ctx)

	var result activity.BackupResult

	switch backup.Type {
	case model.BackupTypeWeb:
		// Get webroot by source_id.
		var webroot model.Webroot
		err = workflow.ExecuteActivity(ctx, "GetWebrootByID", backup.SourceID).Get(ctx, &webroot)
		if err != nil {
			_ = setResourceFailed(ctx, "backups", backupID)
			return err
		}

		backupPath := fmt.Sprintf("/var/backups/hosting/%s/%s.tar.gz", tenant.ID, backupID)
		err = workflow.ExecuteActivity(nodeCtx, "CreateWebBackup", activity.CreateWebBackupParams{
			TenantName:  tenant.ID,
			WebrootName: webroot.Name,
			BackupPath:  backupPath,
		}).Get(ctx, &result)
		if err != nil {
			_ = setResourceFailed(ctx, "backups", backupID)
			return err
		}

	case model.BackupTypeDatabase:
		backupPath := fmt.Sprintf("/var/backups/hosting/%s/%s.sql.gz", tenant.ID, backupID)
		err = workflow.ExecuteActivity(nodeCtx, "CreateMySQLBackup", activity.CreateMySQLBackupParams{
			DatabaseName: backup.SourceName,
			BackupPath:   backupPath,
		}).Get(ctx, &result)
		if err != nil {
			_ = setResourceFailed(ctx, "backups", backupID)
			return err
		}

	default:
		_ = setResourceFailed(ctx, "backups", backupID)
		return fmt.Errorf("unsupported backup type: %s", backup.Type)
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
		_ = setResourceFailed(ctx, "backups", backupID)
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
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Get the backup record.
	var backup model.Backup
	err := workflow.ExecuteActivity(ctx, "GetBackupByID", backupID).Get(ctx, &backup)
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

	// Get tenant, shard, nodes.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", backup.TenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "backups", backupID)
		return err
	}

	if tenant.ShardID == nil {
		_ = setResourceFailed(ctx, "backups", backupID)
		return fmt.Errorf("tenant %s has no shard assigned", backup.TenantID)
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "backups", backupID)
		return err
	}

	if len(nodes) == 0 {
		_ = setResourceFailed(ctx, "backups", backupID)
		return fmt.Errorf("no nodes found for shard %s", *tenant.ShardID)
	}

	switch backup.Type {
	case model.BackupTypeWeb:
		// Get webroot by source_id.
		var webroot model.Webroot
		err = workflow.ExecuteActivity(ctx, "GetWebrootByID", backup.SourceID).Get(ctx, &webroot)
		if err != nil {
			_ = setResourceFailed(ctx, "backups", backupID)
			return err
		}

		// Restore on all shard nodes (shared storage means only 1 needed, but be safe).
		for _, node := range nodes {
			nodeCtx := nodeActivityCtx(ctx, node.ID)
			err = workflow.ExecuteActivity(nodeCtx, "RestoreWebBackup", activity.RestoreWebBackupParams{
				TenantName:  tenant.ID,
				WebrootName: webroot.Name,
				BackupPath:  backup.StoragePath,
			}).Get(ctx, nil)
			if err != nil {
				_ = setResourceFailed(ctx, "backups", backupID)
				return err
			}
		}

	case model.BackupTypeDatabase:
		// Restore on first node only.
		nodeCtx := nodeActivityCtx(ctx, nodes[0].ID)
		err = workflow.ExecuteActivity(nodeCtx, "RestoreMySQLBackup", activity.RestoreMySQLBackupParams{
			DatabaseName: backup.SourceName,
			BackupPath:   backup.StoragePath,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "backups", backupID)
			return err
		}

	default:
		_ = setResourceFailed(ctx, "backups", backupID)
		return fmt.Errorf("unsupported backup type: %s", backup.Type)
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
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Get the backup record.
	var backup model.Backup
	err := workflow.ExecuteActivity(ctx, "GetBackupByID", backupID).Get(ctx, &backup)
	if err != nil {
		_ = setResourceFailed(ctx, "backups", backupID)
		return err
	}

	// Get tenant, shard, nodes.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", backup.TenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "backups", backupID)
		return err
	}

	if tenant.ShardID == nil {
		_ = setResourceFailed(ctx, "backups", backupID)
		return fmt.Errorf("tenant %s has no shard assigned", backup.TenantID)
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "backups", backupID)
		return err
	}

	if len(nodes) == 0 {
		_ = setResourceFailed(ctx, "backups", backupID)
		return fmt.Errorf("no nodes found for shard %s", *tenant.ShardID)
	}

	// Delete backup file on the first node (where the backup lives).
	if backup.StoragePath != "" {
		nodeCtx := nodeActivityCtx(ctx, nodes[0].ID)
		err = workflow.ExecuteActivity(nodeCtx, "DeleteBackupFile", backup.StoragePath).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "backups", backupID)
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
