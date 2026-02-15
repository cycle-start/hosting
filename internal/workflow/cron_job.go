package workflow

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateCronJobWorkflow provisions a cron job on all nodes in the tenant's shard.
func CreateCronJobWorkflow(ctx workflow.Context, cronJobID string) error {
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
		Table:  "cron_jobs",
		ID:     cronJobID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Fetch cron job, webroot, tenant, and nodes in one activity.
	var cronCtx activity.CronJobContext
	err = workflow.ExecuteActivity(ctx, "GetCronJobContext", cronJobID).Get(ctx, &cronCtx)
	if err != nil {
		_ = setResourceFailed(ctx, "cron_jobs", cronJobID, err)
		return err
	}

	if cronCtx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", cronCtx.CronJob.TenantID)
		_ = setResourceFailed(ctx, "cron_jobs", cronJobID, noShardErr)
		return noShardErr
	}

	createParams := activity.CreateCronJobParams{
		ID:               cronCtx.CronJob.ID,
		TenantName:       cronCtx.Tenant.Name,
		WebrootName:      cronCtx.Webroot.Name,
		Name:             cronCtx.CronJob.Name,
		Schedule:         cronCtx.CronJob.Schedule,
		Command:          cronCtx.CronJob.Command,
		WorkingDirectory: cronCtx.CronJob.WorkingDirectory,
		TimeoutSeconds:   cronCtx.CronJob.TimeoutSeconds,
		MaxMemoryMB:      cronCtx.CronJob.MaxMemoryMB,
	}

	// Write unit files on all nodes.
	var errs []string
	for _, node := range cronCtx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		if err := workflow.ExecuteActivity(nodeCtx, "CreateCronJobUnits", createParams).Get(ctx, nil); err != nil {
			errs = append(errs, fmt.Sprintf("node %s: create units: %v", node.ID, err))
		}
	}

	// Enable timer on all nodes — distributed locking via CephFS flock
	// ensures only one node executes the job at a time.
	if cronCtx.CronJob.Enabled {
		timerParams := activity.CronJobTimerParams{
			ID:         cronCtx.CronJob.ID,
			TenantName: cronCtx.Tenant.Name,
		}
		for _, node := range cronCtx.Nodes {
			nodeCtx := nodeActivityCtx(ctx, node.ID)
			if err := workflow.ExecuteActivity(nodeCtx, "EnableCronJobTimer", timerParams).Get(ctx, nil); err != nil {
				errs = append(errs, fmt.Sprintf("node %s: enable timer: %v", node.ID, err))
			}
		}
	}

	if len(errs) > 0 {
		msg := strings.Join(errs, "; ")
		if len(msg) > 4000 {
			msg = msg[:4000]
		}
		_ = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
			Table:         "cron_jobs",
			ID:            cronJobID,
			Status:        model.StatusFailed,
			StatusMessage: &msg,
		}).Get(ctx, nil)
		return fmt.Errorf("create cron job failed: %s", msg)
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "cron_jobs",
		ID:     cronJobID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// UpdateCronJobWorkflow updates the cron job configuration on all nodes.
func UpdateCronJobWorkflow(ctx workflow.Context, cronJobID string) error {
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
		Table:  "cron_jobs",
		ID:     cronJobID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	var cronCtx activity.CronJobContext
	err = workflow.ExecuteActivity(ctx, "GetCronJobContext", cronJobID).Get(ctx, &cronCtx)
	if err != nil {
		_ = setResourceFailed(ctx, "cron_jobs", cronJobID, err)
		return err
	}

	if cronCtx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", cronCtx.CronJob.TenantID)
		_ = setResourceFailed(ctx, "cron_jobs", cronJobID, noShardErr)
		return noShardErr
	}

	updateParams := activity.CreateCronJobParams{
		ID:               cronCtx.CronJob.ID,
		TenantName:       cronCtx.Tenant.Name,
		WebrootName:      cronCtx.Webroot.Name,
		Name:             cronCtx.CronJob.Name,
		Schedule:         cronCtx.CronJob.Schedule,
		Command:          cronCtx.CronJob.Command,
		WorkingDirectory: cronCtx.CronJob.WorkingDirectory,
		TimeoutSeconds:   cronCtx.CronJob.TimeoutSeconds,
		MaxMemoryMB:      cronCtx.CronJob.MaxMemoryMB,
	}

	var errs []string
	for _, node := range cronCtx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		if err := workflow.ExecuteActivity(nodeCtx, "UpdateCronJobUnits", updateParams).Get(ctx, nil); err != nil {
			errs = append(errs, fmt.Sprintf("node %s: update units: %v", node.ID, err))
		}
	}

	// Manage timer state on all nodes — flock ensures single execution.
	timerParams := activity.CronJobTimerParams{
		ID:         cronCtx.CronJob.ID,
		TenantName: cronCtx.Tenant.Name,
	}
	for _, node := range cronCtx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		if cronCtx.CronJob.Enabled {
			if err := workflow.ExecuteActivity(nodeCtx, "EnableCronJobTimer", timerParams).Get(ctx, nil); err != nil {
				errs = append(errs, fmt.Sprintf("node %s: enable timer: %v", node.ID, err))
			}
		} else {
			if err := workflow.ExecuteActivity(nodeCtx, "DisableCronJobTimer", timerParams).Get(ctx, nil); err != nil {
				errs = append(errs, fmt.Sprintf("node %s: disable timer: %v", node.ID, err))
			}
		}
	}

	if len(errs) > 0 {
		msg := strings.Join(errs, "; ")
		if len(msg) > 4000 {
			msg = msg[:4000]
		}
		_ = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
			Table:         "cron_jobs",
			ID:            cronJobID,
			Status:        model.StatusFailed,
			StatusMessage: &msg,
		}).Get(ctx, nil)
		return fmt.Errorf("update cron job failed: %s", msg)
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "cron_jobs",
		ID:     cronJobID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteCronJobWorkflow removes cron job units from all nodes.
