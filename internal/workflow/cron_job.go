package workflow

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// designatedNode returns the node ID responsible for executing a given cron job.
// It uses consistent hashing so that the same cron job always maps to the same
// node as long as the node set is stable.
func designatedNode(cronJobID string, nodes []model.Node) string {
	if len(nodes) == 0 {
		return ""
	}
	sorted := make([]model.Node, len(nodes))
	copy(sorted, nodes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})
	h := fnv.New32a()
	h.Write([]byte(cronJobID))
	idx := int(h.Sum32()) % len(sorted)
	return sorted[idx].ID
}

// CreateCronJobWorkflow provisions a cron job on all nodes in the tenant's shard.
func CreateCronJobWorkflow(ctx workflow.Context, cronJobID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
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
		TenantID:         cronCtx.Tenant.ID,
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

	// Enable timer on designated node if the job is enabled.
	if cronCtx.CronJob.Enabled {
		designated := designatedNode(cronCtx.CronJob.ID, cronCtx.Nodes)
		if designated != "" {
			timerParams := activity.CronJobTimerParams{
				ID:       cronCtx.CronJob.ID,
				TenantID: cronCtx.Tenant.ID,
			}
			nodeCtx := nodeActivityCtx(ctx, designated)
			if err := workflow.ExecuteActivity(nodeCtx, "EnableCronJobTimer", timerParams).Get(ctx, nil); err != nil {
				errs = append(errs, fmt.Sprintf("enable timer on %s: %v", designated, err))
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
			MaximumAttempts: 3,
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
		TenantID:         cronCtx.Tenant.ID,
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

	// Manage timer state: enable on designated node if enabled, disable on all if not.
	if cronCtx.CronJob.Enabled {
		designated := designatedNode(cronCtx.CronJob.ID, cronCtx.Nodes)
		if designated != "" {
			timerParams := activity.CronJobTimerParams{
				ID:       cronCtx.CronJob.ID,
				TenantID: cronCtx.Tenant.ID,
			}
			nodeCtx := nodeActivityCtx(ctx, designated)
			if err := workflow.ExecuteActivity(nodeCtx, "EnableCronJobTimer", timerParams).Get(ctx, nil); err != nil {
				errs = append(errs, fmt.Sprintf("enable timer on %s: %v", designated, err))
			}
		}
	} else {
		timerParams := activity.CronJobTimerParams{
			ID:       cronCtx.CronJob.ID,
			TenantID: cronCtx.Tenant.ID,
		}
		for _, node := range cronCtx.Nodes {
			nodeCtx := nodeActivityCtx(ctx, node.ID)
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
			MaximumAttempts: 3,
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
		TenantID: cronCtx.Tenant.ID,
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
			MaximumAttempts: 3,
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

	designated := designatedNode(cronCtx.CronJob.ID, cronCtx.Nodes)
	if designated == "" {
		noNodeErr := fmt.Errorf("no nodes available in shard for tenant %s", cronCtx.CronJob.TenantID)
		_ = setResourceFailed(ctx, "cron_jobs", cronJobID, noNodeErr)
		return noNodeErr
	}

	timerParams := activity.CronJobTimerParams{
		ID:       cronCtx.CronJob.ID,
		TenantID: cronCtx.Tenant.ID,
	}
	nodeCtx := nodeActivityCtx(ctx, designated)
	if err := workflow.ExecuteActivity(nodeCtx, "EnableCronJobTimer", timerParams).Get(ctx, nil); err != nil {
		_ = setResourceFailed(ctx, "cron_jobs", cronJobID, err)
		return err
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
			MaximumAttempts: 3,
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
		TenantID: cronCtx.Tenant.ID,
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
