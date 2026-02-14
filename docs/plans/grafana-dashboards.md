# Grafana Dashboards Implementation Plan

## Current State

### Infrastructure
- **Grafana** 11.5.2 deployed in k3s with hostNetwork, anonymous viewer access
- **Prometheus** v2.53.3 scraping: `core-api` (:8090), `worker` (:9200), `haproxy` (:8404), `temporal` (:7233), `node-exporter` on all 7 node types (:9100)
- **Loki** 3.4.2 with TSDB schema v13, `allow_structured_metadata: true`, `volume_enabled: true`
- **Alloy** v1.5.1 DaemonSet tailing k3s pod logs, forwarding to Loki with labels: `namespace`, `pod`, `app`, `container`
- Dashboards provisioned via `grafana-dashboards` ConfigMap from JSON files in `docker/grafana/provisioning/dashboards/`

### Available Metrics
| Job | Endpoint | Metric Types |
|-----|----------|-------------|
| `core-api` | 10.10.10.2:8090 | `http_requests_total{method,path,status}`, `http_request_duration_seconds_bucket{method,path}` |
| `worker` | 10.10.10.2:9200 | Go runtime metrics, Temporal SDK metrics |
| `haproxy` | 10.10.10.70:8404 | `haproxy_frontend_*`, `haproxy_backend_*`, `haproxy_server_*` |
| `temporal` | 10.10.10.2:7233 | Temporal server metrics (workflow, activity, task queue) |
| `node-*` | :9100 | node_exporter: `node_cpu_*`, `node_memory_*`, `node_disk_*`, `node_network_*`, `node_filesystem_*` |

### Log Format (zerolog JSON)
Every log line includes base fields set in `internal/logging/logger.go`:
- `time` (timestamp), `level` (trace/debug/info/warn/error/fatal)
- `service` (core-api, worker, node-agent)
- `region`, `cluster`, `shard`, `node_id`, `node_role` (set when configured)

Additional fields per log context:
- **HTTP requests**: `request_id`, `method`, `path`, `status`, `duration`
- **Activities**: `component` (node-local-activity), `tenant`, `webroot`, `database`, `fqdn`, `bucket`, `access_key`, `runtime`, `zone`
- **Agent managers**: `component` (tenant-manager, webroot-manager, nginx-manager, database-manager, valkey-manager, s3-manager, ssh-manager, dns-manager)

### Current Dashboards

1. **API Overview** (`api-overview`): 5 panels, no template variables. Request rate by method, error rate %, latency P50/P95/P99, top 20 paths table, status code pie chart.
2. **Infrastructure** (`infrastructure`): 4 panels, no template variables. HAProxy frontend connections, backend response time, backend status, HTTP responses by code. Only covers HAProxy -- no node-exporter panels.
3. **Log Explorer** (`log-explorer`): 3 panels, 6 template variables (service, region, cluster, shard, node_role, hostname). Log volume by service bar chart, error logs panel (regex match), all logs panel. No resource-level filtering (tenant, FQDN, etc.).

### Gaps Identified
- Log Explorer has no way to search by tenant, webroot, database, FQDN, zone, or any resource ID
- Log Explorer error panel uses naive regex (`|~ "(?i)error|fail|fatal|panic"`) instead of structured JSON level field
- Infrastructure dashboard only covers HAProxy, ignoring node_exporter metrics entirely
- API Overview has no per-endpoint breakdown, no handler-level error rates
- No workflow visibility at all (Temporal metrics are scraped but unused)
- No per-resource dashboards (tenant, database, DNS)
- Alloy pipeline does not extract structured JSON fields -- logs arrive as raw text in Loki with only pod-level labels

---

## Prerequisite: Alloy Pipeline Enhancement

Before the dashboards can leverage structured log fields, Alloy must extract key fields from the JSON log lines and attach them as Loki labels or structured metadata.

### Updated Alloy Config

Add a `loki.process` stage between the source and the write endpoint to parse JSON and extract labels:

```alloy
// Parse JSON logs and extract structured fields
loki.process "json_extract" {
  forward_to = [loki.write.default.receiver]

  // Parse the JSON body
  stage.json {
    expressions = {
      level     = "level",
      service   = "service",
      component = "component",
      region    = "region",
      cluster   = "cluster",
      shard     = "shard",
      node_id   = "node_id",
      node_role = "node_role",
    }
  }

  // Promote key fields to Loki stream labels
  stage.labels {
    values = {
      level     = "",
      service   = "",
      node_role = "",
    }
  }

  // Store resource identifiers as structured metadata (not stream labels,
  // to avoid high cardinality blowup)
  stage.structured_metadata {
    values = {
      component  = "",
      region     = "",
      cluster    = "",
      shard      = "",
      node_id    = "",
      tenant     = "",
      webroot    = "",
      database   = "",
      fqdn       = "",
      bucket     = "",
      zone       = "",
      request_id = "",
      method     = "",
      path       = "",
      status     = "",
    }
  }
}

// Update source to forward through the process pipeline
loki.source.kubernetes "pods" {
  targets    = discovery.relabel.pods.output
  forward_to = [loki.process.json_extract.receiver]
}
```

