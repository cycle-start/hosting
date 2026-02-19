package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateWebrootWorkflow provisions a new webroot on all nodes in the tenant's shard.
func CreateWebrootWorkflow(ctx workflow.Context, webrootID string) error {
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
		Table:  "webroots",
		ID:     webrootID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Fetch webroot, tenant, FQDNs, and nodes in one activity.
	var wctx activity.WebrootContext
	err = workflow.ExecuteActivity(ctx, "GetWebrootContext", webrootID).Get(ctx, &wctx)
	if err != nil {
		_ = setResourceFailed(ctx, "webroots", webrootID, err)
		return err
	}

	fqdnParams := make([]activity.FQDNParam, len(wctx.FQDNs))
	for i, f := range wctx.FQDNs {
		fqdnParams[i] = activity.FQDNParam{
			FQDN:       f.FQDN,
			WebrootID:  f.WebrootID,
			SSLEnabled: f.SSLEnabled,
		}
	}

	if wctx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", wctx.Webroot.TenantID)
		_ = setResourceFailed(ctx, "webroots", webrootID, noShardErr)
		return noShardErr
	}

	// Create webroot on each node in the shard (parallel).
	errs := fanOutNodes(ctx, wctx.Nodes, func(gCtx workflow.Context, node model.Node) error {
		nodeCtx := nodeActivityCtx(gCtx, node.ID)
		return workflow.ExecuteActivity(nodeCtx, "CreateWebroot", activity.CreateWebrootParams{
			ID:             wctx.Webroot.ID,
			TenantName:     wctx.Tenant.Name,
			Name:           wctx.Webroot.Name,
			Runtime:        wctx.Webroot.Runtime,
			RuntimeVersion: wctx.Webroot.RuntimeVersion,
			RuntimeConfig:  string(wctx.Webroot.RuntimeConfig),
			PublicFolder:   wctx.Webroot.PublicFolder,
			EnvFileName:    wctx.Webroot.EnvFileName,
			EnvShellSource: wctx.Webroot.EnvShellSource,
			FQDNs:          fqdnParams,
		}).Get(gCtx, nil)
	})
	if len(errs) > 0 {
		combinedErr := fmt.Errorf("create webroot errors: %s", joinErrors(errs))
		_ = setResourceFailed(ctx, "webroots", webrootID, combinedErr)
		return combinedErr
	}

	// Set status to active.
	err = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "webroots",
		ID:     webrootID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Spawn pending child workflows in parallel.
	var children []ChildWorkflowSpec

	var fqdns []model.FQDN
	_ = workflow.ExecuteActivity(ctx, "ListFQDNsByWebrootID", webrootID).Get(ctx, &fqdns)
	for _, f := range fqdns {
		if f.Status == model.StatusPending {
			children = append(children, ChildWorkflowSpec{WorkflowName: "BindFQDNWorkflow", WorkflowID: fmt.Sprintf("create-fqdn-%s", f.ID), Arg: f.ID})
		}
	}

	var daemons []model.Daemon
	_ = workflow.ExecuteActivity(ctx, "ListDaemonsByWebrootID", webrootID).Get(ctx, &daemons)
	for _, d := range daemons {
		if d.Status == model.StatusPending {
			children = append(children, ChildWorkflowSpec{WorkflowName: "CreateDaemonWorkflow", WorkflowID: fmt.Sprintf("create-daemon-%s", d.ID), Arg: d.ID})
		}
	}

	var cronJobs []model.CronJob
	_ = workflow.ExecuteActivity(ctx, "ListCronJobsByWebrootID", webrootID).Get(ctx, &cronJobs)
	for _, c := range cronJobs {
		if c.Status == model.StatusPending {
			children = append(children, ChildWorkflowSpec{WorkflowName: "CreateCronJobWorkflow", WorkflowID: fmt.Sprintf("create-cron-job-%s", c.ID), Arg: c.ID})
		}
	}

	if errs := fanOutChildWorkflows(ctx, children); len(errs) > 0 {
		workflow.GetLogger(ctx).Warn("child workflow failures", "errors", joinErrors(errs))
	}
	return nil
}

