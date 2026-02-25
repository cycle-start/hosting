# Alerting Implementation Plan

## Overview

This document defines the alerting strategy for the hosting platform. All alerts are evaluated by Prometheus recording/alerting rules and routed through Grafana's built-in alerting pipeline for notification delivery.

### Current State

- **Prometheus** (`prom/prometheus:v2.53.3`) scrapes 11 jobs: `core-api`, `worker`, `haproxy`, `temporal`, `node-web` (2 targets), `node-db`, `node-dns`, `node-valkey`, `node-storage`, `node-dbadmin`, `node-lb`.
- **Grafana** (`grafana/grafana:11.5.2`) has 3 dashboards: API Overview, Infrastructure, Log Explorer.
- **Metrics exposed**: `http_requests_total{method,path,status}` and `http_request_duration_seconds_bucket{method,path}` from core-api; standard `node_exporter` metrics from all VMs; HAProxy exporter metrics from the LB node.
- **No alerting rules** are configured in Prometheus or Grafana.

### Design Decisions

1. **Prometheus alerting rules** define all alert conditions with PromQL. Prometheus evaluates them every 15s (matching `evaluation_interval`).
2. **Alertmanager is NOT deployed.** Grafana receives firing alerts via its built-in Prometheus alerting integration (Grafana evaluates Prometheus-sourced alert rules natively using the Prometheus data source).
3. **Grafana Unified Alerting** is the notification pipeline. Contact points start with a generic webhook; Slack, PagerDuty, and email are added when the platform moves to production.
4. **Every alert includes a `runbook_url` annotation** pointing to `docs/runbooks/<alert-name>.md`.
5. **Severity labels**: `critical` (pages on-call, requires immediate action), `warning` (investigate within 1 hour), `info` (awareness, no action required).

---

## 1. Architecture

```
Prometheus (scrape + evaluate rules)
    |
    v
Grafana Unified Alerting (receives alert states via data source queries)
    |
    v
Contact Points (webhook -> future: Slack, PagerDuty, email)
    |
    v
Notification Policies (route by severity label)
```

### How It Works

Grafana's Unified Alerting queries Prometheus directly. We define alert rules in Grafana provisioning YAML (not in `prometheus.yml` rule_files) so that Grafana manages the full lifecycle: evaluation, state tracking, silencing, and notification routing. This avoids deploying a separate Alertmanager.

### File Layout

```
deploy/k3s/
  grafana.yaml                         # Updated: mount alerting provisioning
docker/grafana/provisioning/
  alerting/
    contact-points.yaml                # Webhook contact point
    notification-policies.yaml         # Route by severity
    alert-rules.yaml                   # All alert rules (Grafana managed)
  dashboards/
    api-overview.json                  # Existing
    infrastructure.json                # Existing
    log-explorer.json                  # Existing
docs/runbooks/
    NodeDown.md
    HighDiskUsage.md
    HighMemoryUsage.md
    HighCpuUsage.md
    CoreApi5xxRate.md
    CoreApiHighLatency.md
    WorkflowFailureRate.md
    TemporalTaskQueueBacklog.md
    TemporalWorkflowTimeout.md
    DatabaseReplicationLag.md
    DatabaseConnectionPoolExhaustion.md
    HAProxyBackendDown.md
    NginxDown.md
    PHPFPMPoolExhausted.md
    NodeExporterDown.md
    CoreApiDown.md
    WorkerDown.md
    HighHAProxyErrorRate.md
    CephHealthDegraded.md
    ValkeyHighMemory.md
    StalwartDown.md
```

---

## 2. Alert Categories and PromQL Expressions

### 2.1 Infrastructure Alerts

#### NodeDown

A node exporter target has been unreachable for over 3 minutes.

```yaml
alert: NodeDown
expr: up{job=~"node-.*"} == 0
for: 3m
labels:
  severity: critical
annotations:
  summary: "Node {{ $labels.instance }} is down"
  description: "Node exporter on {{ $labels.instance }} (job={{ $labels.job }}) has been unreachable for more than 3 minutes."
  runbook_url: "docs/runbooks/NodeDown.md"
```

#### NodeExporterDown

Same as NodeDown but covers the case where the exporter process specifically is gone while the VM might still be reachable.

```yaml
alert: NodeExporterDown
expr: up{job=~"node-.*"} == 0
for: 5m
labels:
  severity: warning
annotations:
  summary: "Node exporter on {{ $labels.instance }} is not responding"
  description: "Prometheus cannot scrape node_exporter on {{ $labels.instance }} for 5 minutes. The VM may be up but the exporter crashed."
  runbook_url: "docs/runbooks/NodeExporterDown.md"
```

#### HighDiskUsage

Filesystem usage above 80% on any mounted volume (excluding tmpfs and overlay).

```yaml
alert: HighDiskUsage
expr: |
  (
    (node_filesystem_size_bytes{fstype!~"tmpfs|overlay"} - node_filesystem_avail_bytes{fstype!~"tmpfs|overlay"})
    / node_filesystem_size_bytes{fstype!~"tmpfs|overlay"}
  ) * 100 > 80
for: 5m
labels:
  severity: warning
annotations:
  summary: "Disk usage above 80% on {{ $labels.instance }}:{{ $labels.mountpoint }}"
  description: "Filesystem {{ $labels.mountpoint }} on {{ $labels.instance }} is at {{ $value | printf \"%.1f\" }}% capacity."
  runbook_url: "docs/runbooks/HighDiskUsage.md"
```

#### HighDiskUsageCritical

Filesystem usage above 95% -- imminent risk of service failure.

```yaml
alert: HighDiskUsageCritical
expr: |
  (
    (node_filesystem_size_bytes{fstype!~"tmpfs|overlay"} - node_filesystem_avail_bytes{fstype!~"tmpfs|overlay"})
    / node_filesystem_size_bytes{fstype!~"tmpfs|overlay"}
  ) * 100 > 95
for: 2m
labels:
  severity: critical
annotations:
  summary: "CRITICAL: Disk usage above 95% on {{ $labels.instance }}:{{ $labels.mountpoint }}"
  description: "Filesystem {{ $labels.mountpoint }} on {{ $labels.instance }} is at {{ $value | printf \"%.1f\" }}% capacity. Immediate action required."
  runbook_url: "docs/runbooks/HighDiskUsage.md"
```

#### HighMemoryUsage

Memory usage above 90% for a sustained period.

