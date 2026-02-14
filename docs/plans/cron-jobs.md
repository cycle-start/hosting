# Tenant Cron Jobs — Implementation Plan

## Overview

Cron jobs allow tenants to schedule recurring commands (e.g., `php artisan schedule:run`, database cleanup scripts, queue workers) that execute on their webroot's runtime environment. Each cron job belongs to a specific webroot, runs as the tenant's Linux user, and operates within the webroot's CephFS storage directory.

Implementation uses systemd timers on web shard nodes, with deterministic single-node execution to prevent duplicate runs across the shard.

---

## 1. Data Model

### 1.1 Core DB Table: `cron_jobs`

Migration file: `migrations/core/00035_cron_jobs.sql`

```sql
-- +goose Up
CREATE TABLE cron_jobs (
    id                TEXT PRIMARY KEY,
    tenant_id         TEXT NOT NULL REFERENCES tenants(id),
    webroot_id        TEXT NOT NULL REFERENCES webroots(id),
    name              TEXT NOT NULL,
    schedule          TEXT NOT NULL,
    command           TEXT NOT NULL,
    working_directory TEXT NOT NULL DEFAULT '',
    enabled           BOOLEAN NOT NULL DEFAULT false,
    timeout_seconds   INT NOT NULL DEFAULT 3600,
    max_memory_mb     INT NOT NULL DEFAULT 512,
    status            TEXT NOT NULL DEFAULT 'pending',
    status_message    TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(webroot_id, name)
);

CREATE INDEX idx_cron_jobs_tenant_id ON cron_jobs(tenant_id);
CREATE INDEX idx_cron_jobs_webroot_id ON cron_jobs(webroot_id);

-- +goose Down
DROP TABLE cron_jobs;
```

**Field notes:**

| Field | Description |
|---|---|
| `id` | Short ID (platform.NewShortID), e.g. `cj_a1b2c3d4` |
| `tenant_id` | Denormalized from the webroot for efficient queries. Set on creation, never updated independently. |
| `webroot_id` | The webroot this cron job belongs to. Defines runtime context (user, directory, runtime version). |
| `name` | Human-readable slug for the cron job (e.g., `artisan-schedule`). Unique per webroot. |
| `schedule` | Standard 5-field cron expression (e.g., `*/5 * * * *`). Validated server-side. |
| `command` | The shell command to execute (e.g., `php artisan schedule:run`). Max 4096 chars. |
| `working_directory` | Optional subfolder within the webroot. Empty string means the webroot root directory. Path-traversal validated. |
| `enabled` | Whether the systemd timer is active. Created as `false` by default, must be explicitly enabled. |
| `timeout_seconds` | Max execution time before the job is killed. Default 3600 (1 hour), max 86400 (24 hours). |
| `max_memory_mb` | Memory limit for the cron process. Default 512 MB, max 4096 MB. |
| `status` | Standard resource status: pending, provisioning, active, failed, deleting, deleted. |
| `status_message` | Error message when status is `failed`. |

### 1.2 Go Model

File: `internal/model/cron_job.go`

```go
package model

import "time"

type CronJob struct {
    ID               string    `json:"id" db:"id"`
    TenantID         string    `json:"tenant_id" db:"tenant_id"`
    WebrootID        string    `json:"webroot_id" db:"webroot_id"`
    Name             string    `json:"name" db:"name"`
    Schedule         string    `json:"schedule" db:"schedule"`
    Command          string    `json:"command" db:"command"`
    WorkingDirectory string    `json:"working_directory" db:"working_directory"`
    Enabled          bool      `json:"enabled" db:"enabled"`
    TimeoutSeconds   int       `json:"timeout_seconds" db:"timeout_seconds"`
    MaxMemoryMB      int       `json:"max_memory_mb" db:"max_memory_mb"`
    Status           string    `json:"status" db:"status"`
    StatusMessage    *string   `json:"status_message,omitempty" db:"status_message"`
    CreatedAt        time.Time `json:"created_at" db:"created_at"`
    UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}
```