// UpdateWebrootWorkflow updates a webroot on all nodes in the tenant's shard.
func UpdateWebrootWorkflow(ctx workflow.Context, webrootID string) error {
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
		Table:  "webroots",
		ID:     webrootID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Fetch webroot, tenant, FQDNs, and nodes in one activity.
	var wctx activity.WebrootContext
	err = workflow.ExecuteActivity(ctx, "GetWebrootContext", webrootID).Get(ctx, &wctx)
	if err != nil {
		_ = setResourceFailed(ctx, "webroots", webrootID, err)
		return err
	}

	fqdnParams := make([]activity.FQDNParam, len(wctx.FQDNs))
	for i, f := range wctx.FQDNs {
		fqdnParams[i] = activity.FQDNParam{
			FQDN:       f.FQDN,
			WebrootID:  f.WebrootID,
			SSLEnabled: f.SSLEnabled,
		}
	}

	if wctx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", wctx.Webroot.TenantID)
		_ = setResourceFailed(ctx, "webroots", webrootID, noShardErr)
		return noShardErr
	}

	// Update webroot on each node in the shard (parallel).
	errs := fanOutNodes(ctx, wctx.Nodes, func(gCtx workflow.Context, node model.Node) error {
		nodeCtx := nodeActivityCtx(gCtx, node.ID)
		return workflow.ExecuteActivity(nodeCtx, "UpdateWebroot", activity.UpdateWebrootParams{
			ID:             wctx.Webroot.ID,
			TenantName:     wctx.Tenant.Name,
			Name:           wctx.Webroot.Name,
			Runtime:        wctx.Webroot.Runtime,
			RuntimeVersion: wctx.Webroot.RuntimeVersion,
			RuntimeConfig:  string(wctx.Webroot.RuntimeConfig),
			PublicFolder:   wctx.Webroot.PublicFolder,
			EnvFileName:    wctx.Webroot.EnvFileName,
			EnvShellSource: wctx.Webroot.EnvShellSource,
			FQDNs:          fqdnParams,
		}).Get(gCtx, nil)
	})
	if len(errs) > 0 {
		combinedErr := fmt.Errorf("update webroot errors: %s", joinErrors(errs))
		_ = setResourceFailed(ctx, "webroots", webrootID, combinedErr)
		return combinedErr
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "webroots",
		ID:     webrootID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteWebrootWorkflow deletes a webroot from all nodes in the tenant's shard.
func DeleteWebrootWorkflow(ctx workflow.Context, webrootID string) error {
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
		Table:  "webroots",
		ID:     webrootID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Fetch webroot, tenant, and nodes in one activity.
	var wctx activity.WebrootContext
	err = workflow.ExecuteActivity(ctx, "GetWebrootContext", webrootID).Get(ctx, &wctx)
	if err != nil {
		_ = setResourceFailed(ctx, "webroots", webrootID, err)
		return err
	}

	if wctx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", wctx.Webroot.TenantID)
		_ = setResourceFailed(ctx, "webroots", webrootID, noShardErr)
		return noShardErr
	}

	// Delete webroot on each node in the shard (parallel, continue-on-error).
	errs := fanOutNodes(ctx, wctx.Nodes, func(gCtx workflow.Context, node model.Node) error {
		nodeCtx := nodeActivityCtx(gCtx, node.ID)
		if err := workflow.ExecuteActivity(nodeCtx, "DeleteWebroot", wctx.Tenant.Name, wctx.Webroot.Name).Get(gCtx, nil); err != nil {
			return fmt.Errorf("node %s: %v", node.ID, err)
		}
		return nil
	})
	if len(errs) > 0 {
		combinedErr := fmt.Errorf("delete webroot errors: %s", joinErrors(errs))
		_ = setResourceFailed(ctx, "webroots", webrootID, combinedErr)
		return combinedErr
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "webroots",
		ID:     webrootID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
