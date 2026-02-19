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
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
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
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
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

	var children []ChildWorkflowSpec
	for _, backup := range oldBackups {
		children = append(children, ChildWorkflowSpec{
			WorkflowName: "DeleteBackupWorkflow",
			WorkflowID:   "cleanup-backup-" + backup.ID,
			Arg:          backup.ID,
		})
	}
	if errs := fanOutChildWorkflows(ctx, children); len(errs) > 0 {
		logger.Error("backup cleanup failures", "errors", joinErrors(errs))
	}

	return nil
}
