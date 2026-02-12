package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/model"
)

// CleanupAuditLogsWorkflow deletes audit log entries older than the specified days.
func CleanupAuditLogsWorkflow(ctx workflow.Context, retentionDays int) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var deleted int64
	err := workflow.ExecuteActivity(ctx, "DeleteOldAuditLogs", retentionDays).Get(ctx, &deleted)
	if err != nil {
		return err
	}

	logger := workflow.GetLogger(ctx)
	logger.Info("cleaned up old audit logs", "deleted", deleted, "retentionDays", retentionDays)

	return nil
}

// CleanupOldBackupsWorkflow deletes backup records that are older than the retention period.
// It fetches all old active backups and starts a child DeleteBackupWorkflow for each.
func CleanupOldBackupsWorkflow(ctx workflow.Context, retentionDays int) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var oldBackups []model.Backup
	err := workflow.ExecuteActivity(ctx, "GetOldBackups", retentionDays).Get(ctx, &oldBackups)
	if err != nil {
		return err
	}

	logger := workflow.GetLogger(ctx)
	logger.Info("found old backups to clean up", "count", len(oldBackups))

	for _, backup := range oldBackups {
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: "cleanup-backup-" + backup.ID,
		})
		err := workflow.ExecuteChildWorkflow(childCtx, DeleteBackupWorkflow, backup.ID).Get(ctx, nil)
		if err != nil {
			logger.Error("failed to delete old backup", "backupID", backup.ID, "error", err)
			// Continue deleting other backups even if one fails.
		}
	}

	return nil
}
