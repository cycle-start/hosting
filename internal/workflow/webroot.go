package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateWebrootWorkflow provisions a new webroot on the node agent.
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

	// Look up the webroot.
	var webroot model.Webroot
	err = workflow.ExecuteActivity(ctx, "GetWebrootByID", webrootID).Get(ctx, &webroot)
	if err != nil {
		_ = setResourceFailed(ctx, "webroots", webrootID)
		return err
	}

	// Look up the tenant for the webroot.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", webroot.TenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "webroots", webrootID)
		return err
	}

	// Look up FQDNs for this webroot.
	var fqdns []model.FQDN
	err = workflow.ExecuteActivity(ctx, "GetFQDNsByWebrootID", webrootID).Get(ctx, &fqdns)
	if err != nil {
		_ = setResourceFailed(ctx, "webroots", webrootID)
		return err
	}

	fqdnParams := make([]activity.FQDNParam, len(fqdns))
	for i, f := range fqdns {
		fqdnParams[i] = activity.FQDNParam{
			FQDN:       f.FQDN,
			WebrootID:  f.WebrootID,
			SSLEnabled: f.SSLEnabled,
		}
	}

	// Look up nodes in the tenant's shard to route gRPC calls.
	if tenant.ShardID == nil {
		_ = setResourceFailed(ctx, "webroots", webrootID)
		return fmt.Errorf("tenant %s has no shard assigned", webroot.TenantID)
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "webroots", webrootID)
		return err
	}

	// Create webroot on each node in the shard.
	for _, node := range nodes {
		if node.GRPCAddress == "" {
			continue
		}
		err = workflow.ExecuteActivity(ctx, "CreateWebrootOnNode", activity.CreateWebrootOnNodeParams{
			NodeAddress: node.GRPCAddress,
			Webroot: activity.CreateWebrootParams{
				ID:             webroot.ID,
				TenantName:     tenant.Name,
				Name:           webroot.Name,
				Runtime:        webroot.Runtime,
				RuntimeVersion: webroot.RuntimeVersion,
				RuntimeConfig:  string(webroot.RuntimeConfig),
				PublicFolder:   webroot.PublicFolder,
				FQDNs:          fqdnParams,
			},
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "webroots", webrootID)
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

// UpdateWebrootWorkflow updates a webroot on the node agent.
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

	// Look up the webroot.
	var webroot model.Webroot
	err = workflow.ExecuteActivity(ctx, "GetWebrootByID", webrootID).Get(ctx, &webroot)
	if err != nil {
		_ = setResourceFailed(ctx, "webroots", webrootID)
		return err
	}

	// Look up the tenant for the webroot.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", webroot.TenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "webroots", webrootID)
		return err
	}

	// Look up FQDNs for this webroot.
	var fqdns []model.FQDN
	err = workflow.ExecuteActivity(ctx, "GetFQDNsByWebrootID", webrootID).Get(ctx, &fqdns)
	if err != nil {
		_ = setResourceFailed(ctx, "webroots", webrootID)
		return err
	}

	fqdnParams := make([]activity.FQDNParam, len(fqdns))
	for i, f := range fqdns {
		fqdnParams[i] = activity.FQDNParam{
			FQDN:       f.FQDN,
			WebrootID:  f.WebrootID,
			SSLEnabled: f.SSLEnabled,
		}
	}

	// Update webroot on node agent.
	err = workflow.ExecuteActivity(ctx, "UpdateWebroot", activity.UpdateWebrootParams{
		ID:             webroot.ID,
		TenantName:     tenant.Name,
		Name:           webroot.Name,
		Runtime:        webroot.Runtime,
		RuntimeVersion: webroot.RuntimeVersion,
		RuntimeConfig:  string(webroot.RuntimeConfig),
		PublicFolder:   webroot.PublicFolder,
		FQDNs:          fqdnParams,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "webroots", webrootID)
		return err
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "webroots",
		ID:     webrootID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteWebrootWorkflow deletes a webroot from the node agent.
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

	// Look up the webroot.
	var webroot model.Webroot
	err = workflow.ExecuteActivity(ctx, "GetWebrootByID", webrootID).Get(ctx, &webroot)
	if err != nil {
		_ = setResourceFailed(ctx, "webroots", webrootID)
		return err
	}

	// Look up the tenant for the webroot.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", webroot.TenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "webroots", webrootID)
		return err
	}

	// Delete webroot on node agent.
	err = workflow.ExecuteActivity(ctx, "DeleteWebroot", tenant.Name, webroot.Name).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "webroots", webrootID)
		return err
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "webroots",
		ID:     webrootID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
