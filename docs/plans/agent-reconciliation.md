# Agent-Side Reconciliation

## Problem Statement

Today the node-agent is purely reactive: it registers Temporal activities and waits for workflows to dispatch work. If a node reboots, loses a config file, or has a process die, nothing detects or repairs the drift until someone manually triggers `POST /shards/{id}/converge`. This is unacceptable at scale -- a hosting platform serving millions of tenants needs autonomous self-healing on every node.

The current `ConvergeShardWorkflow` operates from the control plane and pushes state to every node in a shard. This works for initial provisioning and manual recovery, but it has drawbacks:
- The workflow must enumerate every resource and dispatch an activity per resource per node -- O(resources * nodes) activity calls.
- It is shard-wide and all-or-nothing; a single node's problem triggers work on all nodes.
- It cannot detect purely local problems (crashed PHP-FPM, missing nginx config, dead Valkey process).
- There is no ongoing health monitoring -- the control plane has no visibility into whether a node's services are actually running.

This plan introduces agent-side reconciliation: the node-agent itself becomes a control loop that continuously ensures local state matches desired state, reports health, and fixes drift autonomously.

---

## Architecture Overview

```
                     +-------------------+
                     |    core-api       |
                     |  (desired state)  |
                     +--------+----------+
                              |
                    REST API  |  (read desired state,
                    with mTLS |   report health)
                              |
              +---------------+---------------+
              |               |               |
        +-----+-----+  +-----+-----+  +------+----+
        | node-agent |  | node-agent |  | node-agent|
        | (web-01)   |  | (web-02)   |  | (db-01)   |
        +-----+------+  +-----+------+  +-----+-----+
              |                |               |
        local reconcile  local reconcile  local reconcile
        loop (60s)       loop (60s)       loop (60s)
```

Each node-agent runs two independent loops alongside its existing Temporal worker:

1. **Reconciliation loop** -- fetches desired state from core-api, compares with local disk/process state, fixes drift.
2. **Health reporter** -- collects local health metrics and posts them to core-api.

Both loops are goroutines started alongside `worker.Run()` in `cmd/node-agent/main.go`. The Temporal worker continues to handle on-demand activities as before -- reconciliation is additive, not a replacement.

---

## 1. Startup Reconciliation

### Behavior

When the node-agent process starts, before accepting Temporal activities, it performs a full reconciliation pass. This ensures the node is in a known-good state before it begins processing new work.

### Sequence

1. Agent reads its `NODE_ID`, `SHARD_NAME`, `NODE_ROLE` from environment (already available in `config.Config`).
2. Agent calls `GET /internal/v1/nodes/{nodeID}/desired-state` on core-api to fetch the full desired state for this node.
3. Agent scans local disk/process state to build an inventory of what currently exists.
4. Agent computes the diff and applies convergence operations.
5. Agent reports the result (success, partial failure, full failure) to core-api via `POST /internal/v1/nodes/{nodeID}/reconciliation-report`.
6. Agent begins accepting Temporal activities.

### Why Block on Startup

If a node reboots and immediately starts accepting Temporal activities while its local state is broken (e.g., nginx not running, missing configs), incoming activities will fail and trigger retries. By reconciling first, we ensure the node is healthy before it advertises availability.

Implementation: delay `w.Run()` until after the first reconciliation pass completes (or times out after a configurable deadline, default 5 minutes).

### Startup Reconciliation Timeout

If reconciliation takes too long or core-api is unreachable, the agent should start anyway after a deadline. A node that can process some activities is better than a node that is completely offline. Log a prominent warning and report degraded health.

```go
// In cmd/node-agent/main.go, before w.Run():
reconciler := agent.NewReconciler(logger, agentCfg, srv, coreAPIClient)

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

if err := reconciler.FullReconcile(ctx); err != nil {
    logger.Error().Err(err).Msg("startup reconciliation failed, proceeding anyway")
}
```

---

## 2. Periodic Drift Detection

### Loop Design

After startup, the reconciliation loop runs on a fixed interval (default: 60 seconds, configurable via `RECONCILE_INTERVAL`).

```go
func (r *Reconciler) RunLoop(ctx context.Context) {
    ticker := time.NewTicker(r.interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := r.FullReconcile(ctx); err != nil {
                r.logger.Error().Err(err).Msg("periodic reconciliation failed")
            }
        }
    }
}
```

