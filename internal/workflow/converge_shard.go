package workflow

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/agent"
	"github.com/edvin/hosting/internal/core"
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
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
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

	// Fetch all desired state for the shard in a single batch query.
	var state activity.ShardDesiredState
	err := workflow.ExecuteActivity(ctx, "GetShardDesiredState", shardID).Get(ctx, &state)
	if err != nil {
		return []string{fmt.Sprintf("get shard desired state: %v", err)}
	}

	// Build expected nginx config + FPM pool sets and webroot entries from batch data.
	expectedConfigs := make(map[string]bool)
	expectedPools := make(map[string]bool)
	type webrootEntry struct {
		tenant  model.Tenant
		webroot model.Webroot
		fqdns   []activity.FQDNParam
	}
	var webrootEntries []webrootEntry

	var errs []string
	for _, tenant := range state.Tenants {
		if tenant.Status != model.StatusActive {
			continue
		}
		for _, webroot := range state.Webroots[tenant.ID] {
			confName := fmt.Sprintf("%s_%s.conf", tenant.Name, webroot.Name)
			expectedConfigs[confName] = true
			// FPM pool configs are per-tenant (not per-webroot).
			expectedPools[tenant.Name+".conf"] = true
			webrootEntries = append(webrootEntries, webrootEntry{
				tenant:  tenant,
				webroot: webroot,
				fqdns:   state.FQDNs[webroot.ID],
			})
		}
	}

	// Clean orphaned nginx configs and FPM pools on each node BEFORE creating webroots (parallel).
	cleanErrs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
		nodeCtx := nodeActivityCtx(gCtx, node.ID)

		var nginxResult activity.CleanOrphanedConfigsResult
		if err := workflow.ExecuteActivity(nodeCtx, "CleanOrphanedConfigs", activity.CleanOrphanedConfigsInput{
			ExpectedConfigs: expectedConfigs,
		}).Get(gCtx, &nginxResult); err != nil {
			return fmt.Errorf("clean orphaned nginx configs on node %s: %v", node.ID, err)
		}
		if len(nginxResult.Removed) > 0 {
			logger.Warn("removed orphaned nginx configs", "node", node.ID, "removed", nginxResult.Removed)
		}

		var fpmResult activity.CleanOrphanedFPMPoolsResult
		if err := workflow.ExecuteActivity(nodeCtx, "CleanOrphanedFPMPools", activity.CleanOrphanedFPMPoolsInput{
			ExpectedPools: expectedPools,
		}).Get(gCtx, &fpmResult); err != nil {
			return fmt.Errorf("clean orphaned fpm pools on node %s: %v", node.ID, err)
		}
		if len(fpmResult.Removed) > 0 {
			logger.Warn("removed orphaned PHP-FPM pools", "node", node.ID, "removed", fpmResult.Removed)
		}

		return nil
	})
	errs = append(errs, cleanErrs...)

	// Determine the cluster ID from the first node (all nodes in a shard share the same cluster).
	clusterID := ""
	if len(nodes) > 0 {
		clusterID = nodes[0].ClusterID
	}

	// Create tenants on each node (per-tenant, parallel across nodes).
	for _, tenant := range state.Tenants {
		if tenant.Status != model.StatusActive {
			continue
		}

		t := tenant // capture
		tenantErrs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
			nodeCtx := nodeActivityCtx(gCtx, node.ID)
			if err := workflow.ExecuteActivity(nodeCtx, "CreateTenant", activity.CreateTenantParams{
				ID:             t.ID,
				Name:           t.Name,
				UID:            t.UID,
				SFTPEnabled:    t.SFTPEnabled,
				SSHEnabled:     t.SSHEnabled,
				DiskQuotaBytes: t.DiskQuotaBytes,
			}).Get(gCtx, nil); err != nil {
				return fmt.Errorf("create tenant %s on node %s: %v", t.ID, node.ID, err)
			}

			// Sync SSH/SFTP config on the node.
			if err := workflow.ExecuteActivity(nodeCtx, "SyncSSHConfig", activity.SyncSSHConfigParams{
				TenantName:  t.Name,
				SSHEnabled:  t.SSHEnabled,
				SFTPEnabled: t.SFTPEnabled,
			}).Get(gCtx, nil); err != nil {
				return fmt.Errorf("sync ssh config for %s on node %s: %v", t.ID, node.ID, err)
			}
			return nil
		})
		errs = append(errs, tenantErrs...)
	}

	// Configure tenant ULA addresses on each node (parallel across nodes per tenant).
	for _, tenant := range state.Tenants {
		if tenant.Status != model.StatusActive {
			continue
		}
		t := tenant // capture
		ulaErrs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
			if node.ShardIndex == nil {
				return nil
			}
			nodeCtx := nodeActivityCtx(gCtx, node.ID)
			if err := workflow.ExecuteActivity(nodeCtx, "ConfigureTenantAddresses",
				activity.ConfigureTenantAddressesParams{
					TenantName:   t.Name,
					TenantUID:    t.UID,
					ClusterID:    clusterID,
					NodeShardIdx: *node.ShardIndex,
				}).Get(gCtx, nil); err != nil {
				return fmt.Errorf("configure ULA for %s on %s: %v", t.ID, node.ID, err)
			}
			return nil
		})
		errs = append(errs, ulaErrs...)
	}

	// Configure cross-node ULA routes on each node (parallel).
	routeErrs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
		if node.ShardIndex == nil {
			return nil
		}
		var otherIndices []int
		for _, other := range nodes {
			if other.ID != node.ID && other.ShardIndex != nil {
				otherIndices = append(otherIndices, *other.ShardIndex)
			}
		}
		if len(otherIndices) > 0 {
			nodeCtx := nodeActivityCtx(gCtx, node.ID)
			if err := workflow.ExecuteActivity(nodeCtx, "ConfigureULARoutes",
				activity.ConfigureULARoutesParams{
					ClusterID:        clusterID,
					ThisNodeIndex:    *node.ShardIndex,
					OtherNodeIndices: otherIndices,
				}).Get(gCtx, nil); err != nil {
				return fmt.Errorf("configure ULA routes on %s: %v", node.ID, err)
			}
		}
		return nil
	})
	errs = append(errs, routeErrs...)

	// Daemon data from batch query (keyed by webroot ID).
	webrootDaemons := state.Daemons

	// Build a node index map for ULA computation.
	nodeShardIndex := make(map[string]int) // node ID -> shard_index
	for _, node := range nodes {
		if node.ShardIndex != nil {
			nodeShardIndex[node.ID] = *node.ShardIndex
		}
	}

	// Create webroots on each node (parallel across nodes per webroot).
	for _, entry := range webrootEntries {
		// Build daemon proxy info for this webroot's nginx config.
		var daemonProxies []activity.DaemonProxyInfo
		for _, d := range webrootDaemons[entry.webroot.ID] {
			if d.ProxyPath != nil && d.ProxyPort != nil {
				targetIP := "127.0.0.1"
				if d.NodeID != nil {
					if idx, ok := nodeShardIndex[*d.NodeID]; ok {
						targetIP = core.ComputeTenantULA(clusterID, idx, entry.tenant.UID)
					}
				}
				daemonProxies = append(daemonProxies, activity.DaemonProxyInfo{
					ProxyPath: *d.ProxyPath,
					Port:      *d.ProxyPort,
					TargetIP:  targetIP,
					ProxyURL:  core.FormatDaemonProxyURL(targetIP, *d.ProxyPort),
				})
			}
		}

		e := entry // capture
		dp := daemonProxies
		wrErrs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
			nodeCtx := nodeActivityCtx(gCtx, node.ID)
			if err := workflow.ExecuteActivity(nodeCtx, "CreateWebroot", activity.CreateWebrootParams{
				ID:             e.webroot.ID,
				TenantName:     e.tenant.Name,
				Name:           e.webroot.Name,
				Runtime:        e.webroot.Runtime,
				RuntimeVersion: e.webroot.RuntimeVersion,
				RuntimeConfig:  string(e.webroot.RuntimeConfig),
				PublicFolder:   e.webroot.PublicFolder,
				EnvFileName:    e.webroot.EnvFileName,
				EnvShellSource: e.webroot.EnvShellSource,
				FQDNs:          e.fqdns,
				Daemons:        dp,
			}).Get(gCtx, nil); err != nil {
				return fmt.Errorf("create webroot %s on node %s: %v", e.webroot.ID, node.ID, err)
			}
			return nil
		})
		errs = append(errs, wrErrs...)
	}

	// Converge cron jobs for each webroot.
	for _, entry := range webrootEntries {
		cronJobs := state.CronJobs[entry.webroot.ID]
		for _, job := range cronJobs {
			if job.Status != model.StatusActive {
				continue
			}

			createParams := activity.CreateCronJobParams{
				ID:               job.ID,
				TenantName:       entry.tenant.Name,
				WebrootName:      entry.webroot.Name,
				Name:             job.Name,
				Schedule:         job.Schedule,
				Command:          job.Command,
				WorkingDirectory: job.WorkingDirectory,
				TimeoutSeconds:   job.TimeoutSeconds,
				MaxMemoryMB:      job.MaxMemoryMB,
			}

			// Write unit files on all nodes (parallel).
			j := job // capture
			cronErrs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
				nodeCtx := nodeActivityCtx(gCtx, node.ID)
				if err := workflow.ExecuteActivity(nodeCtx, "CreateCronJobUnits", createParams).Get(gCtx, nil); err != nil {
					return fmt.Errorf("create cron job %s on node %s: %v", j.ID, node.ID, err)
				}
				return nil
			})
			errs = append(errs, cronErrs...)

			// Enable timer on all nodes (parallel) — flock ensures single execution.
			if j.Enabled {
				timerParams := activity.CronJobTimerParams{
					ID:         j.ID,
					TenantName: entry.tenant.Name,
				}
				timerErrs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
					nodeCtx := nodeActivityCtx(gCtx, node.ID)
					if err := workflow.ExecuteActivity(nodeCtx, "EnableCronJobTimer", timerParams).Get(gCtx, nil); err != nil {
						return fmt.Errorf("enable cron timer %s on node %s: %v", j.ID, node.ID, err)
					}
					return nil
				})
				errs = append(errs, timerErrs...)
			}
		}
	}

	// Converge daemons for each webroot.
	for _, entry := range webrootEntries {
		daemons := webrootDaemons[entry.webroot.ID]
		for _, daemon := range daemons {
			if daemon.Status != model.StatusActive {
				continue
			}

			// Compute the tenant's ULA on the daemon's assigned node.
			hostIP := ""
			if daemon.NodeID != nil {
				if idx, ok := nodeShardIndex[*daemon.NodeID]; ok {
					hostIP = core.ComputeTenantULA(clusterID, idx, entry.tenant.UID)
				}
			}

			createParams := activity.CreateDaemonParams{
				ID:           daemon.ID,
				NodeID:       daemon.NodeID,
				TenantName:   entry.tenant.Name,
				WebrootName:  entry.webroot.Name,
				Name:         daemon.Name,
				Command:      daemon.Command,
				ProxyPort:    daemon.ProxyPort,
				HostIP:       hostIP,
				NumProcs:     daemon.NumProcs,
				StopSignal:   daemon.StopSignal,
				StopWaitSecs: daemon.StopWaitSecs,
				MaxMemoryMB:  daemon.MaxMemoryMB,
				Environment:  daemon.Environment,
			}

			// Write supervisord config on all nodes (parallel).
			d := daemon // capture
			daemonErrs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
				nodeCtx := nodeActivityCtx(gCtx, node.ID)
				if err := workflow.ExecuteActivity(nodeCtx, "CreateDaemonConfig", createParams).Get(gCtx, nil); err != nil {
					return fmt.Errorf("create daemon %s on node %s: %v", d.ID, node.ID, err)
				}
				return nil
			})
			errs = append(errs, daemonErrs...)

			// Disable daemon on all nodes if not enabled (parallel).
			if !d.Enabled {
				disableParams := activity.DaemonEnableParams{
					ID:          d.ID,
					TenantName:  entry.tenant.Name,
					WebrootName: entry.webroot.Name,
					Name:        d.Name,
				}
				disableErrs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
					nodeCtx := nodeActivityCtx(gCtx, node.ID)
					if err := workflow.ExecuteActivity(nodeCtx, "DisableDaemon", disableParams).Get(gCtx, nil); err != nil {
						return fmt.Errorf("disable daemon %s on node %s: %v", d.ID, node.ID, err)
					}
					return nil
				})
				errs = append(errs, disableErrs...)
			}
		}
	}

	// Reload nginx and PHP-FPM on all nodes (parallel).
	reloadErrs := fanOutNodes(ctx, nodes, func(gCtx workflow.Context, node model.Node) error {
		nodeCtx := nodeActivityCtx(gCtx, node.ID)
		if err := workflow.ExecuteActivity(nodeCtx, "ReloadNginx").Get(gCtx, nil); err != nil {
			return fmt.Errorf("reload nginx on node %s: %v", node.ID, err)
		}
		if err := workflow.ExecuteActivity(nodeCtx, "ReloadPHPFPM").Get(gCtx, nil); err != nil {
			return fmt.Errorf("reload php-fpm on node %s: %v", node.ID, err)
		}
		return nil
	})
	errs = append(errs, reloadErrs...)

	return errs
}

