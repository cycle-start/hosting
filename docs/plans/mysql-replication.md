# MySQL Replication Management Plan

## Status Quo

The platform currently creates MySQL databases on individual nodes within a DB shard, but there is no replication between nodes. The `convergeDatabaseShard` function iterates all nodes in a shard and runs `CREATE DATABASE` / `CREATE USER` on each, but this creates empty independent databases rather than replicated copies. Shard convergence gives the illusion of redundancy without actual data replication.

Key files:
- `internal/agent/database.go` -- DatabaseManager executes raw MySQL CLI commands
- `internal/workflow/database.go` -- CreateDatabaseWorkflow creates the DB on every node in the shard
- `internal/workflow/converge_shard.go` -- convergeDatabaseShard does the same loop
- `terraform/cloud-init/db-node.yaml.tpl` -- DB node cloud-init (bare MySQL, no replication config)
- `packer/scripts/db.sh` -- installs mysql-server, configures passwordless root

Currently only one DB node exists per shard (db-1 has only `db-1-node-0` at 10.10.10.20). The MySQL instance runs with default config: no GTID, no binary logging, no replication user.

---

## 1. Replication Strategy: GTID-Based Async Replication

### Why GTID

GTID (Global Transaction Identifiers) is the correct choice for this platform:

- **Positional binlog replication** requires tracking `MASTER_LOG_FILE` and `MASTER_LOG_POS` per replica, which is fragile across restarts, crashes, and failovers.
- **GTID** gives every transaction a globally unique ID (`server_uuid:sequence`). A replica simply requests "all transactions I haven't seen yet." Failover to a new primary is trivial -- the replica already knows exactly which transactions it has.
- **Semi-sync is not needed at this scale.** Asynchronous replication with proper monitoring is sufficient. Semi-sync adds latency to every write and can degrade to async anyway under network partitions. The platform can add semi-sync later per-shard if a tenant requires stronger durability guarantees.

### Replication Topology

Each DB shard runs a **single-primary** topology:

```
Shard: db-1
  [Primary]  db-1-node-0  (10.10.10.20)  -- all writes
  [Replica]  db-1-node-1  (10.10.10.21)  -- async replication, read-only
```

This is a **1:1 pair**, not a multi-replica fan-out. Every DB shard has exactly one primary and one replica. This keeps things simple and matches the "replication pairs on local SSDs" architecture described in the project memory.

### MySQL Configuration

Both nodes in a shard need the same base configuration with only `server-id` and `read_only` differing.

#### Packer base config (`packer/scripts/db.sh`)

Add a MySQL config file that enables binary logging and GTID on all DB nodes at image build time:

```bash
# In packer/scripts/db.sh, add:
cat > /etc/mysql/mysql.conf.d/replication.cnf << 'REPL_EOF'
[mysqld]
# --- GTID Replication ---
gtid_mode                = ON
enforce_gtid_consistency = ON
log_bin                  = /var/lib/mysql/binlog
binlog_format            = ROW
binlog_row_image         = FULL
log_slave_updates        = ON
relay_log                = /var/lib/mysql/relay-bin
relay_log_recovery       = ON

# Server ID is set at runtime via cloud-init (unique per node).
# server-id = <set by cloud-init>

# Crash-safe replication
sync_binlog              = 1
innodb_flush_log_at_trx_commit = 1

# Binary log expiration (7 days)
binlog_expire_logs_seconds = 604800

# Parallel replication (MySQL 8.0+)
replica_parallel_workers  = 4
replica_parallel_type     = LOGICAL_CLOCK
replica_preserve_commit_order = ON

# Performance
binlog_transaction_dependency_tracking = WRITESET
transaction_write_set_extraction       = XXHASH64
REPL_EOF
```

#### Cloud-init per-node config (`terraform/cloud-init/db-node.yaml.tpl`)

Each node needs a unique `server-id` and the replication user. Add to the cloud-init template:

```yaml
write_files:
  # ... existing node-agent env file ...

  - path: /etc/mysql/mysql.conf.d/server-id.cnf
    content: |
      [mysqld]
      server-id = ${server_id}

runcmd:
  # Set server-id before starting MySQL
  - systemctl start mysql
  # Create replication user (idempotent)
  - |
    mysql -u root -e "
      CREATE USER IF NOT EXISTS 'repl'@'%' IDENTIFIED BY '${repl_password}';
      GRANT REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO 'repl'@'%';
      FLUSH PRIVILEGES;
    "
  - systemctl daemon-reload
  - systemctl start node-agent
```

The `server_id` must be a unique positive integer. Derive it deterministically from the node index within the shard:

```hcl
# In terraform/nodes.tf, update the db_node cloud-init:
user_data = templatefile("${path.module}/cloud-init/db-node.yaml.tpl", {
  hostname         = var.db_nodes[count.index].name
  node_id          = random_uuid.db_node_id[count.index].result
  shard_name       = var.db_shard_name
  temporal_address = "${var.controlplane_ip}:${var.temporal_port}"
  ssh_public_key   = file(pathexpand(var.ssh_public_key_path))
  region_id        = var.region_id
  cluster_id       = var.cluster_id
  server_id        = (count.index + 1)     # 1-based, unique per shard
  repl_password    = random_password.db_repl.result
  peer_ip          = count.index == 0 ? (
    length(var.db_nodes) > 1 ? var.db_nodes[1].ip : ""
  ) : var.db_nodes[0].ip
})
```

