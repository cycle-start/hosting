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

// ConvergeShardParams holds parameters for the ConvergeShardWorkflow.
type ConvergeShardParams struct {
	ShardID string `json:"shard_id"`
}

// ConvergeShardWorkflow pushes all existing resources on a shard to every node.
// This is used after a new node joins a shard, or for manual convergence.
//
// The workflow sets the shard to "converging" at start, collects errors from
// per-node/per-resource operations without stopping, and sets the shard to
// "active" on full success or "failed" with an error summary on any failure.
func ConvergeShardWorkflow(ctx workflow.Context, params ConvergeShardParams) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Get the shard to determine its role.
	var shard model.Shard
	err := workflow.ExecuteActivity(ctx, "GetShardByID", params.ShardID).Get(ctx, &shard)
	if err != nil {
		return fmt.Errorf("get shard: %w", err)
	}

	// Set shard to converging.
	setShardStatus(ctx, params.ShardID, model.StatusConverging, nil)

	// List all nodes in the shard.
	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", params.ShardID).Get(ctx, &nodes)
	if err != nil {
		setShardStatus(ctx, params.ShardID, model.StatusFailed, strPtr(fmt.Sprintf("list nodes: %v", err)))
		return fmt.Errorf("list nodes: %w", err)
	}

	if len(nodes) == 0 {
		msg := fmt.Sprintf("shard %s has no nodes", params.ShardID)
		setShardStatus(ctx, params.ShardID, model.StatusFailed, &msg)
		return fmt.Errorf("%s", msg)
	}

	var errs []string
	switch shard.Role {
	case model.ShardRoleWeb:
		errs = convergeWebShard(ctx, params.ShardID, nodes)
	case model.ShardRoleDatabase:
		errs = convergeDatabaseShard(ctx, params.ShardID, nodes)
	case model.ShardRoleValkey:
		errs = convergeValkeyShard(ctx, params.ShardID, nodes)
	case model.ShardRoleLB:
		errs = convergeLBShard(ctx, shard, nodes)
	default:
		// Storage, DBAdmin, DNS, email — no convergence needed.
		setShardStatus(ctx, params.ShardID, model.StatusActive, nil)
		return nil
	}

	if len(errs) > 0 {
		msg := strings.Join(errs, "; ")
		if len(msg) > 4000 {
			msg = msg[:4000]
		}
		setShardStatus(ctx, params.ShardID, model.StatusFailed, &msg)
		return fmt.Errorf("convergence completed with %d errors: %s", len(errs), msg)
	}

	setShardStatus(ctx, params.ShardID, model.StatusActive, nil)
	return nil
}

// setShardStatus updates the shard's status and status_message via the UpdateResourceStatus activity.
func setShardStatus(ctx workflow.Context, shardID, status string, msg *string) {
	_ = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:         "shards",
		ID:            shardID,
		Status:        status,
		StatusMessage: msg,
	}).Get(ctx, nil)
}

func convergeLBShard(ctx workflow.Context, shard model.Shard, nodes []model.Node) []string {
	// Fetch all active FQDN-to-backend mappings for this cluster.
	var mappings []activity.FQDNMapping
	err := workflow.ExecuteActivity(ctx, "ListActiveFQDNMappings", shard.ClusterID).Get(ctx, &mappings)
	if err != nil {
		return []string{fmt.Sprintf("list active fqdn mappings: %v", err)}
	}

	var errs []string
	for _, m := range mappings {
		for _, node := range nodes {
			nodeCtx := nodeActivityCtx(ctx, node.ID)
			err = workflow.ExecuteActivity(nodeCtx, "SetLBMapEntry", activity.SetLBMapEntryParams{
				FQDN:      m.FQDN,
				LBBackend: m.LBBackend,
			}).Get(ctx, nil)
			if err != nil {
				errs = append(errs, fmt.Sprintf("set lb map %s on node %s: %v", m.FQDN, node.ID, err))
			}
		}
	}

	return errs
}

