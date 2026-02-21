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
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
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

	// Regenerate nginx config on all nodes to include the new FQDN in server_name.
	if fctx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", fctx.Webroot.TenantID)
		_ = setResourceFailed(ctx, "fqdns", fqdnID, noShardErr)
		return noShardErr
	}

	// Fetch all FQDNs for the webroot (including the one just being bound).
	var allFQDNs []model.FQDN
	err = workflow.ExecuteActivity(ctx, "GetFQDNsByWebrootID", fctx.Webroot.ID).Get(ctx, &allFQDNs)
	if err != nil {
		_ = setResourceFailed(ctx, "fqdns", fqdnID, err)
		return err
	}
	var fqdnParams []activity.FQDNParam
	for _, f := range allFQDNs {
		var webrootID string
		if f.WebrootID != nil {
			webrootID = *f.WebrootID
		}
		fqdnParams = append(fqdnParams, activity.FQDNParam{
			FQDN:       f.FQDN,
			WebrootID:  webrootID,
			SSLEnabled: f.SSLEnabled,
		})
	}

	// Include service hostname as additional server_name if enabled.
	if fctx.Webroot.ServiceHostnameEnabled && fctx.BrandBaseHostname != "" {
		fqdnParams = append(fqdnParams, activity.FQDNParam{
			FQDN:      fmt.Sprintf("%s.%s.%s", fctx.Webroot.Name, fctx.Tenant.Name, fctx.BrandBaseHostname),
			WebrootID: fctx.Webroot.ID,
		})
	}

	bindErrs := fanOutNodes(ctx, fctx.Nodes, func(gCtx workflow.Context, node model.Node) error {
		nodeCtx := nodeActivityCtx(gCtx, node.ID)
		return workflow.ExecuteActivity(nodeCtx, "UpdateWebroot", activity.UpdateWebrootParams{
			ID:             fctx.Webroot.ID,
			TenantName:     fctx.Tenant.Name,
			Name:           fctx.Webroot.Name,
			Runtime:        fctx.Webroot.Runtime,
			RuntimeVersion: fctx.Webroot.RuntimeVersion,
			RuntimeConfig:  string(fctx.Webroot.RuntimeConfig),
			PublicFolder:   fctx.Webroot.PublicFolder,
			FQDNs:          fqdnParams,
		}).Get(gCtx, nil)
	})
	if len(bindErrs) > 0 {
		combinedErr := fmt.Errorf("bind fqdn errors: %s", joinErrors(bindErrs))
		_ = setResourceFailed(ctx, "fqdns", fqdnID, combinedErr)
		return combinedErr
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

	// Update the LB map entry on all LB nodes (parallel).
	lbErrs := fanOutNodes(ctx, fctx.LBNodes, func(gCtx workflow.Context, lbNode model.Node) error {
		lbCtx := nodeActivityCtx(gCtx, lbNode.ID)
		return workflow.ExecuteActivity(lbCtx, "SetLBMapEntry", activity.SetLBMapEntryParams{
			FQDN:      fctx.FQDN.FQDN,
			LBBackend: fctx.Shard.LBBackend,
		}).Get(gCtx, nil)
	})
	if len(lbErrs) > 0 {
		combinedErr := fmt.Errorf("set LB map errors: %s", joinErrors(lbErrs))
		_ = setResourceFailed(ctx, "fqdns", fqdnID, combinedErr)
		return combinedErr
	}

	// Set status to active.
	err = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "fqdns",
		ID:     fqdnID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Spawn pending child workflows in parallel.
	var children []ChildWorkflowSpec

	var emailAccounts []model.EmailAccount
	_ = workflow.ExecuteActivity(ctx, "ListEmailAccountsByFQDNID", fqdnID).Get(ctx, &emailAccounts)
	for _, ea := range emailAccounts {
		if ea.Status == model.StatusPending {
			children = append(children, ChildWorkflowSpec{WorkflowName: "CreateEmailAccountWorkflow", WorkflowID: fmt.Sprintf("create-email-account-%s", ea.ID), Arg: ea.ID})
		}
	}

	if errs := fanOutChildWorkflows(ctx, children); len(errs) > 0 {
		workflow.GetLogger(ctx).Warn("child workflow failures", "errors", joinErrors(errs))
	}
	return nil
}