### 1.3 TenantResourceSummary

Add `CronJobs ResourceStatusCounts` to `TenantResourceSummary` in `internal/model/tenant.go`, and update the dashboard query in `internal/core/dashboard.go` to include the `cron_jobs` table.

---

## 2. API Design

### 2.1 Request Types

File: `internal/api/request/cron_job.go`

```go
package request

type CreateCronJob struct {
    Name             string `json:"name" validate:"required,slug"`
    Schedule         string `json:"schedule" validate:"required"`
    Command          string `json:"command" validate:"required,max=4096"`
    WorkingDirectory string `json:"working_directory" validate:"omitempty,max=255"`
    TimeoutSeconds   int    `json:"timeout_seconds" validate:"omitempty,min=1,max=86400"`
    MaxMemoryMB      int    `json:"max_memory_mb" validate:"omitempty,min=16,max=4096"`
}

type UpdateCronJob struct {
    Schedule         *string `json:"schedule" validate:"omitempty"`
    Command          *string `json:"command" validate:"omitempty,max=4096"`
    WorkingDirectory *string `json:"working_directory" validate:"omitempty,max=255"`
    TimeoutSeconds   *int    `json:"timeout_seconds" validate:"omitempty,min=1,max=86400"`
    MaxMemoryMB      *int    `json:"max_memory_mb" validate:"omitempty,min=16,max=4096"`
}
```

**Cron expression validation:** Add a custom `validate:"cron"` tag that uses a Go cron parser (e.g., `github.com/robfig/cron/v3`) to validate the 5-field expression. Reject `@every`, `@yearly`, etc. aliases to keep expressions explicit and auditable. Minimum interval: 1 minute (`* * * * *`). No seconds field.

**Working directory validation:** The handler must verify that the `working_directory` value does not contain `..`, does not start with `/`, and contains no null bytes. The full path is always `{webStorageDir}/{tenantID}/webroots/{webrootName}/{workingDirectory}`.

### 2.2 Endpoints

All endpoints are brand-scoped and use the `cron_jobs` scope for API key authorization.

**Nested under webroot (list + create):**

| Method | Path | Handler | Scope | Description |
|---|---|---|---|---|
| GET | `/webroots/{webrootID}/cron-jobs` | ListByWebroot | cron_jobs:read | Paginated list of cron jobs for a webroot |
| POST | `/webroots/{webrootID}/cron-jobs` | Create | cron_jobs:write | Create a new cron job (starts disabled) |

**Direct resource operations:**

| Method | Path | Handler | Scope | Description |
|---|---|---|---|---|
| GET | `/cron-jobs/{id}` | Get | cron_jobs:read | Get a single cron job by ID |
| PUT | `/cron-jobs/{id}` | Update | cron_jobs:write | Update schedule, command, working dir, or limits |
| DELETE | `/cron-jobs/{id}` | Delete | cron_jobs:delete | Delete a cron job (async, returns 202) |
| POST | `/cron-jobs/{id}/enable` | Enable | cron_jobs:write | Enable the cron job (starts the timer) |
| POST | `/cron-jobs/{id}/disable` | Disable | cron_jobs:write | Disable the cron job (stops the timer) |
| POST | `/cron-jobs/{id}/retry` | Retry | cron_jobs:write | Retry a failed cron job provisioning |

### 2.3 Route Registration

In `internal/api/server.go`, add alongside the existing webroot routes:

```go
cronJob := handler.NewCronJob(s.services)

// Cron jobs
r.Group(func(r chi.Router) {
    r.Use(mw.RequireScope("cron_jobs", "read"))
    r.Get("/webroots/{webrootID}/cron-jobs", cronJob.ListByWebroot)
    r.Get("/cron-jobs/{id}", cronJob.Get)
})
r.Group(func(r chi.Router) {
    r.Use(mw.RequireScope("cron_jobs", "write"))
    r.Post("/webroots/{webrootID}/cron-jobs", cronJob.Create)
    r.Put("/cron-jobs/{id}", cronJob.Update)
    r.Post("/cron-jobs/{id}/enable", cronJob.Enable)
    r.Post("/cron-jobs/{id}/disable", cronJob.Disable)
    r.Post("/cron-jobs/{id}/retry", cronJob.Retry)
})
r.Group(func(r chi.Router) {
    r.Use(mw.RequireScope("cron_jobs", "delete"))
    r.Delete("/cron-jobs/{id}", cronJob.Delete)
})
```

