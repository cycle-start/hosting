package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateTenantWorkflow provisions a new tenant on all nodes in the tenant's shard.
func CreateTenantWorkflow(ctx workflow.Context, tenantID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "tenants",
		ID:     tenantID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the tenant.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", tenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID)
		return err
	}

	// Look up all nodes in the tenant's shard.
	if tenant.ShardID == nil {
		_ = setResourceFailed(ctx, "tenants", tenantID)
		return fmt.Errorf("tenant %s has no shard assigned", tenantID)
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID)
		return err
	}

	// Create tenant on each node in the shard.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "CreateTenant", activity.CreateTenantParams{
			ID:          tenant.ID,
			UID:         tenant.UID,
			SFTPEnabled: tenant.SFTPEnabled,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "tenants", tenantID)
			return err
		}
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "tenants",
		ID:     tenantID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// UpdateTenantWorkflow updates a tenant on all nodes in the tenant's shard.
func UpdateTenantWorkflow(ctx workflow.Context, tenantID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "tenants",
		ID:     tenantID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the tenant.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", tenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID)
		return err
	}

	if tenant.ShardID == nil {
		_ = setResourceFailed(ctx, "tenants", tenantID)
		return fmt.Errorf("tenant %s has no shard assigned", tenantID)
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID)
		return err
	}

	// Update tenant on each node in the shard.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "UpdateTenant", activity.UpdateTenantParams{
			ID:          tenant.ID,
			UID:         tenant.UID,
			SFTPEnabled: tenant.SFTPEnabled,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "tenants", tenantID)
			return err
		}
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "tenants",
		ID:     tenantID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// SuspendTenantWorkflow suspends a tenant on all nodes in the tenant's shard.
func SuspendTenantWorkflow(ctx workflow.Context, tenantID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Look up the tenant.
	var tenant model.Tenant
	err := workflow.ExecuteActivity(ctx, "GetTenantByID", tenantID).Get(ctx, &tenant)
	if err != nil {
		return err
	}

	if tenant.ShardID == nil {
		_ = setResourceFailed(ctx, "tenants", tenantID)
		return fmt.Errorf("tenant %s has no shard assigned", tenantID)
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID)
		return err
	}

	// Suspend tenant on each node in the shard.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "SuspendTenant", tenant.ID).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "tenants", tenantID)
			return err
		}
	}

	// Set status to suspended.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "tenants",
		ID:     tenantID,
		Status: model.StatusSuspended,
	}).Get(ctx, nil)
}

// UnsuspendTenantWorkflow unsuspends a tenant on all nodes in the tenant's shard.
func UnsuspendTenantWorkflow(ctx workflow.Context, tenantID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "tenants",
		ID:     tenantID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the tenant.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", tenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID)
		return err
	}

	if tenant.ShardID == nil {
		_ = setResourceFailed(ctx, "tenants", tenantID)
		return fmt.Errorf("tenant %s has no shard assigned", tenantID)
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID)
		return err
	}

	// Unsuspend tenant on each node in the shard.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "UnsuspendTenant", tenant.ID).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "tenants", tenantID)
			return err
		}
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "tenants",
		ID:     tenantID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteTenantWorkflow deletes a tenant from all nodes in the tenant's shard.
func DeleteTenantWorkflow(ctx workflow.Context, tenantID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to deleting.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "tenants",
		ID:     tenantID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the tenant.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", tenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID)
		return err
	}

	if tenant.ShardID == nil {
		_ = setResourceFailed(ctx, "tenants", tenantID)
		return fmt.Errorf("tenant %s has no shard assigned", tenantID)
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID)
		return err
	}

	// Delete tenant on each node in the shard.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "DeleteTenant", tenant.ID).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "tenants", tenantID)
			return err
		}
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "tenants",
		ID:     tenantID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