**Important**: The `stage.json` extracts fields from the JSON log body. Fields promoted via `stage.labels` become indexed Loki labels (keep only low-cardinality ones: `level`, `service`, `node_role`). High-cardinality fields (`tenant`, `request_id`, `path`, etc.) go to `structured_metadata` which is queryable via `| tenant = "..."` without creating excessive label streams.

After deploying this change, logs in Loki will have:
- **Stream labels**: `{app="...", namespace="...", pod="...", level="info", service="core-api", node_role="web"}`
- **Structured metadata**: `tenant`, `webroot`, `database`, `fqdn`, `bucket`, `zone`, `request_id`, `method`, `path`, `status`, `component`, `region`, `cluster`, `shard`, `node_id`

---

## Dashboard 1: Log Explorer (Enhanced)

**File**: `docker/grafana/provisioning/dashboards/log-explorer.json`
**UID**: `log-explorer`

### Template Variables

| Variable | Label | Type | Query | Multi | Include All | allValue |
|----------|-------|------|-------|-------|-------------|----------|
| `service` | Service | query | `label_values(service)` | yes | yes | `.+` |
| `level` | Level | custom | `trace,debug,info,warn,error,fatal` | yes | yes | `.+` |
| `node_role` | Node Role | query | `label_values(node_role)` | yes | yes | `.+` |
| `region` | Region | query | `label_values({service=~"$service"}, region)` | yes | yes | `.+` |
| `cluster` | Cluster | query | `label_values({service=~"$service"}, cluster)` | yes | yes | `.+` |
| `shard` | Shard | query | `label_values({service=~"$service"}, shard)` | yes | yes | `.+` |
| `hostname` | Hostname | query | `label_values({service=~"$service"}, hostname)` | yes | yes | `.+` |
| `tenant` | Tenant ID | textbox | (empty default) | no | no | n/a |
| `fqdn` | FQDN | textbox | (empty default) | no | no | n/a |
| `webroot` | Webroot | textbox | (empty default) | no | no | n/a |
| `database` | Database | textbox | (empty default) | no | no | n/a |
| `zone` | Zone | textbox | (empty default) | no | no | n/a |
| `resource_id` | Resource ID (UUID) | textbox | (empty default) | no | no | n/a |
| `request_id` | Request ID | textbox | (empty default) | no | no | n/a |

**Note on textbox variables**: Grafana textbox variables allow free-form input. When empty, the filter is skipped. The LogQL queries use conditional matching so empty values match everything.

### Base LogQL Filter Expression

All panels share this base selector. The stream labels filter on indexed labels; the pipeline filters on structured metadata:

```logql
{service=~"$service", level=~"$level", node_role=~"$node_role"}
  | json
  | region=~"${region:regex}"
  | cluster=~"${cluster:regex}"
  | shard=~"${shard:regex}"
  | tenant=~"${tenant:pipe}"
  | fqdn=~"${fqdn:pipe}"
  | webroot=~"${webroot:pipe}"
  | database=~"${database:pipe}"
  | zone=~"${zone:pipe}"
  | request_id=~"${request_id:pipe}"
  | line_format "{{.message}}" =~ "${resource_id:pipe}"
```

For textbox variables, when the variable is empty, the `${var:pipe}` syntax produces `.*` which matches everything. When filled, it produces the exact value.

**Simplified approach**: Since structured metadata is queryable directly in the selector (Loki 3.x), the more performant query is:

```logql
{service=~"$service", level=~"$level", node_role=~"$node_role"}
  | tenant =~ `${tenant:pipe}`
  | fqdn =~ `${fqdn:pipe}`
  | webroot =~ `${webroot:pipe}`
  | database =~ `${database:pipe}`
  | zone =~ `${zone:pipe}`
  | request_id =~ `${request_id:pipe}`
```

When structured metadata is available directly on the log entry (thanks to the Alloy pipeline), `| json` parsing is not needed for filtering -- structured metadata fields are queryable in the pipeline without parsing. For the UUID free-text search, fall back to `|~ "$resource_id"`.

### Panel Layout

