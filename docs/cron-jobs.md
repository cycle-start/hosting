# Cron Jobs

Cron jobs are scheduled tasks that run on a tenant's web shard. They execute as the tenant's Linux user inside the webroot's working directory.

## API Endpoints

| Method | Endpoint | Description |
|---|---|---|
| POST | `/tenants/{id}/cron-jobs` | Create a cron job |
| GET | `/tenants/{id}/cron-jobs` | List cron jobs |
| GET | `/cron-jobs/{id}` | Get cron job |
| PUT | `/cron-jobs/{id}` | Update cron job |
| DELETE | `/cron-jobs/{id}` | Delete cron job |
| POST | `/cron-jobs/{id}/enable` | Enable cron job |
| POST | `/cron-jobs/{id}/disable` | Disable cron job |
| POST | `/cron-jobs/{id}/retry` | Retry failed provisioning |

### Create Request

```json
{
  "webroot_id": "uuid",
  "schedule": "*/5 * * * *",
  "command": "php artisan schedule:run",
  "working_directory": "",
  "enabled": true,
  "timeout_seconds": 300,
  "max_memory_mb": 256
}
```

- `schedule`: Standard 5-field cron expression, converted to systemd `OnCalendar` format.
- `working_directory`: Relative to the webroot root. Empty means the webroot root itself.
- `timeout_seconds`: Maximum execution time before systemd kills the process.
- `max_memory_mb`: Memory limit enforced by systemd `MemoryMax`.

## How It Works

### Systemd Timer + Service

Each cron job is implemented as a pair of systemd units on the web nodes:

- **Timer:** `cron-{tenantName}-{cronJobID}.timer` — fires on the configured schedule.
- **Service:** `cron-{tenantName}-{cronJobID}.service` — runs the command as a oneshot unit.

The service unit runs with systemd security hardening:
- `NoNewPrivileges=yes`
- `ProtectSystem=strict`
- `ProtectHome=yes`
- `PrivateTmp=yes`
- `CPUQuota=100%`
- `ReadWritePaths` limited to the webroot and lock directory

### Distributed Locking (CephFS flock)

Web shards typically have 2-3 nodes, all with access to the same CephFS storage. Timers are enabled on **all nodes** in the shard, but only one node executes the job at a time using POSIX file locks (`flock`) on CephFS.

When the timer fires on any node:

1. `flock --nonblock` tries to acquire a lock file at `{webStorageDir}/{tenantName}/.locks/cron-{cronJobID}.lock`
2. **Lock acquired** — the command executes normally
3. **Lock contended** — `flock` exits immediately with code 75 (skip, treated as success via `SuccessExitStatus=75`)

This provides:
- **Instant failover:** If a node goes down, the next timer fire on any surviving node acquires the lock and runs the job.
- **No coordination overhead:** No API calls, no database locks, no reassignment workflows.
- **Crash safety:** POSIX locks are automatically released when the process exits, even on crash or OOM kill.

### Lifecycle

1. **Create:** Unit files are written to all nodes in the shard. If the job is enabled, timers are enabled on all nodes.
2. **Update:** Unit files are rewritten on all nodes. Timer state is updated according to the `enabled` flag.
3. **Enable/Disable:** Timers are enabled or disabled on all nodes.
4. **Delete:** Timers are stopped and unit files are removed from all nodes.
5. **Convergence:** When a new node joins the shard, convergence writes unit files and enables timers for all active cron jobs.

## Logging

Cron job output goes to journald via `StandardOutput=journal` / `StandardError=journal`, tagged with `SyslogIdentifier=cron-{tenantName}-{cronJobID}`.

Vector on web nodes collects cron logs from journald and ships them to Tenant Loki with labels:
- `log_type=cron`
- `tenant_id={tenantName}`
- `cron_job_id={cronJobID}`
- `job=cron`

### Querying Cron Logs

Via the tenant logs API:

```bash
# All cron logs for a tenant
curl -H "X-API-Key: ..." \
  "http://api.hosting.test/api/v1/tenants/{id}/logs?log_type=cron"

# Logs for a specific cron job
curl -H "X-API-Key: ..." \
  "http://api.hosting.test/api/v1/tenants/{id}/logs?log_type=cron&cron_job_id={cronJobID}"
```

## Auto-Disable on Repeated Failures

Cron jobs track consecutive execution failures. When a job fails `max_failures` times in a row (default: 5), it is automatically disabled with status `auto_disabled`.

**How it works:**

Each cron service unit has `ExecStopPost=+/usr/local/bin/cron-outcome` which runs as root after every execution:

1. If the exit code was 75 (flock lock contention), the script exits silently — no reporting.
2. Otherwise, the script reads `$SERVICE_RESULT` (set by systemd) and reports the outcome to core-api via `POST /internal/v1/cron-jobs/{id}/outcome`.
3. The core API increments `consecutive_failures` on failure. On success, it resets to 0.
4. When `consecutive_failures >= max_failures`, the job is set to `enabled = false`, `status = "auto_disabled"`, and a `DisableCronJobWorkflow` stops timers on all nodes.

**Re-enabling an auto-disabled job:**

```bash
# Via API
curl -X POST -H "X-API-Key: ..." http://api.hosting.test/api/v1/cron-jobs/{id}/enable

# Or retry (also resets the failure counter)
curl -X POST -H "X-API-Key: ..." http://api.hosting.test/api/v1/cron-jobs/{id}/retry
```

Both `enable` and `retry` accept jobs in `auto_disabled` status and reset the failure counter to 0.

**Configuring the threshold:**

The `max_failures` field can be set per cron job at creation or update time. Set to 0 to disable auto-disable (the job will never be automatically stopped regardless of failures).

## Schedule Format

Standard 5-field cron expressions are supported:

```
minute  hour  day-of-month  month  day-of-week
  *       *        *          *        *
```

Step expressions (`*/5`) and day-of-week names are converted to systemd calendar format. Examples:

| Cron | Meaning |
|---|---|
| `* * * * *` | Every minute |
| `*/5 * * * *` | Every 5 minutes |
| `0 * * * *` | Every hour |
| `0 2 * * *` | Daily at 2:00 AM |
| `0 0 * * 0` | Weekly on Sunday at midnight |
| `0 0 1 * *` | Monthly on the 1st at midnight |

A 15-second randomized delay (`RandomizedDelaySec=15`) is added to timer units to avoid thundering herd effects when multiple cron jobs share the same schedule.