Add a Terraform resource for the replication password:

```hcl
resource "random_password" "db_repl" {
  length  = 32
  special = false
}
```

---

## 2. Primary Election

### Design Decision: Explicit Primary in Core DB

The platform needs an authoritative record of which node is primary for each DB shard. There are three approaches; we choose option (b).

**(a) Convention: first node is always primary.**
Simple but cannot survive failover -- after promotion, the "first node" is now the replica.

**(b) Explicit field in the `shards` table config JSON.**
The shard's `config` JSONB column already exists and is used for storage shard config (`StorageShardConfig`). Add a `DatabaseShardConfig` equivalent.

**(c) Separate `shard_node_roles` table.**
Over-engineered for a 1:1 pair.

### Schema: DatabaseShardConfig

Add a Go struct for the shard config:

```go
// internal/model/shard.go

type DatabaseShardConfig struct {
    PrimaryNodeID string `json:"primary_node_id"`
}
```

When a DB shard is created with 2 nodes, the first node (`count.index == 0`) is designated primary. This is written to `shards.config` during cluster bootstrap via `hostctl cluster apply`.

### How Primary Is Set

1. **Initial bootstrap** (`hostctl cluster apply`): When creating a DB shard with nodes, set `config.primary_node_id` to the first node's ID.
2. **Failover** (see Section 5): Update `config.primary_node_id` to the promoted node's ID.
3. **All workflows read the shard config** to determine which node is primary before executing replication-sensitive operations.

### Reading the Primary

Add a helper that all DB-related workflows use:

```go
// internal/workflow/helpers.go

// dbShardPrimary returns the primary node ID and the full node list for a DB shard.
// The primary is determined by the shard's config.primary_node_id field.
// If no primary is configured, the first node in the list is assumed primary.
func dbShardPrimary(ctx workflow.Context, shardID string) (primaryID string, nodes []model.Node, err error) {
    var shard model.Shard
    err = workflow.ExecuteActivity(ctx, "GetShardByID", shardID).Get(ctx, &shard)
    if err != nil {
        return "", nil, fmt.Errorf("get shard: %w", err)
    }

    var nodes []model.Node
    err = workflow.ExecuteActivity(ctx, "ListNodesByShard", shardID).Get(ctx, &nodes)
    if err != nil {
        return "", nil, fmt.Errorf("list nodes: %w", err)
    }

    var cfg model.DatabaseShardConfig
    if len(shard.Config) > 0 {
        _ = json.Unmarshal(shard.Config, &cfg)
    }

    if cfg.PrimaryNodeID != "" {
        return cfg.PrimaryNodeID, nodes, nil
    }

    // Fallback: first node is primary.
    if len(nodes) > 0 {
        return nodes[0].ID, nodes, nil
    }

    return "", nodes, fmt.Errorf("shard %s has no nodes", shardID)
}
```

---

## 3. Convergence Integration

### Current Problem

`convergeDatabaseShard` creates databases and users on every node independently. This works for a single-node shard but is wrong for replication: the replica should get data via replication, not via independent `CREATE DATABASE` statements (which would create empty databases that diverge from the primary).

### New Convergence Flow

The converge workflow for a DB shard must handle three scenarios:

#### Scenario A: Fresh shard, no replication configured yet

1. Identify primary node from shard config.
2. Create all databases and users on the **primary only**.
3. Configure the replica to replicate from the primary.
4. Verify replication is running.

#### Scenario B: Existing shard, replication already running

1. Verify replication health on replica.
2. Create any new databases/users on primary only (they replicate automatically).
3. If replication is broken, repair it (see Section 4).

#### Scenario C: New replica added to existing shard

1. Stop writes on primary (set `SUPER_READ_ONLY` briefly).
2. Take a full dump from primary using `mysqldump --single-transaction --source-data=2 --all-databases`.
3. Import dump on new replica.
4. Configure replica to point at primary using GTID auto-positioning.
5. Resume writes on primary.

### New Activity: ConfigureReplication