#### Row 1: Overview Stats (y=0, h=4)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Total Log Volume** | stat | w=6, x=0 | `sum(count_over_time({service=~"$service", level=~"$level"} [$__range]))` |
| **Error Count** | stat (red) | w=6, x=6 | `sum(count_over_time({service=~"$service", level=~"error\|fatal"} [$__range]))` |
| **Services Logging** | stat | w=6, x=12 | `count(count by (service) (count_over_time({service=~".+"} [5m])))` |
| **Warn Count** | stat (orange) | w=6, x=18 | `sum(count_over_time({service=~"$service", level="warn"} [$__range]))` |

#### Row 2: Time Series (y=4, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Log Volume by Service** | timeseries (stacked bars) | w=12, x=0 | `sum by (service) (count_over_time({service=~"$service", level=~"$level"} [$__auto]))` |
| **Log Level Distribution** | timeseries (stacked bars) | w=12, x=12 | `sum by (level) (count_over_time({service=~"$service", level=~"$level"} [$__auto]))` |

#### Row 3: Error Analysis (y=12, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Error Rate Over Time** | timeseries | w=12, x=0 | `sum(count_over_time({service=~"$service", level=~"error\|fatal"} [$__auto]))` |
| **Errors by Component** | piechart | w=12, x=12 | `sum by (component) (count_over_time({service=~"$service", level=~"error\|fatal"} [$__auto]))` |

#### Row 4: Filtered Logs (y=20, h=16)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Filtered Logs** | logs | w=24, x=0 | Full base LogQL with all template variable filters applied (see below) |

Full query for the filtered logs panel:

```logql
{service=~"$service", level=~"$level", node_role=~"$node_role"}
  | tenant =~ `${tenant:pipe}`
  | fqdn =~ `${fqdn:pipe}`
  | webroot =~ `${webroot:pipe}`
  | database =~ `${database:pipe}`
  | zone =~ `${zone:pipe}`
  | request_id =~ `${request_id:pipe}`
  ${resource_id:regex}
```

When `$resource_id` is non-empty, append `|~ "$resource_id"` to the pipeline. In Grafana, this is handled by conditionally including the line filter only when the variable has a value. Practically, use:

```logql
{service=~"$service", level=~"$level", node_role=~"$node_role"}
  | tenant =~ `${tenant:pipe}`
  | fqdn =~ `${fqdn:pipe}`
  | webroot =~ `${webroot:pipe}`
  | database =~ `${database:pipe}`
  | zone =~ `${zone:pipe}`
  | request_id =~ `${request_id:pipe}`
  |~ `$resource_id`
```

(When `$resource_id` is empty, `|~ ""` matches all lines.)

**Logs panel options**:
- `showTime: true`
- `showLabels: true`
- `wrapLogMessage: true`
- `sortOrder: "Descending"`
- `enableLogDetails: true` (allows expanding individual log lines to see all structured metadata fields)
- `prettifyLogMessage: false`

---

## Dashboard 2: API Overview (Enhanced)

**File**: `docker/grafana/provisioning/dashboards/api-overview.json`
**UID**: `api-overview`

### Template Variables

| Variable | Label | Type | Query | Multi | Include All |
|----------|-------|------|-------|-------|-------------|
| `method` | Method | query | `label_values(http_requests_total, method)` | yes | yes |
| `path` | Endpoint | query | `label_values(http_requests_total, path)` | yes | yes |

### Panel Layout

#### Row 1: Key Metrics (y=0, h=4)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Total Requests/s** | stat | w=6, x=0 | `sum(rate(http_requests_total{method=~"$method", path=~"$path"}[5m]))` |
| **Error Rate %** | stat (thresholds: green < 1%, orange < 5%, red >= 5%) | w=6, x=6 | `sum(rate(http_requests_total{method=~"$method", path=~"$path", status=~"5.."}[5m])) / sum(rate(http_requests_total{method=~"$method", path=~"$path"}[5m])) * 100` |
| **P95 Latency** | stat | w=6, x=12 | `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{method=~"$method", path=~"$path"}[5m])) by (le))` |
| **P99 Latency** | stat | w=6, x=18 | `histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{method=~"$method", path=~"$path"}[5m])) by (le))` |

#### Row 2: Request Rates (y=4, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Request Rate by Method** | timeseries | w=12, x=0 | `sum(rate(http_requests_total{method=~"$method", path=~"$path"}[5m])) by (method)` |
| **Request Rate by Endpoint** | timeseries | w=12, x=12 | `topk(10, sum(rate(http_requests_total{method=~"$method", path=~"$path"}[5m])) by (path))` |

