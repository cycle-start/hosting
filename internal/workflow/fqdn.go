package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// BindFQDNWorkflow binds an FQDN to a webroot, creating auto-DNS records
// and optionally provisioning a Let's Encrypt certificate.
func BindFQDNWorkflow(ctx workflow.Context, fqdnID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "fqdns",
		ID:     fqdnID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Fetch FQDN context (FQDN, webroot, tenant, shard, nodes, LB addresses).
	var fctx activity.FQDNContext
	err = workflow.ExecuteActivity(ctx, "GetFQDNContext", fqdnID).Get(ctx, &fctx)
	if err != nil {
		_ = setResourceFailed(ctx, "fqdns", fqdnID, err)
		return err
	}

	// Create auto-DNS records if a zone exists for the domain.
	err = workflow.ExecuteActivity(ctx, "AutoCreateDNSRecords", activity.AutoCreateDNSRecordsParams{
		FQDN:         fctx.FQDN.FQDN,
		LBAddresses:  fctx.LBAddresses,
		SourceFQDNID: fqdnID,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "fqdns", fqdnID, err)
		return err
	}

	// Reload nginx on all nodes in the tenant's shard.
	if fctx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", fctx.Webroot.TenantID)
		_ = setResourceFailed(ctx, "fqdns", fqdnID, noShardErr)
		return noShardErr
	}

	for _, node := range fctx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "ReloadNginx").Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "fqdns", fqdnID, err)
			return err
		}
	}

	// If SSL is enabled, start a child workflow for Let's Encrypt provisioning.
	if fctx.FQDN.SSLEnabled {
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: "provision-le-cert-" + fqdnID,
		})
		err = workflow.ExecuteChildWorkflow(childCtx, ProvisionLECertWorkflow, fqdnID).Get(ctx, nil)
		if err != nil {
			// Certificate provisioning failure should not fail the FQDN binding.
			// The FQDN is still usable over HTTP.
			workflow.GetLogger(ctx).Warn("LE cert provisioning failed", "fqdnID", fqdnID, "error", err)
		}
	}

	// Update the LB map entry on all LB nodes.
	for _, lbNode := range fctx.LBNodes {
		lbCtx := nodeActivityCtx(ctx, lbNode.ID)
		err = workflow.ExecuteActivity(lbCtx, "SetLBMapEntry", activity.SetLBMapEntryParams{
			FQDN:      fctx.FQDN.FQDN,
			LBBackend: fctx.Shard.LBBackend,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "fqdns", fqdnID, err)
			return err
		}
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "fqdns",
		ID:     fqdnID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// UnbindFQDNWorkflow removes an FQDN binding, cleaning up DNS records.
func UnbindFQDNWorkflow(ctx workflow.Context, fqdnID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to deleting.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "fqdns",
		ID:     fqdnID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Fetch FQDN context (FQDN, webroot, tenant, shard, nodes, LB addresses).
	var fctx activity.FQDNContext
	err = workflow.ExecuteActivity(ctx, "GetFQDNContext", fqdnID).Get(ctx, &fctx)
	if err != nil {
		_ = setResourceFailed(ctx, "fqdns", fqdnID, err)
		return err
	}

	// Delete auto-DNS records.
	err = workflow.ExecuteActivity(ctx, "AutoDeleteDNSRecords", fctx.FQDN.FQDN).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "fqdns", fqdnID, err)
		return err
	}

	// Reload nginx on all nodes in the tenant's shard.
	if fctx.Tenant.ShardID != nil {
		for _, node := range fctx.Nodes {
			nodeCtx := nodeActivityCtx(ctx, node.ID)
			err = workflow.ExecuteActivity(nodeCtx, "ReloadNginx").Get(ctx, nil)
			if err != nil {
				_ = setResourceFailed(ctx, "fqdns", fqdnID, err)
				return err
			}
		}
	}

	// Delete the LB map entry on all LB nodes.
	for _, lbNode := range fctx.LBNodes {
		lbCtx := nodeActivityCtx(ctx, lbNode.ID)
		err = workflow.ExecuteActivity(lbCtx, "DeleteLBMapEntry", activity.DeleteLBMapEntryParams{
			FQDN: fctx.FQDN.FQDN,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "fqdns", fqdnID, err)
			return err
		}
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "fqdns",
		ID:     fqdnID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
