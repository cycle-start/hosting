# Loki Stream Inventory

All log streams across both Loki instances, with their labels and expected cardinality.

## Platform Loki (port 3100)

Infrastructure logs shipped by Vector (all VMs) and Alloy (k3s).

| Source | Labels | Cardinality |
|--------|--------|-------------|
| node-agent journald | `{job="node-agent", hostname}` | nodes (~10) |
| MySQL journald | `{job="mysql", hostname}` | db nodes (~2) |
| PowerDNS journald | `{job="powerdns", hostname}` | dns nodes (~1) |
| Stalwart journald | `{job="stalwart", hostname}` | email nodes (~1) |
| Valkey journald | `{job="valkey", hostname}` | valkey nodes (~1) |
| Ceph journald | `{job="ceph", hostname}` | storage nodes (~1) |
| HAProxy journald | `{job="haproxy", hostname}` | lb nodes (~1) |
| k3s pod logs (via Alloy) | `{namespace, pod, container}` | ~20 pods |

## Tenant Loki (port 3101)

Per-tenant logs shipped by Vector on web nodes. High-cardinality metadata (webroot_id, cron_job_id, daemon_name) is embedded as JSON in the log line, not as Loki labels.

| Source | Labels | JSON Metadata | Cardinality |
|--------|--------|---------------|-------------|
| nginx access | `{tenant_id, log_type="access", job="nginx", hostname, level=""}` | `webroot_id` | tenants x hosts |
| nginx error | `{tenant_id, log_type="error", job="nginx", hostname, level="error"}` | `webroot_id` | tenants x hosts |
| PHP error | `{tenant_id, log_type="php-error", job="php-fpm", hostname, level="error"}` | -- | tenants x hosts |
| PHP slow | `{tenant_id, log_type="php-slow", job="php-fpm", hostname, level="warn"}` | -- | tenants x hosts |
| App (runtime) | `{tenant_id, log_type="app", job="runtime", hostname, level=""}` | `webroot_id` | tenants x hosts |
| Cron (journald) | `{tenant_id, log_type="cron", job="cron", hostname, level}` | `cron_job_id` | tenants x hosts x 3 levels |
| Daemon (supervisor) | `{tenant_id, log_type="daemon", job="supervisor", hostname, level}` | `daemon_name` | tenants x hosts x 2 levels |

## Design Principles

- **`tenant_id` is the only high-cardinality label** -- it's unavoidable as the primary index for tenant isolation.
- **Per-resource IDs** (`webroot_id`, `cron_job_id`, `daemon_name`) are embedded as JSON fields in the log line body, filtered via LogQL `| json | field="value"`.
- **Stream formula**: `active_tenants x ~8_log_types x 2-3_hosts x ~3_levels ~ tenants x 50`.
- At 100k tenants: ~5M streams. At 1M tenants: ~50M streams (within Loki's capabilities with proper sharding).

## Querying Metadata Fields

Since webroot_id, cron_job_id, and daemon_name are in the JSON log line (not labels), use LogQL JSON parsing to filter:

```logql
# All access logs for a specific webroot
{tenant_id="t-abc123", log_type="access"} | json | webroot_id="w-def456"

# Cron logs for a specific job
{tenant_id="t-abc123", log_type="cron"} | json | cron_job_id="cj-789"

# Daemon logs for a specific daemon
{tenant_id="t-abc123", log_type="daemon"} | json | daemon_name="my-worker"
```

The API handler (`/tenants/{id}/logs`) accepts `webroot_id`, `cron_job_id`, and `daemon_name` query parameters and automatically constructs the correct LogQL with JSON line filters.