```yaml
alert: HighMemoryUsage
expr: |
  (
    1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)
  ) * 100 > 90
for: 5m
labels:
  severity: warning
annotations:
  summary: "Memory usage above 90% on {{ $labels.instance }}"
  description: "Node {{ $labels.instance }} memory usage is at {{ $value | printf \"%.1f\" }}%. Available: {{ with printf \"node_memory_MemAvailable_bytes{instance='%s'}\" $labels.instance | query }}{{ . | first | value | humanize1024 }}{{ end }}."
  runbook_url: "docs/runbooks/HighMemoryUsage.md"
```

#### HighMemoryUsageCritical

Memory usage above 97% -- OOM kill risk.

```yaml
alert: HighMemoryUsageCritical
expr: |
  (
    1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)
  ) * 100 > 97
for: 2m
labels:
  severity: critical
annotations:
  summary: "CRITICAL: Memory usage above 97% on {{ $labels.instance }}"
  description: "Node {{ $labels.instance }} is nearly out of memory ({{ $value | printf \"%.1f\" }}% used). OOM kills are imminent."
  runbook_url: "docs/runbooks/HighMemoryUsage.md"
```

#### HighCpuUsage

Sustained CPU usage above 80% over 10 minutes. Uses `irate` over a 5m window smoothed with the `for` clause to avoid one-off spikes.

```yaml
alert: HighCpuUsage
expr: |
  (
    1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m]))
  ) * 100 > 80
for: 10m
labels:
  severity: warning
annotations:
  summary: "CPU usage sustained above 80% on {{ $labels.instance }}"
  description: "Node {{ $labels.instance }} CPU usage has been at {{ $value | printf \"%.1f\" }}% for more than 10 minutes."
  runbook_url: "docs/runbooks/HighCpuUsage.md"
```

#### HighCpuUsageCritical

Sustained CPU usage above 95% -- the node is saturated.

```yaml
alert: HighCpuUsageCritical
expr: |
  (
    1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m]))
  ) * 100 > 95
for: 5m
labels:
  severity: critical
annotations:
  summary: "CRITICAL: CPU saturated on {{ $labels.instance }}"
  description: "Node {{ $labels.instance }} CPU usage has been at {{ $value | printf \"%.1f\" }}% for more than 5 minutes."
  runbook_url: "docs/runbooks/HighCpuUsage.md"
```

#### DiskWillFillIn4Hours

Predictive alert: linear extrapolation of disk usage predicts the filesystem will be full within 4 hours.

```yaml
alert: DiskWillFillIn4Hours
expr: |
  (
    node_filesystem_avail_bytes{fstype!~"tmpfs|overlay"} > 0
  and
    predict_linear(node_filesystem_avail_bytes{fstype!~"tmpfs|overlay"}[1h], 4 * 3600) < 0
  )
for: 15m
labels:
  severity: warning
annotations:
  summary: "Disk on {{ $labels.instance }}:{{ $labels.mountpoint }} predicted full within 4 hours"
  description: "Based on the trend over the last hour, filesystem {{ $labels.mountpoint }} on {{ $labels.instance }} will run out of space within approximately 4 hours."
  runbook_url: "docs/runbooks/HighDiskUsage.md"
```

---

### 2.2 Application Alerts

#### CoreApiDown

The core-api process is unreachable.

```yaml
alert: CoreApiDown
expr: up{job="core-api"} == 0
for: 1m
labels:
  severity: critical
annotations:
  summary: "Core API is down"
  description: "Prometheus cannot scrape the core-api metrics endpoint at {{ $labels.instance }} for over 1 minute."
  runbook_url: "docs/runbooks/CoreApiDown.md"
```

#### WorkerDown

The Temporal worker process is unreachable.

```yaml
alert: WorkerDown
expr: up{job="worker"} == 0
for: 1m
labels:
  severity: critical
annotations:
  summary: "Worker is down"
  description: "Prometheus cannot scrape the worker metrics endpoint at {{ $labels.instance }} for over 1 minute."
  runbook_url: "docs/runbooks/WorkerDown.md"
```

#### CoreApi5xxRate

The 5xx error rate exceeds 1% of total requests over a 5-minute window.

```yaml
alert: CoreApi5xxRate
expr: |
  (
    sum(rate(http_requests_total{job="core-api", status=~"5.."}[5m]))
    /
    sum(rate(http_requests_total{job="core-api"}[5m]))
  ) * 100 > 1
for: 5m
labels:
  severity: warning
annotations:
  summary: "Core API 5xx error rate above 1%"
  description: "The 5xx error rate is {{ $value | printf \"%.2f\" }}% over the last 5 minutes."
  runbook_url: "docs/runbooks/CoreApi5xxRate.md"
```

#### CoreApi5xxRateCritical

The 5xx error rate exceeds 5% -- something is seriously wrong.

```yaml
alert: CoreApi5xxRateCritical
expr: |
  (
    sum(rate(http_requests_total{job="core-api", status=~"5.."}[5m]))
    /
    sum(rate(http_requests_total{job="core-api"}[5m]))
  ) * 100 > 5
for: 2m
labels:
  severity: critical
annotations:
  summary: "CRITICAL: Core API 5xx error rate above 5%"
  description: "The 5xx error rate is {{ $value | printf \"%.2f\" }}%. Immediate investigation required."
  runbook_url: "docs/runbooks/CoreApi5xxRate.md"
```

#### CoreApiHighLatency

P99 request latency exceeds 2 seconds sustained over 5 minutes.

```yaml
alert: CoreApiHighLatency
expr: |
  histogram_quantile(0.99,
    sum(rate(http_request_duration_seconds_bucket{job="core-api"}[5m])) by (le)
  ) > 2
for: 5m
labels:
  severity: warning
annotations:
  summary: "Core API P99 latency above 2 seconds"
  description: "The P99 request latency is {{ $value | printf \"%.2f\" }}s. Check database performance, Temporal connectivity, and resource utilization."
  runbook_url: "docs/runbooks/CoreApiHighLatency.md"
```

#### CoreApiHighLatencyCritical

P99 request latency exceeds 5 seconds -- likely a downstream dependency failure.

```yaml
alert: CoreApiHighLatencyCritical
expr: |
  histogram_quantile(0.99,
    sum(rate(http_request_duration_seconds_bucket{job="core-api"}[5m])) by (le)
  ) > 5
for: 2m
labels:
  severity: critical
annotations:
  summary: "CRITICAL: Core API P99 latency above 5 seconds"
  description: "The P99 request latency is {{ $value | printf \"%.2f\" }}s. A downstream dependency (database, Temporal) is likely degraded or failing."
  runbook_url: "docs/runbooks/CoreApiHighLatency.md"
```

#### WorkflowFailureRate

