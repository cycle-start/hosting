# User-Facing Log Access

## Problem Statement

Nginx access and error logs are currently written to CephFS at
`/var/www/storage/{tenantName}/logs/{webrootName}-access.log` and
`/var/www/storage/{tenantName}/logs/{webrootName}-error.log` (see
`internal/agent/nginx.go` lines 51-52). PHP-FPM error logs follow the same
pattern at `/var/www/storage/{tenantName}/logs/php-error.log` (see
`internal/agent/runtime/php.go` line 30).

This creates several problems:

1. **CephFS I/O amplification.** Every HTTP request triggers a write to the
   shared filesystem. High-traffic tenants generate sustained sequential I/O
   that competes with file-serving reads for all tenants on the shard. CephFS
   metadata operations for log appends are disproportionately expensive
   compared to content reads.

2. **Unbounded growth.** There is no log rotation in place. Logs accumulate
   until the tenant's CephFS quota (when implemented) or the pool fills up.
   A single busy site can consume gigabytes of log storage that should be
   reserved for application files.

3. **No user-facing log access.** Tenants currently have no way to view their
   own access or error logs. The admin UI has a `LogViewer` component (see
   `web/admin/src/components/shared/log-viewer.tsx`) that queries Loki via
   the core API (`GET /logs`), but it only shows platform-internal logs
   (core-api, worker, node-agent). There is no tenant-scoped log data in Loki
   today.

4. **Node-local divergence.** Each web node in a shard writes its own copy of
   access logs for the requests it served (HAProxy distributes by consistent
   hashing on Host header). The CephFS path means these writes from multiple
   nodes target the same file, creating contention and potential interleaving
   of log lines. Moving to local disk makes the node-local nature explicit and
   eliminates contention.

## Architecture: Two Loki Instances

The platform uses **two separate Loki instances** to cleanly separate concerns:

| Instance | Purpose | Data Sources | Shipped By | Retention |
|---|---|---|---|---|
| **Platform Loki** (existing) | Operational logs | core-api, worker, node-agent | Alloy (k3s DaemonSet) | 30 days |
| **Tenant Loki** (new) | Tenant-facing logs | nginx access/error, PHP-FPM, cron/worker output | Vector (web node VMs) | 7 days |

**Why separate instances:**

- **Isolation.** A high-traffic tenant generating millions of access log entries
  per day should never degrade the query performance of operational logs used
  for debugging the platform itself.
- **Different retention.** Platform logs need longer retention (30 days) for
  incident investigation. Tenant access logs are high-volume, low-value
  after a few days — 7-day retention keeps storage costs manageable.
- **Different cardinality.** Platform logs have low cardinality (a handful of
  services). Tenant logs have high cardinality (`tenant_id` x `webroot_id` x
  `log_type` x `hostname`). Separate instances allow independent tuning of
  Loki limits.
- **Security boundary.** Tenant log queries always go through the tenant-scoped
  API endpoint, which enforces brand access. Platform log queries require
  `audit_logs:read` scope. Separate Loki instances make it impossible for a
  bug in one query path to leak data from the other.

**Application logs** (code written by tenants — e.g., `error_log()` in PHP,
`console.log()` in Node.js, Python logging to files) stay on CephFS as files.
These are the tenant's responsibility and are accessible via SFTP/SSH. We do
**not** ship application log files to Loki because:

- We can't control whether tenants write to files, stdout, syslog, or all three.
- Application log formats are unparseable without per-tenant knowledge.
- Shipping arbitrary tenant file content to our Loki is a security/abuse risk.

## Log Types