func DeleteCronJobWorkflow(ctx workflow.Context, cronJobID string) error {
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
		Table:  "cron_jobs",
		ID:     cronJobID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	var cronCtx activity.CronJobContext
	err = workflow.ExecuteActivity(ctx, "GetCronJobContext", cronJobID).Get(ctx, &cronCtx)
	if err != nil {
		_ = setResourceFailed(ctx, "cron_jobs", cronJobID, err)
		return err
	}

	if cronCtx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", cronCtx.CronJob.TenantID)
		_ = setResourceFailed(ctx, "cron_jobs", cronJobID, noShardErr)
		return noShardErr
	}

	deleteParams := activity.DeleteCronJobParams{
		ID:       cronCtx.CronJob.ID,
		TenantName: cronCtx.Tenant.Name,
	}

	var errs []string
	for _, node := range cronCtx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		if err := workflow.ExecuteActivity(nodeCtx, "DeleteCronJobUnits", deleteParams).Get(ctx, nil); err != nil {
			errs = append(errs, fmt.Sprintf("node %s: delete units: %v", node.ID, err))
		}
	}

	if len(errs) > 0 {
		msg := strings.Join(errs, "; ")
		if len(msg) > 4000 {
			msg = msg[:4000]
		}
		_ = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
			Table:         "cron_jobs",
			ID:            cronJobID,
			Status:        model.StatusFailed,
			StatusMessage: &msg,
		}).Get(ctx, nil)
		return fmt.Errorf("delete cron job failed: %s", msg)
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "cron_jobs",
		ID:     cronJobID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}

// EnableCronJobWorkflow enables the cron timer on the designated node.
func EnableCronJobWorkflow(ctx workflow.Context, cronJobID string) error {
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
		Table:  "cron_jobs",
		ID:     cronJobID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	var cronCtx activity.CronJobContext
	err = workflow.ExecuteActivity(ctx, "GetCronJobContext", cronJobID).Get(ctx, &cronCtx)
	if err != nil {
		_ = setResourceFailed(ctx, "cron_jobs", cronJobID, err)
		return err
	}

	if cronCtx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", cronCtx.CronJob.TenantID)
		_ = setResourceFailed(ctx, "cron_jobs", cronJobID, noShardErr)
		return noShardErr
	}

	if len(cronCtx.Nodes) == 0 {
		noNodeErr := fmt.Errorf("no nodes available in shard for tenant %s", cronCtx.CronJob.TenantID)
		_ = setResourceFailed(ctx, "cron_jobs", cronJobID, noNodeErr)
		return noNodeErr
	}

	timerParams := activity.CronJobTimerParams{
		ID:         cronCtx.CronJob.ID,
		TenantName: cronCtx.Tenant.Name,
	}
	var errs []string
	for _, node := range cronCtx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		if err := workflow.ExecuteActivity(nodeCtx, "EnableCronJobTimer", timerParams).Get(ctx, nil); err != nil {
			errs = append(errs, fmt.Sprintf("node %s: %v", node.ID, err))
		}
	}

	if len(errs) > 0 {
		msg := strings.Join(errs, "; ")
		_ = setResourceFailed(ctx, "cron_jobs", cronJobID, fmt.Errorf("%s", msg))
		return fmt.Errorf("enable cron job failed: %s", msg)
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "cron_jobs",
		ID:     cronJobID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DisableCronJobWorkflow disables the cron timer on all nodes.
func DisableCronJobWorkflow(ctx workflow.Context, cronJobID string) error {
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
		Table:  "cron_jobs",
		ID:     cronJobID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	var cronCtx activity.CronJobContext
	err = workflow.ExecuteActivity(ctx, "GetCronJobContext", cronJobID).Get(ctx, &cronCtx)
	if err != nil {
		_ = setResourceFailed(ctx, "cron_jobs", cronJobID, err)
		return err
	}

	if cronCtx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", cronCtx.CronJob.TenantID)
		_ = setResourceFailed(ctx, "cron_jobs", cronJobID, noShardErr)
		return noShardErr
	}

	timerParams := activity.CronJobTimerParams{
		ID:       cronCtx.CronJob.ID,
		TenantName: cronCtx.Tenant.Name,
	}

	var errs []string
	for _, node := range cronCtx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		if err := workflow.ExecuteActivity(nodeCtx, "DisableCronJobTimer", timerParams).Get(ctx, nil); err != nil {
			errs = append(errs, fmt.Sprintf("node %s: %v", node.ID, err))
		}
	}

	if len(errs) > 0 {
		msg := strings.Join(errs, "; ")
		if len(msg) > 4000 {
			msg = msg[:4000]
		}
		_ = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
			Table:         "cron_jobs",
			ID:            cronJobID,
			Status:        model.StatusFailed,
			StatusMessage: &msg,
		}).Get(ctx, nil)
		return fmt.Errorf("disable cron job failed: %s", msg)
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "cron_jobs",
		ID:     cronJobID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}
