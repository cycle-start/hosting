package workflow

import (
	"fmt"
	"strings"
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
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
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
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	// Look up all nodes in the tenant's shard.
	if tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", tenantID)
		_ = setResourceFailed(ctx, "tenants", tenantID, noShardErr)
		return noShardErr
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	// Determine cluster ID from nodes.
	clusterID := ""
	if len(nodes) > 0 {
		clusterID = nodes[0].ClusterID
	}

	// Create tenant on each node in the shard (parallel).
	errs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
		nodeCtx := nodeActivityCtx(gCtx, node.ID)
		if err := workflow.ExecuteActivity(nodeCtx, "CreateTenant", activity.CreateTenantParams{
			ID:             tenant.ID,
			Name:           tenant.Name,
			UID:            tenant.UID,
			SFTPEnabled:    tenant.SFTPEnabled,
			SSHEnabled:     tenant.SSHEnabled,
			DiskQuotaBytes: tenant.DiskQuotaBytes,
		}).Get(gCtx, nil); err != nil {
			return fmt.Errorf("node %s: create tenant: %v", node.ID, err)
		}

		// Sync SSH/SFTP config on the node.
		if err := workflow.ExecuteActivity(nodeCtx, "SyncSSHConfig", activity.SyncSSHConfigParams{
			TenantName:  tenant.Name,
			SSHEnabled:  tenant.SSHEnabled,
			SFTPEnabled: tenant.SFTPEnabled,
		}).Get(gCtx, nil); err != nil {
			return fmt.Errorf("node %s: sync ssh config: %v", node.ID, err)
		}

		// Configure tenant ULA addresses for daemon networking.
		if node.ShardIndex != nil {
			if err := workflow.ExecuteActivity(nodeCtx, "ConfigureTenantAddresses",
				activity.ConfigureTenantAddressesParams{
					TenantName:   tenant.Name,
					TenantUID:    tenant.UID,
					ClusterID:    clusterID,
					NodeShardIdx: *node.ShardIndex,
				}).Get(gCtx, nil); err != nil {
				return fmt.Errorf("node %s: configure ULA: %v", node.ID, err)
			}
		}
		return nil
	})
	if len(errs) > 0 {
		combinedErr := fmt.Errorf("create tenant errors: %s", joinErrors(errs))
		_ = setResourceFailed(ctx, "tenants", tenantID, combinedErr)
		return combinedErr
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
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
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
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	if tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", tenantID)
		_ = setResourceFailed(ctx, "tenants", tenantID, noShardErr)
		return noShardErr
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	// Update tenant on each node in the shard.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "UpdateTenant", activity.UpdateTenantParams{
			ID:          tenant.ID,
			Name:        tenant.Name,
			UID:         tenant.UID,
			SFTPEnabled: tenant.SFTPEnabled,
			SSHEnabled:  tenant.SSHEnabled,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "tenants", tenantID, err)
			return err
		}

		// Sync SSH/SFTP config on the node.
		err = workflow.ExecuteActivity(nodeCtx, "SyncSSHConfig", activity.SyncSSHConfigParams{
			TenantName:  tenant.Name,
			SSHEnabled:  tenant.SSHEnabled,
			SFTPEnabled: tenant.SFTPEnabled,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "tenants", tenantID, err)
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

// SuspendTenantWorkflow suspends a tenant on all nodes in the tenant's shard
// and cascades suspension to all child resources.
func SuspendTenantWorkflow(ctx workflow.Context, tenantID string) error {
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

	// Look up the tenant.
	var tenant model.Tenant
	err := workflow.ExecuteActivity(ctx, "GetTenantByID", tenantID).Get(ctx, &tenant)
	if err != nil {
		return err
	}

	if tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", tenantID)
		_ = setResourceFailed(ctx, "tenants", tenantID, noShardErr)
		return noShardErr
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	// Suspend tenant on each node in the shard.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "SuspendTenant", tenant.Name).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "tenants", tenantID, err)
			return err
		}
	}

	// Set tenant status to suspended (already done by core service, but ensure consistency).
	_ = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "tenants",
		ID:     tenantID,
		Status: model.StatusSuspended,
	}).Get(ctx, nil)

	// Cascade suspend to child webroots.
	var webroots []model.Webroot
	_ = workflow.ExecuteActivity(ctx, "ListWebrootsByTenantID", tenantID).Get(ctx, &webroots)
	for _, wr := range webroots {
		if wr.Status == model.StatusActive {
			_ = workflow.ExecuteActivity(ctx, "SuspendResource", activity.SuspendResourceParams{
				Table: "webroots", ID: wr.ID, Reason: tenant.SuspendReason,
			}).Get(ctx, nil)
		}
	}

	// Cascade suspend to child databases.
	var databases []model.Database
	_ = workflow.ExecuteActivity(ctx, "ListDatabasesByTenantID", tenantID).Get(ctx, &databases)
	for _, db := range databases {
		if db.Status == model.StatusActive {
			_ = workflow.ExecuteActivity(ctx, "SuspendResource", activity.SuspendResourceParams{
				Table: "databases", ID: db.ID, Reason: tenant.SuspendReason,
			}).Get(ctx, nil)
		}
	}

	// Cascade suspend to child valkey instances.
	var valkeyInstances []model.ValkeyInstance
	_ = workflow.ExecuteActivity(ctx, "ListValkeyInstancesByTenantID", tenantID).Get(ctx, &valkeyInstances)
	for _, vi := range valkeyInstances {
		if vi.Status == model.StatusActive {
			_ = workflow.ExecuteActivity(ctx, "SuspendResource", activity.SuspendResourceParams{
				Table: "valkey_instances", ID: vi.ID, Reason: tenant.SuspendReason,
			}).Get(ctx, nil)
		}
	}

	// Cascade suspend to child S3 buckets.
	var s3Buckets []model.S3Bucket
	_ = workflow.ExecuteActivity(ctx, "ListS3BucketsByTenantID", tenantID).Get(ctx, &s3Buckets)
	for _, b := range s3Buckets {
		if b.Status == model.StatusActive {
			_ = workflow.ExecuteActivity(ctx, "SuspendResource", activity.SuspendResourceParams{
				Table: "s3_buckets", ID: b.ID, Reason: tenant.SuspendReason,
			}).Get(ctx, nil)
		}
	}

	// Cascade suspend to child zones.
	var zones []model.Zone
	_ = workflow.ExecuteActivity(ctx, "ListZonesByTenantID", tenantID).Get(ctx, &zones)
	for _, z := range zones {
		if z.Status == model.StatusActive {
			_ = workflow.ExecuteActivity(ctx, "SuspendResource", activity.SuspendResourceParams{
				Table: "zones", ID: z.ID, Reason: tenant.SuspendReason,
			}).Get(ctx, nil)
		}
	}

	return nil
}