More than 5% of Temporal workflow executions are failing.

Note: This requires the Temporal server to expose `temporal_server_workflow_failed_total` and `temporal_server_workflow_completed_total` metrics. If Temporal SDK metrics are available on the worker, those can be used instead (see alternative expression).

```yaml
alert: WorkflowFailureRate
expr: |
  (
    sum(rate(temporal_workflow_failed_total{job="worker"}[15m]))
    /
    (
      sum(rate(temporal_workflow_completed_total{job="worker"}[15m]))
      + sum(rate(temporal_workflow_failed_total{job="worker"}[15m]))
    )
  ) * 100 > 5
for: 10m
labels:
  severity: warning
annotations:
  summary: "Workflow failure rate above 5%"
  description: "{{ $value | printf \"%.1f\" }}% of Temporal workflows are failing over the last 15 minutes."
  runbook_url: "docs/runbooks/WorkflowFailureRate.md"
```

**Alternative (if using Temporal SDK metrics on the worker):**

```promql
# Temporal Go SDK exposes these when configured with a Prometheus reporter:
# temporal_workflow_task_execution_failed_total
# temporal_workflow_task_execution_latency_bucket
(
  sum(rate(temporal_workflow_task_execution_failed_total{job="worker"}[15m]))
  /
  sum(rate(temporal_workflow_task_execution_latency_count{job="worker"}[15m]))
) * 100 > 5
```

---

### 2.3 Database Alerts

#### DatabaseReplicationLag

MySQL replication lag exceeds 30 seconds. Requires a MySQL exporter (e.g., `mysqld_exporter`) to be deployed on database nodes exposing `mysql_slave_status_seconds_behind_master`.

**Prerequisite**: Deploy `mysqld_exporter` on DB nodes and add a `node-db-mysql` scrape job to Prometheus.

```yaml
alert: DatabaseReplicationLag
expr: mysql_slave_status_seconds_behind_master{job="node-db-mysql"} > 30
for: 5m
labels:
  severity: warning
annotations:
  summary: "MySQL replication lag above 30s on {{ $labels.instance }}"
  description: "Replication is {{ $value | printf \"%.0f\" }}s behind master. This affects read consistency for tenants on this shard."
  runbook_url: "docs/runbooks/DatabaseReplicationLag.md"
```

#### DatabaseReplicationLagCritical

Replication lag exceeds 300 seconds (5 minutes).

```yaml
alert: DatabaseReplicationLagCritical
expr: mysql_slave_status_seconds_behind_master{job="node-db-mysql"} > 300
for: 2m
labels:
  severity: critical
annotations:
  summary: "CRITICAL: MySQL replication lag above 5 minutes on {{ $labels.instance }}"
  description: "Replication is {{ $value | printf \"%.0f\" }}s behind master. Failover may not be safe. Investigate immediately."
  runbook_url: "docs/runbooks/DatabaseReplicationLag.md"
```

#### DatabaseReplicationStopped

The replication SQL or IO thread has stopped.

```yaml
alert: DatabaseReplicationStopped
expr: mysql_slave_status_slave_sql_running{job="node-db-mysql"} == 0 or mysql_slave_status_slave_io_running{job="node-db-mysql"} == 0
for: 1m
labels:
  severity: critical
annotations:
  summary: "MySQL replication stopped on {{ $labels.instance }}"
  description: "The replication thread has stopped. Tenant data is no longer being replicated. Manual intervention required."
  runbook_url: "docs/runbooks/DatabaseReplicationLag.md"
```

#### DatabaseConnectionPoolExhaustion

The core-api pgx connection pool is nearing its limit. Requires the application to export `go_sql_open_connections` or a custom pgx pool metric.

**Prerequisite**: Add pgx pool metrics to core-api (pgx v5 has `pgxpool.Stat()` -- expose as Prometheus gauge).

```yaml
alert: DatabaseConnectionPoolExhaustion
expr: |
  (
    pgxpool_acquired_conns{job="core-api"}
    /
    pgxpool_max_conns{job="core-api"}
  ) * 100 > 80
for: 5m
labels:
  severity: warning
annotations:
  summary: "Core API database connection pool above 80% utilized"
  description: "{{ $value | printf \"%.0f\" }}% of the pgx connection pool is in use. If this reaches 100%, new requests will block or fail."
  runbook_url: "docs/runbooks/DatabaseConnectionPoolExhaustion.md"
```

#### DatabaseConnectionPoolExhausted

Pool is fully utilized.

```yaml
alert: DatabaseConnectionPoolExhausted
expr: |
  pgxpool_acquired_conns{job="core-api"} >= pgxpool_max_conns{job="core-api"}
for: 1m
labels:
  severity: critical
annotations:
  summary: "CRITICAL: Core API database connection pool exhausted"
  description: "All connections in the pgx pool are in use. New requests are being blocked. Immediate investigation required."
  runbook_url: "docs/runbooks/DatabaseConnectionPoolExhaustion.md"
```

---

### 2.4 Temporal Alerts

#### TemporalDown

The Temporal server is unreachable.

```yaml
alert: TemporalDown
expr: up{job="temporal"} == 0
for: 1m
labels:
  severity: critical
annotations:
  summary: "Temporal server is down"
  description: "Prometheus cannot scrape Temporal at {{ $labels.instance }}. No new workflows can be started or completed."
  runbook_url: "docs/runbooks/TemporalWorkflowTimeout.md"
```

#### TemporalTaskQueueBacklog

The number of pending activities or workflow tasks in a task queue is growing, indicating the worker cannot keep up.

**Note**: Temporal server exposes `temporal_task_queue_backlog` (Temporal >= 1.22). If unavailable, use the schedule-to-start latency metric from the SDK.

```yaml
alert: TemporalTaskQueueBacklog
expr: temporal_task_queue_backlog > 100
for: 10m
labels:
  severity: warning
annotations:
  summary: "Temporal task queue backlog exceeding 100 on {{ $labels.task_queue }}"
  description: "Task queue {{ $labels.task_queue }} has {{ $value | printf \"%.0f\" }} pending tasks. The worker may be under-provisioned or stuck."
  runbook_url: "docs/runbooks/TemporalTaskQueueBacklog.md"
```

**Alternative using SDK schedule-to-start latency (available on the worker):**