```go
// internal/agent/database.go

// ConfigureReplication sets up this node as a replica of the given primary.
func (m *DatabaseManager) ConfigureReplication(ctx context.Context, primaryHost string, replUser, replPassword string) error {
    m.logger.Info().Str("primary", primaryHost).Msg("configuring replication")

    // Stop any existing replication.
    _ = m.execMySQL(ctx, "STOP REPLICA")

    // Reset replica state for clean start.
    if err := m.execMySQL(ctx, "RESET REPLICA ALL"); err != nil {
        return fmt.Errorf("reset replica: %w", err)
    }

    // Configure replication source using GTID auto-positioning.
    sql := fmt.Sprintf(
        `CHANGE REPLICATION SOURCE TO
            SOURCE_HOST='%s',
            SOURCE_PORT=3306,
            SOURCE_USER='%s',
            SOURCE_PASSWORD='%s',
            SOURCE_AUTO_POSITION=1,
            SOURCE_CONNECT_RETRY=10,
            SOURCE_RETRY_COUNT=86400,
            GET_SOURCE_PUBLIC_KEY=1`,
        primaryHost, replUser, replPassword,
    )
    if err := m.execMySQL(ctx, sql); err != nil {
        return fmt.Errorf("change replication source: %w", err)
    }

    // Start replication.
    if err := m.execMySQL(ctx, "START REPLICA"); err != nil {
        return fmt.Errorf("start replica: %w", err)
    }

    return nil
}

// SetReadOnly makes this MySQL instance read-only or read-write.
func (m *DatabaseManager) SetReadOnly(ctx context.Context, readOnly bool) error {
    if readOnly {
        return m.execMySQL(ctx, "SET GLOBAL read_only = ON; SET GLOBAL super_read_only = ON")
    }
    return m.execMySQL(ctx, "SET GLOBAL super_read_only = OFF; SET GLOBAL read_only = OFF")
}
```

### New Activity: GetReplicationStatus

```go
// internal/agent/database.go

// ReplicationStatus holds the parsed output of SHOW REPLICA STATUS.
type ReplicationStatus struct {
    IORunning        bool   `json:"io_running"`
    SQLRunning       bool   `json:"sql_running"`
    SecondsBehind    *int   `json:"seconds_behind"` // nil if not replicating
    LastError        string `json:"last_error"`
    ExecutedGTIDSet  string `json:"executed_gtid_set"`
    RetrievedGTIDSet string `json:"retrieved_gtid_set"`
}

// GetReplicationStatus returns the current replication status of this node.
func (m *DatabaseManager) GetReplicationStatus(ctx context.Context) (*ReplicationStatus, error) {
    baseArgs, err := m.mysqlArgs()
    if err != nil {
        return nil, fmt.Errorf("parse mysql DSN: %w", err)
    }

    // Use vertical output for easier parsing.
    args := append(baseArgs, "-e", "SHOW REPLICA STATUS\\G")
    cmd := exec.CommandContext(ctx, "mysql", args...)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return nil, fmt.Errorf("show replica status: %s: %w", string(output), err)
    }

    return parseReplicaStatus(string(output)), nil
}
```

The parser extracts key fields from the vertical `SHOW REPLICA STATUS\G` output by matching lines like `Replica_IO_Running: Yes`.

### Updated convergeDatabaseShard

```go
func convergeDatabaseShard(ctx workflow.Context, shardID string, nodes []model.Node) []string {
    // Determine primary.
    primaryID, nodes, err := dbShardPrimary(ctx, shardID)
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

    // List all databases on this shard.
    var databases []model.Database
    err = workflow.ExecuteActivity(ctx, "ListDatabasesByShard", shardID).Get(ctx, &databases)
    if err != nil {
        return []string{fmt.Sprintf("list databases: %v", err)}
    }

    // Ensure primary is read-write.
    primaryCtx := nodeActivityCtx(ctx, primary.ID)
    err = workflow.ExecuteActivity(primaryCtx, "SetReadOnly", false).Get(ctx, nil)
    if err != nil {
        errs = append(errs, fmt.Sprintf("set primary read-write: %v", err))
    }

    // Create databases and users on the PRIMARY ONLY.
    for _, database := range databases {
        if database.Status != model.StatusActive {
            continue
        }

        err = workflow.ExecuteActivity(primaryCtx, "CreateDatabase", database.ID).Get(ctx, nil)
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
                DatabaseName: database.ID,
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
            // Replication is healthy -- nothing to do.
            continue
        }

        // Configure replication from primary.
        err = workflow.ExecuteActivity(replicaCtx, "ConfigureReplication", activity.ConfigureReplicationParams{
            PrimaryHost:  *primary.IPAddress,
            ReplUser:     "repl",
            ReplPassword: replPassword, // Retrieved from shard config or secret
        }).Get(ctx, nil)
        if err != nil {
            errs = append(errs, fmt.Sprintf("configure replication on %s: %v", replica.ID, err))
        }
    }

    return errs
}
```

### Updated CreateDatabaseWorkflow

The `CreateDatabaseWorkflow` must also change: create on primary only, let replication handle the replica.

```go
func CreateDatabaseWorkflow(ctx workflow.Context, databaseID string) error {
    // ... setup, status to provisioning ...

    primaryID, nodes, err := dbShardPrimary(ctx, *database.ShardID)
    if err != nil {
        _ = setResourceFailed(ctx, "databases", databaseID, err)
        return err
    }

    // Create database on PRIMARY only.
    primaryCtx := nodeActivityCtx(ctx, primaryID)
    err = workflow.ExecuteActivity(primaryCtx, "CreateDatabase", database.ID).Get(ctx, nil)
    if err != nil {
        _ = setResourceFailed(ctx, "databases", databaseID, err)
        return err
    }

    // Set status to active.
    return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
        Table:  "databases",
        ID:     databaseID,
        Status: model.StatusActive,
    }).Get(ctx, nil)
}
```