### 2.4 Handler

File: `internal/api/handler/cron_job.go`

Follows the same pattern as `handler/webroot.go`:

- `NewCronJob(services *core.Services)` constructor takes full services for brand checking.
- Each handler: parse/validate request -> resolve webroot ownership for brand check -> build model -> call service -> return response.
- Create returns 202 Accepted (async workflow).
- Enable/Disable return 202 Accepted (async workflow to toggle timer).
- List returns paginated response with `{items: [...], has_more: bool}`.

**Rate limiting:** The Create handler checks the count of active (non-deleted) cron jobs for the tenant. Default limit: 50 per tenant. This can be made configurable via platform config later.

### 2.5 Lifecycle Rules

- A cron job can only be created when the webroot is `active`.
- A cron job is created in `pending` + `enabled=false` state. The create workflow provisions the systemd unit files but does not start the timer.
- Enable/Disable only work when status is `active`.
- Update triggers a re-provision workflow (rewrites systemd units). If the job is enabled, the timer is restarted with the new config.
- Delete cascades: stops the timer, removes unit files, deletes the DB row.
- When a webroot is deleted, all its cron jobs must be deleted first (cascade in DeleteWebrootWorkflow).

---

## 3. Core Service

File: `internal/core/cron_job.go`

```go
type CronJobService struct {
    db DB
    tc temporalclient.Client
}
```

Methods follow the exact pattern of `WebrootService`:

| Method | DB Operation | Workflow Triggered |
|---|---|---|
| `Create` | INSERT -> signalProvision | `CreateCronJobWorkflow` |
| `GetByID` | SELECT by id | - |
| `ListByWebroot` | SELECT paginated | - |
| `Update` | UPDATE fields -> signalProvision | `UpdateCronJobWorkflow` |
| `Delete` | SET status=deleting -> signalProvision | `DeleteCronJobWorkflow` |
| `Enable` | SET enabled=true -> signalProvision | `EnableCronJobWorkflow` |
| `Disable` | SET enabled=false -> signalProvision | `DisableCronJobWorkflow` |
| `Retry` | Check status=failed -> signalProvision | `CreateCronJobWorkflow` |

Add `CronJob *CronJobService` to the `Services` struct and wire it in `NewServices()`.

Add `resolveTenantIDFromCronJob` to `internal/core/provision.go`:

```go
func resolveTenantIDFromCronJob(ctx context.Context, db DB, cronJobID string) (string, error) {
    var tenantID string
    err := db.QueryRow(ctx, "SELECT tenant_id FROM cron_jobs WHERE id = $1", cronJobID).Scan(&tenantID)
    if err != nil {
        return "", fmt.Errorf("resolve tenant from cron job %s: %w", cronJobID, err)
    }
    return tenantID, nil
}
```

---

## 4. Workflows

File: `internal/workflow/cron_job.go`

### 4.1 CreateCronJobWorkflow

1. Set status to `provisioning`.
2. Fetch `CronJobContext` (cron job + webroot + tenant + shard nodes).
3. Determine the **designated execution node** (see section 6).
4. For **each node** in the shard: call `CreateCronJobUnits` activity (writes systemd timer + service unit files).
5. On the **designated node only**: if `enabled=true`, call `EnableCronJobTimer` activity (starts the timer).
6. Set status to `active`.

### 4.2 UpdateCronJobWorkflow

