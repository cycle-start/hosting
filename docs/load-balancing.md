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
         Node container (nginx)
                 ↓
         Response with X-Served-By + X-Shard headers
```

DNS resolves to the cluster's LB IP addresses. HAProxy receives the request on port 80, looks up the `Host` header in a map file to find the backend name, and forwards to the matching backend. Within the backend, consistent hashing on the `Host` header selects a specific node.

## Two Management Mechanisms

### Backend Configuration (infrequent)

Full `haproxy.cfg` regeneration + container restart. Triggered by:
- Cluster provisioning (`ProvisionClusterWorkflow`)
- Node additions/removals

The `ConfigureHAProxyBackends` activity:
1. Queries all active web shards with `LBBackend` set
2. For each shard, queries active nodes + their `node_deployments` for container names
3. Generates a complete `haproxy.cfg` from template
4. Writes the config into the HAProxy container via `ExecInContainer`
5. Validates with `haproxy -c`
6. Restarts HAProxy (stop + start)

### Map Entries (frequent)

HAProxy Runtime API via `socat` — instant, no reload. Triggered by:
- FQDN bind (`BindFQDNWorkflow` → `SetLBMapEntry`)
- FQDN unbind (`UnbindFQDNWorkflow` → `DeleteLBMapEntry`)

Commands are executed inside the HAProxy container:
```
echo "set map /var/lib/haproxy/maps/fqdn-to-shard.map example.com shard-web-1" | socat stdio /var/run/haproxy/admin.sock
echo "del map /var/lib/haproxy/maps/fqdn-to-shard.map example.com" | socat stdio /var/run/haproxy/admin.sock
```

Map entries survive HAProxy restarts because the map file is persisted via volume mount.

## Balancing Strategy

```
backend shard-web-1
    balance hdr(Host)
    hash-type consistent
    server web-1-node-0 <container>:80 check
    server web-1-node-1 <container>:80 check
```

- `balance hdr(Host)` — the same FQDN always hits the same node, which is good for PHP opcache locality
- `hash-type consistent` — when a node is added or removed, only ~1/N of requests get redistributed (not all of them)
- Health checks (`check`) — HAProxy automatically removes unhealthy nodes from rotation

## Key Components

### `Shard.LBBackend`

Auto-generated as `shard-{name}` during cluster provisioning (e.g. `shard-web-1`). This is the HAProxy backend name that maps receive when FQDNs are bound.

### Cluster Config: `haproxy_container`

The Docker container name for the cluster's HAProxy instance. Read from the cluster's `config` JSON field. Falls back to `hosting-haproxy` if not set.

### Worker → HAProxy Communication

The worker communicates with HAProxy via Docker exec (`ExecInContainer`). The worker container has `/var/run/docker.sock` mounted, giving it access to all containers on the Docker host. The `resolveHAProxy` helper reads the container name from cluster config and finds an active host machine for Docker API access.

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

Open http://localhost:8404/stats to see backend health, active connections, and server status.

### Check response headers

```bash
curl -v http://localhost -H "Host: acme.hosting.localhost"
```

Look for:
- `X-Served-By` — the node hostname that handled the request
- `X-Shard` — the shard name the node belongs to

### Verify consistent hashing

Repeat the same curl multiple times — `X-Served-By` should stay the same for the same `Host` header (assuming all nodes are healthy).

### Check node containers

```bash
docker ps | grep node-
```

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
| `internal/deployer/deployer.go` | `ExecInContainer` interface method |
| `internal/deployer/docker.go` | Docker SDK implementation of exec |
| `internal/activity/lb.go` | `SetLBMapEntry`, `DeleteLBMapEntry` via socat |
| `internal/activity/cluster.go` | `ConfigureHAProxyBackends` — full cfg generation |
| `internal/workflow/fqdn.go` | Calls LB activities with `ClusterID` |
| `internal/workflow/cluster.go` | Sets `LBBackend` on shard creation |
| `docker/haproxy/Dockerfile` | HAProxy image with socat installed |
| `docker/haproxy/haproxy.cfg` | Base config (backends generated dynamically) |