The same pattern applies to `CreateDatabaseUserWorkflow`, `UpdateDatabaseUserWorkflow`, and `DeleteDatabaseWorkflow` -- all write operations go to primary only.

---

## 4. Health Monitoring

### Replication Health Check Activity

The `GetReplicationStatus` activity (defined above) is the foundation. Add a higher-level health check:

```go
// internal/agent/database.go

// CheckReplicationHealth returns nil if replication is healthy, or an error describing the problem.
func (m *DatabaseManager) CheckReplicationHealth(ctx context.Context, maxLagSeconds int) error {
    status, err := m.GetReplicationStatus(ctx)
    if err != nil {
        return fmt.Errorf("get replication status: %w", err)
    }

    if !status.IORunning {
        return fmt.Errorf("replication IO thread not running: %s", status.LastError)
    }
    if !status.SQLRunning {
        return fmt.Errorf("replication SQL thread not running: %s", status.LastError)
    }
    if status.SecondsBehind != nil && *status.SecondsBehind > maxLagSeconds {
        return fmt.Errorf("replication lag: %d seconds (max: %d)", *status.SecondsBehind, maxLagSeconds)
    }

    return nil
}
```

### Periodic Health Check Workflow (Cron)

Register a Temporal cron workflow that checks replication health across all DB shards:

```go
// internal/workflow/replication_health.go

// CheckReplicationHealthWorkflow runs on a cron schedule and checks all DB shard replicas.
func CheckReplicationHealthWorkflow(ctx workflow.Context) error {
    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 30 * time.Second,
        RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 2},
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    // List all database shards.
    var shards []model.Shard
    err := workflow.ExecuteActivity(ctx, "ListShardsByRole", model.ShardRoleDatabase).Get(ctx, &shards)
    if err != nil {
        return fmt.Errorf("list database shards: %w", err)
    }

    for _, shard := range shards {
        primaryID, nodes, err := dbShardPrimary(ctx, shard.ID)
        if err != nil {
            workflow.GetLogger(ctx).Warn("failed to get primary for shard",
                "shard", shard.ID, "error", err)
            continue
        }

        for _, node := range nodes {
            if node.ID == primaryID {
                continue // Skip primary, it doesn't replicate.
            }

            nodeCtx := nodeActivityCtx(ctx, node.ID)
            var status ReplicationStatus
            err = workflow.ExecuteActivity(nodeCtx, "GetReplicationStatus").Get(ctx, &status)
            if err != nil {
                workflow.GetLogger(ctx).Error("replication check failed",
                    "shard", shard.ID, "node", node.ID, "error", err)
                // Update shard status to degraded.
                setShardStatus(ctx, shard.ID, "degraded",
                    strPtr(fmt.Sprintf("replication check failed on node %s: %v", node.ID, err)))
                continue
            }

            if !status.IORunning || !status.SQLRunning {
                workflow.GetLogger(ctx).Error("replication broken",
                    "shard", shard.ID, "node", node.ID,
                    "io_running", status.IORunning,
                    "sql_running", status.SQLRunning,
                    "last_error", status.LastError)
                setShardStatus(ctx, shard.ID, "degraded",
                    strPtr(fmt.Sprintf("replication broken on node %s: %s", node.ID, status.LastError)))
            } else if status.SecondsBehind != nil && *status.SecondsBehind > 300 {
                workflow.GetLogger(ctx).Warn("high replication lag",
                    "shard", shard.ID, "node", node.ID,
                    "seconds_behind", *status.SecondsBehind)
                setShardStatus(ctx, shard.ID, "degraded",
                    strPtr(fmt.Sprintf("replication lag %ds on node %s", *status.SecondsBehind, node.ID)))
            }
        }
    }

    return nil
}
```

Register this as a cron workflow in the worker, running every 60 seconds:

```go
// In cmd/worker/main.go, after registering other workflows:
_, err = tc.ScheduleClient().Create(ctx, temporalclient.ScheduleOptions{
    ID: "check-replication-health",
    Spec: temporalclient.ScheduleSpec{
        CronExpressions: []string{"* * * * *"}, // Every minute
    },
    Action: &temporalclient.ScheduleWorkflowAction{
        Workflow:  workflow.CheckReplicationHealthWorkflow,
        TaskQueue: "worker",
    },
})
```

### Prometheus Metrics

The node-agent should expose replication metrics for Grafana dashboards. Add to the node-agent when running on a DB node:

```go
// internal/agent/database_metrics.go

var (
    replIORunning = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "mysql_replication_io_running",
        Help: "Whether the replication IO thread is running (1=yes, 0=no)",
    })
    replSQLRunning = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "mysql_replication_sql_running",
        Help: "Whether the replication SQL thread is running (1=yes, 0=no)",
    })
    replSecondsBehind = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "mysql_replication_seconds_behind",
        Help: "Seconds behind primary (-1 if not replicating)",
    })
)
```

