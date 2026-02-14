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

## Log Types

| Type | Source | Current Location | Content |
|---|---|---|---|
| Nginx access log | nginx per-webroot | CephFS `logs/{webroot}-access.log` | Request method, path, status, duration, bytes, user agent |
| Nginx error log | nginx per-webroot | CephFS `logs/{webroot}-error.log` | PHP fatal errors, upstream timeouts, permission errors |
| PHP-FPM error log | php-fpm per-tenant | CephFS `logs/php-error.log` | PHP errors, warnings, notices (runtime) |
| PHP-FPM slow log | php-fpm per-tenant | Not configured | Stack traces for requests exceeding threshold |
| Runtime stdout/stderr | Node.js / Python / Ruby | Not captured | Application console output, crash traces |
| Cron output | Future: scheduled tasks | Not implemented | stdout/stderr from cron job executions |
| Worker output | Future: background workers | Not implemented | supervisord-captured stdout/stderr |

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

1. **`internal/agent/nginx.go`** -- Change the nginx template log paths:

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

2. **`internal/agent/runtime/php.go`** -- Change the PHP-FPM pool template:

   ```
   # Before
   php_admin_value[error_log] = /var/www/storage/{{ .TenantName }}/logs/php-error.log

   # After
   php_admin_value[error_log] = /var/log/hosting/{{ .TenantID }}/php-error.log
   php_admin_value[slowlog] = /var/log/hosting/{{ .TenantID }}/php-slow.log
   php_admin_value[request_slowlog_timeout] = 5s
   ```

   Template data struct needs `TenantID` field added.

3. **`internal/agent/tenant.go`** -- Create the local log directory during
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

4. **`internal/agent/tenant.go`** -- Remove the CephFS `logs/` directory from
   the tenant directory layout. The `webroots/`, `home/`, and `tmp/`
   directories remain on CephFS. The `logs/` directory is no longer created
   on CephFS.

5. **Nginx main config (Terraform/cloud-init)** -- Add the `log_format`
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

   This produces one JSON object per line -- no regex parsing needed downstream.

### Phase 2: Ship Logs to Loki via Vector

Web nodes already run Vector with the base config (`deploy/vector/base.toml`)
and the web overlay (`deploy/vector/web.toml`). The web overlay currently ships
the global nginx access.log and error.log. Replace this with per-tenant file
discovery.

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

# Ship all tenant logs to Loki.
[sinks.loki_tenant]
type = "loki"
inputs = [
  "parse_tenant_access",
  "parse_tenant_error",
  "parse_tenant_php",
  "parse_tenant_php_slow",
  "parse_tenant_app",
]
endpoint = "http://10.10.10.2:3100"
encoding.codec = "text"