func convergeWebShard(ctx workflow.Context, shardID string, nodes []model.Node) []string {
	// List all tenants on this shard.
	var tenants []model.Tenant
	err := workflow.ExecuteActivity(ctx, "ListTenantsByShard", shardID).Get(ctx, &tenants)
	if err != nil {
		return []string{fmt.Sprintf("list tenants: %v", err)}
	}

	var errs []string
	for _, tenant := range tenants {
		if tenant.Status != model.StatusActive {
			continue
		}

		// Create tenant on each node.
		for _, node := range nodes {
			nodeCtx := nodeActivityCtx(ctx, node.ID)
			err = workflow.ExecuteActivity(nodeCtx, "CreateTenant", activity.CreateTenantParams{
				ID:          tenant.ID,
				UID:         tenant.UID,
				SFTPEnabled: tenant.SFTPEnabled,
				SSHEnabled:  tenant.SSHEnabled,
			}).Get(ctx, nil)
			if err != nil {
				errs = append(errs, fmt.Sprintf("create tenant %s on node %s: %v", tenant.ID, node.ID, err))
				continue
			}

			// Sync SSH/SFTP config on the node.
			err = workflow.ExecuteActivity(nodeCtx, "SyncSSHConfig", activity.SyncSSHConfigParams{
				TenantName:  tenant.ID,
				SSHEnabled:  tenant.SSHEnabled,
				SFTPEnabled: tenant.SFTPEnabled,
			}).Get(ctx, nil)
			if err != nil {
				errs = append(errs, fmt.Sprintf("sync ssh config for %s on node %s: %v", tenant.ID, node.ID, err))
			}
		}

		// List webroots for this tenant.
		var webroots []model.Webroot
		err = workflow.ExecuteActivity(ctx, "ListWebrootsByTenantID", tenant.ID).Get(ctx, &webroots)
		if err != nil {
			errs = append(errs, fmt.Sprintf("list webroots for tenant %s: %v", tenant.ID, err))
			continue
		}

		for _, webroot := range webroots {
			if webroot.Status != model.StatusActive {
				continue
			}

			// Get FQDNs for this webroot.
			var fqdns []model.FQDN
			err = workflow.ExecuteActivity(ctx, "GetFQDNsByWebrootID", webroot.ID).Get(ctx, &fqdns)
			if err != nil {
				errs = append(errs, fmt.Sprintf("get fqdns for webroot %s: %v", webroot.ID, err))
				continue
			}

			var fqdnParams []activity.FQDNParam
			for _, f := range fqdns {
				if f.Status != model.StatusActive {
					continue
				}
				fqdnParams = append(fqdnParams, activity.FQDNParam{
					FQDN:       f.FQDN,
					WebrootID:  f.WebrootID,
					SSLEnabled: f.SSLEnabled,
				})
			}

			// Create webroot on each node.
			for _, node := range nodes {
				nodeCtx := nodeActivityCtx(ctx, node.ID)
				err = workflow.ExecuteActivity(nodeCtx, "CreateWebroot", activity.CreateWebrootParams{
					ID:             webroot.ID,
					TenantName:     tenant.ID,
					Name:           webroot.Name,
					Runtime:        webroot.Runtime,
					RuntimeVersion: webroot.RuntimeVersion,
					RuntimeConfig:  string(webroot.RuntimeConfig),
					PublicFolder:   webroot.PublicFolder,
					FQDNs:          fqdnParams,
				}).Get(ctx, nil)
				if err != nil {
					errs = append(errs, fmt.Sprintf("create webroot %s on node %s: %v", webroot.ID, node.ID, err))
				}
			}
		}
	}

	// Reload nginx on all nodes.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err := workflow.ExecuteActivity(nodeCtx, "ReloadNginx").Get(ctx, nil)
		if err != nil {
			errs = append(errs, fmt.Sprintf("reload nginx on node %s: %v", node.ID, err))
		}
	}

	return errs
}