```yaml
alert: TemporalScheduleToStartLatencyHigh
expr: |
  histogram_quantile(0.95,
    sum(rate(temporal_activity_schedule_to_start_latency_bucket{job="worker"}[5m])) by (le, task_queue)
  ) > 30
for: 5m
labels:
  severity: warning
annotations:
  summary: "Temporal activity schedule-to-start latency above 30s"
  description: "Activities on task queue {{ $labels.task_queue }} are waiting {{ $value | printf \"%.0f\" }}s to be picked up. Scale the worker or investigate stuck activities."
  runbook_url: "docs/runbooks/TemporalTaskQueueBacklog.md"
```

#### TemporalWorkflowTimeout

Workflow execution timeouts are occurring.

```yaml
alert: TemporalWorkflowTimeout
expr: |
  sum(rate(temporal_workflow_timed_out_total{job="worker"}[15m])) > 0
for: 5m
labels:
  severity: warning
annotations:
  summary: "Temporal workflows are timing out"
  description: "{{ $value | printf \"%.2f\" }} workflows/sec are hitting their execution timeout. Check workflow history for stuck activities."
  runbook_url: "docs/runbooks/TemporalWorkflowTimeout.md"
```

#### TemporalActivityFailureRate

More than 10% of activity executions are failing (after retries exhaust).

```yaml
alert: TemporalActivityFailureRate
expr: |
  (
    sum(rate(temporal_activity_execution_failed_total{job="worker"}[15m]))
    /
    (
      sum(rate(temporal_activity_execution_latency_count{job="worker"}[15m]))
      + sum(rate(temporal_activity_execution_failed_total{job="worker"}[15m]))
    )
  ) * 100 > 10
for: 10m
labels:
  severity: warning
annotations:
  summary: "Temporal activity failure rate above 10%"
  description: "{{ $value | printf \"%.1f\" }}% of activities are failing. Node agents or external services may be unavailable."
  runbook_url: "docs/runbooks/WorkflowFailureRate.md"
```

---

### 2.5 Service Alerts

#### HAProxyBackendDown

An HAProxy backend has zero healthy servers.

```yaml
alert: HAProxyBackendDown
expr: haproxy_backend_active_servers{job="haproxy"} == 0
for: 1m
labels:
  severity: critical
annotations:
  summary: "HAProxy backend {{ $labels.proxy }} has no healthy servers"
  description: "All servers in HAProxy backend {{ $labels.proxy }} are down. Traffic to this backend is failing."
  runbook_url: "docs/runbooks/HAProxyBackendDown.md"
```

#### HAProxyBackendDegraded

An HAProxy backend has lost some (but not all) servers.

```yaml
alert: HAProxyBackendDegraded
expr: |
  haproxy_backend_active_servers{job="haproxy"} < haproxy_backend_servers_total{job="haproxy"}
  and
  haproxy_backend_active_servers{job="haproxy"} > 0
for: 5m
labels:
  severity: warning
annotations:
  summary: "HAProxy backend {{ $labels.proxy }} is degraded"
  description: "Backend {{ $labels.proxy }} has {{ $value | printf \"%.0f\" }} active servers out of expected. Some capacity has been lost."
  runbook_url: "docs/runbooks/HAProxyBackendDown.md"
```

#### HighHAProxyErrorRate

HAProxy is returning 5xx responses at an elevated rate.

```yaml
alert: HighHAProxyErrorRate
expr: |
  (
    sum(rate(haproxy_frontend_http_responses_total{job="haproxy", code="5xx"}[5m]))
    /
    sum(rate(haproxy_frontend_http_responses_total{job="haproxy"}[5m]))
  ) * 100 > 5
for: 5m
labels:
  severity: warning
annotations:
  summary: "HAProxy 5xx error rate above 5%"
  description: "{{ $value | printf \"%.1f\" }}% of HAProxy frontend responses are 5xx errors."
  runbook_url: "docs/runbooks/HighHAProxyErrorRate.md"
```

#### HAProxyHighConnectionRate

HAProxy is approaching its connection limit.

```yaml
alert: HAProxyHighConnectionRate
expr: |
  haproxy_frontend_current_sessions{job="haproxy"}
  /
  haproxy_frontend_limit_sessions{job="haproxy"} * 100 > 80
for: 5m
labels:
  severity: warning
annotations:
  summary: "HAProxy frontend {{ $labels.proxy }} at 80% session capacity"
  description: "Frontend {{ $labels.proxy }} is using {{ $value | printf \"%.0f\" }}% of its configured session limit."
  runbook_url: "docs/runbooks/HighHAProxyErrorRate.md"
```

#### NginxDown

Nginx on a web node is not responding. Requires an nginx status endpoint or a blackbox probe.

**Prerequisite**: Deploy `nginx-prometheus-exporter` on web nodes or use a blackbox exporter probe. Add a `node-web-nginx` scrape job.

```yaml
alert: NginxDown
expr: nginx_up{job="node-web-nginx"} == 0
for: 2m
labels:
  severity: critical
annotations:
  summary: "Nginx is down on {{ $labels.instance }}"
  description: "Nginx on web node {{ $labels.instance }} has been unresponsive for over 2 minutes. Tenant websites served by this node are affected."
  runbook_url: "docs/runbooks/NginxDown.md"
```

#### PHPFPMPoolExhausted

PHP-FPM active processes equal max_children, meaning new requests are queuing.

**Prerequisite**: Deploy `php-fpm_exporter` on web nodes or expose PHP-FPM status page metrics. Add a `node-web-phpfpm` scrape job.

```yaml
alert: PHPFPMPoolExhausted
expr: |
  phpfpm_active_processes{job="node-web-phpfpm"}
  >= phpfpm_max_children{job="node-web-phpfpm"}
for: 5m
labels:
  severity: warning
annotations:
  summary: "PHP-FPM pool exhausted on {{ $labels.instance }}"
  description: "All {{ $value | printf \"%.0f\" }} PHP-FPM workers are busy on {{ $labels.instance }}. New requests are queuing."
  runbook_url: "docs/runbooks/PHPFPMPoolExhausted.md"
```

#### PHPFPMListenQueueOverflow

The FPM listen queue is non-empty, meaning requests are waiting for a free worker.

```yaml
alert: PHPFPMListenQueueOverflow
expr: phpfpm_listen_queue{job="node-web-phpfpm"} > 0
for: 3m
labels:
  severity: warning
annotations:
  summary: "PHP-FPM listen queue non-empty on {{ $labels.instance }}"
  description: "{{ $value | printf \"%.0f\" }} requests are waiting in the PHP-FPM listen queue. Workers may need scaling."
  runbook_url: "docs/runbooks/PHPFPMPoolExhausted.md"
```

#### CephHealthDegraded

Ceph cluster health is not HEALTH_OK. Relevant for the S3 storage node.

