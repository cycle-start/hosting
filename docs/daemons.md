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
    proxy_pass http://127.0.0.1:14523;
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

This supports WebSocket connections through nginx (HTTP Upgrade headers + 24-hour timeout).

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
environment=PORT="14523",APP_ENV="production"
```

### Convergence

Daemons are converged as part of web shard convergence (`ConvergeShardWorkflow`):
1. Daemon proxy info is fetched per webroot and included in nginx config generation
2. Supervisord configs are written to all nodes
3. Disabled daemons are explicitly stopped

## Examples

### Laravel Reverb (WebSocket)

```json
{
  "command": "php artisan reverb:start --port=$PORT",
  "proxy_path": "/app",
  "max_memory_mb": 256
}
```

The PHP webroot handles REST traffic normally. WebSocket connections to `/app` are proxied to the Reverb daemon.

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

## Planned: Per-Tenant IPv6 & Single-Node Execution

The current implementation runs daemons on all shard nodes and proxies via `127.0.0.1`. The planned architecture assigns each daemon to a **single node** and uses **per-tenant per-node ULA IPv6 addresses** for cross-node proxying. See [daemon-networking plan](plans/daemon-networking.md) for full details.

Key changes:
- Each daemon gets a `node_id` — runs on one node only, distributed across shard for load balancing
- Per-tenant IPv6 ULA: `fd00:{cluster}:{node_index}::{tenant_uid}` — deterministic, never floats
- Nginx on all nodes proxies to daemon node's tenant IPv6 address
- nftables UID-based binding restriction for network isolation
- `$HOST` env var auto-injected alongside `$PORT`