| Type | Source | Current Location | New Location | Ships to Tenant Loki |
|---|---|---|---|---|
| Nginx access log | nginx per-webroot | CephFS `logs/{webroot}-access.log` | Local SSD `/var/log/hosting/{tenantID}/{webrootID}-access.log` | Yes |
| Nginx error log | nginx per-webroot | CephFS `logs/{webroot}-error.log` | Local SSD `/var/log/hosting/{tenantID}/{webrootID}-error.log` | Yes |
| PHP-FPM error log | php-fpm per-tenant | CephFS `logs/php-error.log` | Local SSD `/var/log/hosting/{tenantID}/php-error.log` | Yes |
| PHP-FPM slow log | php-fpm per-tenant | Not configured | Local SSD `/var/log/hosting/{tenantID}/php-slow.log` | Yes |
| Runtime stdout/stderr | Node.js / Python / Ruby | Not captured | Local SSD `/var/log/hosting/{tenantID}/{webrootID}-app.log` | Yes |
| Cron output | Future: scheduled tasks | Not implemented | Local SSD `/var/log/hosting/{tenantID}/cron-{jobID}.log` | Yes |
| Worker output | Future: background workers | Not implemented | Local SSD `/var/log/hosting/{tenantID}/worker-{webrootID}.log` | Yes |
| Application logs | Tenant code (files) | CephFS | CephFS (unchanged) | No — user's responsibility |

## Design

### Phase 1: Move Logs to Local Disk

Move all log writes from CephFS to local SSD on each web node. The node-agent
already runs on local disk; logs should live alongside it.

**New log directory structure:**

```
/var/log/hosting/
  {tenantID}/
    {webrootID}-access.log      # nginx access log (JSON format)
    {webrootID}-error.log       # nginx error log
    php-error.log               # PHP-FPM error log
    php-slow.log                # PHP-FPM slow log (new)
    {webrootID}-app.log         # Runtime stdout/stderr (node/python/ruby)
```

Using tenant ID (UUID) instead of tenant name avoids conflicts if tenants are
renamed. The webroot ID is also a UUID. This matches the Loki label scheme
(IDs are stable, names are not).

**Code changes:**

1. **`internal/agent/nginx.go`** — Change the nginx template log paths:

   ```
   # Before
   access_log /var/www/storage/{{ .TenantName }}/logs/{{ .WebrootName }}-access.log;
   error_log  /var/www/storage/{{ .TenantName }}/logs/{{ .WebrootName }}-error.log;

   # After
   access_log /var/log/hosting/{{ .TenantID }}/{{ .WebrootID }}-access.log hosting_json;
   error_log  /var/log/hosting/{{ .TenantID }}/{{ .WebrootID }}-error.log warn;
   ```

   The `hosting_json` log format is a custom nginx `log_format` defined in the
   main nginx config (see Structured Logging section below). Template data
   struct needs `TenantID` and `WebrootID` fields added.

2. **`internal/agent/runtime/php.go`** — Change the PHP-FPM pool template:

   ```
   # Before
   php_admin_value[error_log] = /var/www/storage/{{ .TenantName }}/logs/php-error.log

   # After
   php_admin_value[error_log] = /var/log/hosting/{{ .TenantID }}/php-error.log
   php_admin_value[slowlog] = /var/log/hosting/{{ .TenantID }}/php-slow.log
   php_admin_value[request_slowlog_timeout] = 5s
   ```

   Template data struct needs `TenantID` field added.

3. **`internal/agent/tenant.go`** — Create the local log directory during
   tenant provisioning (in addition to existing CephFS dirs):

   ```go
   // In TenantManager.Create(), after creating CephFS dirs:
   logDir := filepath.Join("/var/log/hosting", info.ID)
   os.MkdirAll(logDir, 0750)
   // Owned by the tenant user so php-fpm (running as tenant) can write.
   // Nginx runs as www-data and needs write access too.
   chown(logDir, info.Name, "www-data")
   ```

   On tenant deletion, remove `/var/log/hosting/{tenantID}/`.

4. **`internal/agent/tenant.go`** — Remove the CephFS `logs/` directory from
   the tenant directory layout. The `webroots/`, `home/`, and `tmp/`
   directories remain on CephFS. The `logs/` directory is no longer created
   on CephFS.

5. **Nginx main config (Terraform/cloud-init)** — Add the `log_format`
   directive. This goes in the `http {}` block in `/etc/nginx/nginx.conf`:

   ```nginx
   log_format hosting_json escape=json
     '{'
       '"time":"$time_iso8601",'
       '"remote_addr":"$remote_addr",'
       '"method":"$request_method",'
       '"uri":"$request_uri",'
       '"status":$status,'
       '"bytes_sent":$bytes_sent,'
       '"request_time":$request_time,'
       '"upstream_time":"$upstream_response_time",'
       '"http_referer":"$http_referer",'
       '"http_user_agent":"$http_user_agent",'
       '"host":"$host",'
       '"server_name":"$server_name"'
     '}';
   ```

   This produces one JSON object per line — no regex parsing needed downstream.