**Prerequisite**: Deploy `ceph_exporter` or use the Ceph MGR Prometheus module on the storage node. Add a `node-storage-ceph` scrape job.

```yaml
alert: CephHealthDegraded
expr: ceph_health_status{job="node-storage-ceph"} > 0
for: 5m
labels:
  severity: warning
annotations:
  summary: "Ceph cluster health is degraded"
  description: "Ceph health status is {{ $value }} (0=HEALTH_OK, 1=HEALTH_WARN, 2=HEALTH_ERR). S3 storage may be at risk."
  runbook_url: "docs/runbooks/CephHealthDegraded.md"
```

#### CephHealthError

Ceph reports HEALTH_ERR -- data loss risk.

```yaml
alert: CephHealthError
expr: ceph_health_status{job="node-storage-ceph"} >= 2
for: 1m
labels:
  severity: critical
annotations:
  summary: "CRITICAL: Ceph cluster is in HEALTH_ERR state"
  description: "Ceph is reporting a critical error. Data integrity may be at risk. Immediate investigation required."
  runbook_url: "docs/runbooks/CephHealthDegraded.md"
```

#### ValkeyHighMemory

Valkey memory usage exceeds 80% of configured maxmemory.

**Prerequisite**: Deploy `redis_exporter` (compatible with Valkey) on the Valkey node. Add a `node-valkey-redis` scrape job.

```yaml
alert: ValkeyHighMemory
expr: |
  redis_memory_used_bytes{job="node-valkey-redis"}
  /
  redis_memory_max_bytes{job="node-valkey-redis"} * 100 > 80
for: 5m
labels:
  severity: warning
annotations:
  summary: "Valkey memory usage above 80% on {{ $labels.instance }}"
  description: "Valkey is using {{ $value | printf \"%.0f\" }}% of its configured maxmemory. Eviction policies may start affecting tenants."
  runbook_url: "docs/runbooks/ValkeyHighMemory.md"
```

#### StalwartDown

The Stalwart email server is unreachable.

**Prerequisite**: Add a scrape job or blackbox probe for Stalwart health.

```yaml
alert: StalwartDown
expr: up{job="node-email-stalwart"} == 0
for: 2m
labels:
  severity: critical
annotations:
  summary: "Stalwart email server is down"
  description: "Stalwart on {{ $labels.instance }} has been unresponsive for over 2 minutes. Email delivery and access is impacted."
  runbook_url: "docs/runbooks/StalwartDown.md"
```

---

## 3. Alert Delivery Configuration

### 3.1 Contact Points

Start with a generic webhook. This can forward to any system: a custom handler, Slack incoming webhook, PagerDuty events API, or a log sink.

**File: `docker/grafana/provisioning/alerting/contact-points.yaml`**

```yaml
apiVersion: 1

contactPoints:
  - orgId: 1
    name: default-webhook
    receivers:
      - uid: webhook-1
        type: webhook
        settings:
          url: "http://10.10.10.2:9095/alerts"
          httpMethod: POST
        disableResolveMessage: false
```

When ready for production, add additional contact points:

```yaml
  - orgId: 1
    name: slack-alerts
    receivers:
      - uid: slack-1
        type: slack
        settings:
          url: "${SLACK_WEBHOOK_URL}"
          recipient: "#hosting-alerts"
          title: |
            {{ `{{ .CommonAnnotations.summary }}` }}
          text: |
            {{ `{{ .CommonAnnotations.description }}` }}
        disableResolveMessage: false

  - orgId: 1
    name: pagerduty-critical
    receivers:
      - uid: pd-1
        type: pagerduty
        settings:
          integrationKey: "${PAGERDUTY_INTEGRATION_KEY}"
          severity: critical
        disableResolveMessage: false
```

### 3.2 Notification Policies

Route alerts by severity label. Critical goes to the on-call channel, warnings go to the team channel, info is logged only.

**File: `docker/grafana/provisioning/alerting/notification-policies.yaml`**

```yaml
apiVersion: 1

policies:
  - orgId: 1
    receiver: default-webhook
    group_by:
      - alertname
      - instance
    group_wait: 30s
    group_interval: 5m
    repeat_interval: 4h
    routes:
      - receiver: default-webhook
        matchers:
          - severity = critical
        group_wait: 10s
        group_interval: 1m
        repeat_interval: 1h
        continue: false
      - receiver: default-webhook
        matchers:
          - severity = warning
        group_wait: 30s
        group_interval: 5m
        repeat_interval: 4h
        continue: false
      - receiver: default-webhook
        matchers:
          - severity = info
        group_wait: 1m
        group_interval: 15m
        repeat_interval: 12h
        continue: false
```

### 3.3 Repeat Intervals

| Severity | Group Wait | Group Interval | Repeat Interval |
|----------|-----------|----------------|-----------------|
| critical | 10s       | 1m             | 1h              |
| warning  | 30s       | 5m             | 4h              |
| info     | 1m        | 15m            | 12h             |

---

## 4. Grafana Alert Rule Provisioning

All alert rules are defined in a single provisioning file. Grafana evaluates them directly against the Prometheus data source.

**File: `docker/grafana/provisioning/alerting/alert-rules.yaml`**

