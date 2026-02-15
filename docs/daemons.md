# Daemons

Daemons are long-running processes attached to a webroot — WebSocket servers, queue workers, custom background services, etc. They are managed by supervisord on web shard nodes and optionally expose an HTTP endpoint through nginx reverse proxy.

## Key Concepts

- **Sub-resource of webroots:** Daemons belong to a webroot, not a tenant directly. They run as the tenant's Linux user in the webroot's working directory.
- **Optional proxy_path:** When set (e.g., `/ws`, `/app`), nginx adds a `location` block that proxies to the daemon with WebSocket Upgrade headers. The daemon's `$PORT` environment variable is auto-set.
- **Port allocation:** Proxy ports are deterministically computed via FNV hash into the 10000-19999 range from `tenant_name/webroot_name/daemon_name`.
- **Supervisord:** Each daemon gets a supervisord program config with configurable `numprocs`, stop signal, stop wait, and memory limit.

## API Endpoints

| Method | Endpoint | Description |
|---|---|---|
| POST | `/webroots/{id}/daemons` | Create a daemon |
| GET | `/webroots/{id}/daemons` | List daemons for a webroot |
| GET | `/daemons/{id}` | Get daemon |
| PUT | `/daemons/{id}` | Update daemon |
| DELETE | `/daemons/{id}` | Delete daemon |
| POST | `/daemons/{id}/enable` | Enable (start) daemon |
| POST | `/daemons/{id}/disable` | Disable (stop) daemon |
| POST | `/daemons/{id}/retry` | Retry failed provisioning |

### Create Request

```json
{
  "command": "php artisan reverb:start --port=$PORT",
  "proxy_path": "/app",
  "num_procs": 1,
  "stop_signal": "TERM",
  "stop_wait_secs": 30,
  "max_memory_mb": 512,
  "environment": {"APP_ENV": "production"}
}
```

### Response

```json
{
  "id": "uuid",
  "tenant_id": "uuid",
  "webroot_id": "uuid",
  "name": "daemon_a1b2c3d4e5",
  "command": "php artisan reverb:start --port=$PORT",
  "proxy_path": "/app",
  "proxy_port": 14523,
  "num_procs": 1,
  "stop_signal": "TERM",
  "stop_wait_secs": 30,
  "max_memory_mb": 512,
  "environment": {"APP_ENV": "production"},
  "enabled": true,
  "status": "provisioning",
  "created_at": "...",
  "updated_at": "..."
}

```

## Architecture

### Nginx Integration

When a daemon has `proxy_path` set, the parent webroot's nginx config is regenerated to include a proxy location block:

```nginx
location /app {
    proxy_pass http://[fd00:1:2::2742]:14523;
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

The `proxy_pass` target uses the tenant's ULA IPv6 address on the daemon's assigned node. This supports WebSocket connections through nginx (HTTP Upgrade headers + 24-hour timeout) and cross-node proxying.

### Supervisord Config

Each daemon creates a supervisord config at `/etc/supervisor/conf.d/daemon-{tenantName}-{daemonName}.conf`:

```ini
[program:daemon-{tenantName}-{daemonName}]
command={command}
directory=/var/www/storage/{tenantName}/webroots/{webrootName}
user={tenantName}
numprocs=1
autostart=true
autorestart=unexpected
stopsignal=TERM
stopwaitsecs=30
environment=HOST="fd00:1:2::2742",PORT="14523",APP_ENV="production"
```

When `proxy_path` is set, `PORT` and `HOST` are auto-injected. `HOST` is the tenant's ULA IPv6 address on the daemon's assigned node — the daemon should bind to `$HOST:$PORT`.

### Convergence

Daemons are converged as part of web shard convergence (`ConvergeShardWorkflow`):
1. Daemon proxy info is fetched per webroot and included in nginx config generation
2. Supervisord configs are written to the daemon's assigned node
3. Disabled daemons are explicitly stopped

## Examples

### Laravel Reverb (WebSocket)

```json
{
  "command": "php artisan reverb:start --host=$HOST --port=$PORT",
  "proxy_path": "/app",
  "max_memory_mb": 256
}
```

The PHP webroot handles REST traffic normally. WebSocket connections to `/app` are proxied to the Reverb daemon. The daemon binds to the tenant's ULA IPv6 address (`$HOST`) on its allocated port (`$PORT`).

### Background Queue Worker

```json
{
  "command": "php artisan queue:work --sleep=3 --tries=3",
  "num_procs": 2,
  "max_memory_mb": 256
}
```

No `proxy_path` — this is a pure background process with no nginx integration. Two worker processes run via supervisord.

### Custom Node.js WebSocket Server

```json
{
  "command": "node ws-server.js",
  "proxy_path": "/ws",
  "environment": {"NODE_ENV": "production"}
}
```

## Single-Node Execution & Per-Tenant IPv6

Each daemon runs on **one specific node** in the shard, assigned via least-loaded round-robin on creation. The daemon's `node_id` is stored in the database and used for all lifecycle operations.

Each tenant gets a **unique ULA IPv6 address on every node**, computed deterministically:

```
fd00:{cluster_hash}:{node_shard_index}::{tenant_uid_hex}
```

Nginx on **all nodes** proxies daemon traffic to the correct node's tenant ULA address. This means a request hitting any node in the shard will be proxied to the daemon's specific node.

### Environment Variables

When `proxy_path` is set, two environment variables are auto-injected:

| Variable | Example | Purpose |
|---|---|---|
| `PORT` | `14523` | Allocated port number (FNV hash into 10000-19999) |
| `HOST` | `fd00:1:2::2742` | Tenant's ULA on the daemon's assigned node |

### Remaining Work

- **Tenant ULA management on nodes:** `tenant0` dummy interface, `ConfigureTenantAddresses` activity, nftables UID-based binding restriction. See [daemon-networking plan](plans/daemon-networking.md).
- **Failover:** manual daemon reassignment on node failure (automatic later).