Start a goroutine in the node-agent that polls `SHOW REPLICA STATUS` every 15 seconds and updates these gauges.

### Shard Status Values

Extend the shard status vocabulary:

| Status       | Meaning                                           |
|-------------|---------------------------------------------------|
| `active`     | All nodes healthy, replication running             |
| `converging` | Shard convergence in progress                      |
| `degraded`   | Replication broken or lagging > 5 min              |
| `failed`     | Convergence failed or critical error               |
| `failover`   | Failover in progress                               |

---

## 5. Failover

### Manual Failover (Phase 1)

Automated failover is dangerous if done wrong. Start with a manual failover workflow triggered via API.

#### Failover Workflow

```go
// internal/workflow/replication_failover.go

type FailoverDBShardParams struct {
    ShardID      string `json:"shard_id"`
    NewPrimaryID string `json:"new_primary_id"` // Empty = auto-select replica
}

func FailoverDBShardWorkflow(ctx workflow.Context, params FailoverDBShardParams) error {
    ao := workflow.ActivityOptions{
        StartToCloseTimeout: 2 * time.Minute,
        RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 3},
    }
    ctx = workflow.WithActivityOptions(ctx, ao)

    // Set shard to failover status.
    setShardStatus(ctx, params.ShardID, "failover", strPtr("failover in progress"))

    // Get current primary and nodes.
    oldPrimaryID, nodes, err := dbShardPrimary(ctx, params.ShardID)
    if err != nil {
        setShardStatus(ctx, params.ShardID, model.StatusFailed, strPtr(err.Error()))
        return err
    }

    // Determine new primary.
    newPrimaryID := params.NewPrimaryID
    if newPrimaryID == "" {
        // Auto-select: pick the first replica.
        for _, n := range nodes {
            if n.ID != oldPrimaryID {
                newPrimaryID = n.ID
                break
            }
        }
    }
    if newPrimaryID == "" {
        msg := "no replica available for failover"
        setShardStatus(ctx, params.ShardID, model.StatusFailed, &msg)
        return fmt.Errorf("%s", msg)
    }

    // Step 1: Try to set old primary to read-only (best effort -- it may be down).
    oldPrimaryCtx := nodeActivityCtx(ctx, oldPrimaryID)
    _ = workflow.ExecuteActivity(oldPrimaryCtx, "SetReadOnly", true).Get(ctx, nil)

    // Step 2: Wait for replica to catch up (check GTID gap).
    newPrimaryCtx := nodeActivityCtx(ctx, newPrimaryID)

    // Poll replication status until caught up or timeout.
    var replStatus agent.ReplicationStatus
    for i := 0; i < 30; i++ { // 30 iterations * 2s = 60s max wait
        err = workflow.ExecuteActivity(newPrimaryCtx, "GetReplicationStatus").Get(ctx, &replStatus)
        if err != nil {
            break // Can't check, proceed anyway.
        }
        if replStatus.SecondsBehind != nil && *replStatus.SecondsBehind == 0 {
            break // Caught up.
        }
        _ = workflow.Sleep(ctx, 2*time.Second)
    }

    // Step 3: Stop replication on the new primary.
    _ = workflow.ExecuteActivity(newPrimaryCtx, "StopReplication").Get(ctx, nil)

    // Step 4: Promote new primary to read-write.
    err = workflow.ExecuteActivity(newPrimaryCtx, "SetReadOnly", false).Get(ctx, nil)
    if err != nil {
        setShardStatus(ctx, params.ShardID, model.StatusFailed, strPtr(err.Error()))
        return fmt.Errorf("promote new primary: %w", err)
    }

    // Step 5: Update shard config with new primary.
    err = workflow.ExecuteActivity(ctx, "UpdateShardConfig", activity.UpdateShardConfigParams{
        ShardID: params.ShardID,
        Config:  model.DatabaseShardConfig{PrimaryNodeID: newPrimaryID},
    }).Get(ctx, nil)
    if err != nil {
        setShardStatus(ctx, params.ShardID, model.StatusFailed, strPtr(err.Error()))
        return fmt.Errorf("update shard config: %w", err)
    }

    // Step 6: Update ProxySQL to route to new primary (see Section 5b).
    err = workflow.ExecuteActivity(ctx, "UpdateProxySQLPrimary", activity.UpdateProxySQLPrimaryParams{
        ShardID:    params.ShardID,
        PrimaryIP:  nodeIPByID(nodes, newPrimaryID),
        ReplicaIPs: nodeIPsExcluding(nodes, newPrimaryID),
    }).Get(ctx, nil)
    if err != nil {
        // Non-fatal -- ProxySQL update can be retried.
        workflow.GetLogger(ctx).Error("failed to update ProxySQL", "error", err)
    }

    // Step 7: If old primary is reachable, reconfigure as replica of new primary.
    // This is best-effort; it may be down.
    _ = workflow.ExecuteActivity(oldPrimaryCtx, "ConfigureReplication", activity.ConfigureReplicationParams{
        PrimaryHost:  nodeIPByID(nodes, newPrimaryID),
        ReplUser:     "repl",
        ReplPassword: replPassword,
    }).Get(ctx, nil)
    _ = workflow.ExecuteActivity(oldPrimaryCtx, "SetReadOnly", true).Get(ctx, nil)

    // Step 8: Set shard to active.
    setShardStatus(ctx, params.ShardID, model.StatusActive, nil)
    return nil
}
```