```yaml
apiVersion: 1

groups:
  - orgId: 1
    name: infrastructure
    folder: Alerts
    interval: 1m
    rules:
      - uid: node-down
        title: NodeDown
        condition: A
        data:
          - refId: A
            relativeTimeRange:
              from: 300
              to: 0
            datasourceUid: prometheus
            model:
              expr: up{job=~"node-.*"} == 0
              instant: true
              refId: A
        for: 3m
        labels:
          severity: critical
        annotations:
          summary: "Node {{ $labels.instance }} is down"
          runbook_url: "docs/runbooks/NodeDown.md"

      - uid: high-disk-usage
        title: HighDiskUsage
        condition: A
        data:
          - refId: A
            relativeTimeRange:
              from: 300
              to: 0
            datasourceUid: prometheus
            model:
              expr: >-
                ((node_filesystem_size_bytes{fstype!~"tmpfs|overlay"} - node_filesystem_avail_bytes{fstype!~"tmpfs|overlay"})
                / node_filesystem_size_bytes{fstype!~"tmpfs|overlay"}) * 100 > 80
              instant: true
              refId: A
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Disk above 80% on {{ $labels.instance }}:{{ $labels.mountpoint }}"
          runbook_url: "docs/runbooks/HighDiskUsage.md"

      - uid: high-memory-usage
        title: HighMemoryUsage
        condition: A
        data:
          - refId: A
            relativeTimeRange:
              from: 300
              to: 0
            datasourceUid: prometheus
            model:
              expr: (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100 > 90
              instant: true
              refId: A
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Memory above 90% on {{ $labels.instance }}"
          runbook_url: "docs/runbooks/HighMemoryUsage.md"

      - uid: high-cpu-usage
        title: HighCpuUsage
        condition: A
        data:
          - refId: A
            relativeTimeRange:
              from: 600
              to: 0
            datasourceUid: prometheus
            model:
              expr: (1 - avg by (instance)(rate(node_cpu_seconds_total{mode="idle"}[5m]))) * 100 > 80
              instant: true
              refId: A
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "CPU sustained above 80% on {{ $labels.instance }}"
          runbook_url: "docs/runbooks/HighCpuUsage.md"

  - orgId: 1
    name: application
    folder: Alerts
    interval: 30s
    rules:
      - uid: core-api-down
        title: CoreApiDown
        condition: A
        data:
          - refId: A
            relativeTimeRange:
              from: 300
              to: 0
            datasourceUid: prometheus
            model:
              expr: up{job="core-api"} == 0
              instant: true
              refId: A
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Core API is down"
          runbook_url: "docs/runbooks/CoreApiDown.md"

      - uid: worker-down
        title: WorkerDown
        condition: A
        data:
          - refId: A
            relativeTimeRange:
              from: 300
              to: 0
            datasourceUid: prometheus
            model:
              expr: up{job="worker"} == 0
              instant: true
              refId: A
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Worker is down"
          runbook_url: "docs/runbooks/WorkerDown.md"

      - uid: core-api-5xx-rate
        title: CoreApi5xxRate
        condition: A
        data:
          - refId: A
            relativeTimeRange:
              from: 300
              to: 0
            datasourceUid: prometheus
            model:
              expr: >-
                (sum(rate(http_requests_total{job="core-api",status=~"5.."}[5m]))
                / sum(rate(http_requests_total{job="core-api"}[5m]))) * 100 > 1
              instant: true
              refId: A
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Core API 5xx rate above 1%"
          runbook_url: "docs/runbooks/CoreApi5xxRate.md"

      - uid: core-api-high-latency
        title: CoreApiHighLatency
        condition: A
        data:
          - refId: A
            relativeTimeRange:
              from: 300
              to: 0
            datasourceUid: prometheus
            model:
              expr: >-
                histogram_quantile(0.99,
                  sum(rate(http_request_duration_seconds_bucket{job="core-api"}[5m])) by (le)
                ) > 2
              instant: true
              refId: A
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Core API P99 latency above 2s"
          runbook_url: "docs/runbooks/CoreApiHighLatency.md"

  - orgId: 1
    name: temporal
    folder: Alerts
    interval: 1m
    rules:
      - uid: temporal-down
        title: TemporalDown
        condition: A
        data:
          - refId: A
            relativeTimeRange:
              from: 300
              to: 0
            datasourceUid: prometheus
            model:
              expr: up{job="temporal"} == 0
              instant: true
              refId: A
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Temporal server is down"
          runbook_url: "docs/runbooks/TemporalWorkflowTimeout.md"

  - orgId: 1
    name: services
    folder: Alerts
    interval: 1m
    rules:
      - uid: haproxy-backend-down
        title: HAProxyBackendDown
        condition: A
        data:
          - refId: A
            relativeTimeRange:
              from: 300
              to: 0
            datasourceUid: prometheus
            model:
              expr: haproxy_backend_active_servers{job="haproxy"} == 0
              instant: true
              refId: A
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "HAProxy backend {{ $labels.proxy }} has no healthy servers"
          runbook_url: "docs/runbooks/HAProxyBackendDown.md"
```

> The full provisioning file should contain all rules from Section 2. The above is a representative subset showing the structure. Each alert from Section 2 maps to one rule entry.

---

## 5. Deployment Changes

### 5.1 Prometheus Configuration Update

Add `evaluation_interval` confirmation and rule_files (optional -- only needed if also evaluating Prometheus-native rules for recording rules):

```yaml
# deploy/k3s/prometheus.yaml ConfigMap addition
global:
  scrape_interval: 15s
  evaluation_interval: 15s

# Optional: Prometheus recording rules for pre-computed aggregations
rule_files:
  - /etc/prometheus/rules/*.yml
```

Recording rules to add for dashboard performance (not alerting -- those go through Grafana):

```yaml
# recording-rules.yml
groups:
  - name: hosting_recording_rules
    interval: 30s
    rules:
      - record: job:http_requests_total:rate5m
        expr: sum(rate(http_requests_total[5m])) by (job, method, status)

      - record: job:http_request_duration_seconds:p99_5m
        expr: histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (job, le))

      - record: job:http_request_duration_seconds:p95_5m
        expr: histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (job, le))

      - record: instance:node_cpu_utilization:ratio_5m
        expr: 1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m]))

      - record: instance:node_memory_utilization:ratio
        expr: 1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)

      - record: instance:node_filesystem_utilization:ratio
        expr: |
          (node_filesystem_size_bytes{fstype!~"tmpfs|overlay"} - node_filesystem_avail_bytes{fstype!~"tmpfs|overlay"})
          / node_filesystem_size_bytes{fstype!~"tmpfs|overlay"}
```

### 5.2 Grafana Deployment Update

Mount the alerting provisioning directory. Update `deploy/k3s/grafana.yaml`:

1. Add the alerting config files to the `grafana-config` ConfigMap (or create a separate `grafana-alerting` ConfigMap).
2. Mount to `/etc/grafana/provisioning/alerting/`.
3. Enable Unified Alerting (enabled by default in Grafana 11.x but set explicitly for clarity):

```yaml
env:
  - name: GF_UNIFIED_ALERTING_ENABLED
    value: "true"
  - name: GF_ALERTING_ENABLED
    value: "false"  # Disable legacy alerting
```

### 5.3 New Scrape Targets (Prerequisites)

These exporters need to be deployed on the respective nodes before their alerts become functional:

| Exporter | Node(s) | Port | Scrape Job Name |
|----------|---------|------|-----------------|
| `mysqld_exporter` | DB nodes (10.10.10.20) | 9104 | `node-db-mysql` |
| `nginx-prometheus-exporter` | Web nodes (10.10.10.10, .11) | 9113 | `node-web-nginx` |
| `php-fpm_exporter` | Web nodes (10.10.10.10, .11) | 9253 | `node-web-phpfpm` |
| `redis_exporter` | Valkey node (10.10.10.40) | 9121 | `node-valkey-redis` |
| `ceph_exporter` (or MGR module) | Storage node (10.10.10.50) | 9283 | `node-storage-ceph` |

