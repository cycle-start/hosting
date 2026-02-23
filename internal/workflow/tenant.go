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
			Name:           tenant.ID,
			UID:            tenant.UID,
			SFTPEnabled:    tenant.SFTPEnabled,
			SSHEnabled:     tenant.SSHEnabled,
			DiskQuotaBytes: tenant.DiskQuotaBytes,
		}).Get(gCtx, nil); err != nil {
			return fmt.Errorf("node %s: create tenant: %v", node.ID, err)
		}

		// Sync SSH/SFTP config on the node.
		if err := workflow.ExecuteActivity(nodeCtx, "SyncSSHConfig", activity.SyncSSHConfigParams{
			TenantName:  tenant.ID,
			SSHEnabled:  tenant.SSHEnabled,
			SFTPEnabled: tenant.SFTPEnabled,
		}).Get(gCtx, nil); err != nil {
			return fmt.Errorf("node %s: sync ssh config: %v", node.ID, err)
		}

		// Configure tenant ULA addresses for daemon networking.
		if node.ShardIndex != nil {
			if err := workflow.ExecuteActivity(nodeCtx, "ConfigureTenantAddresses",
				activity.ConfigureTenantAddressesParams{
					TenantName:   tenant.ID,
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
	err = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "tenants",
		ID:     tenantID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Spawn pending child workflows in parallel.
	var children []ChildWorkflowSpec

	var webroots []model.Webroot
	_ = workflow.ExecuteActivity(ctx, "ListWebrootsByTenantID", tenantID).Get(ctx, &webroots)
	for _, w := range webroots {
		if w.Status == model.StatusPending {
			children = append(children, ChildWorkflowSpec{WorkflowName: "CreateWebrootWorkflow", WorkflowID: fmt.Sprintf("create-webroot-%s", w.ID), Arg: w.ID})
		}
	}

	var databases []model.Database
	_ = workflow.ExecuteActivity(ctx, "ListDatabasesByTenantID", tenantID).Get(ctx, &databases)
	for _, d := range databases {
		if d.Status == model.StatusPending {
			children = append(children, ChildWorkflowSpec{WorkflowName: "CreateDatabaseWorkflow", WorkflowID: fmt.Sprintf("create-database-%s", d.ID), Arg: d.ID})
		}
	}

	var zones []model.Zone
	_ = workflow.ExecuteActivity(ctx, "ListZonesByTenantID", tenantID).Get(ctx, &zones)
	for _, z := range zones {
		if z.Status == model.StatusPending {
			children = append(children, ChildWorkflowSpec{WorkflowName: "CreateZoneWorkflow", WorkflowID: fmt.Sprintf("create-zone-%s", z.ID), Arg: z.ID})
		}
	}

	var valkeyInstances []model.ValkeyInstance
	_ = workflow.ExecuteActivity(ctx, "ListValkeyInstancesByTenantID", tenantID).Get(ctx, &valkeyInstances)
	for _, vi := range valkeyInstances {
		if vi.Status == model.StatusPending {
			children = append(children, ChildWorkflowSpec{WorkflowName: "CreateValkeyInstanceWorkflow", WorkflowID: fmt.Sprintf("create-valkey-instance-%s", vi.ID), Arg: vi.ID})
		}
	}

	var s3Buckets []model.S3Bucket
	_ = workflow.ExecuteActivity(ctx, "ListS3BucketsByTenantID", tenantID).Get(ctx, &s3Buckets)
	for _, b := range s3Buckets {
		if b.Status == model.StatusPending {
			children = append(children, ChildWorkflowSpec{WorkflowName: "CreateS3BucketWorkflow", WorkflowID: fmt.Sprintf("create-s3-bucket-%s", b.ID), Arg: b.ID})
		}
	}

	var sshKeys []model.SSHKey
	_ = workflow.ExecuteActivity(ctx, "ListSSHKeysByTenantID", tenantID).Get(ctx, &sshKeys)
	for _, k := range sshKeys {
		if k.Status == model.StatusPending {
			children = append(children, ChildWorkflowSpec{WorkflowName: "AddSSHKeyWorkflow", WorkflowID: fmt.Sprintf("add-ssh-key-%s", k.ID), Arg: k.ID})
		}
	}

	var egressRules []model.TenantEgressRule
	_ = workflow.ExecuteActivity(ctx, "ListEgressRulesByTenantID", tenantID).Get(ctx, &egressRules)
	for _, er := range egressRules {
		if er.Status == model.StatusPending {
			children = append(children, ChildWorkflowSpec{WorkflowName: "SyncEgressRulesWorkflow", WorkflowID: fmt.Sprintf("sync-egress-%s", tenantID), Arg: tenantID})
			break // Only need ONE sync for all egress rules
		}
	}

	var backups []model.Backup
	_ = workflow.ExecuteActivity(ctx, "ListBackupsByTenantID", tenantID).Get(ctx, &backups)
	for _, b := range backups {
		if b.Status == model.StatusPending {
			children = append(children, ChildWorkflowSpec{WorkflowName: "CreateBackupWorkflow", WorkflowID: fmt.Sprintf("create-backup-%s", b.ID), Arg: b.ID})
		}
	}

	if errs := fanOutChildWorkflows(ctx, children); len(errs) > 0 {
		workflow.GetLogger(ctx).Warn("child workflow failures", "errors", joinErrors(errs))
	}
	return nil
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

	// Update tenant on each node in the shard (parallel).
	errs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
		nodeCtx := nodeActivityCtx(gCtx, node.ID)
		if err := workflow.ExecuteActivity(nodeCtx, "UpdateTenant", activity.UpdateTenantParams{
			ID:          tenant.ID,
			Name:        tenant.ID,
			UID:         tenant.UID,
			SFTPEnabled: tenant.SFTPEnabled,
			SSHEnabled:  tenant.SSHEnabled,
		}).Get(gCtx, nil); err != nil {
			return fmt.Errorf("node %s: update tenant: %v", node.ID, err)
		}

		// Sync SSH/SFTP config on the node.
		if err := workflow.ExecuteActivity(nodeCtx, "SyncSSHConfig", activity.SyncSSHConfigParams{
			TenantName:  tenant.ID,
			SSHEnabled:  tenant.SSHEnabled,
			SFTPEnabled: tenant.SFTPEnabled,
		}).Get(gCtx, nil); err != nil {
			return fmt.Errorf("node %s: sync ssh: %v", node.ID, err)
		}
		return nil
	})
	if len(errs) > 0 {
		combinedErr := fmt.Errorf("update tenant errors: %s", joinErrors(errs))
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

	// Suspend tenant on each node in the shard (parallel).
	errs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
		nodeCtx := nodeActivityCtx(gCtx, node.ID)
		if err := workflow.ExecuteActivity(nodeCtx, "SuspendTenant", tenant.ID).Get(gCtx, nil); err != nil {
			return fmt.Errorf("node %s: suspend tenant: %v", node.ID, err)
		}
		return nil
	})
	if len(errs) > 0 {
		combinedErr := fmt.Errorf("suspend tenant errors: %s", joinErrors(errs))
		_ = setResourceFailed(ctx, "tenants", tenantID, combinedErr)
		return combinedErr
	}

	// Set tenant status to suspended (already done by core service, but ensure consistency).
	_ = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "tenants",
		ID:     tenantID,
		Status: model.StatusSuspended,
	}).Get(ctx, nil)

	// Cascade suspend to all child resources in parallel.
	var webroots []model.Webroot
	var databases []model.Database
	var valkeyInstances []model.ValkeyInstance
	var s3Buckets []model.S3Bucket
	var zones []model.Zone

	_ = workflow.ExecuteActivity(ctx, "ListWebrootsByTenantID", tenantID).Get(ctx, &webroots)
	_ = workflow.ExecuteActivity(ctx, "ListDatabasesByTenantID", tenantID).Get(ctx, &databases)
	_ = workflow.ExecuteActivity(ctx, "ListValkeyInstancesByTenantID", tenantID).Get(ctx, &valkeyInstances)
	_ = workflow.ExecuteActivity(ctx, "ListS3BucketsByTenantID", tenantID).Get(ctx, &s3Buckets)
	_ = workflow.ExecuteActivity(ctx, "ListZonesByTenantID", tenantID).Get(ctx, &zones)

	wg := workflow.NewWaitGroup(ctx)
	suspendResource := func(table, id string) {
		wg.Add(1)
		workflow.Go(ctx, func(gCtx workflow.Context) {
			defer wg.Done()
			_ = workflow.ExecuteActivity(gCtx, "SuspendResource", activity.SuspendResourceParams{
				Table: table, ID: id, Reason: tenant.SuspendReason,
			}).Get(gCtx, nil)
		})
	}

	for _, wr := range webroots {
		if wr.Status == model.StatusActive {
			suspendResource("webroots", wr.ID)
		}
	}
	for _, db := range databases {
		if db.Status == model.StatusActive {
			suspendResource("databases", db.ID)
		}
	}
	for _, vi := range valkeyInstances {
		if vi.Status == model.StatusActive {
			suspendResource("valkey_instances", vi.ID)
		}
	}
	for _, b := range s3Buckets {
		if b.Status == model.StatusActive {
			suspendResource("s3_buckets", b.ID)
		}
	}
	for _, z := range zones {
		if z.Status == model.StatusActive {
			suspendResource("zones", z.ID)
		}
	}
	wg.Wait(ctx)

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

	// Unsuspend tenant on each node in the shard (parallel).
	errs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
		nodeCtx := nodeActivityCtx(gCtx, node.ID)
		if err := workflow.ExecuteActivity(nodeCtx, "UnsuspendTenant", tenant.ID).Get(gCtx, nil); err != nil {
			return fmt.Errorf("node %s: unsuspend tenant: %v", node.ID, err)
		}
		return nil
	})
	if len(errs) > 0 {
		combinedErr := fmt.Errorf("unsuspend tenant errors: %s", joinErrors(errs))
		_ = setResourceFailed(ctx, "tenants", tenantID, combinedErr)
		return combinedErr
	}

	// Set status to active and clear suspend reason.
	_ = workflow.ExecuteActivity(ctx, "UnsuspendResource", activity.SuspendResourceParams{
		Table: "tenants", ID: tenantID,
	}).Get(ctx, nil)

	// Cascade unsuspend to all child resources in parallel.
	var webroots []model.Webroot
	var databases []model.Database
	var valkeyInstances []model.ValkeyInstance
	var s3Buckets []model.S3Bucket
	var zones []model.Zone

	_ = workflow.ExecuteActivity(ctx, "ListWebrootsByTenantID", tenantID).Get(ctx, &webroots)
	_ = workflow.ExecuteActivity(ctx, "ListDatabasesByTenantID", tenantID).Get(ctx, &databases)
	_ = workflow.ExecuteActivity(ctx, "ListValkeyInstancesByTenantID", tenantID).Get(ctx, &valkeyInstances)
	_ = workflow.ExecuteActivity(ctx, "ListS3BucketsByTenantID", tenantID).Get(ctx, &s3Buckets)
	_ = workflow.ExecuteActivity(ctx, "ListZonesByTenantID", tenantID).Get(ctx, &zones)

	wg := workflow.NewWaitGroup(ctx)
	unsuspendResource := func(table, id string) {
		wg.Add(1)
		workflow.Go(ctx, func(gCtx workflow.Context) {
			defer wg.Done()
			_ = workflow.ExecuteActivity(gCtx, "UnsuspendResource", activity.SuspendResourceParams{
				Table: table, ID: id,
			}).Get(gCtx, nil)
		})
	}

	for _, wr := range webroots {
		if wr.Status == model.StatusSuspended {
			unsuspendResource("webroots", wr.ID)
		}
	}
	for _, db := range databases {
		if db.Status == model.StatusSuspended {
			unsuspendResource("databases", db.ID)
		}
	}
	for _, vi := range valkeyInstances {
		if vi.Status == model.StatusSuspended {
			unsuspendResource("valkey_instances", vi.ID)
		}
	}
	for _, b := range s3Buckets {
		if b.Status == model.StatusSuspended {
			unsuspendResource("s3_buckets", b.ID)
		}
	}
	for _, z := range zones {
		if z.Status == model.StatusSuspended {
			unsuspendResource("zones", z.ID)
		}
	}
	wg.Wait(ctx)

	return nil
}