### Phase 2: Deploy Tenant Loki Instance

Deploy a second Loki instance in k3s dedicated to tenant logs.

**New file: `deploy/k3s/loki-tenant.yaml`**

Runs alongside the existing platform Loki (`deploy/k3s/loki.yaml`) on a
different port:

```yaml
# StatefulSet running Loki on port 3101 (platform Loki uses 3100)
# Config differences from platform Loki:
#   - retention_period: 168h (7 days, vs 720h for platform)
#   - max_streams_per_user: 100000 (higher for tenant cardinality)
#   - ingestion_rate_mb: 20 (higher for access log volume)
#   - PVC: 10Gi (larger for access log volume)
#   - Separate TSDB/chunks storage path
```

Key configuration:

| Setting | Platform Loki | Tenant Loki |
|---|---|---|
| Port | 3100 | 3101 |
| Retention | 720h (30 days) | 168h (7 days) |
| PVC size | 5Gi | 10Gi |
| max_streams_per_user | 10000 | 100000 |
| ingestion_rate_mb | 10 | 20 |

### Phase 3: Ship Logs to Tenant Loki via Vector

Web nodes already run Vector with the base config (`deploy/vector/base.toml`)
and the web overlay (`deploy/vector/web.toml`). The web overlay currently ships
the global nginx access.log and error.log. Replace this with per-tenant file
discovery shipping to the **tenant Loki instance** (port 3101).

**Updated `deploy/vector/web.toml`:**

```toml
# Per-tenant nginx access logs (JSON format).
[sources.tenant_access_logs]
type = "file"
include = ["/var/log/hosting/*/**-access.log"]
read_from = "end"
glob_minimum_cooldown_ms = 5000

[transforms.parse_tenant_access]
type = "remap"
inputs = ["tenant_access_logs"]
source = '''
# Extract tenant_id and webroot_id from file path.
# Path: /var/log/hosting/{tenant_id}/{webroot_id}-access.log
parts = split(to_string(.file) ?? "", "/")
if length(parts) >= 5 {
  .tenant_id = parts[4]
  filename = parts[5] ?? ""
  .webroot_id = replace(filename, r'-access\.log$', "")
}
.log_type = "access"
.job = "nginx"

# Parse the JSON access log line.
parsed, err = parse_json(.message)
if err == null {
  .message = to_string(parsed.method ?? "") + " " +
             to_string(parsed.uri ?? "") + " " +
             to_string(parsed.status ?? "")
  .status = to_int(parsed.status ?? 0) ?? 0
  .method = to_string(parsed.method ?? "")
  .request_time = to_float(parsed.request_time ?? 0) ?? 0.0
  .host = to_string(parsed.host ?? "")
  .structured = parsed
}
'''

# Per-tenant nginx error logs.
[sources.tenant_error_logs]
type = "file"
include = ["/var/log/hosting/*/**-error.log"]
read_from = "end"
glob_minimum_cooldown_ms = 5000

[transforms.parse_tenant_error]
type = "remap"
inputs = ["tenant_error_logs"]
source = '''
parts = split(to_string(.file) ?? "", "/")
if length(parts) >= 5 {
  .tenant_id = parts[4]
  filename = parts[5] ?? ""
  .webroot_id = replace(filename, r'-error\.log$', "")
}
.log_type = "error"
.job = "nginx"
.level = "error"
'''

# PHP-FPM error logs.
[sources.tenant_php_logs]
type = "file"
include = ["/var/log/hosting/*/php-error.log"]
read_from = "end"
glob_minimum_cooldown_ms = 5000

[transforms.parse_tenant_php]
type = "remap"
inputs = ["tenant_php_logs"]
source = '''
parts = split(to_string(.file) ?? "", "/")
if length(parts) >= 5 {
  .tenant_id = parts[4]
}
.log_type = "php-error"
.job = "php-fpm"
.level = "error"
'''

# PHP-FPM slow logs.
[sources.tenant_php_slow]
type = "file"
include = ["/var/log/hosting/*/php-slow.log"]
read_from = "end"
glob_minimum_cooldown_ms = 5000
multiline.mode = "halt_before"
multiline.start_pattern = '^\[.+\] \[pool '
multiline.condition_pattern = '^\[.+\] \[pool '
multiline.timeout_ms = 1000

[transforms.parse_tenant_php_slow]
type = "remap"
inputs = ["tenant_php_slow"]
source = '''
parts = split(to_string(.file) ?? "", "/")
if length(parts) >= 5 {
  .tenant_id = parts[4]
}
.log_type = "php-slow"
.job = "php-fpm"
.level = "warn"
'''

# Runtime application logs (Node.js, Python, Ruby stdout/stderr).
[sources.tenant_app_logs]
type = "file"
include = ["/var/log/hosting/*/**-app.log"]
read_from = "end"
glob_minimum_cooldown_ms = 5000

[transforms.parse_tenant_app]
type = "remap"
inputs = ["tenant_app_logs"]
source = '''
parts = split(to_string(.file) ?? "", "/")
if length(parts) >= 5 {
  .tenant_id = parts[4]
  filename = parts[5] ?? ""
  .webroot_id = replace(filename, r'-app\.log$', "")
}
.log_type = "app"
.job = "runtime"
'''

# Ship all tenant logs to TENANT Loki (port 3101, NOT platform Loki 3100).
[sinks.loki_tenant]
type = "loki"
inputs = [
  "parse_tenant_access",
  "parse_tenant_error",
  "parse_tenant_php",
  "parse_tenant_php_slow",
  "parse_tenant_app",
]
endpoint = "http://10.10.10.2:3101"
encoding.codec = "text"

[sinks.loki_tenant.labels]
job = "{{ job }}"
tenant_id = "{{ tenant_id }}"
webroot_id = "{{ webroot_id }}"
log_type = "{{ log_type }}"
hostname = "{{ host }}"
level = "{{ level }}"
```