#### Row 3: Error Breakdown (y=12, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Error Rate by Endpoint** | timeseries | w=12, x=0 | `topk(10, sum(rate(http_requests_total{method=~"$method", path=~"$path", status=~"5.."}[5m])) by (path))` |
| **Status Code Distribution** | piechart | w=12, x=12 | `sum(increase(http_requests_total{method=~"$method", path=~"$path"}[1h])) by (status)` with color overrides: 200=green, 201=green, 204=green, 400=yellow, 404=yellow, 500=red |

#### Row 4: Latency (y=20, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Latency P50/P95/P99** | timeseries | w=12, x=0 | Three targets: `histogram_quantile(0.50/0.95/0.99, sum(rate(http_request_duration_seconds_bucket{method=~"$method", path=~"$path"}[5m])) by (le))` |
| **Latency by Endpoint (P95)** | timeseries | w=12, x=12 | `topk(10, histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{method=~"$method", path=~"$path"}[5m])) by (le, path)))` |

#### Row 5: Endpoint Detail Table (y=28, h=10)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Endpoint Detail Table** | table | w=24, x=0 | Three queries merged via transformations: (A) `sum(rate(http_requests_total{method=~"$method", path=~"$path"}[5m])) by (method, path)` [instant], (B) `sum(rate(http_requests_total{method=~"$method", path=~"$path", status=~"5.."}[5m])) by (method, path)` [instant], (C) `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{method=~"$method", path=~"$path"}[5m])) by (le, method, path))` [instant] |

Table columns: Method, Path, Requests/s, Errors/s, P95 Latency. Sort by Requests/s descending.

#### Row 6: Workflow Status (y=38, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Active Workflows** | stat | w=8, x=0 | `sum(temporal_workflow_active_count)` (from Temporal server metrics) |
| **Failed Workflows (1h)** | stat (red) | w=8, x=8 | `sum(increase(temporal_workflow_failed_total[1h]))` |
| **Workflow Task Queue Latency** | timeseries | w=8, x=16 | `histogram_quantile(0.95, sum(rate(temporal_workflow_task_schedule_to_start_latency_bucket[5m])) by (le))` |

---

## Dashboard 3: Infrastructure (Enhanced)

**File**: `docker/grafana/provisioning/dashboards/infrastructure.json`
**UID**: `infrastructure`

### Template Variables

| Variable | Label | Type | Query | Multi | Include All |
|----------|-------|------|-------|-------|-------------|
| `node` | Node | query | `label_values(node_uname_info, nodename)` | no | yes |
| `job` | Job | query | `label_values(up, job)` | yes | yes |

### Panel Layout

#### Row 1: Cluster Health (y=0, h=4)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Nodes Up** | stat (green) | w=6, x=0 | `count(up{job=~"node-.*"} == 1)` |
| **Nodes Down** | stat (red) | w=6, x=6 | `count(up{job=~"node-.*"} == 0) OR vector(0)` |
| **HAProxy Backends UP** | stat (green) | w=6, x=12 | `count(haproxy_backend_status == 1)` |
| **HAProxy Backends DOWN** | stat (red) | w=6, x=18 | `count(haproxy_backend_status == 0) OR vector(0)` |

#### Row 2: CPU & Memory Overview (y=4, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **CPU Usage by Node** | timeseries | w=12, x=0 | `100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle", job=~"$job"}[5m])) * 100)` legendFormat: `{{instance}}` |
| **Memory Usage by Node** | timeseries | w=12, x=12 | `(1 - node_memory_MemAvailable_bytes{job=~"$job"} / node_memory_MemTotal_bytes{job=~"$job"}) * 100` legendFormat: `{{instance}}` |

#### Row 3: Disk (y=12, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Disk Usage %** | gauge (thresholds: green<70, orange<85, red>=85) | w=12, x=0 | `100 - (node_filesystem_avail_bytes{mountpoint="/", job=~"$job"} / node_filesystem_size_bytes{mountpoint="/", job=~"$job"} * 100)` |
| **Disk I/O Rate** | timeseries | w=12, x=12 | Two targets: Read: `rate(node_disk_read_bytes_total{job=~"$job"}[5m])`, Write: `rate(node_disk_written_bytes_total{job=~"$job"}[5m])` |

#### Row 4: Network (y=20, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Network RX/TX by Node** | timeseries | w=12, x=0 | Two targets: RX: `rate(node_network_receive_bytes_total{device!="lo", job=~"$job"}[5m])`, TX: `rate(node_network_transmit_bytes_total{device!="lo", job=~"$job"}[5m])` |
| **Network Errors** | timeseries | w=12, x=12 | `rate(node_network_receive_errs_total{device!="lo", job=~"$job"}[5m]) + rate(node_network_transmit_errs_total{device!="lo", job=~"$job"}[5m])` |