### Per-Role Reconciliation

The reconciliation logic differs by shard role. The `Reconciler` dispatches to role-specific reconcilers based on `NODE_ROLE`:

| Role | What to reconcile |
|------|-------------------|
| `web` | Linux users, webroot dirs, nginx configs, runtime processes (PHP-FPM, Node, Python, Ruby), SSH configs, SSL certs |
| `database` | MySQL databases, MySQL users |
| `valkey` | Valkey instance configs + processes, Valkey ACL users |
| `lb` | HAProxy FQDN-to-backend map entries |
| `storage` | S3 buckets, RGW users, access keys |
| `dns` | No local reconciliation needed (PowerDNS DB managed by worker) |
| `email` | No local reconciliation needed (Stalwart managed by worker) |

### Web Shard Reconciliation (Most Complex)

This is the most critical reconciler because web shards host the most diverse set of resources.

#### Step 1: Fetch Desired State

Call `GET /internal/v1/nodes/{nodeID}/desired-state` which returns:

```json
{
  "shard_role": "web",
  "tenants": [
    {
      "id": "acme-corp",
      "uid": 5001,
      "sftp_enabled": true,
      "ssh_enabled": false,
      "webroots": [
        {
          "id": "wr-123",
          "name": "main",
          "runtime": "php",
          "runtime_version": "8.5",
          "runtime_config": "{}",
          "public_folder": "public",
          "fqdns": [
            {"fqdn": "acme.example.com", "ssl_enabled": true}
          ]
        }
      ]
    }
  ]
}
```

#### Step 2: Scan Local State

Build an inventory by scanning the filesystem and checking processes:

```go
type LocalState struct {
    // Linux users found via /etc/passwd or `id` checks
    Users map[string]bool
    // Directories found under /var/www/storage/{tenant}/webroots/
    Webroots map[string]map[string]bool  // tenant -> set of webroot names
    // Nginx configs found in /etc/nginx/sites-enabled/
    NginxConfigs map[string]bool  // "{tenant}_{webroot}.conf"
    // Running PHP-FPM pools (via socket file existence or systemctl)
    RuntimeProcesses map[string]bool  // "{tenant}-{runtime}{version}.sock" or service name
    // SSH configs found in /etc/ssh/sshd_config.d/
    SSHConfigs map[string]bool  // "tenant-{name}.conf"
    // SSL certs found in /etc/ssl/hosting/
    SSLCerts map[string]bool  // FQDN directories with fullchain.pem + privkey.pem
}
```

Scanning methods (all lightweight, no external API calls):
- **Users**: `id {tenantName}` -- exit code 0 means exists.
- **Webroot dirs**: `os.ReadDir("/var/www/storage/{tenant}/webroots/")`.
- **Nginx configs**: `os.ReadDir("/etc/nginx/sites-enabled/")` filtering `*_*.conf`.
- **Runtime processes**: Check socket file existence or `systemctl is-active`.
- **SSH configs**: `os.ReadDir("/etc/ssh/sshd_config.d/")` filtering `tenant-*.conf`.
- **SSL certs**: `os.ReadDir("/etc/ssl/hosting/")` checking for `fullchain.pem` + `privkey.pem`.

#### Step 3: Compute Diff

```go
type DiffResult struct {
    // Resources that should exist but don't
    Missing []DiffEntry
    // Resources that exist but shouldn't
    Orphaned []DiffEntry
    // Resources that exist but have wrong config (e.g., stale nginx config)
    Drifted []DiffEntry
}

type DiffEntry struct {
    Kind     string  // "tenant", "webroot", "nginx_config", "runtime", "ssh_config", "ssl_cert"
    Tenant   string
    Resource string
    Detail   string
}
```

#### Step 4: Apply Fixes

See section 4 (Reconciliation Strategy) for the decision logic on what to fix automatically.

### Database Shard Reconciliation

1. Fetch desired databases + users from core-api.
2. Query local MySQL: `SHOW DATABASES` to list existing databases.
3. For each desired database: `CREATE DATABASE IF NOT EXISTS` (already idempotent).
4. For each desired user: verify grants match, re-grant if drifted.
5. Orphan detection: databases that exist locally but not in desired state are logged but NOT dropped (data loss guard rail).

### Valkey Shard Reconciliation