**Label design for Tenant Loki:**

| Label | Values | Purpose |
|---|---|---|
| `job` | `nginx`, `php-fpm`, `runtime` | Identifies the log source program |
| `tenant_id` | UUID | Tenant scoping for queries |
| `webroot_id` | UUID | Webroot scoping for queries |
| `log_type` | `access`, `error`, `php-error`, `php-slow`, `app` | Distinguishes log types |
| `hostname` | node hostname | Identifies which node served the request |
| `level` | `info`, `warn`, `error` | Severity filtering |

**Cardinality note:** `tenant_id` and `webroot_id` are high-cardinality labels.
At millions of tenants, this will require Loki to be sized appropriately. For
the near term (thousands of tenants), this is fine. At massive scale, consider
using structured metadata (Loki 3.x `| tenant_id=...`) instead of stream
labels for tenant_id and webroot_id. The current Loki deployment already has
`allow_structured_metadata: true` enabled (see `deploy/k3s/loki.yaml` line 48),
so a future migration to structured metadata is straightforward.

### Phase 4: Log Rotation

Deploy logrotate configuration via Terraform cloud-init on web nodes.

**`/etc/logrotate.d/hosting-tenant-logs`:**

```
/var/log/hosting/*/*.log {
    daily
    rotate 2
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
    maxsize 100M
}
```

Key decisions:

- **`copytruncate`** instead of `create` — avoids needing to signal nginx/php-fpm
  to reopen log files. Vector handles file truncation correctly (it detects the
  file shrinking and re-reads from the beginning). This means no log lines are
  lost during rotation.
- **`rotate 2`** — keep 2 days. Vector ships logs in near-real-time (seconds of
  delay), so local retention is only a safety buffer.
- **`maxsize 100M`** — rotate mid-day if a single log file exceeds 100 MB.
  Protects against traffic spikes filling the local SSD.
- **`daily`** — normal rotation cadence.
- **`compress` / `delaycompress`** — compress the previous rotation (not the
  current one, since Vector may still be reading it).

### Phase 5: Config and Helm Chart

**`internal/config/config.go`** — Add `TenantLokiURL` field:

```go
TenantLokiURL string // TENANT_LOKI_URL — Tenant Loki query endpoint (default: http://127.0.0.1:3101)
```