#### Row 5: HAProxy (y=28, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **HAProxy Frontend Connections** | timeseries | w=12, x=0 | `haproxy_frontend_current_sessions` |
| **HAProxy Backend Response Time** | timeseries | w=12, x=12 | `haproxy_backend_response_time_average_seconds` |

#### Row 6: HAProxy Detail (y=36, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **HAProxy Backend Status** | stat | w=12, x=0 | `haproxy_backend_status` with value mappings: 0=DOWN (red), 1=UP (green) |
| **HAProxy HTTP Responses by Code** | timeseries (stacked) | w=12, x=12 | `sum(rate(haproxy_frontend_http_responses_total[5m])) by (code)` |

#### Row 7: Per-Node Detail (y=44, h=10) -- Collapsed by default, expand via `$node` selector

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Node CPU Detail** | timeseries | w=8, x=0 | `rate(node_cpu_seconds_total{instance=~"$node.*"}[5m])` by mode |
| **Node Memory Detail** | timeseries | w=8, x=8 | `node_memory_MemTotal_bytes{instance=~"$node.*"}`, `node_memory_MemAvailable_bytes{instance=~"$node.*"}`, `node_memory_Cached_bytes{instance=~"$node.*"}`, `node_memory_Buffers_bytes{instance=~"$node.*"}` |
| **Node Load Average** | timeseries | w=8, x=16 | `node_load1{instance=~"$node.*"}`, `node_load5{instance=~"$node.*"}`, `node_load15{instance=~"$node.*"}` |

---

## Dashboard 4: Tenant Dashboard (New)

**File**: `docker/grafana/provisioning/dashboards/tenant.json`
**UID**: `tenant`

### Purpose
Operational view of tenant-related activity across the platform. Uses log-derived metrics since tenant resource counts are in the core database, not Prometheus. For resource counts, we query Loki for activity log patterns.

### Template Variables

| Variable | Label | Type | Query | Multi | Include All |
|----------|-------|------|-------|-------|-------------|
| `tenant` | Tenant ID | textbox | (empty) | no | no |

### Panel Layout

#### Row 1: Tenant Activity (y=0, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Tenant Operations Over Time** | timeseries (stacked bars) | w=12, x=0 | `sum by (msg) (count_over_time({service=~".+"} \| tenant =~ "$tenant" \| json \| msg =~ "CreateTenant\|UpdateTenant\|SuspendTenant\|DeleteTenant" [$__auto]))` |
| **Webroot Operations Over Time** | timeseries (stacked bars) | w=12, x=12 | `sum by (msg) (count_over_time({service=~".+"} \| tenant =~ "$tenant" \| json \| msg =~ "CreateWebroot\|UpdateWebroot\|DeleteWebroot" [$__auto]))` |

#### Row 2: Resource Provisioning (y=8, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Database Operations** | timeseries (stacked bars) | w=12, x=0 | `sum by (msg) (count_over_time({service=~".+"} \| tenant =~ "$tenant" \| json \| msg =~ "CreateDatabase\|DeleteDatabase" [$__auto]))` |
| **S3 Operations** | timeseries (stacked bars) | w=12, x=12 | `sum by (msg) (count_over_time({service=~".+"} \| tenant =~ "$tenant" \| json \| msg =~ "CreateS3Bucket\|DeleteS3Bucket\|CreateS3AccessKey\|DeleteS3AccessKey" [$__auto]))` |

#### Row 3: Provisioning Errors (y=16, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Error Logs for Tenant** | logs | w=24, x=0 | `{service=~".+", level=~"error\|fatal"} \| tenant =~ "$tenant"` |

#### Row 4: Full Tenant Activity Log (y=24, h=14)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **All Tenant Logs** | logs | w=24, x=0 | `{service=~".+"} \| tenant =~ "$tenant"` with `showTime: true`, `enableLogDetails: true` |

### Future Enhancement
When the core-api exposes `/metrics` with resource count gauges (e.g., `hosting_tenants_total`, `hosting_webroots_total{tenant="..."}`), replace the log-derived panels with direct Prometheus queries for real-time resource counts.

---

## Dashboard 5: Workflow Dashboard (New)

**File**: `docker/grafana/provisioning/dashboards/workflow.json`
**UID**: `workflow`

### Purpose
Visibility into Temporal workflow execution. Uses Temporal server metrics scraped from `:7233` and worker SDK metrics from `:9200`.

### Template Variables