func convergeDatabaseShard(ctx workflow.Context, shardID string, nodes []model.Node) []string {
	// List all databases on this shard.
	var databases []model.Database
	err := workflow.ExecuteActivity(ctx, "ListDatabasesByShard", shardID).Get(ctx, &databases)
	if err != nil {
		return []string{fmt.Sprintf("list databases: %v", err)}
	}

	var errs []string
	for _, database := range databases {
		if database.Status != model.StatusActive {
			continue
		}

		// Create database on each node.
		for _, node := range nodes {
			nodeCtx := nodeActivityCtx(ctx, node.ID)
			err = workflow.ExecuteActivity(nodeCtx, "CreateDatabase", database.Name).Get(ctx, nil)
			if err != nil {
				errs = append(errs, fmt.Sprintf("create database %s on node %s: %v", database.ID, node.ID, err))
			}
		}

		// List users for this database.
		var users []model.DatabaseUser
		err = workflow.ExecuteActivity(ctx, "ListDatabaseUsersByDatabaseID", database.ID).Get(ctx, &users)
		if err != nil {
			errs = append(errs, fmt.Sprintf("list users for database %s: %v", database.ID, err))
			continue
		}

		for _, user := range users {
			if user.Status != model.StatusActive {
				continue
			}

			// Create user on each node.
			for _, node := range nodes {
				nodeCtx := nodeActivityCtx(ctx, node.ID)
				err = workflow.ExecuteActivity(nodeCtx, "CreateDatabaseUser", activity.CreateDatabaseUserParams{
					DatabaseName: database.Name,
					Username:     user.Username,
					Password:     user.Password,
					Privileges:   user.Privileges,
				}).Get(ctx, nil)
				if err != nil {
					errs = append(errs, fmt.Sprintf("create db user %s on node %s: %v", user.ID, node.ID, err))
				}
			}
		}
	}

	return errs
}

func convergeValkeyShard(ctx workflow.Context, shardID string, nodes []model.Node) []string {
	// List all valkey instances on this shard.
	var instances []model.ValkeyInstance
	err := workflow.ExecuteActivity(ctx, "ListValkeyInstancesByShard", shardID).Get(ctx, &instances)
	if err != nil {
		return []string{fmt.Sprintf("list valkey instances: %v", err)}
	}

	var errs []string
	for _, instance := range instances {
		if instance.Status != model.StatusActive {
			continue
		}

		// Create instance on each node.
		for _, node := range nodes {
			nodeCtx := nodeActivityCtx(ctx, node.ID)
			err = workflow.ExecuteActivity(nodeCtx, "CreateValkeyInstance", activity.CreateValkeyInstanceParams{
				Name:        instance.Name,
				Port:        instance.Port,
				Password:    instance.Password,
				MaxMemoryMB: instance.MaxMemoryMB,
			}).Get(ctx, nil)
			if err != nil {
				errs = append(errs, fmt.Sprintf("create valkey instance %s on node %s: %v", instance.ID, node.ID, err))
			}
		}

		// List users for this instance.
		var users []model.ValkeyUser
		err = workflow.ExecuteActivity(ctx, "ListValkeyUsersByInstanceID", instance.ID).Get(ctx, &users)
		if err != nil {
			errs = append(errs, fmt.Sprintf("list users for valkey instance %s: %v", instance.ID, err))
			continue
		}

		for _, user := range users {
			if user.Status != model.StatusActive {
				continue
			}

			// Create user on each node.
			for _, node := range nodes {
				nodeCtx := nodeActivityCtx(ctx, node.ID)
				err = workflow.ExecuteActivity(nodeCtx, "CreateValkeyUser", activity.CreateValkeyUserParams{
					InstanceName: instance.Name,
					Port:         instance.Port,
					Username:     user.Username,
					Password:     user.Password,
					Privileges:   user.Privileges,
					KeyPattern:   user.KeyPattern,
				}).Get(ctx, nil)
				if err != nil {
					errs = append(errs, fmt.Sprintf("create valkey user %s on node %s: %v", user.ID, node.ID, err))
				}
			}
		}
	}

	return errs
}

func strPtr(s string) *string { return &s }