1. Set status to `provisioning`.
2. Fetch `CronJobContext`.
3. Determine the designated execution node.
4. For each node: call `UpdateCronJobUnits` activity (rewrites unit files).
5. On the designated node: if enabled, call `RestartCronJobTimer` activity (reload with new schedule).
6. Set status to `active`.

### 4.3 DeleteCronJobWorkflow

1. Set status to `deleting`.
2. Fetch `CronJobContext`.
3. For each node: call `DeleteCronJobUnits` activity (stop timer if running, remove unit files).
4. Set status to `deleted`.

### 4.4 EnableCronJobWorkflow

1. Set status to `provisioning`.
2. Fetch `CronJobContext`.
3. Determine the designated execution node.
4. On the designated node: call `EnableCronJobTimer` activity.
5. Set status to `active`.

### 4.5 DisableCronJobWorkflow

1. Set status to `provisioning`.
2. Fetch `CronJobContext`.
3. For each node: call `DisableCronJobTimer` activity (stops timer on whichever node was running it).
4. Set status to `active`.

---

## 5. Node Agent Implementation (Systemd Timers)

### 5.1 Why Systemd Timers

Systemd timers are the right choice for this platform:

- **Journald integration** -- all output (stdout + stderr) is captured in the journal with structured metadata, queryable by unit name. No need to build custom log capture.
- **Per-user execution** -- the `User=` directive runs the command as the tenant's Linux user with no privilege escalation.
- **Resource limits** -- `MemoryMax=`, `CPUQuota=`, `TimeoutStopSec=` are native systemd controls. cgroups enforcement is automatic.
- **Calendar syntax** -- systemd's `OnCalendar=` supports cron-like expressions (translated from standard cron syntax).
- **Observability** -- `systemctl list-timers` shows next fire time, last trigger, etc. `systemd-run --scope` is not needed.
- **Idempotent** -- writing unit files + `systemctl daemon-reload` + `systemctl enable --now` is fully idempotent.

### 5.2 CronManager

File: `internal/agent/cron.go`

```go
type CronManager struct {
    logger        zerolog.Logger
    webStorageDir string
    unitDir       string // /etc/systemd/system
}

func NewCronManager(logger zerolog.Logger, cfg Config) *CronManager {
    return &CronManager{
        logger:        logger.With().Str("component", "cron-manager").Logger(),
        webStorageDir: cfg.WebStorageDir,
        unitDir:       "/etc/systemd/system",
    }
}
```

### 5.3 Unit File Naming

Each cron job produces two systemd units:

- **Timer**: `cron-{tenantID}-{cronJobID}.timer`
- **Service**: `cron-{tenantID}-{cronJobID}.service`

Using the cron job ID (not name) in the unit filename avoids issues with renames and guarantees uniqueness.

### 5.4 Service Unit Template

```ini
# cron-{tenantID}-{cronJobID}.service
[Unit]
Description=Cron job: {name} for tenant {tenantID}
After=network.target

[Service]
Type=oneshot
User={tenantID}
Group={tenantID}
WorkingDirectory={workingDirectory}
ExecStart=/bin/bash -c {command}
TimeoutStopSec={timeoutSeconds}
MemoryMax={maxMemoryMB}M
CPUQuota=100%
StandardOutput=journal
StandardError=journal
SyslogIdentifier=cron-{tenantID}-{cronJobID}

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths={webrootPath}
PrivateTmp=yes
```

**WorkingDirectory** is computed as:
```
/var/www/storage/{tenantID}/webroots/{webrootName}/{workingDirectory}
```

Where `workingDirectory` is the optional subfolder. If empty, the webroot root is used.

### 5.5 Timer Unit Template

```ini
# cron-{tenantID}-{cronJobID}.timer
[Unit]
Description=Timer for cron job: {name} for tenant {tenantID}

[Timer]
OnCalendar={systemdCalendar}
Persistent=true
RandomizedDelaySec=15

[Install]
WantedBy=timers.target
```

**Cron-to-systemd calendar conversion:** Standard 5-field cron expressions (`minute hour day-of-month month day-of-week`) need to be converted to systemd's `OnCalendar=` format. Implementation:

```go
// cronToSystemdCalendar converts a standard 5-field cron expression
// to systemd OnCalendar format.
// Example: "*/5 * * * *" -> "*-*-* *:0/5:00"
func cronToSystemdCalendar(cron string) (string, error)
```

Common conversions:
- `* * * * *` (every minute) -> `*-*-* *:*:00`
- `*/5 * * * *` (every 5 minutes) -> `*-*-* *:0/5:00`
- `0 * * * *` (every hour) -> `*-*-* *:00:00`
- `0 2 * * *` (daily at 2am) -> `*-*-* 02:00:00`
- `30 4 * * 1` (Monday 4:30am) -> `Mon *-*-* 04:30:00`

**`Persistent=true`** ensures that if a timer was missed (e.g., node was down), it fires once when the node comes back up.

**`RandomizedDelaySec=15`** adds up to 15 seconds of jitter to prevent thundering herd when many timers fire at the same minute boundary.

### 5.6 CronManager Methods

```go
// CreateUnits writes the timer and service unit files to disk and runs daemon-reload.
func (m *CronManager) CreateUnits(ctx context.Context, info *CronJobInfo) error

// UpdateUnits rewrites the unit files and runs daemon-reload.
// If the timer is currently active, it is restarted.
func (m *CronManager) UpdateUnits(ctx context.Context, info *CronJobInfo) error

// DeleteUnits stops the timer (if running), disables it, removes unit files,
// and runs daemon-reload.
func (m *CronManager) DeleteUnits(ctx context.Context, info *CronJobInfo) error

// EnableTimer starts and enables the timer unit.
func (m *CronManager) EnableTimer(ctx context.Context, info *CronJobInfo) error

// DisableTimer stops and disables the timer unit.
func (m *CronManager) DisableTimer(ctx context.Context, info *CronJobInfo) error
```

```go
type CronJobInfo struct {
    ID               string
    TenantID         string
    WebrootName      string
    Name             string
    Schedule         string // original cron expression
    Command          string
    WorkingDirectory string // optional subfolder within webroot
    TimeoutSeconds   int
    MaxMemoryMB      int
}
```

### 5.7 Node-Local Activities

File: `internal/activity/node_local.go` -- add new methods to the existing `NodeLocal` struct.

Add a `cron *agent.CronManager` field to the `NodeLocal` struct and pass it through `NewNodeLocal()`.

```go
// CreateCronJobUnits writes systemd timer+service units for a cron job on this node.
func (a *NodeLocal) CreateCronJobUnits(ctx context.Context, params CreateCronJobParams) error

// UpdateCronJobUnits rewrites systemd units for a cron job on this node.
func (a *NodeLocal) UpdateCronJobUnits(ctx context.Context, params UpdateCronJobParams) error

// DeleteCronJobUnits stops, disables, and removes systemd units on this node.
func (a *NodeLocal) DeleteCronJobUnits(ctx context.Context, params DeleteCronJobParams) error

// EnableCronJobTimer starts the systemd timer on this node.
func (a *NodeLocal) EnableCronJobTimer(ctx context.Context, params CronJobTimerParams) error

// DisableCronJobTimer stops the systemd timer on this node.
func (a *NodeLocal) DisableCronJobTimer(ctx context.Context, params CronJobTimerParams) error
```

Activity params (add to `internal/activity/params.go`):

```go
type CreateCronJobParams struct {
    ID               string
    TenantID         string
    WebrootName      string
    Name             string
    Schedule         string
    Command          string
    WorkingDirectory string
    TimeoutSeconds   int
    MaxMemoryMB      int
}

type UpdateCronJobParams = CreateCronJobParams

type DeleteCronJobParams struct {
    ID       string
    TenantID string
}

type CronJobTimerParams struct {
    ID       string
    TenantID string
}
```

### 5.8 CoreDB Activities

Add to `internal/activity/core_db.go`:

```go
// GetCronJobContext fetches a cron job with its webroot, tenant, and shard nodes.
func (a *CoreDB) GetCronJobContext(ctx context.Context, cronJobID string) (*CronJobContext, error)

// ListCronJobsByWebroot retrieves all active cron jobs for a webroot.
func (a *CoreDB) ListCronJobsByWebroot(ctx context.Context, webrootID string) ([]model.CronJob, error)

// ListCronJobsByTenant retrieves all active cron jobs for a tenant (used in convergence).
func (a *CoreDB) ListCronJobsByTenant(ctx context.Context, tenantID string) ([]model.CronJob, error)
```

Context struct (add to `internal/activity/context.go` or similar):

```go
type CronJobContext struct {
    CronJob model.CronJob
    Webroot model.Webroot
    Tenant  model.Tenant
    Nodes   []model.Node
}
```

---

## 6. Single-Node Execution Strategy

### 6.1 The Problem

Web shards have 2-3 nodes that all share the same CephFS storage and converge to the same configuration. If a cron timer fires on all nodes, the command runs 2-3 times concurrently -- a correctness problem for most workloads.

### 6.2 Solution: Deterministic Node Assignment

Use **deterministic assignment** based on the cron job ID and the sorted list of active nodes in the shard. The systemd unit files are written to **all nodes** (so convergence and failover work), but the timer is only **enabled** on the designated node.

```go
// designatedNode returns the node ID responsible for executing a given cron job.
// It sorts nodes by ID (stable order) and uses consistent hashing to pick one.
func designatedNode(cronJobID string, nodes []model.Node) string {
    if len(nodes) == 0 {
        return ""
    }
    sort.Slice(nodes, func(i, j int) bool {
        return nodes[i].ID < nodes[j].ID
    })
    h := fnv.New32a()
    h.Write([]byte(cronJobID))
    idx := int(h.Sum32()) % len(nodes)
    return nodes[idx].ID
}
```

**Why this works:**
- Every workflow call computes the same designated node for a given set of nodes.
- When a node leaves the shard, the next convergence reassigns cron timers to remaining nodes.
- When a node joins, convergence redistributes (some timers move to the new node).
- Unit files exist on all nodes so failover requires only enabling the timer on the new designated node.

### 6.3 Convergence Behavior

During `convergeWebShard`, after creating tenants and webroots on all nodes, add a cron job convergence step:

1. For each tenant on the shard, fetch all active cron jobs (via `ListCronJobsByTenant`).
2. For each cron job:
   a. Write unit files on **all nodes** (`CreateCronJobUnits`).
   b. Compute the designated node for this cron job.
   c. Enable the timer on the designated node (`EnableCronJobTimer`).
   d. Disable the timer on all other nodes (`DisableCronJobTimer`).

This is added to `convergeWebShard()` in `internal/workflow/converge_shard.go`.

### 6.4 Node Failure / Removal

When a node is removed from a shard and convergence runs:
- The designated node for some cron jobs changes (because the node list changed).
- Convergence enables timers on the new designated nodes and disables them on others.
- The removed node's timers are orphaned but harmless (the node is gone).

No special failure detection is needed. Convergence is the recovery mechanism.

### 6.5 Future Enhancement: Active Health Monitoring

For a future iteration, consider a Temporal cron workflow (`CronJobHealthCheckWorkflow`) that runs every 5 minutes and verifies that all enabled cron jobs have their timer active on exactly one node. If a designated node is unresponsive, it reassigns to another node. This is not needed for MVP since shard convergence handles recovery.

---

## 7. Webroot Deletion Cascade

When a webroot is deleted (`DeleteWebrootWorkflow`), all its cron jobs must be cleaned up first. Modify `DeleteWebrootWorkflow` in `internal/workflow/webroot.go`:

1. Before deleting the webroot itself, fetch all cron jobs for the webroot.
2. For each cron job, delete the systemd units on all nodes.
3. Set cron job status to `deleted` in the core DB.
4. Proceed with the existing webroot deletion logic.