| Variable | Label | Type | Query | Multi | Include All |
|----------|-------|------|-------|-------|-------------|
| `workflow_type` | Workflow Type | query | `label_values(temporal_workflow_completed_total, workflow_type)` | yes | yes |
| `task_queue` | Task Queue | query | `label_values(temporal_activity_schedule_to_start_latency_bucket, task_queue)` | yes | yes |

### Temporal Metric Names

Temporal server exposes metrics at `:7233/metrics`. Key metrics (names may vary by Temporal version; the queries below use the common naming convention):

- `temporal_workflow_completed_total{workflow_type}` -- completed workflow count
- `temporal_workflow_failed_total{workflow_type}` -- failed workflow count
- `temporal_workflow_canceled_total{workflow_type}` -- canceled workflow count
- `temporal_workflow_active_count{workflow_type}` -- currently running workflows
- `temporal_workflow_endtoend_latency_bucket{workflow_type}` -- end-to-end duration histogram
- `temporal_activity_schedule_to_start_latency_bucket{task_queue}` -- queue wait time
- `temporal_activity_execution_failed_total{activity_type}` -- failed activities
- `temporal_workflow_task_schedule_to_start_latency_bucket` -- workflow task latency

**Note**: The exact metric names depend on the Temporal server version and SDK configuration. After deployment, run `curl -s http://10.10.10.2:7233/metrics | head -100` and `curl -s http://10.10.10.2:9200/metrics | head -100` to verify actual metric names and adjust queries accordingly.

### Panel Layout

#### Row 1: Summary Stats (y=0, h=4)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Active Workflows** | stat (blue) | w=6, x=0 | `sum(temporal_workflow_active_count{workflow_type=~"$workflow_type"})` |
| **Completed (1h)** | stat (green) | w=6, x=6 | `sum(increase(temporal_workflow_completed_total{workflow_type=~"$workflow_type"}[1h]))` |
| **Failed (1h)** | stat (red) | w=6, x=12 | `sum(increase(temporal_workflow_failed_total{workflow_type=~"$workflow_type"}[1h]))` |
| **Failure Rate %** | stat (thresholds: green<1, orange<5, red>=5) | w=6, x=18 | `sum(increase(temporal_workflow_failed_total{workflow_type=~"$workflow_type"}[1h])) / (sum(increase(temporal_workflow_completed_total{workflow_type=~"$workflow_type"}[1h])) + sum(increase(temporal_workflow_failed_total{workflow_type=~"$workflow_type"}[1h]))) * 100` |

#### Row 2: Workflow Throughput (y=4, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Completions by Type** | timeseries (stacked) | w=12, x=0 | `sum by (workflow_type) (rate(temporal_workflow_completed_total{workflow_type=~"$workflow_type"}[5m]))` |
| **Failures by Type** | timeseries (stacked) | w=12, x=12 | `sum by (workflow_type) (rate(temporal_workflow_failed_total{workflow_type=~"$workflow_type"}[5m]))` |

#### Row 3: Latency & Queue Depth (y=12, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Workflow Duration P50/P95** | timeseries | w=12, x=0 | `histogram_quantile(0.50, sum(rate(temporal_workflow_endtoend_latency_bucket{workflow_type=~"$workflow_type"}[5m])) by (le))` and `histogram_quantile(0.95, ...)` |
| **Task Queue Schedule-to-Start Latency P95** | timeseries | w=12, x=12 | `histogram_quantile(0.95, sum(rate(temporal_activity_schedule_to_start_latency_bucket{task_queue=~"$task_queue"}[5m])) by (le, task_queue))` |

#### Row 4: Activity Failures (y=20, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Activity Failures by Type** | timeseries | w=12, x=0 | `sum by (activity_type) (rate(temporal_activity_execution_failed_total[5m]))` |
| **Activity Retries** | timeseries | w=12, x=12 | `sum by (activity_type) (rate(temporal_activity_execution_latency_count[5m])) - sum by (activity_type) (rate(temporal_activity_schedule_to_start_latency_count[5m]))` or use `temporal_activity_execution_failed_total` if retry metrics are not directly available |

#### Row 5: Workflow Logs (y=28, h=12)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Workflow Error Logs** | logs | w=24, x=0 | `{service="worker", level=~"error\|warn"}` |

---

## Dashboard 6: Database Dashboard (New)

**File**: `docker/grafana/provisioning/dashboards/database.json`
**UID**: `database`

### Purpose
MySQL database operations and node health. Currently limited to node_exporter metrics from db nodes plus log-derived activity counts. Future: add mysqld_exporter for query-level metrics.

### Template Variables

| Variable | Label | Type | Query | Multi | Include All |
|----------|-------|------|-------|-------|-------------|
| `database` | Database Name | textbox | (empty) | no | no |