1. Fetch desired instances + users from core-api.
2. Scan `/etc/valkey/*.conf` for existing instance configs.
3. For each desired instance: ensure config file exists, process is running, ACL users match.
4. For missing instances: create config, start process (reuse `ValkeyManager.CreateInstance` which is already idempotent).
5. For orphaned instances: log warning, do NOT auto-delete (data loss guard rail).

### LB Shard Reconciliation

1. Fetch desired FQDN mappings from core-api.
2. Read current map file from `/var/lib/haproxy/maps/fqdn-to-shard.map`.
3. Add missing entries, remove orphaned entries via HAProxy Runtime API.
4. LB reconciliation is safe to auto-fix because map entries are stateless metadata.

### Storage Shard Reconciliation

1. Fetch desired S3 buckets + access keys from core-api.
2. List existing RGW users via `radosgw-admin user list`.
3. For each desired bucket: ensure bucket exists, policy is correct.
4. Orphaned buckets: log warning, do NOT auto-delete (data loss guard rail).

---

## 3. Health Reporting

### What to Report

The agent collects health metrics locally and reports them to core-api every 30 seconds (separate from the 60-second reconciliation loop).

#### Health Report Payload

```json
{
  "node_id": "abc-123",
  "timestamp": "2026-02-15T10:30:00Z",
  "status": "healthy",
  "checks": {
    "nginx": {
      "status": "healthy",
      "pid": 1234,
      "config_test": "ok",
      "active_connections": 42
    },
    "php_fpm": {
      "status": "healthy",
      "pools_running": 15,
      "pools_expected": 15
    },
    "disk": {
      "status": "healthy",
      "total_bytes": 107374182400,
      "used_bytes": 42949672960,
      "usage_percent": 40.0
    },
    "ceph_mount": {
      "status": "healthy",
      "mount_point": "/var/www/storage",
      "mounted": true
    },
    "memory": {
      "status": "healthy",
      "total_bytes": 8589934592,
      "used_bytes": 4294967296,
      "usage_percent": 50.0
    },
    "load": {
      "load_1m": 0.5,
      "load_5m": 0.3,
      "load_15m": 0.2
    },
    "temporal": {
      "status": "healthy",
      "connected": true
    }
  },
  "reconciliation": {
    "last_run": "2026-02-15T10:29:30Z",
    "last_result": "success",
    "drift_detected": 0,
    "drift_fixed": 0,
    "drift_unfixed": 0
  }
}
```

#### Per-Role Checks

| Role | Checks |
|------|--------|
| `web` | nginx status, PHP-FPM pool count, CephFS mount, disk usage, runtime process count |
| `database` | MySQL process running, replication status (if replica), disk usage |
| `valkey` | Valkey instance count, per-instance PING, memory usage |
| `lb` | HAProxy process, Runtime API reachable, map file size |
| `storage` | Ceph health (`ceph health`), RGW process, disk usage |

#### Health Collection Methods

All checks use lightweight local operations:
- **nginx**: `nginx -t` exit code + read `/run/nginx.pid`.
- **PHP-FPM**: count socket files in `/run/php/` or `systemctl list-units 'php*'`.
- **Disk**: `syscall.Statfs()` on relevant mount points.
- **CephFS mount**: check `/proc/mounts` for the storage mount point.
- **Memory**: read `/proc/meminfo`.
- **Load**: read `/proc/loadavg`.
- **MySQL**: `mysqladmin ping`.
- **Valkey**: `valkey-cli PING` per instance.
- **HAProxy**: TCP connect to Runtime API port 9999.
- **Temporal**: check the existing `tc.CheckHealth()` call.

### Health Status Derivation

The overall node status is derived from individual check statuses:

- `healthy` -- all checks pass.
- `degraded` -- some non-critical checks fail (e.g., one PHP-FPM pool is down out of 15).
- `unhealthy` -- critical checks fail (e.g., nginx is down, CephFS is unmounted, MySQL is unreachable).

---

## 4. Reconciliation Strategy

### Core Principle: Auto-Fix Safe Operations, Report Unsafe Ones

Not all drift is safe to fix automatically. The strategy differentiates between "safe" operations (idempotent, no data risk) and "unsafe" operations (data risk, ambiguous intent).

### Auto-Fix (Safe Operations)

These operations are idempotent and carry no data-loss risk. The agent fixes them silently and logs the action:

| Operation | Shard Role | Rationale |
|-----------|------------|-----------|
| Create missing Linux user | web | `useradd` is safe, user might have been lost to image re-provision |
| Create missing webroot directory | web | `mkdir` is safe, files live on CephFS (already replicated) |
| Write missing/stale nginx config | web | Generated from desired state, always safe to overwrite |
| Reload nginx | web | Safe after config is fixed |
| Start stopped PHP-FPM/Node/Python/Ruby runtime | web | Process crashed or was killed, restart is expected |
| Reconfigure runtime (pool config file) | web | Generated from desired state, safe to overwrite |
| Write missing SSH config | web | Generated from desired state |
| Create missing MySQL database | database | `CREATE DATABASE IF NOT EXISTS` is idempotent |
| Re-grant MySQL user privileges | database | `GRANT` is idempotent |
| Create missing Valkey instance | valkey | Config + start is idempotent |
| Recreate missing Valkey ACL user | valkey | ACL SETUSER is idempotent |
| Set missing LB map entry | lb | Stateless metadata |
| Remove orphaned LB map entry | lb | Stateless metadata, safe to remove |
| Remove orphaned nginx config | web | Already implemented in convergence workflow |

### Report-Only (Unsafe Operations)

These operations are logged and reported to core-api but NOT automatically executed. An operator must intervene via manual convergence (`POST /shards/{id}/converge`) or explicit action:

| Observation | Shard Role | Rationale |
|-------------|------------|-----------|
| Orphaned Linux user (exists locally, not in desired state) | web | May have in-flight tenant data; deleting could destroy files |
| Orphaned webroot directory | web | May contain user data not yet backed up |
| Orphaned MySQL database | database | Dropping a database is irreversible data loss |
| Orphaned Valkey instance | valkey | May contain data not yet backed up |
| Orphaned S3 bucket | storage | May contain user data |
| CephFS unmounted | web | May indicate a cluster-wide Ceph problem; remounting blindly could cause issues |
| MySQL replication broken | database | Requires operator judgment (skip vs reset) |
| Nginx config test failure after fix attempt | web | Something deeper is wrong; do not force reload |

### Why Not Auto-Delete Orphans

Orphaned resources are the most dangerous category. An orphan might exist because:
1. A delete workflow is in progress but hasn't reached this node yet.
2. A create workflow partially succeeded and the DB rolled back, but the local resource was already created.
3. A migration moved the resource to a different shard but cleanup hasn't run yet.

Auto-deleting any of these could cause data loss. The safe default is to report orphans and let the platform operator decide.

### Drift Event Log

Every drift detection and fix is logged to a structured local log and reported to core-api:

```go
type DriftEvent struct {
    Timestamp time.Time `json:"timestamp"`
    NodeID    string    `json:"node_id"`
    Kind      string    `json:"kind"`       // "tenant", "webroot", "nginx_config", etc.
    Resource  string    `json:"resource"`   // e.g., "acme-corp/main"
    Action    string    `json:"action"`     // "auto_fixed", "reported", "skipped"
    Detail    string    `json:"detail"`     // human-readable description
}
```

---

## 5. API for Agent Reporting

### New Internal API Endpoints

These endpoints live under `/internal/v1/` and are authenticated with a separate mechanism (node agent tokens, not user API keys). They are NOT exposed to external consumers.

#### Authentication: Node Agent Tokens

Each node-agent authenticates to core-api using a pre-shared token set via environment variable (`CORE_API_TOKEN`). This token is a long-lived bearer token stored in the core DB's `nodes` table (hashed). It is generated during node registration and injected via cloud-init or Terraform.

The `/internal/v1/` routes use a separate auth middleware that validates the node token and extracts the node ID from the token claims or a lookup.

Alternatively (simpler, recommended for v1): use the existing API key system with a dedicated "node-agent" API key per node, scoped to only the internal endpoints. This avoids building a separate auth system.

#### `GET /internal/v1/nodes/{nodeID}/desired-state`

Returns the complete desired state for a node, structured by shard role.

**Response** (web shard example):

```json
{
  "node_id": "abc-123",
  "shard_id": "shard-web-01",
  "shard_role": "web",
  "tenants": [
    {
      "id": "acme-corp",
      "uid": 5001,
      "sftp_enabled": true,
      "ssh_enabled": false,
      "status": "active",
      "webroots": [
        {
          "id": "wr-123",
          "name": "main",
          "runtime": "php",
          "runtime_version": "8.5",
          "runtime_config": "{}",
          "public_folder": "public",
          "status": "active",
          "fqdns": [
            {
              "fqdn": "acme.example.com",
              "ssl_enabled": true,
              "status": "active"
            }
          ]
        }
      ],
      "ssh_keys": ["ssh-ed25519 AAAA... user@host"]
    }
  ]
}
```