[sinks.loki_tenant.labels]
job = "{{ job }}"
tenant_id = "{{ tenant_id }}"
webroot_id = "{{ webroot_id }}"
log_type = "{{ log_type }}"
hostname = "{{ host }}"
level = "{{ level }}"
```

**Label design for Loki:**

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

### Phase 3: Log Rotation

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

- **`copytruncate`** instead of `create` -- avoids needing to signal nginx/php-fpm
  to reopen log files. Vector handles file truncation correctly (it detects the
  file shrinking and re-reads from the beginning). This means no log lines are
  lost during rotation.
- **`rotate 2`** -- keep 2 days. Vector ships logs in near-real-time (seconds of
  delay), so local retention is only a safety buffer.
- **`maxsize 100M`** -- rotate mid-day if a single log file exceeds 100 MB.
  Protects against traffic spikes filling the local SSD.
- **`daily`** -- normal rotation cadence.
- **`compress` / `delaycompress`** -- compress the previous rotation (not the
  current one, since Vector may still be reading it).

### Phase 4: Tenant-Scoped Log Query API

Extend the existing Loki proxy endpoint to support tenant-scoped queries.

**Current state:** `GET /logs` (see `internal/api/handler/logs.go`) accepts an
arbitrary LogQL query string and proxies it to Loki. It is protected by the
`audit_logs:read` API key scope. This is an admin-only endpoint.

**New endpoint:** Add a tenant-scoped log endpoint:

```
GET /tenants/{tenantID}/logs?log_type=access&webroot_id=xxx&start=1h&limit=500
```

This endpoint constructs the LogQL query server-side, ensuring tenants can only
see their own logs. The handler:

1. Extracts the tenant ID from the URL path.
2. Validates brand scope (same as all other tenant-scoped endpoints).
3. Builds a LogQL query: `{tenant_id="{tenantID}"}` with optional filters for
   `log_type` and `webroot_id`.
4. Proxies to Loki via the same mechanism as the existing `logs.Query` handler.

**Handler implementation sketch:**

```go
// TenantLogs godoc
//
//  @Summary     Query tenant logs
//  @Description Query logs for a specific tenant (access, error, PHP, app)
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

    // Reuse existing Loki proxy logic with the constructed query.
    // ...
}
```

**Route registration in `internal/api/server.go`:**

```go
r.Route("/tenants/{tenantID}", func(r chi.Router) {
    // ... existing routes ...
    r.Get("/logs", logs.TenantLogs)
})
```

**API key scope:** This endpoint should require a new `tenant_logs:read` scope
(or reuse the existing `tenants:read` scope since it is tenant-scoped data).

### Phase 5: Admin UI Integration

The existing `LogViewer` component (see
`web/admin/src/components/shared/log-viewer.tsx`) already handles Loki log
display with time range selection, auto-refresh, service filtering, and
expandable JSON details. It queries `GET /logs?query=...`.

**Changes needed:**

1. **New `useTenantLogs` hook** in `web/admin/src/lib/hooks.ts`:

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

2. **New `TenantLogViewer` component** -- wraps `LogViewer` with a log type
   selector dropdown (Access, Error, PHP Error, PHP Slow, Application) and an
   optional webroot filter dropdown. This component calls the tenant-scoped
   API endpoint instead of the raw LogQL endpoint.

3. **Update detail pages:**

   - **`tenant-detail.tsx`** -- Replace the current `LogViewer` (which queries
     platform logs mentioning the tenant ID) with `TenantLogViewer` showing
     actual tenant application logs. Keep a separate "Platform Logs" tab for
     admins to see core-api/worker/node-agent logs related to this tenant.

   - **`webroot-detail.tsx`** -- Replace the current `LogViewer` with
     `TenantLogViewer` filtered to the specific webroot ID. Show access logs
     by default.

   - **`fqdn-detail.tsx`** -- Show access logs filtered by the FQDN's webroot.

4. **Log type display formatting:** The `LogEntryRow` component currently
   parses JSON logs from zerolog. Tenant access logs are also JSON (from the
   nginx `hosting_json` format), so the existing expand-to-see-JSON behavior
   works. The summary line should show `method uri status` for access logs
   and the raw message for error logs. Detect by the `log_type` label in the
   entry.

### Phase 6: Future -- Customer Portal Access

When an OIDC-authenticated customer portal is implemented, tenants access their
own logs through it. The architecture is:

```
Customer Portal (SPA)
  -> Customer API (Go, OIDC-authenticated)
    -> GET /my/logs?log_type=access&webroot_id=xxx
      -> Core API GET /tenants/{tenantID}/logs (internal, API-key-authenticated)
        -> Loki
