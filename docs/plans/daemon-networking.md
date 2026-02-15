# Daemon Networking: Per-Tenant IPv6, Single-Node Execution & Worker Removal

## Problem Statement

The current daemon implementation runs on **all nodes in a web shard** and proxies via `127.0.0.1`. This has three problems:

1. **Resource waste:** Running N copies of every daemon across N nodes. Many daemons (WebSocket servers, stateful workers) can't safely run as multiple instances.
2. **No network isolation:** All daemons bind to localhost — any tenant's process can connect to any other tenant's daemon port.
3. **Worker runtime is redundant:** The `worker` runtime (standalone webroot with supervisord) is architecturally inferior to daemons (sub-resource of webroot). Daemons subsume it entirely.

## Solution Overview

Each daemon runs on **one specific node** in the shard. Each tenant gets a **unique IPv6 ULA address on every node**, and nginx on all nodes proxies daemon traffic to the correct node's address. Binding is restricted by `nftables` rules so only the tenant's Linux UID can bind to their address.

```
                    ┌─────────────────────────────────────────────────┐
                    │               Web Shard (3 nodes)               │
                    │                                                 │
  Client ──HTTP──▶  │  Node 0 (nginx)─────┐                          │
                    │  Node 1 (nginx)──────┼──IPv6──▶ Node 2 (daemon) │
                    │  Node 2 (nginx)─────┘                          │
                    │                                                 │
                    │  Daemon runs on ONE node.                       │
                    │  All nodes proxy to it via tenant IPv6.         │
                    └─────────────────────────────────────────────────┘
```

## Per-Tenant Per-Node IPv6 Addressing

### Addressing Scheme

Each tenant gets a **unique ULA address on each node**. Addresses are deterministic — computed from cluster ID, a per-node index, and the tenant's Linux UID:

```
fd00:{cluster_id}:{node_index}::{tenant_uid}
```

**Examples** (cluster "osl-1", tenant UID 10042):

| Node | Index | Address |
|---|---|---|
| web-0 | 1 | `fd00:1:1::2742` |
| web-1 | 2 | `fd00:1:2::2742` |
| web-2 | 3 | `fd00:1:3::2742` |

Where `2742` is `10042` in hex.

### Why Per-Node Unique Addresses (Not Floating VIPs)

- **No ARP/NDP cache issues:** Each address lives on exactly one node, permanently. No neighbor cache invalidation on failover.
- **No address conflicts:** Multiple nodes never compete for the same address.
- **Simple failover:** Stop daemon on old node, start on new node, reconverge nginx to point at new node's address. No IP migration.
- **Deterministic:** Address can be computed from (cluster, node_index, uid) without any state lookup.

### Address Lifecycle

1. **Tenant provisioned on shard** → compute ULA for each node, add to node's loopback/dummy interface
2. **Daemon created with proxy_path** → daemon binds to its node's tenant ULA on the allocated port
3. **Nginx regenerated** → all nodes' nginx configs point `proxy_pass` at the daemon node's tenant ULA
4. **Tenant deleted** → remove ULA from all nodes

### Interface

Addresses are assigned to a dedicated dummy interface (`tenant0`) on each node, not the primary NIC:

```bash
# On node web-0 (index 1), for tenant UID 10042:
ip -6 addr add fd00:1:1::2742/128 dev tenant0
```

This interface is created once during cloud-init and persists. Addresses are managed by the node-agent during convergence.

### nftables Binding Restriction

Each tenant's ULA is locked to their Linux UID via nftables, preventing other tenants from binding:

```nft
table ip6 tenant_binding {
    chain output {
        type filter hook output priority 0; policy accept;
        # Only UID 10042 can bind to fd00:1:1::2742
        ip6 saddr fd00:1:1::2742 meta skuid != 10042 reject
    }
}
```

Rules are managed by the node-agent as tenants are provisioned/removed. This ensures a tenant's daemon process (running as their Linux user) is the only process that can bind to their address.

## Single-Node Daemon Execution

### Per-Daemon `node_id`

Each daemon is assigned to a specific node in the shard. The `node_id` is stored in the daemon record and used to route the supervisord config + start/stop to that one node only.

```sql
ALTER TABLE daemons ADD COLUMN node_id TEXT REFERENCES nodes(id);
```

Node assignment strategy:
- **Round-robin on creation:** distribute daemons across shard nodes for even load
- **Sticky:** once assigned, a daemon stays on its node unless manually moved or the node fails
- **Rebalance on failure:** if a node goes down, its daemons are redistributed to healthy nodes

### How It Works