**Response** (database shard example):

```json
{
  "node_id": "def-456",
  "shard_id": "shard-db-01",
  "shard_role": "database",
  "databases": [
    {
      "id": "db-789",
      "name": "acme_production",
      "status": "active",
      "users": [
        {
          "id": "dbu-012",
          "username": "acme_app",
          "password": "...",
          "privileges": ["ALL PRIVILEGES"],
          "status": "active"
        }
      ]
    }
  ]
}
```

**Response** (valkey shard example):

```json
{
  "node_id": "ghi-789",
  "shard_id": "shard-valkey-01",
  "shard_role": "valkey",
  "valkey_instances": [
    {
      "id": "vi-345",
      "name": "acme_cache",
      "port": 6380,
      "password": "...",
      "max_memory_mb": 256,
      "status": "active",
      "users": [
        {
          "id": "vu-678",
          "username": "acme_app",
          "password": "...",
          "privileges": ["+@all"],
          "key_pattern": "~*",
          "status": "active"
        }
      ]
    }
  ]
}
```

**Response** (lb shard example):

```json
{
  "node_id": "jkl-012",
  "shard_id": "shard-lb-01",
  "shard_role": "lb",
  "fqdn_mappings": [
    {"fqdn": "acme.example.com", "lb_backend": "shard-web-01"},
    {"fqdn": "blog.example.com", "lb_backend": "shard-web-01"}
  ]
}
```

**Implementation**: This endpoint queries the same tables as the existing `ConvergeShardWorkflow` activities (`ListTenantsByShard`, `ListWebrootsByTenantID`, etc.) but returns the data directly via HTTP instead of through Temporal activities. The service layer method can be shared.

**Performance consideration**: For shards with thousands of tenants, this response could be large. Use ETag/If-None-Match caching so that if nothing changed since the last poll, the agent gets a `304 Not Modified` with zero body. The ETag can be derived from the shard's `updated_at` timestamp or a hash of the response.

#### `POST /internal/v1/nodes/{nodeID}/health`

Accepts the health report payload described in section 3.

**Behavior**:
- Upserts into a `node_health` table (one row per node, overwritten each report).
- Updates `nodes.last_health_at` timestamp.
- If status transitions to `unhealthy`, the core-api could emit an alert (future: webhook/notification system).

**Schema addition** (`node_health` table):