Load with `getEnv("TENANT_LOKI_URL", "http://127.0.0.1:3101")`.

The existing `LokiURL` remains unchanged (points to platform Loki on 3100).

**`deploy/helm/hosting/templates/configmap.yaml`** — Add `TENANT_LOKI_URL`.

**`deploy/helm/hosting/values.yaml`** — Add `tenantLokiUrl: "http://127.0.0.1:3101"`.

### Phase 6: Tenant-Scoped Log Query API

Add a tenant-scoped log endpoint that queries the **tenant Loki** instance:

```
GET /tenants/{tenantID}/logs?log_type=access&webroot_id=xxx&start=1h&limit=500
```

This endpoint constructs the LogQL query server-side, ensuring tenants can only
see their own logs. The handler:

1. Extracts the tenant ID from the URL path.
2. Validates brand scope (same as all other tenant-scoped endpoints).
3. Builds a LogQL query: `{tenant_id="{tenantID}"}` with optional filters for
   `log_type` and `webroot_id`.
4. Queries **tenant Loki** (not platform Loki) via the same proxy mechanism.

**Handler — extend `internal/api/handler/logs.go`:**

```go
// Logs now holds both Loki URLs.
type Logs struct {
    lokiURL       string       // platform Loki
    tenantLokiURL string       // tenant Loki
    client        *http.Client
}

func NewLogs(lokiURL, tenantLokiURL string) *Logs

// TenantLogs godoc
//
//  @Summary     Query tenant logs
//  @Description Query access, error, and application logs for a specific tenant
//  @Tags        Tenants
//  @Security    ApiKeyAuth
//  @Param       tenantID   path  string true  "Tenant ID"
//  @Param       log_type   query string false "Log type filter (access, error, php-error, php-slow, app)"
//  @Param       webroot_id query string false "Filter by webroot ID"
//  @Param       start      query string false "Start time (RFC3339 or relative like '1h')"
//  @Param       limit      query int    false "Max entries (default 500, max 5000)"
//  @Success     200 {object} LogQueryResponse
//  @Router      /tenants/{tenantID}/logs [get]
func (h *Logs) TenantLogs(w http.ResponseWriter, r *http.Request) {
    tenantID := chi.URLParam(r, "tenantID")

    // Build LogQL query with mandatory tenant_id label selector.
    selectors := []string{fmt.Sprintf(`tenant_id="%s"`, tenantID)}

    if lt := r.URL.Query().Get("log_type"); lt != "" {
        selectors = append(selectors, fmt.Sprintf(`log_type="%s"`, lt))
    }
    if wid := r.URL.Query().Get("webroot_id"); wid != "" {
        selectors = append(selectors, fmt.Sprintf(`webroot_id="%s"`, wid))
    }

    query := fmt.Sprintf("{%s}", strings.Join(selectors, ", "))

    // Query TENANT Loki (h.tenantLokiURL), not platform Loki.
    h.queryLoki(w, r, h.tenantLokiURL, query)
}
```

Refactor the existing `Query` method to share a common `queryLoki(w, r, lokiURL, query)` helper.

**Route registration in `internal/api/server.go`:**

```go
r.Route("/tenants/{tenantID}", func(r chi.Router) {
    // ... existing routes ...
    r.Get("/logs", logs.TenantLogs)
})
```

**API key scope:** Require `tenants:read` scope (same as other tenant-scoped
read endpoints). Brand access is enforced via `checkTenantBrandAccess`.

### Phase 7: Admin UI Integration

The admin UI shows tenant logs from the **tenant Loki** instance and platform
logs from the **platform Loki** instance. Both are available on different
detail pages.

**1. New `useTenantLogs` hook** in `web/admin/src/lib/hooks.ts`:

```typescript
export function useTenantLogs(
  tenantId: string,
  logType?: string,
  webrootId?: string,
  range: string = '1h',
  enabled = true,
) {
  const params = new URLSearchParams({ start: range, limit: '500' })
  if (logType) params.set('log_type', logType)
  if (webrootId) params.set('webroot_id', webrootId)

  return useQuery({
    queryKey: ['tenant-logs', tenantId, logType, webrootId, range],
    queryFn: () =>
      api.get<LogQueryResponse>(
        `/tenants/${tenantId}/logs?${params.toString()}`
      ),
    enabled,
    refetchInterval: 10000,
  })
}
```