```

The customer API resolves the authenticated user's tenant ID from the OIDC token
and calls the core API tenant-scoped log endpoint. The core API does not need
to know about OIDC. This is the same pattern used for all other customer-facing
resources.

No additional work is needed in the core platform for this -- the tenant-scoped
log endpoint from Phase 4 is the foundation.

## Alternative Considered: Skip Disk, Ship Directly to Syslog

Instead of writing to local log files and having Vector tail them, nginx and
php-fpm could write to syslog, and Vector could listen on a local syslog socket.

**Pros:**
- Eliminates local disk I/O for logs entirely.
- No logrotate configuration needed.
- Slightly simpler pipeline (no file discovery).

**Cons:**
- Syslog loses logs if Vector is restarting or down. File-based tailing with
  Vector positions tracking is more resilient -- Vector picks up where it left
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
- Mixing Alloy (k3s) and Vector (VMs) is fine -- they serve different
  environments. Alloy uses Kubernetes service discovery; Vector uses file
  discovery. No reason to switch.

## Performance Considerations

| Concern | Mitigation |
|---|---|
| Local SSD write throughput | NVMe SSDs handle 100K+ sequential writes/sec. Even the busiest tenant will not saturate this. |
| Vector CPU/memory on web nodes | Vector is lightweight. File tailing is inotify-based, not polling. Memory usage is bounded by batch size (default 1 MB). |
| Loki ingestion rate | Loki single-node handles ~10 GB/day. At scale, move to Loki SimpleScalable or microservices mode. |
| Loki label cardinality | `tenant_id` x `webroot_id` x `log_type` x `hostname`. At 10K tenants x 3 webroots x 5 types x 3 nodes = 450K streams. This is within Loki's comfort zone for SimpleScalable deployment. At millions of tenants, migrate to structured metadata. |
| CephFS relief | Removing log writes eliminates the most write-heavy I/O pattern on CephFS. Remaining CephFS I/O is file serving (reads) and deployments (occasional writes). |
| Query latency | Loki queries by label are O(1) lookup + sequential scan of matching chunks. Tenant-scoped queries touch only that tenant's streams. Sub-second for typical queries. |

## Migration Path

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
- Update `deploy/vector/web.toml` with per-tenant file sources and transforms.
- Redeploy Vector on web nodes (Terraform apply or rolling restart).
- Verify logs appear in Loki with correct labels via Grafana Explore.

### Phase 3 (same sprint)
- Add logrotate config to web node cloud-init (Terraform).
- Verify rotation works with Vector (no log loss).

### Phase 4 (next sprint)
- Implement `GET /tenants/{tenantID}/logs` handler.
- Register route in `internal/api/server.go`.
- Add API key scope for tenant log access.
- Add tests for the handler (similar to existing `logs_test.go` pattern).

### Phase 5 (next sprint)
- Add `useTenantLogs` hook and `TenantLogViewer` component.
- Update tenant, webroot, and FQDN detail pages.
- Rebuild admin UI (`just vm-deploy`).

### Phase 6 (future)
- Customer portal integration (depends on OIDC/customer API work).

## Files to Modify

| File | Change |
|---|---|
| `internal/agent/nginx.go` | Change log paths to `/var/log/hosting/{tenantID}/`, add `TenantID`/`WebrootID` to template data, use `hosting_json` log format |
| `internal/agent/nginx_test.go` | Update expected output in tests |
| `internal/agent/runtime/php.go` | Change error_log path, add slowlog directive, add `TenantID` to template data |
| `internal/agent/runtime/php_test.go` | Update expected output in tests |
| `internal/agent/tenant.go` | Create `/var/log/hosting/{tenantID}/` on provision, remove on delete, stop creating CephFS `logs/` dir |
| `internal/api/handler/logs.go` | Add `TenantLogs` handler method |
| `internal/api/server.go` | Register `GET /tenants/{tenantID}/logs` route |
| `deploy/vector/web.toml` | Replace global nginx sources with per-tenant file discovery and Loki labels |
| `terraform/modules/web-node/` | Add `log_format hosting_json` to nginx.conf, add logrotate config |
| `web/admin/src/lib/hooks.ts` | Add `useTenantLogs` hook |
| `web/admin/src/lib/types.ts` | Add tenant log query params type if needed |
| `web/admin/src/components/shared/` | Add `TenantLogViewer` component |
| `web/admin/src/pages/tenant-detail.tsx` | Use `TenantLogViewer` for application logs |
| `web/admin/src/pages/webroot-detail.tsx` | Use `TenantLogViewer` filtered by webroot |
| `web/admin/src/pages/fqdn-detail.tsx` | Use `TenantLogViewer` filtered by webroot |
