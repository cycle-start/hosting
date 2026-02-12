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

	// Look up the FQDN.
	var fqdn model.FQDN
	err = workflow.ExecuteActivity(ctx, "GetFQDNByID", fqdnID).Get(ctx, &fqdn)
	if err != nil {
		_ = setResourceFailed(ctx, "fqdns", fqdnID)
		return err
	}

	// Look up the webroot.
	var webroot model.Webroot
	err = workflow.ExecuteActivity(ctx, "GetWebrootByID", fqdn.WebrootID).Get(ctx, &webroot)
	if err != nil {
		_ = setResourceFailed(ctx, "fqdns", fqdnID)
		return err
	}

	// Look up the tenant.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", webroot.TenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "fqdns", fqdnID)
		return err
	}

	// Fetch cluster LB addresses.
	var lbAddresses []model.ClusterLBAddress
	err = workflow.ExecuteActivity(ctx, "GetClusterLBAddresses", tenant.ClusterID).Get(ctx, &lbAddresses)
	if err != nil {
		_ = setResourceFailed(ctx, "fqdns", fqdnID)
		return err
	}

	// Create auto-DNS records if a zone exists for the domain.
	err = workflow.ExecuteActivity(ctx, "AutoCreateDNSRecords", activity.AutoCreateDNSRecordsParams{
		FQDN:         fqdn.FQDN,
		LBAddresses:  lbAddresses,
		SourceFQDNID: fqdnID,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "fqdns", fqdnID)
		return err
	}

	// Reload nginx on all nodes in the tenant's shard.
	if tenant.ShardID == nil {
		_ = setResourceFailed(ctx, "fqdns", fqdnID)
		return fmt.Errorf("tenant %s has no shard assigned", webroot.TenantID)
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "fqdns", fqdnID)
		return err
	}

	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "ReloadNginx").Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "fqdns", fqdnID)
			return err
		}
	}

	// If SSL is enabled, start a child workflow for Let's Encrypt provisioning.
	if fqdn.SSLEnabled {
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

	// Update the LB map entry if the tenant is assigned to a shard.
	if tenant.ShardID != nil {
		var shard model.Shard
		err = workflow.ExecuteActivity(ctx, "GetShardByID", *tenant.ShardID).Get(ctx, &shard)
		if err != nil {
			_ = setResourceFailed(ctx, "fqdns", fqdnID)
			return err
		}

		err = workflow.ExecuteActivity(ctx, "SetLBMapEntry", activity.SetLBMapEntryParams{
			ClusterID: tenant.ClusterID,
			FQDN:      fqdn.FQDN,
			LBBackend: shard.LBBackend,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "fqdns", fqdnID)
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

	// Look up the FQDN.
	var fqdn model.FQDN
	err = workflow.ExecuteActivity(ctx, "GetFQDNByID", fqdnID).Get(ctx, &fqdn)
	if err != nil {
		_ = setResourceFailed(ctx, "fqdns", fqdnID)
		return err
	}

	// Look up the webroot and tenant for shard-aware nginx reload.
	var ubWebroot model.Webroot
	err = workflow.ExecuteActivity(ctx, "GetWebrootByID", fqdn.WebrootID).Get(ctx, &ubWebroot)
	if err != nil {
		_ = setResourceFailed(ctx, "fqdns", fqdnID)
		return err
	}

	var ubTenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", ubWebroot.TenantID).Get(ctx, &ubTenant)
	if err != nil {
		_ = setResourceFailed(ctx, "fqdns", fqdnID)
		return err
	}

	// Delete auto-DNS records.
	err = workflow.ExecuteActivity(ctx, "AutoDeleteDNSRecords", fqdn.FQDN).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "fqdns", fqdnID)
		return err
	}

	// Reload nginx on all nodes in the tenant's shard.
	if ubTenant.ShardID != nil {
		var ubNodes []model.Node
		err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *ubTenant.ShardID).Get(ctx, &ubNodes)
		if err != nil {
			_ = setResourceFailed(ctx, "fqdns", fqdnID)
			return err
		}

		for _, node := range ubNodes {
			nodeCtx := nodeActivityCtx(ctx, node.ID)
			err = workflow.ExecuteActivity(nodeCtx, "ReloadNginx").Get(ctx, nil)
			if err != nil {
				_ = setResourceFailed(ctx, "fqdns", fqdnID)
				return err
			}
		}
	}

	// Delete the LB map entry.
	err = workflow.ExecuteActivity(ctx, "DeleteLBMapEntry", activity.DeleteLBMapEntryParams{
		ClusterID: ubTenant.ClusterID,
		FQDN:      fqdn.FQDN,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "fqdns", fqdnID)
		return err
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "fqdns",
		ID:     fqdnID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