1. **Create daemon** → assign `node_id` (round-robin across active shard nodes) → write supervisord config to **that node only** → start on that node → regenerate nginx on **all nodes** (pointing at the daemon node's tenant ULA)
2. **Enable/disable** → start/stop on the **assigned node only**
3. **Delete daemon** → stop on assigned node → remove config → regenerate nginx on all nodes
4. **Node failure** → reassign orphaned daemons to healthy nodes → converge

### Nginx Proxy Configuration

When a daemon has `proxy_path`, nginx on **every node** gets a location block pointing to the daemon's specific node:

```nginx
# On ALL nodes in the shard:
location /app {
    proxy_pass http://[fd00:1:2::2742]:14523;  # Node 1's address for this tenant
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_read_timeout 86400s;
    proxy_send_timeout 86400s;
}
```

The target address changes based on which node the daemon is assigned to. When a daemon is reassigned (failover, rebalance), only the nginx configs need regenerating — the addresses themselves never move.

### Daemon Environment Variables

When a daemon has `proxy_path`, two environment variables are auto-injected:

| Variable | Value | Purpose |
|---|---|---|
| `PORT` | `14523` | Allocated port number |
| `HOST` | `fd00:1:2::2742` | Tenant's ULA on the daemon's assigned node |

The daemon should bind to `$HOST:$PORT`. Example:

```json
{
  "command": "php artisan reverb:start --host=$HOST --port=$PORT",
  "proxy_path": "/app"
}
```

### Inter-Daemon Communication

Daemons on different nodes can communicate directly via their tenant's ULA addresses:

```
Daemon A (node 0) ──IPv6──▶ Daemon B (node 1)
fd00:1:1::2742:15000        fd00:1:2::2742:16000
```

Or through the nginx proxy path if preferred:

```
Daemon A ──HTTP──▶ nginx (any node) ──proxy──▶ Daemon B
```

## Worker Runtime Removal

Daemons completely replace the worker runtime. A daemon without `proxy_path` is functionally identical to a worker — a pure background process managed by supervisord.

### What to Remove

| File | Action |
|---|---|
| `internal/agent/runtime/phpworker.go` | Delete |
| `internal/agent/runtime/phpworker_test.go` | Delete |
| `docs/plans/php-worker-runtime.md` | Delete |
| `docs/daemons.md` "Daemon vs Worker" table | Remove section |
| References to "worker" runtime in `STATUS.md` | Remove |
| `internal/agent/runtime/manager.go` | Remove worker case |
| Convergence in `converge_shard.go` | Remove worker-specific logic |
| Admin UI runtime dropdown | Remove "worker" option |
| Validation allowing `runtime: "worker"` | Remove |

### Migration Path

No backwards compatibility needed — this is pre-release software. The migration edits the original schema (per project convention). Any existing worker webroots in dev are wiped with the DB.

## Schema Changes

### Daemon Table (edit `migrations/core/00037_daemons.sql`)

Add columns:

```sql
node_id TEXT REFERENCES nodes(id)  -- assigned execution node
```

### Node Index

Nodes need a stable, per-shard index for deterministic ULA computation. Options:

**Option A: Computed from node ordering** — sort shard nodes by ID, index = position. Simple but index changes if a node is removed from the middle.

**Option B: Stored `node_index` column** — explicit, assigned on shard assignment. Survives node removal. Recommended.

```sql
-- In nodes table (edit original migration):
ALTER TABLE nodes ADD COLUMN shard_index INT;
-- UNIQUE(shard_id, shard_index) enforced at application level
```

### Tenant IPv6 Tracking

ULA addresses are deterministic (computed from cluster_id + node_index + uid), so they don't need to be stored in the database. The node-agent computes them during convergence. However, the cluster ID needs to map to a numeric value for the ULA prefix. Options:

**Option A: Hash cluster ID to 16-bit value** — `fnv32(cluster_id) % 0xFFFF`. Deterministic, no extra storage. Tiny collision risk across clusters (acceptable since ULAs are cluster-local).

**Option B: Explicit `ipv6_prefix` on cluster** — stored in cluster config. Full control, no collision risk.

Recommend **Option A** for simplicity — ULAs are link-local to the cluster network, collisions between different clusters don't matter.

## Implementation Phases

### Phase 1: Worker Runtime Removal ✅
- Deleted `phpworker.go` and test file
- Removed worker references from runtime manager, convergence, validation, admin UI
- Removed `docs/plans/php-worker-runtime.md`
- Updated docs

### Phase 2: Node Index ✅
- Added `shard_index` to nodes migration with partial unique index
- Auto-assign index on shard assignment (max + 1 within shard)
- Exposed in node model

### Phase 3: Tenant ULA Management ✅
- Added `tenant0` dummy interface to cloud-init for web nodes (systemd-networkd)
- `TenantULAManager` in node-agent: manages ULA addrs on `tenant0` + nftables binding restrictions
- `ConfigureTenantAddresses` / `RemoveTenantAddresses` activities wired into convergence, create, and delete workflows
- nftables `ip6 tenant_binding` table with `allowed` set: `(addr, uid)` pairs, system users (UID < 1000) exempt
- Cross-node ULA routing via transit addresses (`fd00:{hash}:0::{index}/64`) on primary interface
- `ConfigureULARoutes` activity sets up routes during convergence so nginx can proxy to daemons on other nodes

### Phase 4: Single-Node Daemon Assignment ✅
- Added `node_id` to daemon migration and model
- Least-loaded round-robin assignment in daemon creation
- All daemon workflows target single assigned node
- Delete workflow targets assigned node

### Phase 5: Cross-Node Nginx Proxy ✅
- `DaemonProxyInfo` includes `TargetIP` and `ProxyURL` (pre-formatted)
- Nginx template: `proxy_pass {{ .ProxyURL }}` (handles IPv6 bracket notation)
- `$HOST` and `$PORT` env vars auto-injected in supervisord config when proxy_path is set
- Nginx regenerated on all nodes when daemon is created/updated/deleted
- ULA computed via `core.ComputeTenantULA(clusterID, nodeShardIndex, tenantUID)`

### Phase 6: Failover
- Detect node failure (health checks)
- Reassign daemons from failed node to healthy nodes
- Reconverge: stop on old (if reachable), start on new, regenerate nginx on all

## Open Questions

1. **Health-check-based failover** — should daemon reassignment be automatic (on node health failure) or manual (API call)? Recommend manual first, automatic later (matching MySQL replication decision).

2. **Port collision with Node.js runtime** — Node.js runtime uses ports 3000-9999, daemons use 10000-19999. These ranges don't overlap. But both the Node.js runtime and daemons with `proxy_path` need to bind to the tenant ULA. Need to extend ULA binding to Node.js runtime proxy as well.

3. **Max daemons per webroot** — should we limit daemon count? Recommend 10 per webroot initially.

4. **Daemon logs** — currently written to local files. After Vector integration, should ship to Loki. No change needed now.
