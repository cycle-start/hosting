# Shard Convergence

Shard convergence is the process of pushing all existing resources on a shard to every node in that shard. It ensures that when a new node joins a shard (or after a manual trigger), the node receives the complete set of resources it needs to serve traffic.

## When Convergence Runs

- **New node joins a shard** -- after adding a node, convergence ensures it receives all tenants, webroots, databases, etc.
- **Manual trigger** -- operators can start convergence via the API to re-sync a shard after maintenance or drift.

## How It Works

The `ConvergeShardWorkflow` is a Temporal workflow that:

1. Fetches the shard to determine its role.
2. Sets shard status to `converging`.
3. Lists all nodes in the shard.
4. Delegates to a role-specific convergence function.
5. On full success, sets shard status to `active`. On any failure, sets `failed` with an error summary.

Activities use a 30-second `StartToCloseTimeout` with up to 3 retry attempts.

## Role-Specific Convergence

### Web Shards

Web convergence is the most complex, handling tenants, webroots, FQDNs, SSH/SFTP config, and nginx.

1. **List tenants** on the shard (skip non-active tenants).
2. **Build expected nginx config set** -- for each active tenant, list active webroots and compute the expected config filename (`{tenantID}_{webrootName}.conf`). Also collects FQDN data for each webroot.
3. **Clean orphaned nginx configs** -- on every node, the `CleanOrphanedConfigs` activity removes any nginx config files not in the expected set. This runs *before* creating webroots to prevent stale configs from causing `nginx -t` failures that would block all new provisioning.
4. **Create tenants** -- calls `CreateTenant` on each node for each active tenant (sets up system user, home directory), then `SyncSSHConfig` to configure SSH/SFTP access.
5. **Create webroots** -- calls `CreateWebroot` on each node for each active webroot, including runtime config and FQDN assignments.
6. **Reload nginx** -- calls `ReloadNginx` on all nodes to apply the new configurations.

### Database Shards

1. **List databases** on the shard (skip non-active).
2. For each database, call `CreateDatabase` on every node.
3. **List users** for each database (skip non-active).
4. For each user, call `CreateDatabaseUser` on every node with database name, username, password, and privileges.

### Valkey Shards

1. **List Valkey instances** on the shard (skip non-active).
2. For each instance, call `CreateValkeyInstance` on every node with name, port, password, and max memory.
3. **List users** for each instance (skip non-active).
4. For each user, call `CreateValkeyUser` on every node with instance name, port, username, password, privileges, and key pattern.

### Load Balancer Shards

1. **List active FQDN-to-backend mappings** for the shard's cluster.
2. For each mapping, call `SetLBMapEntry` on every node with the FQDN and backend address.

### Other Roles

Storage, DBAdmin, DNS, and email shards have no convergence logic. The workflow sets their status to `active` immediately.

## Error Handling

Convergence uses a **continue-on-error** strategy:

- Errors from individual resource/node operations are collected into a list rather than aborting the workflow.
- After all operations complete, if any errors occurred, the shard status is set to `failed` with a joined error summary (truncated to 4000 characters).
- If no errors occurred, the shard status is set to `active`.

This means a single unreachable node does not prevent convergence of the remaining nodes. The error summary indicates exactly which operations failed.

Early failures that prevent any work (shard not found, no nodes) cause an immediate abort with `failed` status.

## Inactive Resource Filtering

Convergence only processes resources with `active` status. Resources in `provisioning`, `failed`, `deleting`, or `deleted` states are skipped. This prevents partially-provisioned resources from being pushed to new nodes.

## Orphaned Config Cleanup

The orphan cleanup step in web convergence addresses a specific operational problem: if a webroot is deleted but its nginx config file remains on disk, `nginx -t` will fail and block all subsequent webroot provisioning. By computing the expected config set and removing anything not in it before creating new webroots, the workflow self-heals from config drift.

## Integration with Provisioning

Individual resource provisioning workflows (e.g., `TenantProvisionWorkflow`) push resources to nodes directly. Convergence is not part of the normal provisioning path -- it is a separate reconciliation mechanism used when nodes need to catch up to the current desired state.

## Node Activity Routing

Activities that run on specific nodes use a `nodeActivityCtx` helper that sets the Temporal task queue to `node-{nodeID}`. This ensures the activity is picked up by the correct node agent.

## Source Files

- Workflow: `internal/workflow/converge_shard.go`
- Tests: `internal/workflow/converge_shard_test.go`