Add to `deploy/k3s/prometheus.yaml`:

```yaml
scrape_configs:
  # ... existing jobs ...

  - job_name: node-db-mysql
    static_configs:
      - targets: ["10.10.10.20:9104"]

  - job_name: node-web-nginx
    static_configs:
      - targets: ["10.10.10.10:9113", "10.10.10.11:9113"]

  - job_name: node-web-phpfpm
    static_configs:
      - targets: ["10.10.10.10:9253", "10.10.10.11:9253"]

  - job_name: node-valkey-redis
    static_configs:
      - targets: ["10.10.10.40:9121"]

  - job_name: node-storage-ceph
    static_configs:
      - targets: ["10.10.10.50:9283"]
```

### 5.4 Application Code Changes

Add pgx pool metrics to core-api. In `cmd/core-api/main.go` or a dedicated metrics package:

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/jackc/pgx/v5/pgxpool"
)

func registerPoolMetrics(pool *pgxpool.Pool) {
    acquiredConns := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
        Name: "pgxpool_acquired_conns",
        Help: "Number of currently acquired connections in the pool",
    }, func() float64 {
        return float64(pool.Stat().AcquiredConns())
    })

    maxConns := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
        Name: "pgxpool_max_conns",
        Help: "Maximum number of connections in the pool",
    }, func() float64 {
        return float64(pool.Stat().MaxConns())
    })

    totalConns := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
        Name: "pgxpool_total_conns",
        Help: "Total number of connections in the pool",
    }, func() float64 {
        return float64(pool.Stat().TotalConns())
    })

    idleConns := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
        Name: "pgxpool_idle_conns",
        Help: "Number of idle connections in the pool",
    }, func() float64 {
        return float64(pool.Stat().IdleConns())
    })

    prometheus.MustRegister(acquiredConns, maxConns, totalConns, idleConns)
}
```

---

## 6. Runbook Structure

Each runbook follows a standard template. Create them in `docs/runbooks/`.

### Template

```markdown
# <AlertName>

## What It Means

<1-2 sentences describing the alert condition and its impact.>

## Severity

<critical|warning|info> -- <what the severity means for this alert>

## Likely Causes

1. <Cause 1>
2. <Cause 2>
3. <Cause 3>

## Investigation Steps

1. <Step 1 with specific commands>
2. <Step 2>
3. <Step 3>

## Remediation

### Immediate

<Steps to restore service immediately>

### Long-term

<Steps to prevent recurrence>

## Escalation

<When and to whom to escalate>
```

### Example: NodeDown.md

```markdown
# NodeDown

## What It Means

A node VM's node_exporter has been unreachable for over 3 minutes. The entire
VM may be down, or only the exporter process crashed.

## Severity

critical -- The node cannot serve workloads. Tenants assigned to shards on this
node may experience service disruption.

## Likely Causes

1. VM crashed or was shut down by the hypervisor
2. Network connectivity lost between control plane and the node
3. node_exporter process crashed or was stopped
4. Firewall rule blocking port 9100

## Investigation Steps

1. Check if the VM is reachable:
   ssh root@<node-ip> uptime

2. Check node_exporter status:
   ssh root@<node-ip> systemctl status node_exporter

3. Check VM status in libvirt:
   virsh list --all | grep <node-hostname>

4. Check network from controlplane:
   ping -c 3 <node-ip>

5. Check Temporal task queue for the node (if node-agent is also down):
   tctl taskqueue describe -tq node-<node-id>

## Remediation

### Immediate

- If VM is down: `virsh start <node-hostname>`
- If exporter crashed: `ssh root@<node-ip> systemctl restart node_exporter`
- If network issue: check hypervisor bridge and routing

### Long-term

- Ensure node_exporter is configured with `Restart=always` in its systemd unit
- Add the node to a watchdog/out-of-band monitoring system
- Consider HA for critical node roles

## Escalation

If the VM cannot be recovered within 15 minutes, escalate to the infrastructure
team to investigate the hypervisor host.
```

---

## 7. Silencing and Maintenance Windows

### 7.1 Grafana Silence API

Silences suppress alert notifications during planned maintenance. Use the Grafana API:

**Create a silence (2-hour maintenance window):**

```bash
curl -s -X POST http://grafana.massive-hosting.com/api/alertmanager/grafana/api/v2/silences \
  -H "Content-Type: application/json" \
  -u admin:admin \
  -d '{
    "comment": "Scheduled maintenance: DB node OS upgrade",
    "createdBy": "edvin",
    "startsAt": "2026-02-15T22:00:00Z",
    "endsAt": "2026-02-16T00:00:00Z",
    "matchers": [
      {
        "name": "instance",
        "value": "10.10.10.20:9100",
        "isRegex": false,
        "isEqual": true
      }
    ]
  }'
```

**Silence all alerts for a severity:**

```bash
curl -s -X POST http://grafana.massive-hosting.com/api/alertmanager/grafana/api/v2/silences \
  -H "Content-Type: application/json" \
  -u admin:admin \
  -d '{
    "comment": "Full platform maintenance window",
    "createdBy": "edvin",
    "startsAt": "2026-02-15T22:00:00Z",
    "endsAt": "2026-02-16T02:00:00Z",
    "matchers": [
      {
        "name": "severity",
        "value": "warning|info",
        "isRegex": true,
        "isEqual": true
      }
    ]
  }'
```

**List active silences:**

```bash
curl -s http://grafana.massive-hosting.com/api/alertmanager/grafana/api/v2/silences \
  -u admin:admin | jq '.[] | {id: .id, comment: .comment, endsAt: .endsAt, status: .status.state}'
```

**Delete (expire) a silence:**

```bash
curl -s -X DELETE http://grafana.massive-hosting.com/api/alertmanager/grafana/api/v2/silence/<silence-id> \
  -u admin:admin
```

### 7.2 Maintenance Workflow

1. **Before maintenance**: Create a silence covering the affected instances and expected duration (add 30-minute buffer).
2. **During maintenance**: Alerts fire but notifications are suppressed. Grafana UI shows silenced alerts distinctly.
3. **After maintenance**: Verify all targets are `up` in Prometheus (`up{instance="..."} == 1`). Delete the silence if it has not expired.
4. **Unplanned extension**: Update the silence `endsAt` via API or Grafana UI.

### 7.3 UI Access

Grafana provides a built-in Silence management UI at:
`http://grafana.massive-hosting.com/alerting/silences`