### Panel Layout

#### Row 1: DB Node Health (y=0, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **DB Node CPU** | timeseries | w=12, x=0 | `100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle", job="node-db"}[5m])) * 100)` |
| **DB Node Memory** | timeseries | w=12, x=12 | `(1 - node_memory_MemAvailable_bytes{job="node-db"} / node_memory_MemTotal_bytes{job="node-db"}) * 100` |

#### Row 2: DB Node Disk (y=8, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **DB Disk I/O** | timeseries | w=12, x=0 | Read: `rate(node_disk_read_bytes_total{job="node-db"}[5m])`, Write: `rate(node_disk_written_bytes_total{job="node-db"}[5m])` |
| **DB Disk Usage %** | gauge | w=12, x=12 | `100 - (node_filesystem_avail_bytes{mountpoint="/", job="node-db"} / node_filesystem_size_bytes{mountpoint="/", job="node-db"} * 100)` |

#### Row 3: Database Operations (y=16, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Database Create/Delete Operations** | timeseries (stacked bars) | w=12, x=0 | `sum by (msg) (count_over_time({service=~".+"} \| database =~ "$database" \| json \| msg =~ "CreateDatabase\|DeleteDatabase\|DumpMySQLDatabase\|ImportMySQLDatabase" [$__auto]))` |
| **Database Backup/Restore** | timeseries (stacked bars) | w=12, x=12 | `sum by (msg) (count_over_time({service=~".+"} \| database =~ "$database" \| json \| msg =~ "CreateMySQLBackup\|RestoreMySQLBackup" [$__auto]))` |

#### Row 4: Database Logs (y=24, h=12)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Database Error Logs** | logs | w=24, x=0 | `{service=~".+", level=~"error\|warn"} \| database =~ "$database"` |

### Future Enhancement: mysqld_exporter
Add a `mysqld-exporter` scrape target in Prometheus for each DB node to get:
- `mysql_global_status_connections` -- active connections
- `mysql_global_status_queries` -- query rate
- `mysql_slave_status_seconds_behind_master` -- replication lag
- `mysql_global_status_slow_queries` -- slow query count
- `mysql_global_status_innodb_buffer_pool_reads` -- buffer pool efficiency

When mysqld_exporter is available, add these panels:

| Panel | Query |
|-------|-------|
| **MySQL Active Connections** | `mysql_global_status_threads_connected` |
| **MySQL Query Rate** | `rate(mysql_global_status_queries[5m])` |
| **Replication Lag** | `mysql_slave_status_seconds_behind_master` |
| **Slow Queries/min** | `rate(mysql_global_status_slow_queries[1m]) * 60` |
| **InnoDB Buffer Pool Hit Rate** | `1 - rate(mysql_global_status_innodb_buffer_pool_reads[5m]) / rate(mysql_global_status_innodb_buffer_pool_read_requests[5m])` |

---

## Dashboard 7: DNS Dashboard (New)

**File**: `docker/grafana/provisioning/dashboards/dns.json`
**UID**: `dns`

### Purpose
DNS node health and zone operations. PowerDNS query stats require PowerDNS built-in HTTP API or a dedicated exporter (future).

### Template Variables

| Variable | Label | Type | Query | Multi | Include All |
|----------|-------|------|-------|-------|-------------|
| `zone` | Zone Name | textbox | (empty) | no | no |

### Panel Layout

#### Row 1: DNS Node Health (y=0, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **DNS Node CPU** | timeseries | w=12, x=0 | `100 - (avg by (instance) (rate(node_cpu_seconds_total{mode="idle", job="node-dns"}[5m])) * 100)` |
| **DNS Node Memory** | timeseries | w=12, x=12 | `(1 - node_memory_MemAvailable_bytes{job="node-dns"} / node_memory_MemTotal_bytes{job="node-dns"}) * 100` |

#### Row 2: Zone Operations (y=8, h=8)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **Zone Operations Over Time** | timeseries (stacked bars) | w=24, x=0 | `sum by (msg) (count_over_time({service=~".+"} \| zone =~ "$zone" \| json \| msg =~ "zone\|record\|dns" [$__auto]))` |

#### Row 3: DNS Logs (y=16, h=12)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **DNS-Related Logs** | logs | w=24, x=0 | `{service=~".+", node_role="dns"} \| zone =~ "$zone"` (when `$zone` is empty, shows all DNS node logs) |

#### Row 4: DNS Error Logs (y=28, h=10)

| Panel | Type | Position | Query |
|-------|------|----------|-------|
| **DNS Errors** | logs | w=24, x=0 | `{service=~".+", node_role="dns", level=~"error\|warn"} \| zone =~ "$zone"` |