#### API Endpoint

```
POST /api/v1/shards/{id}/failover
{
    "new_primary_id": "optional-node-uuid"
}
```

Returns `202 Accepted`. Only available to admin users.

### Automated Failover (Phase 2, Future)

Automated failover can be added later as a Temporal workflow that:

1. Detects primary unreachable (3 consecutive health check failures over 3 minutes).
2. Verifies the primary is actually down (not just a network partition between health checker and primary) by checking from multiple vantage points.
3. Runs the same failover workflow.

**Requirements before enabling automated failover:**
- Fencing: STONITH or equivalent to ensure the old primary cannot accept writes after failover.
- Split-brain detection: If both nodes think they are primary, freeze writes on both and alert.
- At least 3 minutes of consecutive failures before triggering (avoid flapping).

---

## 5b. ProxySQL Integration

### Architecture

ProxySQL runs on each DB shard node (co-located, not on a separate VM). Tenants connect to ProxySQL on port 6033 instead of MySQL on port 3306. ProxySQL handles:

1. **Connection pooling** -- multiplexes thousands of tenant connections onto a smaller pool of MySQL connections.
2. **Read/write splitting** -- routes writes to primary, reads to replica (Phase 2).
3. **VIP failover** -- after primary promotion, update ProxySQL's hostgroup assignments.

### ProxySQL Hostgroup Design

```
Hostgroup 10: WRITER (primary)   -- all write traffic
Hostgroup 20: READER (replicas)  -- read traffic (Phase 2)
```

### ProxySQL Base Config

Install ProxySQL via packer and configure it at cloud-init time:

```sql
-- ProxySQL admin interface: 127.0.0.1:6032
-- ProxySQL MySQL interface: 0.0.0.0:6033

-- Add MySQL servers
INSERT INTO mysql_servers (hostgroup_id, hostname, port, weight, max_connections)
VALUES
    (10, '10.10.10.20', 3306, 1000, 500),  -- primary
    (20, '10.10.10.21', 3306, 1000, 500);  -- replica

-- Replication hostgroup: ProxySQL will auto-move servers between writer/reader
-- based on read_only flag
INSERT INTO mysql_replication_hostgroups (writer_hostgroup, reader_hostgroup, check_type)
VALUES (10, 20, 'innodb_read_only');

-- Default routing rule: all queries to writer hostgroup
INSERT INTO mysql_query_rules (rule_id, active, match_pattern, destination_hostgroup, apply)
VALUES
    (1, 1, '^SELECT .* FOR UPDATE', 10, 1),         -- SELECT FOR UPDATE -> writer
    (2, 1, '^SELECT', 20, 1),                        -- All other SELECTs -> reader (Phase 2)
    (100, 1, '.*', 10, 1);                            -- Everything else -> writer

-- Monitor user (ProxySQL uses this to check server health)
UPDATE global_variables SET variable_value='repl' WHERE variable_name='mysql-monitor_username';
UPDATE global_variables SET variable_value='<repl_password>' WHERE variable_name='mysql-monitor_password';
UPDATE global_variables SET variable_value='2000' WHERE variable_name='mysql-monitor_connect_interval';
UPDATE global_variables SET variable_value='2000' WHERE variable_name='mysql-monitor_ping_interval';
UPDATE global_variables SET variable_value='2000' WHERE variable_name='mysql-monitor_read_only_interval';

LOAD MYSQL SERVERS TO RUNTIME;
LOAD MYSQL QUERY RULES TO RUNTIME;
LOAD MYSQL VARIABLES TO RUNTIME;
SAVE MYSQL SERVERS TO DISK;
SAVE MYSQL QUERY RULES TO DISK;
SAVE MYSQL VARIABLES TO DISK;
```

### ProxySQL + read_only Failover

This is the key insight: **ProxySQL can use `mysql_replication_hostgroups` to automatically detect failover.**

When we set `read_only=ON` on the old primary and `read_only=OFF` on the new primary, ProxySQL's monitor thread detects the change and automatically moves the servers between writer (HG 10) and reader (HG 20) hostgroups. No explicit ProxySQL reconfiguration needed during failover.

This means:
- The failover workflow only needs to toggle `read_only` on MySQL nodes.
- ProxySQL handles the routing change automatically within seconds.
- The `UpdateProxySQLPrimary` activity becomes a no-op or just a verification step.

### Where ProxySQL Runs

**Option A: On each DB node** (recommended for Phase 1).
Tenants get a connection string pointing to the shard's VIP (a virtual IP managed by keepalived between the two DB nodes). ProxySQL on the VIP holder routes traffic appropriately.

**Option B: On a dedicated proxy tier.**
Better for scale but adds infrastructure complexity. Defer to Phase 2.