// UnsuspendTenantWorkflow unsuspends a tenant on all nodes in the tenant's shard
// and cascades unsuspension to all child resources.
func UnsuspendTenantWorkflow(ctx workflow.Context, tenantID string) error {
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
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	if tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", tenantID)
		_ = setResourceFailed(ctx, "tenants", tenantID, noShardErr)
		return noShardErr
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	// Unsuspend tenant on each node in the shard.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "UnsuspendTenant", tenant.Name).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "tenants", tenantID, err)
			return err
		}
	}

	// Set status to active and clear suspend reason.
	_ = workflow.ExecuteActivity(ctx, "UnsuspendResource", activity.SuspendResourceParams{
		Table: "tenants", ID: tenantID,
	}).Get(ctx, nil)

	// Cascade unsuspend to child webroots.
	var webroots []model.Webroot
	_ = workflow.ExecuteActivity(ctx, "ListWebrootsByTenantID", tenantID).Get(ctx, &webroots)
	for _, wr := range webroots {
		if wr.Status == model.StatusSuspended {
			_ = workflow.ExecuteActivity(ctx, "UnsuspendResource", activity.SuspendResourceParams{
				Table: "webroots", ID: wr.ID,
			}).Get(ctx, nil)
		}
	}

	// Cascade unsuspend to child databases.
	var databases []model.Database
	_ = workflow.ExecuteActivity(ctx, "ListDatabasesByTenantID", tenantID).Get(ctx, &databases)
	for _, db := range databases {
		if db.Status == model.StatusSuspended {
			_ = workflow.ExecuteActivity(ctx, "UnsuspendResource", activity.SuspendResourceParams{
				Table: "databases", ID: db.ID,
			}).Get(ctx, nil)
		}
	}

	// Cascade unsuspend to child valkey instances.
	var valkeyInstances []model.ValkeyInstance
	_ = workflow.ExecuteActivity(ctx, "ListValkeyInstancesByTenantID", tenantID).Get(ctx, &valkeyInstances)
	for _, vi := range valkeyInstances {
		if vi.Status == model.StatusSuspended {
			_ = workflow.ExecuteActivity(ctx, "UnsuspendResource", activity.SuspendResourceParams{
				Table: "valkey_instances", ID: vi.ID,
			}).Get(ctx, nil)
		}
	}

	// Cascade unsuspend to child S3 buckets.
	var s3Buckets []model.S3Bucket
	_ = workflow.ExecuteActivity(ctx, "ListS3BucketsByTenantID", tenantID).Get(ctx, &s3Buckets)
	for _, b := range s3Buckets {
		if b.Status == model.StatusSuspended {
			_ = workflow.ExecuteActivity(ctx, "UnsuspendResource", activity.SuspendResourceParams{
				Table: "s3_buckets", ID: b.ID,
			}).Get(ctx, nil)
		}
	}

	// Cascade unsuspend to child zones.
	var zones []model.Zone
	_ = workflow.ExecuteActivity(ctx, "ListZonesByTenantID", tenantID).Get(ctx, &zones)
	for _, z := range zones {
		if z.Status == model.StatusSuspended {
			_ = workflow.ExecuteActivity(ctx, "UnsuspendResource", activity.SuspendResourceParams{
				Table: "zones", ID: z.ID,
			}).Get(ctx, nil)
		}
	}

	return nil
}