This is the preferred method for ad-hoc silences. The API is for automation and scripted maintenance windows.

---

## 8. Implementation Phases

### Phase 1: Core Alerting (Do Now)

These alerts require no new exporters -- they use metrics already being scraped.

| Alert | Data Source |
|-------|------------|
| NodeDown | `up{job=~"node-.*"}` |
| CoreApiDown | `up{job="core-api"}` |
| WorkerDown | `up{job="worker"}` |
| TemporalDown | `up{job="temporal"}` |
| HighDiskUsage / Critical | `node_filesystem_*` |
| HighMemoryUsage / Critical | `node_memory_*` |
| HighCpuUsage / Critical | `node_cpu_seconds_total` |
| DiskWillFillIn4Hours | `node_filesystem_avail_bytes` |
| CoreApi5xxRate / Critical | `http_requests_total` |
| CoreApiHighLatency / Critical | `http_request_duration_seconds_bucket` |
| HAProxyBackendDown / Degraded | `haproxy_backend_active_servers` |
| HighHAProxyErrorRate | `haproxy_frontend_http_responses_total` |

**Tasks:**
1. Create `docker/grafana/provisioning/alerting/contact-points.yaml`
2. Create `docker/grafana/provisioning/alerting/notification-policies.yaml`
3. Create `docker/grafana/provisioning/alerting/alert-rules.yaml` with all Phase 1 rules
4. Update `deploy/k3s/grafana.yaml` to mount alerting provisioning and enable Unified Alerting
5. Add Prometheus recording rules for dashboard query performance
6. Deploy a simple webhook receiver (can be as minimal as a Go program that logs to stdout)
7. Write runbooks for all Phase 1 alerts
8. Run `just vm-deploy` and verify alerts appear in Grafana UI

### Phase 2: Application Metrics (Next)

Requires code changes to core-api and/or Temporal SDK configuration.

| Alert | Prerequisite |
|-------|-------------|
| DatabaseConnectionPoolExhaustion | Expose pgx pool metrics from core-api |
| WorkflowFailureRate | Configure Temporal SDK Prometheus metrics handler on worker |
| TemporalTaskQueueBacklog | Temporal SDK metrics or server metrics |
| TemporalWorkflowTimeout | Temporal SDK metrics |
| TemporalActivityFailureRate | Temporal SDK metrics |

**Tasks:**
1. Add `registerPoolMetrics()` to core-api startup
2. Configure Temporal Go SDK with a Prometheus metrics handler in the worker
3. Add corresponding alert rules to Grafana provisioning
4. Write runbooks

### Phase 3: Service Exporters (Later)

Requires deploying additional exporters on node VMs via Terraform/cloud-init.

| Alert | Exporter Required |
|-------|------------------|
| NginxDown | nginx-prometheus-exporter on web nodes |
| PHPFPMPoolExhausted | php-fpm_exporter on web nodes |
| PHPFPMListenQueueOverflow | php-fpm_exporter on web nodes |
| DatabaseReplicationLag / Critical / Stopped | mysqld_exporter on DB nodes |
| CephHealthDegraded / Error | ceph_exporter on storage node |
| ValkeyHighMemory | redis_exporter on Valkey node |
| StalwartDown | Stalwart metrics or blackbox probe |

**Tasks:**
1. Add exporter packages to Terraform cloud-init for each node role
2. Add scrape targets to Prometheus config
3. Add corresponding alert rules to Grafana provisioning
4. Write runbooks

---

## 9. Alert Summary Table

| Alert Name | Severity | For | Trigger Condition | Phase |
|-----------|----------|-----|-------------------|-------|
| NodeDown | critical | 3m | Node exporter unreachable | 1 |
| NodeExporterDown | warning | 5m | Node exporter unreachable | 1 |
| HighDiskUsage | warning | 5m | Disk > 80% | 1 |
| HighDiskUsageCritical | critical | 2m | Disk > 95% | 1 |
| HighMemoryUsage | warning | 5m | Memory > 90% | 1 |
| HighMemoryUsageCritical | critical | 2m | Memory > 97% | 1 |
| HighCpuUsage | warning | 10m | CPU > 80% sustained | 1 |
| HighCpuUsageCritical | critical | 5m | CPU > 95% sustained | 1 |
| DiskWillFillIn4Hours | warning | 15m | Linear prediction | 1 |
| CoreApiDown | critical | 1m | Core API unreachable | 1 |
| WorkerDown | critical | 1m | Worker unreachable | 1 |
| CoreApi5xxRate | warning | 5m | 5xx > 1% | 1 |
| CoreApi5xxRateCritical | critical | 2m | 5xx > 5% | 1 |
| CoreApiHighLatency | warning | 5m | P99 > 2s | 1 |
| CoreApiHighLatencyCritical | critical | 2m | P99 > 5s | 1 |
| WorkflowFailureRate | warning | 10m | Failure > 5% | 2 |
| TemporalDown | critical | 1m | Temporal unreachable | 1 |
| TemporalTaskQueueBacklog | warning | 10m | Backlog > 100 | 2 |
| TemporalWorkflowTimeout | warning | 5m | Timeouts occurring | 2 |
| TemporalActivityFailureRate | warning | 10m | Failure > 10% | 2 |
| DatabaseReplicationLag | warning | 5m | Lag > 30s | 3 |
| DatabaseReplicationLagCritical | critical | 2m | Lag > 300s | 3 |
| DatabaseReplicationStopped | critical | 1m | Thread stopped | 3 |
| DatabaseConnectionPoolExhaustion | warning | 5m | Pool > 80% | 2 |
| DatabaseConnectionPoolExhausted | critical | 1m | Pool = 100% | 2 |
| HAProxyBackendDown | critical | 1m | Zero healthy servers | 1 |
| HAProxyBackendDegraded | warning | 5m | Some servers lost | 1 |
| HighHAProxyErrorRate | warning | 5m | 5xx > 5% | 1 |
| HAProxyHighConnectionRate | warning | 5m | Sessions > 80% limit | 1 |
| NginxDown | critical | 2m | Nginx unreachable | 3 |
| PHPFPMPoolExhausted | warning | 5m | Workers = max_children | 3 |
| PHPFPMListenQueueOverflow | warning | 3m | Queue > 0 | 3 |
| CephHealthDegraded | warning | 5m | Status != HEALTH_OK | 3 |
| CephHealthError | critical | 1m | Status = HEALTH_ERR | 3 |
| ValkeyHighMemory | warning | 5m | Memory > 80% maxmemory | 3 |
| StalwartDown | critical | 2m | Stalwart unreachable | 3 |