```sql
CREATE TABLE node_health (
    node_id       TEXT PRIMARY KEY REFERENCES nodes(id),
    status        TEXT NOT NULL,       -- healthy, degraded, unhealthy
    checks        JSONB NOT NULL,      -- full health check payload
    reconciliation JSONB,              -- last reconciliation summary
    reported_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Also add to existing `nodes` table:

```sql
ALTER TABLE nodes ADD COLUMN last_health_at TIMESTAMPTZ;
```

(Per migration policy: edit the original migration file, wipe/restart DB.)

#### `POST /internal/v1/nodes/{nodeID}/drift-events`

Accepts a batch of drift events for logging and alerting.

**Request**:

```json
{
  "events": [
    {
      "timestamp": "2026-02-15T10:30:00Z",
      "kind": "nginx_config",
      "resource": "acme-corp/main",
      "action": "auto_fixed",
      "detail": "nginx config was missing, regenerated from desired state"
    }
  ]
}
```

**Behavior**:
- Stores events in a `drift_events` table for audit/observability.
- Events are immutable (append-only).
- Retention: 30 days (same as backup retention, configurable).

**Schema** (`drift_events` table):

```sql
CREATE TABLE drift_events (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id    TEXT NOT NULL REFERENCES nodes(id),
    kind       TEXT NOT NULL,
    resource   TEXT NOT NULL,
    action     TEXT NOT NULL,
    detail     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_drift_events_node_id ON drift_events(node_id);
CREATE INDEX idx_drift_events_created_at ON drift_events(created_at);
```

---

## 6. Guard Rails

### Preventing Reconciliation Loops

**Problem**: If reconciliation triggers a Temporal workflow (e.g., "I found drift, let me re-converge"), and that workflow dispatches activities back to the same node, which triggers another reconciliation, we get an infinite loop.

**Solution**: Reconciliation NEVER triggers Temporal workflows. It only uses local manager operations (same code path as activities, but called directly). The reconciliation loop and the Temporal worker are independent -- they share the same manager instances but do not invoke each other.

### Handling Partially-Provisioned Resources

**Problem**: A resource might be in `provisioning` or `deleting` status in the core DB. The reconciliation loop should not interfere with in-flight workflows.

**Solution**: The desired state endpoint only returns resources with `status = 'active'` (or `status = 'suspended'` for tenants). Resources in transitional states (`pending`, `provisioning`, `converging`, `deleting`) are excluded from the desired state response. This means:

- A resource being provisioned will not appear in desired state until the workflow sets it to `active`.
- A resource being deleted will disappear from desired state as soon as the workflow sets it to `deleting` (and certainly by `deleted`).
- The reconciler will never try to create a resource that is still being provisioned by a workflow.
- The reconciler will never try to delete a resource that the control plane hasn't explicitly removed from desired state.

### Race Condition: Workflow Activity vs. Reconciliation

**Problem**: A Temporal activity and the reconciliation loop might try to modify the same resource simultaneously (e.g., both trying to write an nginx config).

**Solution**: Use a per-resource mutex within the agent process.

```go
type Reconciler struct {
    mu sync.Map  // key: "{kind}:{tenant}:{resource}", value: *sync.Mutex
}

func (r *Reconciler) lockResource(kind, tenant, resource string) func() {
    key := kind + ":" + tenant + ":" + resource
    mu, _ := r.mu.LoadOrStore(key, &sync.Mutex{})
    mu.(*sync.Mutex).Lock()
    return mu.(*sync.Mutex).Unlock
}
```

Activities should acquire the same lock before operating on resources. Since both run in the same process, this provides process-level mutual exclusion.

### Rate Limiting API Calls

**Problem**: If 100 nodes in a cluster all poll core-api every 60 seconds, that is ~100 requests/minute to the desired-state endpoint.

**Solutions**:
- **Jitter**: Each agent adds random jitter (0-30s) to its reconciliation interval to spread load.
- **ETag caching**: The desired-state endpoint returns an ETag. Agents send `If-None-Match` on subsequent requests. If nothing changed, the server returns `304 Not Modified` with no body, which is extremely cheap.
- **Backoff on failure**: If the API call fails, the agent backs off exponentially (60s -> 120s -> 240s -> max 600s) before retrying.
- **Separate health and reconciliation intervals**: Health reports (30s) are tiny payloads. Reconciliation (60s) uses ETag caching. Neither is expensive.

At scale (10,000 nodes), with ETag caching and jitter, the desired-state endpoint would see ~10,000 requests/minute, but ~95% of those would be `304 Not Modified` responses served from a simple ETag comparison. This is well within the capacity of a single PostgreSQL-backed API server.

### Guard Against Aggressive Auto-Fixing

**Problem**: If the desired state itself is wrong (e.g., a bug in core-api returns stale data), the reconciler could destructively "fix" things that are actually correct.

**Solutions**:
- **Never auto-delete user data**: Orphan detection is report-only (see section 4).
- **Max fixes per cycle**: The reconciler limits itself to a configurable maximum number of auto-fix operations per cycle (default: 50). If more drift is detected, it fixes the first 50 and reports the rest. This prevents a thundering-herd scenario where a bad desired-state response causes mass reconfiguration.
- **Circuit breaker**: If 3 consecutive reconciliation cycles each detect more than `MAX_DRIFT_THRESHOLD` (default: 100) drifted resources, the reconciler enters "report-only" mode and stops auto-fixing until an operator manually resets it (via a signal or API call). This protects against cascading failures from a bad control-plane state.

### Handling Core-API Unreachability

If core-api is unreachable:
- The reconciler skips the current cycle and retries next interval with backoff.
- The agent continues running its Temporal worker normally (Temporal has its own connection management).
- Health reporting also skips but logs locally.
- After 5 consecutive failures, the agent logs at ERROR level and exposes a Prometheus metric (`node_agent_reconcile_failures_total`).
- The agent NEVER makes local changes when it cannot fetch desired state. No desired state = no reconciliation. This prevents the agent from acting on stale data.

### Preventing Conflict with ConvergeShardWorkflow

The existing `ConvergeShardWorkflow` pushes state to nodes via Temporal activities. With agent-side reconciliation, there are now two paths to convergence. They should not conflict:

- **ConvergeShardWorkflow continues to exist** for manual, operator-triggered full convergence. It is useful for "I just changed something in the DB, push it now" scenarios without waiting for the next reconciliation cycle.
- **Both paths use the same manager code**: The activity methods (`CreateTenant`, `CreateWebroot`, etc.) and the reconciler both call the same manager methods (`tenant.Create()`, `webroot.Create()`, etc.), which are all idempotent. Running both simultaneously on the same resource is safe.
- **The per-resource mutex** (described above) prevents them from stepping on each other's file writes.

---

## 7. New Components

### `internal/agent/reconciler.go`

The main reconciliation coordinator. Implements `FullReconcile()` which:
1. Fetches desired state from core-api.
2. Delegates to role-specific reconcilers.
3. Collects drift events.
4. Reports results.

### `internal/agent/reconcile_web.go`

Web shard reconciliation: tenants, webroots, nginx, runtimes, SSH, SSL certs.

### `internal/agent/reconcile_database.go`

Database shard reconciliation: MySQL databases and users.

### `internal/agent/reconcile_valkey.go`

Valkey shard reconciliation: instances and ACL users.

### `internal/agent/reconcile_lb.go`

LB shard reconciliation: HAProxy map entries.

### `internal/agent/reconcile_storage.go`

Storage shard reconciliation: S3 buckets and access keys.

### `internal/agent/health.go`

Health check collection and reporting.

### `internal/agent/apiclient.go`

HTTP client for core-api internal endpoints. Handles authentication, ETag caching, retries with backoff, and deserialization of desired-state responses.

### `internal/api/handler/internal_node.go`

Handlers for the `/internal/v1/nodes/` endpoints.

### `internal/core/node_health.go`

Core service layer for node health and drift event storage.

---

## 8. Configuration

New environment variables for the node-agent:

| Variable | Default | Description |
|----------|---------|-------------|
| `CORE_API_URL` | `""` | Base URL of the core-api (e.g., `https://api.hosting.test`). Required for reconciliation. |
| `CORE_API_TOKEN` | `""` | Bearer token for authenticating to core-api internal endpoints. |
| `RECONCILE_ENABLED` | `true` | Enable/disable the reconciliation loop. |
| `RECONCILE_INTERVAL` | `60s` | Interval between reconciliation cycles. |
| `RECONCILE_STARTUP_TIMEOUT` | `5m` | Maximum time to wait for startup reconciliation. |
| `HEALTH_REPORT_INTERVAL` | `30s` | Interval between health reports. |
| `RECONCILE_MAX_FIXES` | `50` | Maximum auto-fix operations per cycle. |
| `RECONCILE_CIRCUIT_THRESHOLD` | `100` | Drift count that triggers circuit breaker after 3 consecutive cycles. |

Per Helm chart sync rules: add these to `deploy/helm/hosting/values.yaml` and reference them in `templates/configmap.yaml` for node-agent pods.

---

## 9. Observability

### Prometheus Metrics

New metrics exposed on the node-agent's `/metrics` endpoint:

| Metric | Type | Description |
|--------|------|-------------|
| `node_agent_reconcile_duration_seconds` | Histogram | Duration of each reconciliation cycle |
| `node_agent_reconcile_total` | Counter | Total reconciliation cycles (label: result=success/failure) |
| `node_agent_drift_detected_total` | Counter | Total drift events detected (labels: kind, action) |
| `node_agent_drift_fixed_total` | Counter | Total drift events auto-fixed (labels: kind) |
| `node_agent_health_report_total` | Counter | Total health reports sent (label: result=success/failure) |
| `node_agent_health_status` | Gauge | Current health status (1=healthy, 0.5=degraded, 0=unhealthy) |
| `node_agent_reconcile_failures_total` | Counter | Consecutive reconciliation failures (resets on success) |
| `node_agent_desired_state_fetch_seconds` | Histogram | Time to fetch desired state from core-api |
| `node_agent_circuit_breaker_open` | Gauge | 1 if circuit breaker is open, 0 otherwise |

### Structured Logging

All reconciliation actions are logged with structured fields:

```json
{
  "level": "info",
  "component": "reconciler",
  "kind": "nginx_config",
  "tenant": "acme-corp",
  "resource": "main",
  "action": "auto_fixed",
  "msg": "regenerated missing nginx config from desired state"
}
```

### Admin UI Integration

The admin dashboard should surface:
- Per-node health status (green/yellow/red).
- Last reconciliation time and result.
- Recent drift events.
- Circuit breaker status.

This data is already available via the `node_health` table and `drift_events` table, queryable through new admin API endpoints.

---

## 10. Rollout Plan

### Phase 1: Health Reporting Only

1. Implement the health collection (`internal/agent/health.go`).
2. Add `POST /internal/v1/nodes/{nodeID}/health` endpoint.
3. Add `node_health` table.
4. Deploy to all nodes. Observe health data flowing in.
5. No reconciliation yet -- this phase is read-only from the node's perspective.

### Phase 2: Read-Only Drift Detection

1. Implement the desired-state endpoint (`GET /internal/v1/nodes/{nodeID}/desired-state`).
2. Implement local state scanning.
3. Implement diff computation.
4. Run the reconciliation loop in **report-only mode** (detect drift, log it, report it, but fix nothing).
5. Add `drift_events` table and endpoint.
6. Observe what drift is actually being detected in production. Tune thresholds.

### Phase 3: Auto-Fix Safe Operations

1. Enable auto-fix for the safest operations first: nginx configs, runtime process restarts.
2. Monitor for false positives or unexpected behavior.
3. Gradually enable more auto-fix categories.

### Phase 4: Startup Reconciliation

1. Enable blocking startup reconciliation.
2. Test node reboot scenarios.
3. Verify that nodes come up healthy and ready to serve within the startup timeout.

### Phase 5: Circuit Breaker and Production Hardening

1. Enable circuit breaker.
2. Load test with large shard sizes (1000+ tenants per shard).
3. Tune intervals, max-fixes, and thresholds based on production data.
4. Add admin UI panels for drift visibility.

---

## 11. Relationship to Existing ConvergeShardWorkflow

The agent-side reconciliation does NOT replace `ConvergeShardWorkflow`. They serve complementary purposes:

| Aspect | ConvergeShardWorkflow | Agent Reconciliation |
|--------|----------------------|----------------------|
| Trigger | Manual (`POST /shards/{id}/converge`) or new node join | Automatic (periodic timer) |
| Scope | All nodes in a shard | Single node (self) |
| Coordination | Temporal (centralized) | Local (no coordination needed) |
| Use case | "Push everything now" | "Is my local state correct?" |
| Data flow | Core DB -> Temporal -> Activities -> Node | Core API -> Agent -> Local managers |

Over time, as agent reconciliation proves reliable, the `ConvergeShardWorkflow` may become less frequently needed -- it becomes the "big hammer" for when you need guaranteed immediate convergence across all nodes, while agent reconciliation handles the steady-state self-healing.

---

## 12. Open Questions

1. **Should the reconciler handle SSL certificate drift?** Certificates require the private key, which is not stored in the core DB (only on disk after installation). The desired-state endpoint would need to include certificate PEM data, which has security implications. Alternative: certificates are only pushed via workflows and the reconciler only checks for their existence, not content correctness.

2. **Should database password drift be checked?** Verifying a MySQL user's password requires attempting authentication. The reconciler could simply re-run `CREATE USER ... IDENTIFIED BY` (idempotent via `IF NOT EXISTS` + `ALTER USER`), but this changes the password on every cycle if it drifted. Desired-state responses would need to include plaintext passwords, which requires careful transport security (mTLS between agent and core-api).

3. **Should we implement a "dry run" mode?** A mode where the reconciler computes the diff and logs what it would do, without actually doing it. Useful for initial deployment and debugging. Recommendation: yes, controlled by `RECONCILE_DRY_RUN=true`.

4. **Multi-node CephFS coordination**: On web shards with 2-3 nodes sharing CephFS, directory creation by one node's reconciler is visible to all nodes. Should only one node be the "leader" for directory operations? Recommendation: no, since all operations are idempotent, all nodes can safely reconcile independently. The CephFS layer handles concurrent access.

5. **Notification system**: When should drift events trigger operator notifications (email, Slack, PagerDuty)? This is out of scope for the initial implementation but the `drift_events` table provides the foundation. A future webhook/alerting system can query this table.