// DeleteTenantWorkflow deletes a tenant and all its resources across all shards.
//
// Phase 1: Cross-shard resource cleanup — uses existing delete workflows for
// databases, valkey instances, S3 buckets, zones, and email accounts.
// Phase 2: Web-node cleanup — removes ULA addresses, SSH config, user account,
// bind mounts, CephFS directory, and log directory on each shard node.
// Phase 3: Shard convergence — triggers convergence to clean orphaned configs
// (nginx sites, PHP-FPM pools, supervisor daemons, cron systemd units).
// Phase 4: DB row cleanup — bulk-deletes all remaining child rows and the
// tenant itself in a single transaction.
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

	// ── Phase 1: Cross-shard resource cleanup ────────────────────────────
	// List all cross-shard resources and spawn delete workflows in parallel.
	// Errors are collected but non-fatal — Phase 4 catches leftovers.

	var databases []model.Database
	var valkeyInstances []model.ValkeyInstance
	var s3Buckets []model.S3Bucket
	var zones []model.Zone
	var emailAccounts []model.EmailAccount

	_ = workflow.ExecuteActivity(ctx, "ListDatabasesByTenantID", tenantID).Get(ctx, &databases)
	_ = workflow.ExecuteActivity(ctx, "ListValkeyInstancesByTenantID", tenantID).Get(ctx, &valkeyInstances)
	_ = workflow.ExecuteActivity(ctx, "ListS3BucketsByTenantID", tenantID).Get(ctx, &s3Buckets)
	_ = workflow.ExecuteActivity(ctx, "ListZonesByTenantID", tenantID).Get(ctx, &zones)
	_ = workflow.ExecuteActivity(ctx, "ListEmailAccountsByTenantID", tenantID).Get(ctx, &emailAccounts)

	var children []ChildWorkflowSpec
	for _, d := range databases {
		children = append(children, ChildWorkflowSpec{
			WorkflowName: "DeleteDatabaseWorkflow",
			WorkflowID:   fmt.Sprintf("delete-database-%s", d.ID),
			Arg:          d.ID,
		})
	}
	for _, vi := range valkeyInstances {
		children = append(children, ChildWorkflowSpec{
			WorkflowName: "DeleteValkeyInstanceWorkflow",
			WorkflowID:   fmt.Sprintf("delete-valkey-instance-%s", vi.ID),
			Arg:          vi.ID,
		})
	}
	for _, b := range s3Buckets {
		children = append(children, ChildWorkflowSpec{
			WorkflowName: "DeleteS3BucketWorkflow",
			WorkflowID:   fmt.Sprintf("delete-s3-bucket-%s", b.ID),
			Arg:          b.ID,
		})
	}
	for _, z := range zones {
		children = append(children, ChildWorkflowSpec{
			WorkflowName: "DeleteZoneWorkflow",
			WorkflowID:   fmt.Sprintf("delete-zone-%s", z.ID),
			Arg:          z.ID,
		})
	}
	for _, ea := range emailAccounts {
		children = append(children, ChildWorkflowSpec{
			WorkflowName: "DeleteEmailAccountWorkflow",
			WorkflowID:   fmt.Sprintf("delete-email-account-%s", ea.ID),
			Arg:          ea.ID,
		})
	}

	if errs := fanOutChildWorkflows(ctx, children); len(errs) > 0 {
		workflow.GetLogger(ctx).Warn("phase 1: cross-shard cleanup failures (non-fatal)", "errors", joinErrors(errs))
	}

	// ── Phase 2: Web-node cleanup ────────────────────────────────────────
	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *tenant.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	clusterID := ""
	if len(nodes) > 0 {
		clusterID = nodes[0].ClusterID
	}

	errs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
		nodeCtx := nodeActivityCtx(gCtx, node.ID)
		var nodeErrs []string

		if node.ShardIndex != nil {
			if err := workflow.ExecuteActivity(nodeCtx, "RemoveTenantAddresses",
				activity.ConfigureTenantAddressesParams{
					TenantName:   tenant.ID,
					TenantUID:    tenant.UID,
					ClusterID:    clusterID,
					NodeShardIdx: *node.ShardIndex,
				}).Get(gCtx, nil); err != nil {
				nodeErrs = append(nodeErrs, fmt.Sprintf("remove ULA: %v", err))
			}
		}

		if err := workflow.ExecuteActivity(nodeCtx, "RemoveSSHConfig", tenant.ID).Get(gCtx, nil); err != nil {
			nodeErrs = append(nodeErrs, fmt.Sprintf("remove SSH config: %v", err))
		}

		if err := workflow.ExecuteActivity(nodeCtx, "DeleteTenant", tenant.ID).Get(gCtx, nil); err != nil {
			nodeErrs = append(nodeErrs, fmt.Sprintf("delete tenant: %v", err))
		}

		if len(nodeErrs) > 0 {
			return fmt.Errorf("node %s: %s", node.ID, strings.Join(nodeErrs, "; "))
		}
		return nil
	})
	if len(errs) > 0 {
		combinedErr := fmt.Errorf("phase 2: node cleanup errors: %s", joinErrors(errs))
		_ = setResourceFailed(ctx, "tenants", tenantID, combinedErr)
		return combinedErr
	}

	// ── Phase 3: Shard convergence ───────────────────────────────────────
	convergeCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: fmt.Sprintf("converge-shard-%s-delete-tenant-%s", *tenant.ShardID, tenantID),
		TaskQueue:  "hosting-tasks",
	})
	if err := workflow.ExecuteChildWorkflow(convergeCtx, ConvergeShardWorkflow,
		ConvergeShardParams{ShardID: *tenant.ShardID}).Get(ctx, nil); err != nil {
		workflow.GetLogger(ctx).Warn("phase 3: shard convergence failed (non-fatal)", "error", err)
	}

	// ── Phase 4: DB row cleanup ──────────────────────────────────────────
	longCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    5,
			InitialInterval:    2 * time.Second,
			MaximumInterval:    30 * time.Second,
			BackoffCoefficient: 2.0,
		},
	})
	if err := workflow.ExecuteActivity(longCtx, "DeleteTenantDBRows", tenantID).Get(ctx, nil); err != nil {
		_ = setResourceFailed(ctx, "tenants", tenantID, err)
		return err
	}

	return nil
}