### Keepalived for VIP

Each DB shard pair shares a virtual IP managed by keepalived. The primary node holds the VIP by default. On failover, the VIP floats to the new primary.

Add to packer (`packer/scripts/db.sh`):

```bash
apt-get install -y proxysql keepalived
```

Keepalived config (via cloud-init):

```
vrrp_instance VI_DB {
    state BACKUP         # Both start as BACKUP; priority determines master
    interface ens3
    virtual_router_id 51
    priority ${is_primary ? 101 : 100}
    advert_int 1
    authentication {
        auth_type PASS
        auth_pass ${vrrp_password}
    }
    virtual_ipaddress {
        ${shard_vip}/24
    }
    track_script {
        chk_mysql
    }
}

vrrp_script chk_mysql {
    script "/usr/bin/mysqladmin ping -h 127.0.0.1"
    interval 2
    weight -20        # If MySQL is down, drop priority by 20 (causes VIP failover)
    fall 3
    rise 2
}
```

### Terraform: DB Shard VIP

Add a VIP variable per DB shard:

```hcl
variable "db_shard_vip" {
  description = "Virtual IP for the DB shard (ProxySQL endpoint)"
  type        = string
  default     = "10.10.10.25"
}
```

Pass it through cloud-init to both DB nodes.

### Connection String for Tenants

Tenant connection strings point to the shard VIP on the ProxySQL port:

```
mysql://username:password@10.10.10.25:6033/dbname
```

This is what the platform returns when a tenant creates a database. The VIP floats between nodes; ProxySQL routes to the correct primary.

---

## 6. Read/Write Splitting

### Phase 1: All Traffic to Primary

For the initial implementation, all traffic goes to the primary via ProxySQL hostgroup 10. The read/write splitting query rules are installed but the reader hostgroup (20) is not used. This avoids stale read issues during the initial rollout.

### Phase 2: Opt-In Read Splitting

Add a per-database or per-tenant flag:

```go
type Database struct {
    // ... existing fields ...
    ReadReplicaEnabled bool `json:"read_replica_enabled" db:"read_replica_enabled"`
}
```

When enabled, ProxySQL query rules route `SELECT` statements (excluding `SELECT ... FOR UPDATE`) to hostgroup 20 (reader). This is configured per-user in ProxySQL:

```sql
-- For a user with read splitting enabled:
INSERT INTO mysql_users (username, password, default_hostgroup, transaction_persistent)
VALUES ('tenant_user', 'hashed_password', 10, 1);
-- transaction_persistent=1 ensures that once a transaction starts on the writer,
-- all queries in that transaction stay on the writer.
```

### Stale Read Considerations

- Async replication means replicas can be up to N seconds behind.
- Read-after-write consistency is not guaranteed. If a tenant writes a row and immediately reads it, they might not see it on the replica.
- Mitigation: `transaction_persistent=1` in ProxySQL ensures that within a transaction, all queries go to the same server. Most frameworks use transactions for write+read patterns.
- Additional mitigation: ProxySQL's `mysql-monitor_replication_lag_interval` can exclude replicas that are too far behind from the reader hostgroup.

---

## 7. Implementation Phases

### Phase 1: Foundation (Required)

| Task | Files to Change |
|------|----------------|
| Add `DatabaseShardConfig` struct | `internal/model/shard.go` |
| Enable GTID + binlog in packer | `packer/scripts/db.sh` |
| Add `server-id` and `repl` user to cloud-init | `terraform/cloud-init/db-node.yaml.tpl`, `terraform/nodes.tf` |
| Add second DB node to Terraform | `terraform/variables.tf` (change `db_nodes` default) |
| Add `repl_password` Terraform variable | `terraform/variables.tf`, `terraform/nodes.tf` |
| Implement `ConfigureReplication` activity | `internal/agent/database.go` |
| Implement `SetReadOnly` activity | `internal/agent/database.go` |
| Implement `GetReplicationStatus` activity | `internal/agent/database.go` |
| Implement `StopReplication` activity | `internal/agent/database.go` |
| Add activity params | `internal/activity/params.go` |
| Register new activities | `internal/activity/node_local.go` |
| Add `dbShardPrimary` helper | `internal/workflow/helpers.go` |
| Update `convergeDatabaseShard` | `internal/workflow/converge_shard.go` |
| Update `CreateDatabaseWorkflow` (primary only) | `internal/workflow/database.go` |
| Update `CreateDatabaseUserWorkflow` (primary only) | `internal/workflow/database_user.go` |
| Update `DeleteDatabaseWorkflow` (primary only) | `internal/workflow/database.go` |
| Update `MigrateDatabaseWorkflow` | `internal/workflow/migrate_database.go` |
| Add replication health check workflow | `internal/workflow/replication_health.go` (new) |
| Register health check as cron | `cmd/worker/main.go` |
| Tests for all new activities and workflows | `internal/agent/database_test.go`, `internal/workflow/*_test.go` |

### Phase 2: ProxySQL + VIP

