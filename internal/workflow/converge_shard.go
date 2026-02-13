package workflow

import (
	"fmt"
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

	// List all nodes in the shard.
	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", params.ShardID).Get(ctx, &nodes)
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	if len(nodes) == 0 {
		return fmt.Errorf("shard %s has no nodes", params.ShardID)
	}

	switch shard.Role {
	case model.ShardRoleWeb:
		return convergeWebShard(ctx, params.ShardID, nodes)
	case model.ShardRoleDatabase:
		return convergeDatabaseShard(ctx, params.ShardID, nodes)
	case model.ShardRoleValkey:
		return convergeValkeyShard(ctx, params.ShardID, nodes)
	case model.ShardRoleStorage:
		// Storage nodes (S3/CephFS) are configured at boot via cloud-init; no convergence needed.
		return nil
	case model.ShardRoleDBAdmin:
		// DB admin nodes are configured at boot via cloud-init; no convergence needed.
		return nil
	default:
		// DNS/email shards don't have convergence yet.
		return nil
	}
}

func convergeWebShard(ctx workflow.Context, shardID string, nodes []model.Node) error {
	// List all tenants on this shard.
	var tenants []model.Tenant
	err := workflow.ExecuteActivity(ctx, "ListTenantsByShard", shardID).Get(ctx, &tenants)
	if err != nil {
		return fmt.Errorf("list tenants: %w", err)
	}

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
				return fmt.Errorf("create tenant %s on node %s: %w", tenant.ID, node.ID, err)
			}

			// Sync SSH/SFTP config on the node.
			err = workflow.ExecuteActivity(nodeCtx, "SyncSSHConfig", activity.SyncSSHConfigParams{
				TenantName:  tenant.ID,
				SSHEnabled:  tenant.SSHEnabled,
				SFTPEnabled: tenant.SFTPEnabled,
			}).Get(ctx, nil)
			if err != nil {
				return fmt.Errorf("sync ssh config for %s on node %s: %w", tenant.ID, node.ID, err)
			}
		}

		// List webroots for this tenant.
		var webroots []model.Webroot
		err = workflow.ExecuteActivity(ctx, "ListWebrootsByTenantID", tenant.ID).Get(ctx, &webroots)
		if err != nil {
			return fmt.Errorf("list webroots for tenant %s: %w", tenant.ID, err)
		}

		for _, webroot := range webroots {
			if webroot.Status != model.StatusActive {
				continue
			}

			// Get FQDNs for this webroot.
			var fqdns []model.FQDN
			err = workflow.ExecuteActivity(ctx, "GetFQDNsByWebrootID", webroot.ID).Get(ctx, &fqdns)
			if err != nil {
				return fmt.Errorf("get fqdns for webroot %s: %w", webroot.ID, err)
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
					return fmt.Errorf("create webroot %s on node %s: %w", webroot.ID, node.ID, err)
				}
			}
		}
	}

	// Reload nginx on all nodes.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err := workflow.ExecuteActivity(nodeCtx, "ReloadNginx").Get(ctx, nil)
		if err != nil {
			return fmt.Errorf("reload nginx on node %s: %w", node.ID, err)
		}
	}

	return nil
}

func convergeDatabaseShard(ctx workflow.Context, shardID string, nodes []model.Node) error {
	// List all databases on this shard.
	var databases []model.Database
	err := workflow.ExecuteActivity(ctx, "ListDatabasesByShard", shardID).Get(ctx, &databases)
	if err != nil {
		return fmt.Errorf("list databases: %w", err)
	}

	for _, database := range databases {
		if database.Status != model.StatusActive {
			continue
		}

		// Create database on each node.
		for _, node := range nodes {
			nodeCtx := nodeActivityCtx(ctx, node.ID)
			err = workflow.ExecuteActivity(nodeCtx, "CreateDatabase", database.Name).Get(ctx, nil)
			if err != nil {
				return fmt.Errorf("create database %s on node %s: %w", database.ID, node.ID, err)
			}
		}

		// List users for this database.
		var users []model.DatabaseUser
		err = workflow.ExecuteActivity(ctx, "ListDatabaseUsersByDatabaseID", database.ID).Get(ctx, &users)
		if err != nil {
			return fmt.Errorf("list users for database %s: %w", database.ID, err)
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
					return fmt.Errorf("create db user %s on node %s: %w", user.ID, node.ID, err)
				}
			}
		}
	}

	return nil
}

func convergeValkeyShard(ctx workflow.Context, shardID string, nodes []model.Node) error {
	// List all valkey instances on this shard.
	var instances []model.ValkeyInstance
	err := workflow.ExecuteActivity(ctx, "ListValkeyInstancesByShard", shardID).Get(ctx, &instances)
	if err != nil {
		return fmt.Errorf("list valkey instances: %w", err)
	}

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
				return fmt.Errorf("create valkey instance %s on node %s: %w", instance.ID, node.ID, err)
			}
		}

		// List users for this instance.
		var users []model.ValkeyUser
		err = workflow.ExecuteActivity(ctx, "ListValkeyUsersByInstanceID", instance.ID).Get(ctx, &users)
		if err != nil {
			return fmt.Errorf("list users for valkey instance %s: %w", instance.ID, err)
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
					return fmt.Errorf("create valkey user %s on node %s: %w", user.ID, node.ID, err)
				}
			}
		}
	}

	return nil
}