This is the same cascade pattern used for FQDNs when a webroot is deleted.

---

## 8. Tenant Suspension

When a tenant is suspended (`SuspendTenantWorkflow`), all cron timers must be stopped:

1. Fetch all enabled cron jobs for the tenant.
2. On all nodes: stop and disable all cron timers for the tenant.
3. The `enabled` flag in the DB is **not** changed (so unsuspend restores the previous state).

When a tenant is unsuspended (`UnsuspendTenantWorkflow`):

1. Fetch all cron jobs where `enabled=true` for the tenant.
2. Re-run the designated node assignment.
3. Enable timers on designated nodes.

---

## 9. Logging and Output

### 9.1 journald

All cron job output (stdout + stderr) is captured by journald via the systemd service unit. The `SyslogIdentifier=cron-{tenantID}-{cronJobID}` tag makes it queryable:

```bash
journalctl -u cron-{tenantID}-{cronJobID}.service --since "1 hour ago"
```

### 9.2 Log Collection

The existing Grafana Alloy (log forwarder) on each node can be configured to collect journal entries matching the `cron-*` syslog identifier pattern and forward them to Loki. This integrates with the existing `/api/v1/logs` endpoint that proxies to Loki.

### 9.3 Execution History (Future Enhancement)

For MVP, cron job output is available only through journald/Loki. A future enhancement could add:

- A `cron_job_runs` table recording each execution (start time, end time, exit code, output snippet).
- A `GET /cron-jobs/{id}/runs` endpoint returning recent execution history.
- A Temporal activity that queries journald after each run and stores the result.

This is deferred because journald + Loki already provides the observability needed, and adding a runs table introduces significant write amplification for high-frequency cron jobs.

---

## 10. Security

### 10.1 Command Execution

- Commands run as the **tenant's Linux user** (UID from the tenant record), never as root.
- The systemd service unit uses `NoNewPrivileges=yes` to prevent privilege escalation.
- `ProtectSystem=strict` makes the filesystem read-only except for `ReadWritePaths`.
- `ReadWritePaths` is restricted to the webroot's CephFS directory.
- `PrivateTmp=yes` gives each cron job its own `/tmp`.

### 10.2 Input Validation

- **Cron expressions**: Parsed and validated using `robfig/cron/v3`. Reject expressions that would fire more than once per minute. Reject special strings like `@reboot`.
- **Command**: Max 4096 characters. No validation on command content (the tenant can run whatever their user has access to -- same as SSH).
- **Working directory**: Must not contain `..`, must not start with `/`, must not contain null bytes. The full path is always anchored under the webroot storage path.
- **Name**: Standard slug validation (lowercase alphanumeric + hyphens, 1-63 chars).

### 10.3 Resource Limits

| Limit | Default | Max | Enforced By |
|---|---|---|---|
| Max cron jobs per tenant | 50 | 50 | API handler (count check) |
| Execution timeout | 3600s | 86400s | systemd `TimeoutStopSec` |
| Memory | 512 MB | 4096 MB | systemd `MemoryMax` (cgroups v2) |
| CPU | 100% of one core | 100% | systemd `CPUQuota` |
| Minimum interval | 1 minute | - | Cron expression validation |

### 10.4 API Key Scopes

Cron job endpoints use the `cron_jobs` scope with read/write/delete actions, following the existing pattern. Add `"cron_jobs"` to the list of valid scopes in `internal/api/middleware/auth.go`.

---

## 11. Files to Create or Modify

### New Files

| File | Description |
|---|---|
| `migrations/core/00035_cron_jobs.sql` | Database migration |
| `internal/model/cron_job.go` | Go model struct |
| `internal/api/request/cron_job.go` | Request validation structs |
| `internal/api/handler/cron_job.go` | HTTP handler |
| `internal/core/cron_job.go` | Core service (DB + Temporal) |
| `internal/workflow/cron_job.go` | Temporal workflows (create, update, delete, enable, disable) |
| `internal/agent/cron.go` | CronManager (systemd unit file generation + systemctl) |