// UnbindFQDNWorkflow removes an FQDN binding, cleaning up DNS records.
func UnbindFQDNWorkflow(ctx workflow.Context, fqdnID string) error {
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

	// Regenerate nginx config on all nodes to remove the FQDN from server_name.
	if fctx.Tenant.ShardID != nil {
		var remainingFQDNs []model.FQDN
		err = workflow.ExecuteActivity(ctx, "GetFQDNsByWebrootID", fctx.Webroot.ID).Get(ctx, &remainingFQDNs)
		if err != nil {
			_ = setResourceFailed(ctx, "fqdns", fqdnID, err)
			return err
		}
		var fqdnParams []activity.FQDNParam
		for _, f := range remainingFQDNs {
			// Skip the FQDN being unbound (it's still in the DB as "deleting").
			if f.ID == fqdnID {
				continue
			}
			var webrootID string
			if f.WebrootID != nil {
				webrootID = *f.WebrootID
			}
			fqdnParams = append(fqdnParams, activity.FQDNParam{
				FQDN:       f.FQDN,
				WebrootID:  webrootID,
				SSLEnabled: f.SSLEnabled,
			})
		}

		// Include service hostname as additional server_name if enabled.
		if fctx.Webroot.ServiceHostnameEnabled && fctx.BrandBaseHostname != "" {
			fqdnParams = append(fqdnParams, activity.FQDNParam{
				FQDN:      fmt.Sprintf("%s.%s.%s", fctx.Webroot.Name, fctx.Tenant.Name, fctx.BrandBaseHostname),
				WebrootID: fctx.Webroot.ID,
			})
		}

		unbindErrs := fanOutNodes(ctx, fctx.Nodes, func(gCtx workflow.Context, node model.Node) error {
			nodeCtx := nodeActivityCtx(gCtx, node.ID)
			return workflow.ExecuteActivity(nodeCtx, "UpdateWebroot", activity.UpdateWebrootParams{
				ID:             fctx.Webroot.ID,
				TenantName:     fctx.Tenant.Name,
				Name:           fctx.Webroot.Name,
				Runtime:        fctx.Webroot.Runtime,
				RuntimeVersion: fctx.Webroot.RuntimeVersion,
				RuntimeConfig:  string(fctx.Webroot.RuntimeConfig),
				PublicFolder:   fctx.Webroot.PublicFolder,
				FQDNs:          fqdnParams,
			}).Get(gCtx, nil)
		})
		if len(unbindErrs) > 0 {
			combinedErr := fmt.Errorf("unbind fqdn errors: %s", joinErrors(unbindErrs))
			_ = setResourceFailed(ctx, "fqdns", fqdnID, combinedErr)
			return combinedErr
		}
	}

	// Delete the LB map entry on all LB nodes (parallel).
	lbErrs := fanOutNodes(ctx, fctx.LBNodes, func(gCtx workflow.Context, lbNode model.Node) error {
		lbCtx := nodeActivityCtx(gCtx, lbNode.ID)
		return workflow.ExecuteActivity(lbCtx, "DeleteLBMapEntry", activity.DeleteLBMapEntryParams{
			FQDN: fctx.FQDN.FQDN,
		}).Get(gCtx, nil)
	})
	if len(lbErrs) > 0 {
		combinedErr := fmt.Errorf("delete LB map errors: %s", joinErrors(lbErrs))
		_ = setResourceFailed(ctx, "fqdns", fqdnID, combinedErr)
		return combinedErr
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "fqdns",
		ID:     fqdnID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
