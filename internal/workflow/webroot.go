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

	// Add service hostname as an additional server_name if enabled.
	serviceHostname := webrootServiceHostname(wctx)
	if serviceHostname != "" {
		fqdnParams = append(fqdnParams, activity.FQDNParam{
			FQDN:      serviceHostname,
			WebrootID: wctx.Webroot.ID,
		})
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

	// Create service hostname DNS + LB entries if enabled.
	if serviceHostname != "" {
		setupServiceHostname(ctx, wctx, serviceHostname)
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

	// Add service hostname as an additional server_name if enabled.
	serviceHostname := webrootServiceHostname(wctx)
	if serviceHostname != "" {
		fqdnParams = append(fqdnParams, activity.FQDNParam{
			FQDN:      serviceHostname,
			WebrootID: wctx.Webroot.ID,
		})
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

	// Ensure service hostname DNS + LB entries are in sync.
	if serviceHostname != "" {
		setupServiceHostname(ctx, wctx, serviceHostname)
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

	// Clean up service hostname DNS + LB entries if enabled.
	serviceHostname := webrootServiceHostname(wctx)
	if serviceHostname != "" {
		teardownServiceHostname(ctx, wctx, serviceHostname)
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

// webrootServiceHostname computes the service hostname for a webroot.
// Returns empty string if service hostname is disabled or brand info is missing.
func webrootServiceHostname(wctx activity.WebrootContext) string {
	if !wctx.Webroot.ServiceHostnameEnabled || wctx.BrandBaseHostname == "" {
		return ""
	}
	return fmt.Sprintf("%s.%s.%s", wctx.Webroot.Name, wctx.Tenant.Name, wctx.BrandBaseHostname)
}

// setupServiceHostname creates DNS A records and LB map entries for a service hostname.
func setupServiceHostname(ctx workflow.Context, wctx activity.WebrootContext, hostname string) {
	logger := workflow.GetLogger(ctx)

	// Create DNS A records pointing to cluster LB addresses.
	err := workflow.ExecuteActivity(ctx, "AutoCreateDNSRecords", activity.AutoCreateDNSRecordsParams{
		FQDN:        hostname,
		LBAddresses: wctx.LBAddresses,
	}).Get(ctx, nil)
	if err != nil {
		logger.Warn("failed to create service hostname DNS records", "hostname", hostname, "error", err)
	}

	// Set LB map entry on all LB nodes.
	for _, lbNode := range wctx.LBNodes {
		lbCtx := nodeActivityCtx(ctx, lbNode.ID)
		err := workflow.ExecuteActivity(lbCtx, "SetLBMapEntry", activity.SetLBMapEntryParams{
			FQDN:      hostname,
			LBBackend: wctx.Shard.LBBackend,
		}).Get(ctx, nil)
		if err != nil {
			logger.Warn("failed to set service hostname LB map entry", "hostname", hostname, "node", lbNode.ID, "error", err)
		}
	}
}

// teardownServiceHostname removes DNS records and LB map entries for a service hostname.
func teardownServiceHostname(ctx workflow.Context, wctx activity.WebrootContext, hostname string) {
	logger := workflow.GetLogger(ctx)

	// Delete DNS records.
	err := workflow.ExecuteActivity(ctx, "AutoDeleteDNSRecords", hostname).Get(ctx, nil)
	if err != nil {
		logger.Warn("failed to delete service hostname DNS records", "hostname", hostname, "error", err)
	}

	// Delete LB map entry on all LB nodes.
	for _, lbNode := range wctx.LBNodes {
		lbCtx := nodeActivityCtx(ctx, lbNode.ID)
		err := workflow.ExecuteActivity(lbCtx, "DeleteLBMapEntry", activity.DeleteLBMapEntryParams{
			FQDN: hostname,
		}).Get(ctx, nil)
		if err != nil {
			logger.Warn("failed to delete service hostname LB map entry", "hostname", hostname, "node", lbNode.ID, "error", err)
		}
	}
}