### Modified Files

| File | Change |
|---|---|
| `internal/model/tenant.go` | Add `CronJobs` to `TenantResourceSummary` |
| `internal/model/status.go` | No change needed (existing statuses suffice) |
| `internal/api/server.go` | Register cron job routes |
| `internal/core/services.go` | Add `CronJob *CronJobService` |
| `internal/core/provision.go` | Add `resolveTenantIDFromCronJob` |
| `internal/activity/params.go` | Add cron job param structs |
| `internal/activity/core_db.go` | Add `GetCronJobContext`, `ListCronJobsByWebroot`, `ListCronJobsByTenant` |
| `internal/activity/node_local.go` | Add cron job activities + `cron` field to `NodeLocal` |
| `internal/workflow/converge_shard.go` | Add cron job convergence to `convergeWebShard` |
| `internal/workflow/webroot.go` | Add cron job cascade to `DeleteWebrootWorkflow` |
| `internal/workflow/tenant.go` | Add cron timer stop/start to suspend/unsuspend workflows |
| `internal/core/dashboard.go` | Include cron_jobs in resource summary query |
| `internal/core/search.go` | Include cron_jobs in search results |
| `internal/api/middleware/auth.go` | Add `cron_jobs` to valid scopes |

---

## 12. Implementation Order

1. **Data model**: Migration, Go model, request structs.
2. **Core service**: CRUD + Temporal workflow signals.
3. **API handler + routes**: Wire up endpoints.
4. **CronManager**: Agent-side systemd unit generation and systemctl operations.
5. **Activities**: Node-local activities and CoreDB context queries.
6. **Workflows**: Create, Update, Delete, Enable, Disable workflows.
7. **Convergence**: Add cron jobs to `convergeWebShard`.
8. **Cascade**: Add cron job cleanup to `DeleteWebrootWorkflow` and suspend/unsuspend.
9. **Tests**: Unit tests for CronManager, handler, service, workflow.
10. **Cron expression library**: Add `robfig/cron/v3` dependency, implement validation and cron-to-systemd conversion.

---

## 13. Testing Strategy

### 13.1 Unit Tests

- **CronManager**: Test unit file generation (template output), cron-to-systemd calendar conversion, path validation.
- **Handler**: Test request validation (invalid cron expressions, path traversal in working_directory, command length limits).
- **Core Service**: Test DB operations with mock (same pattern as `webroot_test.go`).
- **Workflow**: Test activity sequencing with Temporal test framework (same pattern as existing workflow tests).
- **Designated Node**: Test deterministic assignment with varying node counts.

### 13.2 E2E Tests

Add to `tests/e2e/`:

1. Create tenant + webroot + cron job -> verify status goes to `active`.
2. Enable cron job -> verify systemd timer is active on one node.
3. Disable cron job -> verify timer is stopped.
4. Delete webroot -> verify cron jobs are cascade-deleted.
5. Update cron schedule -> verify timer unit is rewritten.

---

## 14. Open Questions / Future Work

1. **Execution notifications**: Should the platform notify (webhook/email) on cron job failures? Deferred -- users can monitor via Loki/Grafana.
2. **Output size limits**: journald has its own retention policy. If we add a `cron_job_runs` table, we need to decide how much output to store per run (suggest: last 10KB of combined stdout+stderr).
3. **Per-brand cron job limits**: Currently a flat 50-per-tenant limit. Could be made configurable per brand when brands are fully implemented.
4. **Time zones**: systemd timers default to UTC. If tenants need local time zones, the timer can use `OnCalendar=` with a `TZ=` prefix (e.g., `OnCalendar=Europe/Oslo *-*-* 02:00:00`). Deferred for MVP -- UTC-only.
5. **Concurrent execution policy**: systemd timers skip the next trigger if the previous run is still executing (default `oneshot` behavior). This is the correct default. A future `allow_overlap` flag could be added if needed.
