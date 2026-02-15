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
	logger := workflow.GetLogger(ctx)

	// List all tenants on this shard.
	var tenants []model.Tenant
	err := workflow.ExecuteActivity(ctx, "ListTenantsByShard", shardID).Get(ctx, &tenants)
	if err != nil {
		return []string{fmt.Sprintf("list tenants: %v", err)}
	}

	// Build the expected nginx config set from all active webroots across all
	// active tenants. This is used to clean orphaned configs before and after
	// creating webroots, preventing stale configs from blocking nginx reloads.
	expectedConfigs := make(map[string]bool)
	// We also collect webroot data upfront to avoid fetching it twice.
	type webrootEntry struct {
		tenant  model.Tenant
		webroot model.Webroot
		fqdns   []activity.FQDNParam
	}
	var webrootEntries []webrootEntry

	var errs []string
	for _, tenant := range tenants {
		if tenant.Status != model.StatusActive {
			continue
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

			// Register this webroot's config as expected.
			confName := fmt.Sprintf("%s_%s.conf", tenant.ID, webroot.Name)
			expectedConfigs[confName] = true

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

			webrootEntries = append(webrootEntries, webrootEntry{
				tenant:  tenant,
				webroot: webroot,
				fqdns:   fqdnParams,
			})
		}
	}

	// Clean orphaned nginx configs on each node BEFORE creating webroots.
	// This prevents stale configs from causing nginx -t failures that block
	// all new webroot provisioning.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		var result activity.CleanOrphanedConfigsResult
		err = workflow.ExecuteActivity(nodeCtx, "CleanOrphanedConfigs", activity.CleanOrphanedConfigsInput{
			ExpectedConfigs: expectedConfigs,
		}).Get(ctx, &result)
		if err != nil {
			errs = append(errs, fmt.Sprintf("clean orphaned configs on node %s: %v", node.ID, err))
		} else if len(result.Removed) > 0 {
			logger.Warn("removed orphaned nginx configs", "node", node.ID, "removed", result.Removed)
		}
	}

	// Create tenants and webroots on each node.
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
	}

	// Create webroots on each node.
	for _, entry := range webrootEntries {
		for _, node := range nodes {
			nodeCtx := nodeActivityCtx(ctx, node.ID)
			err = workflow.ExecuteActivity(nodeCtx, "CreateWebroot", activity.CreateWebrootParams{
				ID:             entry.webroot.ID,
				TenantName:     entry.tenant.ID,
				Name:           entry.webroot.Name,
				Runtime:        entry.webroot.Runtime,
				RuntimeVersion: entry.webroot.RuntimeVersion,
				RuntimeConfig:  string(entry.webroot.RuntimeConfig),
				PublicFolder:   entry.webroot.PublicFolder,
				FQDNs:          entry.fqdns,
			}).Get(ctx, nil)
			if err != nil {
				errs = append(errs, fmt.Sprintf("create webroot %s on node %s: %v", entry.webroot.ID, node.ID, err))
			}
		}
	}

	// Converge cron jobs for each webroot.
	for _, entry := range webrootEntries {
		var cronJobs []model.CronJob
		err = workflow.ExecuteActivity(ctx, "ListCronJobsByWebroot", entry.webroot.ID).Get(ctx, &cronJobs)
		if err != nil {
			errs = append(errs, fmt.Sprintf("list cron jobs for webroot %s: %v", entry.webroot.ID, err))
			continue
		}

		for _, job := range cronJobs {
			if job.Status != model.StatusActive {
				continue
			}

			createParams := activity.CreateCronJobParams{
				ID:               job.ID,
				TenantID:         entry.tenant.ID,
				WebrootName:      entry.webroot.Name,
				Name:             job.Name,
				Schedule:         job.Schedule,
				Command:          job.Command,
				WorkingDirectory: job.WorkingDirectory,
				TimeoutSeconds:   job.TimeoutSeconds,
				MaxMemoryMB:      job.MaxMemoryMB,
			}

			// Write unit files on all nodes.
			for _, node := range nodes {
				nodeCtx := nodeActivityCtx(ctx, node.ID)
				err = workflow.ExecuteActivity(nodeCtx, "CreateCronJobUnits", createParams).Get(ctx, nil)
				if err != nil {
					errs = append(errs, fmt.Sprintf("create cron job %s on node %s: %v", job.ID, node.ID, err))
				}
			}

			// Enable timer on designated node if the job is enabled.
			if job.Enabled {
				designated := designatedNode(job.ID, nodes)
				if designated != "" {
					timerParams := activity.CronJobTimerParams{
						ID:       job.ID,
						TenantID: entry.tenant.ID,
					}
					nodeCtx := nodeActivityCtx(ctx, designated)
					err = workflow.ExecuteActivity(nodeCtx, "EnableCronJobTimer", timerParams).Get(ctx, nil)
					if err != nil {
						errs = append(errs, fmt.Sprintf("enable cron timer %s on node %s: %v", job.ID, designated, err))
					}
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
