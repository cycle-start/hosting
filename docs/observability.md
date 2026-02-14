# Monitoring & Logging

The hosting platform uses a standard Grafana observability stack running on the k3s control plane. All components run with `hostNetwork: true` on `10.10.10.2`.

## Stack Overview

| Component | Version | Port | Purpose |
|-----------|---------|------|---------|
| Prometheus | v2.53.3 | 9090 | Metrics collection and storage |
| Grafana | v11.5.2 | 3000 | Dashboards and exploration |
| Loki | v3.4.2 | 3100 | Log aggregation |
| Alloy | v1.5.1 | 12345 | Pod log collection and shipping |

## Prometheus

Prometheus scrapes metrics from all platform components every 15 seconds with 15-day retention. Data is stored on a 5Gi PVC.

### Scrape targets

| Job | Target(s) | What it scrapes |
|-----|-----------|-----------------|
| `core-api` | `10.10.10.2:8090` | Core API `/metrics` endpoint |
| `worker` | `10.10.10.2:9200` | Temporal worker metrics |
| `haproxy` | `10.10.10.70:8404` | HAProxy stats endpoint |
| `temporal` | `10.10.10.2:7233` | Temporal server metrics |
| `node-web` | `10.10.10.10:9100`, `10.10.10.11:9100` | Node Exporter on web nodes |
| `node-db` | `10.10.10.20:9100` | Node Exporter on database node |
| `node-dns` | `10.10.10.30:9100` | Node Exporter on DNS node |
| `node-valkey` | `10.10.10.40:9100` | Node Exporter on Valkey node |
| `node-storage` | `10.10.10.50:9100` | Node Exporter on S3/Ceph node |
| `node-dbadmin` | `10.10.10.60:9100` | Node Exporter on DB admin node |
| `node-lb` | `10.10.10.70:9100` | Node Exporter on load balancer node |

### Core API metrics

The `Metrics` middleware (`internal/api/middleware/metrics.go`) exposes two Prometheus metrics:

- **`http_requests_total`** -- Counter with labels `method`, `path`, `status`. Tracks total request count per route and status code.
- **`http_request_duration_seconds`** -- Histogram with labels `method`, `path`. Uses default Prometheus buckets.

The `path` label uses chi's route pattern (e.g. `/tenants/{id}`) rather than the raw URL path, preventing high-cardinality label explosion from path parameters.

## Grafana

Grafana runs at `http://grafana.hosting.test` (port 3000) with anonymous read access enabled (`GF_AUTH_ANONYMOUS_ENABLED=true`, viewer role). Admin credentials are `admin`/`admin`.

### Pre-configured datasources

| Name | Type | URL | Default |
|------|------|-----|---------|
| Prometheus | `prometheus` | `http://127.0.0.1:9090` | Yes |
| Loki | `loki` | `http://127.0.0.1:3100` | No |

Both datasources are provisioned via ConfigMap and marked as non-editable. Dashboards are loaded from a file provider at `/var/lib/grafana/dashboards` (sourced from the `grafana-dashboards` ConfigMap, when present).

Data is stored on a 1Gi PVC for persistence across restarts.

## Loki

Loki provides centralized log aggregation. It runs in single-node mode with filesystem storage on a 5Gi PVC.

### Configuration highlights

- **Auth disabled** (`auth_enabled: false`) -- single-tenant mode
- **Storage:** TSDB index + filesystem chunks
- **Schema:** v13 with 24h index period
- **Structured metadata:** enabled (`allow_structured_metadata: true`)
- **Replication factor:** 1 (single-node)
- **Ring:** in-memory KV store

## Alloy (Log Collection)

Grafana Alloy runs as a DaemonSet that collects logs from all k3s pods and ships them to Loki.

### How it works

1. **Discovery:** Alloy uses `discovery.kubernetes` to find all pods in the cluster.
2. **Relabeling:** Extracts useful labels from pod metadata:
   - `namespace` -- Kubernetes namespace
   - `pod` -- Pod name
   - `app` -- From `app.kubernetes.io/component` label, falling back to `app` label
   - `container` -- Container name
   - Drops pods in `Pending`, `Succeeded`, `Failed`, or `Unknown` phases
3. **Collection:** `loki.source.kubernetes` tails `/var/log/pods` on each node
4. **Shipping:** `loki.write` pushes logs to `http://127.0.0.1:3100/loki/api/v1/push`

### RBAC

Alloy has a dedicated ServiceAccount with ClusterRole permissions to `get`, `list`, and `watch` pods, pod logs, nodes, and namespaces.

### Resource limits

- Requests: 50m CPU, 64Mi memory
- Limits: 256Mi memory

## Log Proxy API

The core API includes a log proxy endpoint that queries Loki and returns normalized results. This powers the admin UI log viewer without exposing Loki directly.

### Endpoint

```
GET /logs?query={logql}&start={time}&end={time}&limit={n}
```

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `query` | Yes | -- | LogQL query (e.g. `{app="core-api"}`) |
| `start` | No | 1 hour ago | Start time (RFC3339 or relative: `1h`, `30m`, `7d`) |
| `end` | No | now | End time (RFC3339) |
| `limit` | No | 500 | Max entries (1-5000) |

### Response format

```json
{
  "entries": [
    {
      "timestamp": "2025-01-15T10:30:00.123456789Z",
      "line": "{\"level\":\"info\",\"msg\":\"request completed\"}",
      "labels": {
        "app": "core-api",
        "namespace": "default",
        "pod": "core-api-abc123"
      }
    }
  ]
}
```

Entries are sorted chronologically (oldest first). Timestamps are converted from Loki's nanosecond format to RFC3339Nano. The proxy flattens Loki's stream-based response into a flat list of entries with their stream labels attached.

### Relative time parsing

The `start` parameter supports relative durations:
- `15m` -- 15 minutes ago
- `1h` -- 1 hour ago
- `6h` -- 6 hours ago
- `7d` -- 7 days ago

## Admin UI LogViewer

The `LogViewer` React component (`web/admin/src/components/shared/log-viewer.tsx`) provides an embedded log viewer used across resource detail pages (tenants, webroots, FQDNs, databases, zones, Valkey instances, S3 buckets).

### Features

- **Time range selector** -- 15m, 1h, 6h, 24h, 7d presets
- **Service filter** -- Filter by `app` label (core-api, worker, node-agent)
- **Pause/resume** -- Stop auto-refresh for reading
- **Auto-scroll** -- Scrolls to newest entries when not paused
- **Expandable rows** -- Click to see full JSON log payload
- **Level coloring** -- Color-coded badges for debug, info, warn, error, fatal
- **Grafana link** -- Direct link to Grafana Explore with the same LogQL query

### Log line parsing

Each log line is parsed as JSON. The component extracts `level`, `msg`, and `error` fields for display. If parsing fails, the raw line is shown.

### Polling

The component polls the `/logs` API endpoint using the `useLogs` hook. Polling is paused when the user clicks the pause button, allowing them to read log output without it scrolling away.
