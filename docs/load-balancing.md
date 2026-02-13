# Load Balancing

## Request Flow

```
Client → DNS → HAProxy (port 80)
                 ↓
         Host header map lookup
         (fqdn-to-shard.map)
                 ↓
         Backend (e.g. shard-web-1)
                 ↓
         Consistent hash on Host header
                 ↓
         Node VM (nginx)
                 ↓
         Response with X-Served-By + X-Shard headers
```

DNS resolves to the cluster's LB IP addresses. HAProxy receives the request on port 80, looks up the `Host` header in a map file to find the backend name, and forwards to the matching backend. Within the backend, consistent hashing on the `Host` header selects a specific node.

## Two Management Mechanisms

### Backend Configuration (infrequent)

Full `haproxy.cfg` regeneration + reload. Triggered by node additions/removals or shard changes. Backend server lists are derived from active nodes in each web shard.

### Map Entries (frequent)

HAProxy Runtime API via TCP socket (port 9999) — instant, no reload. Triggered by:
- FQDN bind (`BindFQDNWorkflow` → `SetLBMapEntry`)
- FQDN unbind (`UnbindFQDNWorkflow` → `DeleteLBMapEntry`)

Commands are sent via direct TCP connection to HAProxy's admin socket:
```
echo "set map /var/lib/haproxy/maps/fqdn-to-shard.map example.com shard-web-1" | nc localhost 9999
echo "del map /var/lib/haproxy/maps/fqdn-to-shard.map example.com" | nc localhost 9999
```

The `haproxy_admin_addr` is read from the cluster's config JSON field. Falls back to `localhost:9999`.

Map entries survive HAProxy restarts because the map file is persisted via volume mount.

## Balancing Strategy

```
backend shard-web-1
    balance hdr(Host)
    hash-type consistent
    server web-1-node-0 <ip>:80 check
    server web-1-node-1 <ip>:80 check
```

- `balance hdr(Host)` — the same FQDN always hits the same node, which is good for PHP opcache locality
- `hash-type consistent` — when a node is added or removed, only ~1/N of requests get redistributed (not all of them)
- Health checks (`check`) — HAProxy automatically removes unhealthy nodes from rotation

## Key Components

### `Shard.LBBackend`

Auto-generated as `shard-{name}` during cluster provisioning (e.g. `shard-web-1`). This is the HAProxy backend name that maps receive when FQDNs are bound.

### Cluster Config: `haproxy_admin_addr`

The TCP address for the cluster's HAProxy Runtime API. Read from the cluster's `config` JSON field. Falls back to `localhost:9999` if not set.

### Worker → HAProxy Communication

The worker communicates with HAProxy via direct TCP connection to the Runtime API. The `resolveHAProxyAddr` helper reads the address from the cluster config JSON.

## Debugging

### Show current map entries

```bash
just lb-show
```

### Set a map entry manually

```bash
just lb-set www.example.com shard-web-1
```

### Delete a map entry

```bash
just lb-del www.example.com
```

### HAProxy stats UI

Open http://10.10.10.2:8404/stats (or `http://localhost:8404/stats` if using Docker Compose) to see backend health, active connections, and server status.

### Check response headers

```bash
curl -v http://10.10.10.2 -H "Host: acme.hosting.test"
```

Look for:
- `X-Served-By` — the node hostname that handled the request
- `X-Shard` — the shard name the node belongs to

### Verify consistent hashing

Repeat the same curl multiple times — `X-Served-By` should stay the same for the same `Host` header (assuming all nodes are healthy).

## Architecture Diagram

```
                    ┌──────────────────────────┐
                    │      HAProxy (LB)        │
                    │  ┌────────────────────┐  │
                    │  │ fqdn-to-shard.map  │  │
                    │  │ example.com →      │  │
                    │  │   shard-web-1      │  │
                    │  └────────────────────┘  │
                    │            │              │
                    │   balance hdr(Host)       │
                    │   hash-type consistent    │
                    └──────────┬───────────────┘
                               │
                    ┌──────────┴───────────┐
                    │                      │
             ┌──────┴──────┐       ┌──────┴──────┐
             │  web-1      │       │  web-1      │
             │  node-0     │       │  node-1     │
             │  (nginx)    │       │  (nginx)    │
             └─────────────┘       └─────────────┘
```

## Implementation Files

| File | Purpose |
|------|---------|
| `internal/activity/lb.go` | `SetLBMapEntry`, `DeleteLBMapEntry` via TCP Runtime API |
| `internal/workflow/fqdn.go` | Calls LB activities with `ClusterID` |
| `docker/haproxy/haproxy.cfg` | Base config with TCP admin socket on port 9999 |