**2. New `TenantLogViewer` component** — `web/admin/src/components/shared/tenant-log-viewer.tsx`

Wraps the same visual design as `LogViewer` but with tenant-specific controls:

- **Log type selector:** Dropdown with options: All, Access, Error, PHP Error,
  PHP Slow, Application. Defaults to "All".
- **Webroot filter:** Dropdown populated from the tenant's webroots. Defaults to
  "All webroots".
- **Time range, pause/resume, entry count** — same as `LogViewer`.
- **Grafana link** — Opens tenant Loki in Grafana Explore with the same query.

**Log entry display:** For access log entries (`log_type=access`), the summary
line shows `METHOD URI STATUS` parsed from the JSON message. For error entries,
shows the raw error text. For PHP slow log entries, shows the function trace.
Same expandable JSON detail on click.

**3. Update detail pages:**

- **`tenant-detail.tsx`** — Two log tabs:
  - **"Access Logs" tab:** `TenantLogViewer` with all log types for this tenant.
    Shows nginx access/error, PHP errors, and runtime output.
  - **"Platform Logs" tab (existing):** The current `LogViewer` querying platform
    Loki for `|= "{tenantID}"`. Shows core-api/worker/node-agent operational
    logs about this tenant.

- **`webroot-detail.tsx`** — Add `TenantLogViewer` filtered to specific webroot.
  Shows access logs by default.

- **`fqdn-detail.tsx`** — Add `TenantLogViewer` filtered to the FQDN's webroot.
  Shows access logs by default.

- **`database-detail.tsx`**, **`zone-detail.tsx`**, **`valkey-detail.tsx`**,
  **`s3-bucket-detail.tsx`** — These resources don't generate tenant Loki logs
  (no nginx/PHP involved). Keep only the existing platform `LogViewer` for
  operational logs about these resources.

### Phase 8: Future — Customer Portal Access

When an OIDC-authenticated customer portal is implemented, tenants access their
own logs through it. The architecture is:

```
Customer Portal (SPA)
  -> Customer API (Go, OIDC-authenticated)
    -> GET /my/logs?log_type=access&webroot_id=xxx
      -> Core API GET /tenants/{tenantID}/logs (internal, API-key-authenticated)
        -> Tenant Loki
```

The customer API resolves the authenticated user's tenant ID from the OIDC token
and calls the core API tenant-scoped log endpoint. The core API does not need
to know about OIDC. This is the same pattern used for all other customer-facing
resources.

No additional work is needed in the core platform for this — the tenant-scoped
log endpoint from Phase 6 is the foundation.

## Log Deletion for Abuse/DDoS Scenarios

When a tenant is targeted by a DDoS attack, millions of access log entries can
flood tenant Loki. We need the ability to delete logs for a specific tenant to
reclaim storage.

**Loki Compactor with retention and deletion:**

Tenant Loki must be configured with the Compactor component and deletion API
enabled:

```yaml
compactor:
  working_directory: /loki/compactor
  retention_enabled: true
  delete_request_store: filesystem
limits_config:
  allow_deletes: true
```

This enables the `DELETE /loki/api/v1/delete` API endpoint, which accepts a
label selector and time range:

```
POST /loki/api/v1/delete?query={tenant_id="abc123"}&start=...&end=...
```

**Core API endpoint:**

```
DELETE /tenants/{tenantID}/logs?start=...&end=...
```

The handler proxies to tenant Loki's delete API with the tenant_id label
selector. Requires `tenants:write` scope. Optional `start`/`end` params allow
deleting a specific time window (e.g., the attack window). If omitted, deletes
all logs for the tenant.

Deletion in Loki is async — the compactor processes delete requests during its
next compaction cycle. The API returns 204 immediately. A subsequent query will
show progressively fewer results as compaction runs.

**Admin UI:** Add a "Purge Logs" button on the tenant Access Logs tab with a
confirmation dialog. Shows the time range being purged (defaults to "all").

## Alternative Considered: Skip Disk, Ship Directly to Syslog

Instead of writing to local log files and having Vector tail them, nginx and
php-fpm could write to syslog, and Vector could listen on a local syslog socket.