func convergeDatabaseShard(ctx workflow.Context, shardID string, nodes []model.Node) []string {
	// Determine primary node.
	primaryID, _, err := dbShardPrimary(ctx, shardID)
	if err != nil {
		return []string{fmt.Sprintf("determine primary: %v", err)}
	}

	var primary model.Node
	var replicas []model.Node
	for _, n := range nodes {
		if n.ID == primaryID {
			primary = n
		} else {
			replicas = append(replicas, n)
		}
	}

	var errs []string

	// Ensure primary is read-write.
	primaryCtx := nodeActivityCtx(ctx, primary.ID)
	err = workflow.ExecuteActivity(primaryCtx, "SetReadOnly", false).Get(ctx, nil)
	if err != nil {
		errs = append(errs, fmt.Sprintf("set primary read-write: %v", err))
	}

	// List all databases on this shard.
	var databases []model.Database
	err = workflow.ExecuteActivity(ctx, "ListDatabasesByShard", shardID).Get(ctx, &databases)
	if err != nil {
		return []string{fmt.Sprintf("list databases: %v", err)}
	}

	// Create databases and users on the PRIMARY ONLY.
	for _, database := range databases {
		if database.Status != model.StatusActive {
			continue
		}

		err = workflow.ExecuteActivity(primaryCtx, "CreateDatabase", database.Name).Get(ctx, nil)
		if err != nil {
			errs = append(errs, fmt.Sprintf("create database %s on primary: %v", database.ID, err))
		}

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
			err = workflow.ExecuteActivity(primaryCtx, "CreateDatabaseUser", activity.CreateDatabaseUserParams{
				DatabaseName: database.Name,
				Username:     user.Username,
				Password:     user.Password,
				Privileges:   user.Privileges,
			}).Get(ctx, nil)
			if err != nil {
				errs = append(errs, fmt.Sprintf("create db user %s on primary: %v", user.ID, err))
			}
		}
	}

	// Configure replication on each replica.
	for _, replica := range replicas {
		replicaCtx := nodeActivityCtx(ctx, replica.ID)

		// Set replica to read-only.
		err = workflow.ExecuteActivity(replicaCtx, "SetReadOnly", true).Get(ctx, nil)
		if err != nil {
			errs = append(errs, fmt.Sprintf("set replica %s read-only: %v", replica.ID, err))
		}

		// Check if replication is already running and healthy.
		var status agent.ReplicationStatus
		err = workflow.ExecuteActivity(replicaCtx, "GetReplicationStatus").Get(ctx, &status)
		if err == nil && status.IORunning && status.SQLRunning {
			continue // Replication healthy, nothing to do.
		}

		// Configure replication from primary.
		if primary.IPAddress != nil {
			err = workflow.ExecuteActivity(replicaCtx, "ConfigureReplication", activity.ConfigureReplicationParams{
				PrimaryHost:  *primary.IPAddress,
				ReplUser:     "repl",
				ReplPassword: "repl", // Default replication password for dev
			}).Get(ctx, nil)
			if err != nil {
				errs = append(errs, fmt.Sprintf("configure replication on %s: %v", replica.ID, err))
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