| Task | Files to Change |
|------|----------------|
| Install ProxySQL + keepalived in packer | `packer/scripts/db.sh` |
| Add ProxySQL base config to cloud-init | `terraform/cloud-init/db-node.yaml.tpl` |
| Add keepalived config to cloud-init | `terraform/cloud-init/db-node.yaml.tpl` |
| Add `db_shard_vip` Terraform variable | `terraform/variables.tf` |
| Add VIP to cloud-init template | `terraform/nodes.tf` |
| Return VIP-based connection strings from API | `internal/api/handler/database.go` |
| Prometheus metrics for replication | `internal/agent/database_metrics.go` (new) |

### Phase 3: Failover

| Task | Files to Change |
|------|----------------|
| Implement `FailoverDBShardWorkflow` | `internal/workflow/replication_failover.go` (new) |
| Add `POST /shards/{id}/failover` API | `internal/api/handler/shard.go` |
| Add `UpdateShardConfig` activity | `internal/activity/core_db.go` |
| Register failover workflow | `cmd/worker/main.go` |
| Tests | `internal/workflow/replication_failover_test.go` (new) |

### Phase 4: Read Splitting (Future)

| Task | Files to Change |
|------|----------------|
| Add `read_replica_enabled` to Database model | `internal/model/database.go`, migration |
| Configure per-user ProxySQL routing | `internal/agent/database.go` |
| API support for toggling read splitting | `internal/api/handler/database.go` |

---

## 8. Security Considerations

### Replication User

- The `repl` user has only `REPLICATION SLAVE, REPLICATION CLIENT` grants -- no data access.
- The replication password is generated by Terraform (`random_password`) and passed via cloud-init. It is stored in `/etc/default/node-agent` on each DB node.
- For production, the replication password should come from a secrets manager (Vault, etc.) rather than Terraform state.

### Network

- MySQL replication traffic (port 3306 between DB nodes) should be restricted by firewall rules to only allow traffic within the shard.
- ProxySQL admin interface (port 6032) must only listen on 127.0.0.1.
- ProxySQL client interface (port 6033) should only accept connections from the cluster's network CIDR.

### Encryption

- Enable SSL for replication: `SOURCE_SSL=1` in the `CHANGE REPLICATION SOURCE TO` command.
- Generate per-shard TLS certificates for replication (can use the platform's ACME infrastructure or self-signed CA).

---

## 9. Data Safety Invariants

These invariants must hold at all times:

1. **Single writer**: Only one node in a shard is writable (`read_only=OFF`). All others have `super_read_only=ON`.
2. **GTID consistency**: All nodes in a shard must have `gtid_mode=ON` and `enforce_gtid_consistency=ON`. There is no partial GTID -- it is all or nothing.
3. **No direct writes to replicas**: All platform workflows (create, delete, update database/user) route to the primary only. The `convergeDatabaseShard` function never writes to replicas.
4. **Crash-safe replication**: `sync_binlog=1` and `innodb_flush_log_at_trx_commit=1` on primary. `relay_log_recovery=ON` on replicas. These ensure no committed transactions are lost on crash.
5. **Binlog retention**: 7 days of binary logs retained. If a replica is offline for more than 7 days, it must be re-provisioned from a full dump rather than catching up via binlog.
6. **Idempotent convergence**: The convergence workflow can run multiple times safely. `CREATE DATABASE IF NOT EXISTS` on primary is safe. Replication setup checks if already running before reconfiguring.

---

## 10. Disaster Recovery

### Scenario: Primary Dies, Data on Local SSD

1. Replica is promoted to primary (failover workflow).
2. Replica has all committed transactions that were replicated (async means up to a few seconds of data loss).
3. A new VM is provisioned to replace the dead primary.
4. The new VM is configured as a replica of the new primary.
5. Full dump from new primary is imported on the new replica.
6. Replication starts from the GTID position after import.

### Scenario: Both Nodes Die

1. Restore from the most recent backup (see existing `CreateMySQLBackup` / `RestoreMySQLBackup` activities).
2. Data loss = time since last backup.
3. This is why the backup schedule for DB shards should be aggressive (every 6 hours minimum).

### Scenario: Replica Falls Too Far Behind (> 7 Days)

1. Stop replication on replica.
2. Take full dump from primary.
3. Wipe replica data.
4. Import full dump.
5. Restart replication with GTID auto-positioning.

This is equivalent to Scenario C in the convergence section and is handled by the same code path.

---

## 11. Migration Path from Current State

Since the platform is pre-release and uses the "edit original migration + wipe DB" policy, the migration is straightforward:

1. Update packer image with GTID + binlog config.
2. Add second DB node to Terraform.
3. Update cloud-init templates with server-id, repl user, and replication password.
4. Deploy new code with updated workflows.
5. `terraform apply` to create new VMs.
6. `hostctl cluster apply` to register nodes and trigger convergence.
7. Convergence workflow detects two nodes, configures replication automatically.

No data migration is needed since this is a development environment. In production, migrating an existing single-node shard to a replication pair would follow the "add new replica" flow from Scenario C.