### Future Enhancement: PowerDNS Exporter
Add `powerdns_exporter` or enable the PowerDNS HTTP API and scrape from Prometheus:
- `powerdns_authoritative_queries_total` -- query rate
- `powerdns_authoritative_answers_total` -- answer rate by rcode
- `powerdns_authoritative_cache_hits_total` -- cache hit rate
- `powerdns_authoritative_zone_count` -- number of zones served

---

## Implementation Order

### Phase 1: Alloy Pipeline (prerequisite for everything else)
1. Update `deploy/k3s/alloy.yaml` with JSON extraction and structured metadata stages
2. Deploy with `just vm-deploy`
3. Verify in Grafana Explore that structured metadata fields appear on log entries:
   - Query: `{app="core-api"} | json` -- expand a log line and confirm `tenant`, `method`, `path` etc. appear as parsed fields
   - Query: `{service="node-agent"} | tenant != ""` -- confirm filtering works

### Phase 2: Log Explorer Enhancement
1. Add all template variables (dropdowns + textboxes)
2. Add stat panels (Row 1)
3. Add time series panels (Row 2, Row 3)
4. Replace existing log panels with filtered versions using structured metadata queries
5. Test by entering a known tenant ID in the textbox and verifying filtered results

### Phase 3: API Overview Enhancement
1. Add `method` and `path` template variables
2. Add stat row
3. Add per-endpoint breakdowns
4. Add endpoint detail table
5. Add workflow summary row

### Phase 4: Infrastructure Enhancement
1. Add `node` and `job` template variables
2. Add cluster health stat row
3. Add CPU, memory, disk, network panels using node_exporter
4. Keep existing HAProxy panels, reorganize
5. Add per-node detail row (collapsed)

### Phase 5: New Dashboards
1. Workflow dashboard (most immediately useful for operations)
2. Tenant dashboard
3. Database dashboard
4. DNS dashboard

### Phase 6: Future Exporters
1. Add mysqld_exporter to DB nodes (Terraform cloud-init)
2. Add PowerDNS exporter or HTTP API scraping to DNS nodes
3. Add nginx-exporter to web nodes (nginx stub_status + exporter)
4. Enhance dashboards with the newly available metrics

---

## Deployment Process

All dashboards are JSON files in `docker/grafana/provisioning/dashboards/`. They are loaded into k3s via the `grafana-dashboards` ConfigMap.

To deploy changes:
1. Edit the JSON files in `docker/grafana/provisioning/dashboards/`
2. Run `just vm-deploy` which rebuilds and re-applies all k3s manifests
3. Grafana automatically picks up changes from the provisioned dashboard path

The Alloy config change in `deploy/k3s/alloy.yaml` is deployed as part of the same process. The Alloy DaemonSet will restart and begin extracting structured metadata from that point forward. Historical logs (before the Alloy change) will not have structured metadata and will not be filterable by resource fields.

---

## Notes on LogQL Query Patterns

### Filtering by structured metadata (post-Alloy enhancement)
```logql
# Filter by tenant using structured metadata (fast, no JSON parsing needed)
{service=~".+"} | tenant = "abc-123"

# Combine multiple filters
{service="node-agent", level="error"} | tenant = "abc-123" | webroot = "main"

# Regex match on structured metadata
{service=~".+"} | fqdn =~ ".*example\\.com"
```

### Filtering by JSON field (fallback, before Alloy enhancement)
```logql
# Parse JSON and filter (slower, requires scanning log content)
{service=~".+"} | json | tenant = "abc-123"

# With line_format to extract readable message
{service=~".+"} | json | tenant = "abc-123" | line_format "{{.level}} {{.msg}} tenant={{.tenant}}"
```

### Free-text UUID search
```logql
# Search for any UUID anywhere in the log line
{service=~".+"} |~ "550e8400-e29b-41d4-a716-446655440000"
```

### Aggregate metrics from logs
```logql
# Count logs per service over time
sum by (service) (count_over_time({service=~".+"} [$__auto]))

# Error rate from logs
sum(count_over_time({level="error"} [$__auto])) / sum(count_over_time({level=~".+"} [$__auto])) * 100

# Count operations by message type
sum by (msg) (count_over_time({service=~".+"} | json [$__auto]))
```

### Template variable syntax in Grafana LogQL
- `$var` -- simple substitution
- `${var:regex}` -- for multi-value dropdown variables, produces `value1|value2|value3`
- `${var:pipe}` -- for single textbox variables, produces the value as-is or `.*` when empty
- When using textbox variables in structured metadata filters, use: `| tenant =~ "${tenant:pipe}"`