**Pros:**
- Eliminates local disk I/O for logs entirely.
- No logrotate configuration needed.
- Slightly simpler pipeline (no file discovery).

**Cons:**
- Syslog loses logs if Vector is restarting or down. File-based tailing with
  Vector positions tracking is more resilient — Vector picks up where it left
  off.
- PHP-FPM error_log does not support syslog well (it can write to syslog, but
  multiline stack traces get split across syslog messages).
- Harder to debug on the node itself (`tail -f` is useful for operators).
- nginx `access_log syslog:` loses the JSON format control we want.

**Decision:** Use local files. The local SSD I/O cost is negligible, and the
resilience and debuggability benefits outweigh the slight complexity of
logrotate.

## Alternative Considered: Alloy Instead of Vector on Web Nodes

The control plane uses Alloy (Grafana's agent) in k3s to ship pod logs to Loki
(see `deploy/k3s/alloy.yaml`). Web nodes use Vector instead.

**Why keep Vector on web nodes:**
- Vector is already deployed on all node VMs via Terraform cloud-init (see
  `deploy/vector/base.toml` and `deploy/vector/web.toml`).
- Vector's file source handles glob discovery, log rotation, and position
  tracking well.
- Mixing Alloy (k3s) and Vector (VMs) is fine — they serve different
  environments. Alloy uses Kubernetes service discovery; Vector uses file
  discovery. No reason to switch.

## Alternative Considered: Single Loki with Multi-Tenancy

Loki supports multi-tenancy via the `X-Scope-OrgID` header. We could use a
single Loki instance with separate org IDs for "platform" and "tenant" logs.

**Why separate instances instead:**
- Simpler operational model — each instance has its own retention, limits, PVC.
- No risk of one workload's misconfiguration affecting the other.
- Tenant Loki can be independently scaled (or replaced with Loki SimpleScalable)
  when log volume grows.
- Easier to reason about storage capacity and costs per instance.

## Performance Considerations

| Concern | Mitigation |
|---|---|
| Local SSD write throughput | NVMe SSDs handle 100K+ sequential writes/sec. Even the busiest tenant will not saturate this. |
| Vector CPU/memory on web nodes | Vector is lightweight. File tailing is inotify-based, not polling. Memory usage is bounded by batch size (default 1 MB). |
| Tenant Loki ingestion rate | Single-node Loki handles ~10 GB/day. Access logs are ~200 bytes each; 50M requests/day = 10 GB. At that scale, move to SimpleScalable mode. |
| Loki label cardinality | `tenant_id` x `webroot_id` x `log_type` x `hostname`. At 10K tenants x 3 webroots x 5 types x 3 nodes = 450K streams. This is within Loki's comfort zone for SimpleScalable deployment. At millions of tenants, migrate to structured metadata. |
| CephFS relief | Removing log writes eliminates the most write-heavy I/O pattern on CephFS. Remaining CephFS I/O is file serving (reads) and deployments (occasional writes). |
| Query latency | Loki queries by label are O(1) lookup + sequential scan of matching chunks. Tenant-scoped queries touch only that tenant's streams. Sub-second for typical queries. |

## Implementation Order

### Phase 1 (immediate)
- Change nginx log paths in `internal/agent/nginx.go` template.
- Change PHP-FPM log paths in `internal/agent/runtime/php.go` template.
- Add `TenantID` / `WebrootID` to nginx template data struct.
- Add local log directory creation in `internal/agent/tenant.go`.
- Add `log_format hosting_json` to nginx main config (Terraform).
- Remove `logs/` directory from CephFS tenant layout.
- Node-agent convergence will regenerate all nginx and PHP-FPM configs on next
  shard convergence cycle, moving logs to local disk automatically.

### Phase 2 (same sprint)
- Deploy `deploy/k3s/loki-tenant.yaml` — second Loki instance on port 3101.
- Verify it's running and accepting pushes.

### Phase 3 (same sprint)
- Update `deploy/vector/web.toml` with per-tenant file sources and transforms.
- Change Vector sink to point to tenant Loki (`http://10.10.10.2:3101`).
- Redeploy Vector on web nodes (Terraform apply or rolling restart).
- Verify logs appear in tenant Loki via Grafana Explore.

### Phase 4 (same sprint)
- Add logrotate config to web node cloud-init (Terraform).
- Verify rotation works with Vector (no log loss).

### Phase 5 (same sprint)
- Add `TenantLokiURL` to config, Helm chart.
- Update `NewLogs` constructor to accept both Loki URLs.
- Refactor `Logs.Query` to use shared `queryLoki` helper.

### Phase 6 (next sprint)
- Implement `GET /tenants/{tenantID}/logs` handler (`TenantLogs`).
- Register route in `internal/api/server.go`.
- Add tests for the handler.

### Phase 7 (next sprint)
- Add `useTenantLogs` hook.
- Create `TenantLogViewer` component.
- Update tenant, webroot, and FQDN detail pages with access log tabs.
- Keep existing platform `LogViewer` on all detail pages for operational logs.
- Rebuild admin UI (`just vm-deploy`).

### Phase 8 (future)
- Customer portal integration (depends on OIDC/customer API work).

## Files to Modify

| File | Change |
|---|---|
| `internal/agent/nginx.go` | Change log paths to `/var/log/hosting/{tenantID}/`, add `TenantID`/`WebrootID` to template data, use `hosting_json` log format |
| `internal/agent/nginx_test.go` | Update expected output in tests |
| `internal/agent/runtime/php.go` | Change error_log path, add slowlog directive, add `TenantID` to template data |
| `internal/agent/runtime/php_test.go` | Update expected output in tests |
| `internal/agent/tenant.go` | Create `/var/log/hosting/{tenantID}/` on provision, remove on delete, stop creating CephFS `logs/` dir |
| `internal/config/config.go` | Add `TenantLokiURL` field |
| `internal/api/handler/logs.go` | Add `tenantLokiURL` field, `TenantLogs` handler, `DeleteTenantLogs` handler, refactor `queryLoki` helper |
| `internal/api/server.go` | Register `GET /tenants/{tenantID}/logs` and `DELETE /tenants/{tenantID}/logs` routes, pass `tenantLokiURL` to `NewLogs` |
| `deploy/k3s/loki-tenant.yaml` | **New** — Second Loki StatefulSet on port 3101 |
| `deploy/vector/web.toml` | Replace global nginx sources with per-tenant file discovery, ship to tenant Loki |
| `terraform/modules/web-node/` | Add `log_format hosting_json` to nginx.conf, add logrotate config |
| `deploy/helm/hosting/templates/configmap.yaml` | Add `TENANT_LOKI_URL` |
| `deploy/helm/hosting/values.yaml` | Add `tenantLokiUrl` default |
| `web/admin/src/lib/hooks.ts` | Add `useTenantLogs` hook |
| `web/admin/src/lib/types.ts` | Add tenant log query params type if needed |
| `web/admin/src/components/shared/tenant-log-viewer.tsx` | **New** — Tenant log viewer with log type/webroot filters |
| `web/admin/src/pages/tenant-detail.tsx` | Add "Access Logs" tab with `TenantLogViewer` |
| `web/admin/src/pages/webroot-detail.tsx` | Add `TenantLogViewer` filtered by webroot |
| `web/admin/src/pages/fqdn-detail.tsx` | Add `TenantLogViewer` filtered by webroot |

## Verification

1. Deploy tenant Loki, verify it's running on port 3101.
2. Deploy updated Vector on web nodes, generate traffic, verify logs appear in
   tenant Loki via Grafana Explore (`{job="nginx"}`).
3. Verify platform Loki still only has core-api/worker/node-agent logs (no
   tenant access logs leaked in).
4. Query `GET /api/v1/tenants/{id}/logs?log_type=access&start=1h&limit=10` —
   should return nginx access log entries.
5. Navigate to tenant detail → Access Logs tab in admin UI — verify entries.
6. Filter by webroot — verify only that webroot's logs show.
7. Navigate to webroot detail — verify access logs show for that webroot.
8. Navigate to tenant detail → Platform Logs tab — verify core-api/worker logs
   still show (from platform Loki).
9. Verify logrotate works: generate 100MB+ of logs, confirm rotation and no
   log loss in Loki.
10. Click "View in Grafana" — verify Grafana opens with correct query against
    tenant Loki datasource.
