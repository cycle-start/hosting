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
			MaximumAttempts: 3,
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

	// Create webroot on each node in the shard.
	for _, node := range wctx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "CreateWebroot", activity.CreateWebrootParams{
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
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "webroots", webrootID, err)
			return err
		}
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "webroots",
		ID:     webrootID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// UpdateWebrootWorkflow updates a webroot on all nodes in the tenant's shard.
func UpdateWebrootWorkflow(ctx workflow.Context, webrootID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
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

	// Update webroot on each node in the shard.
	for _, node := range wctx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "UpdateWebroot", activity.UpdateWebrootParams{
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
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "webroots", webrootID, err)
			return err
		}
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
			MaximumAttempts: 3,
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

	// Delete webroot on each node in the shard.
	for _, node := range wctx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "DeleteWebroot", wctx.Tenant.Name, wctx.Webroot.Name).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "webroots", webrootID, err)
			return err
		}
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "webroots",
		ID:     webrootID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