// DeleteTenantWorkflow deletes a tenant from all nodes in the tenant's shard.
func DeleteTenantWorkflow(ctx workflow.Context, tenantID string) error {
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
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	if tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", tenantID)
		_ = setResourceFailed(ctx, "tenants", tenantID, noShardErr)
		return noShardErr
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	// Determine cluster ID from nodes.
	deleteClusterID := ""
	if len(nodes) > 0 {
		deleteClusterID = nodes[0].ClusterID
	}

	// Delete tenant on each node in the shard (parallel, continue-on-error).
	errs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
		nodeCtx := nodeActivityCtx(gCtx, node.ID)
		var nodeErrs []string

		// Remove tenant ULA addresses before deleting the tenant.
		if node.ShardIndex != nil {
			if err := workflow.ExecuteActivity(nodeCtx, "RemoveTenantAddresses",
				activity.ConfigureTenantAddressesParams{
					TenantName:   tenant.Name,
					TenantUID:    tenant.UID,
					ClusterID:    deleteClusterID,
					NodeShardIdx: *node.ShardIndex,
				}).Get(gCtx, nil); err != nil {
				nodeErrs = append(nodeErrs, fmt.Sprintf("remove ULA: %v", err))
			}
		}

		// Remove SSH config before deleting the tenant.
		if err := workflow.ExecuteActivity(nodeCtx, "RemoveSSHConfig", tenant.Name).Get(gCtx, nil); err != nil {
			nodeErrs = append(nodeErrs, fmt.Sprintf("remove SSH config: %v", err))
		}

		if err := workflow.ExecuteActivity(nodeCtx, "DeleteTenant", tenant.Name).Get(gCtx, nil); err != nil {
			nodeErrs = append(nodeErrs, fmt.Sprintf("delete tenant: %v", err))
		}

		if len(nodeErrs) > 0 {
			return fmt.Errorf("node %s: %s", node.ID, strings.Join(nodeErrs, "; "))
		}
		return nil
	})
	if len(errs) > 0 {
		combinedErr := fmt.Errorf("delete errors: %s", joinErrors(errs))
		_ = setResourceFailed(ctx, "tenants", tenantID, combinedErr)
		return combinedErr
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "tenants",
		ID:     tenantID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
