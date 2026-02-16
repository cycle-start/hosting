package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// SyncEgressRulesWorkflow syncs all tenant egress rules to nftables on every
// node in the tenant's web shard. It fetches the full rule set and applies it
// atomically, replacing any previous rules for that tenant UID.
func SyncEgressRulesWorkflow(ctx workflow.Context, tenantID string) error {
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

	// Set all pending/deleting rules for this tenant to provisioning.
	err := workflow.ExecuteActivity(ctx, "SetTenantEgressRulesProvisioning", tenantID).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Fetch tenant info (need UID and shard).
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", tenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "tenant_egress_rules", tenantID, err)
		return err
	}

	if tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", tenantID)
		_ = setResourceFailed(ctx, "tenant_egress_rules", tenantID, noShardErr)
		return noShardErr
	}

	// Get nodes in the shard.
	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "tenant_egress_rules", tenantID, err)
		return err
	}

	// Get all active egress rules for this tenant.
	var rules []model.TenantEgressRule
	err = workflow.ExecuteActivity(ctx, "GetActiveEgressRules", tenantID).Get(ctx, &rules)
	if err != nil {
		_ = setResourceFailed(ctx, "tenant_egress_rules", tenantID, err)
		return err
	}

	// Apply rules on each node.
	errs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
		nodeCtx := nodeActivityCtx(gCtx, node.ID)
		return workflow.ExecuteActivity(nodeCtx, "SyncEgressRules", activity.SyncEgressRulesParams{
			TenantUID: tenant.UID,
			Rules:     rules,
		}).Get(gCtx, nil)
	})

	if len(errs) > 0 {
		combinedErr := fmt.Errorf("sync egress rules errors: %s", joinErrors(errs))
		_ = setResourceFailed(ctx, "tenant_egress_rules", tenantID, combinedErr)
		return combinedErr
	}

	// Finalize rules: active ones stay active, deleting ones get hard-deleted.
	return workflow.ExecuteActivity(ctx, "FinalizeTenantEgressRules", tenantID).Get(ctx, nil)
}
