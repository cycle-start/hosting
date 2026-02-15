package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
)

// MigrateTenantWorkflow moves a tenant from its current shard to a target shard
// within the same cluster.
func MigrateTenantWorkflow(ctx workflow.Context, params core.MigrateTenantParams) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	tenantID := params.TenantID

	// Set tenant status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "tenants",
		ID:     tenantID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Get the tenant.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", tenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	if tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no current shard assignment", tenantID)
		_ = setResourceFailed(ctx, "tenants", tenantID, noShardErr)
		return noShardErr
	}

	// Get source and target shards.
	var sourceShard model.Shard
	err = workflow.ExecuteActivity(ctx, "GetShardByID", *tenant.ShardID).Get(ctx, &sourceShard)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	var targetShard model.Shard
	err = workflow.ExecuteActivity(ctx, "GetShardByID", params.TargetShardID).Get(ctx, &targetShard)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	// Validate: same cluster.
	if sourceShard.ClusterID != targetShard.ClusterID {
		clusterErr := fmt.Errorf("source shard cluster %s != target shard cluster %s", sourceShard.ClusterID, targetShard.ClusterID)
		_ = setResourceFailed(ctx, "tenants", tenantID, clusterErr)
		return clusterErr
	}

	// Validate: target shard is a web shard.
	if targetShard.Role != model.ShardRoleWeb {
		roleErr := fmt.Errorf("target shard %s is not a web shard (role: %s)", targetShard.ID, targetShard.Role)
		_ = setResourceFailed(ctx, "tenants", tenantID, roleErr)
		return roleErr
	}

	// Get target shard nodes.
	var targetNodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", params.TargetShardID).Get(ctx, &targetNodes)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	// Provision tenant on each target node.
	for _, node := range targetNodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "CreateTenant", activity.CreateTenantParams{
			ID:             tenant.ID,
			Name:           tenant.Name,
			UID:            tenant.UID,
			SFTPEnabled:    tenant.SFTPEnabled,
			SSHEnabled:     tenant.SSHEnabled,
			DiskQuotaBytes: tenant.DiskQuotaBytes,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "tenants", tenantID, err)
			return fmt.Errorf("create tenant on node %s: %w", node.ID, err)
		}
	}

	// Provision webroots on target nodes.
	var webroots []model.Webroot
	err = workflow.ExecuteActivity(ctx, "ListWebrootsByTenantID", tenantID).Get(ctx, &webroots)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	for _, webroot := range webroots {
		// Get FQDNs for the webroot.
		var fqdns []model.FQDN
		err = workflow.ExecuteActivity(ctx, "GetFQDNsByWebrootID", webroot.ID).Get(ctx, &fqdns)
		if err != nil {
			_ = setResourceFailed(ctx, "tenants", tenantID, err)
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

		for _, node := range targetNodes {
			nodeCtx := nodeActivityCtx(ctx, node.ID)
			err = workflow.ExecuteActivity(nodeCtx, "CreateWebroot", activity.CreateWebrootParams{
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
				_ = setResourceFailed(ctx, "tenants", tenantID, err)
				return fmt.Errorf("create webroot %s on node %s: %w", webroot.Name, node.ID, err)
			}
		}
	}

	// Update LB map entries if requested.
	if params.MigrateFQDNs {
		for _, webroot := range webroots {
			var fqdns []model.FQDN
			_ = workflow.ExecuteActivity(ctx, "GetFQDNsByWebrootID", webroot.ID).Get(ctx, &fqdns)

			for _, fqdn := range fqdns {
				err = workflow.ExecuteActivity(ctx, "SetLBMapEntry", activity.SetLBMapEntryParams{
					FQDN:      fqdn.FQDN,
					LBBackend: targetShard.LBBackend,
				}).Get(ctx, nil)
				if err != nil {
					_ = setResourceFailed(ctx, "tenants", tenantID, err)
					return fmt.Errorf("update LB map for %s: %w", fqdn.FQDN, err)
				}
			}
		}
	}

	// Update tenant shard assignment in core DB.
	err = workflow.ExecuteActivity(ctx, "UpdateTenantShardID", tenantID, params.TargetShardID).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	// Cleanup: remove tenant from source shard nodes.
	var sourceNodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &sourceNodes)
	if err != nil {
		// Non-fatal: tenant is already migrated, just log cleanup failure.
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	for _, node := range sourceNodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		// Delete webroots from source nodes.
		for _, webroot := range webroots {
			_ = workflow.ExecuteActivity(nodeCtx, "DeleteWebroot", tenant.Name, webroot.Name).Get(ctx, nil)
		}
		// Delete tenant from source nodes.
		_ = workflow.ExecuteActivity(nodeCtx, "DeleteTenant", tenant.Name).Get(ctx, nil)
	}

	// Set tenant status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "tenants",
		ID:     tenantID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}
