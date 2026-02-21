# Webroots & Runtimes

A **webroot** is a website document root belonging to a tenant. Each webroot has a runtime (language), version, and optional configuration. Webroots are served by nginx on web shard nodes and can have FQDNs bound to them.

## Model

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Auto-generated short ID |
| `tenant_id` | string | Owning tenant |
| `subscription_id` | string | Subscription grouping (required) |
| `name` | string | Slug name (e.g. `main`, `blog`) |
| `runtime` | string | One of: `php`, `node`, `python`, `ruby`, `static` |
| `runtime_version` | string | Version string (e.g. `8.5`, `20`, `3.12`) |
| `runtime_config` | JSON | Runtime-specific configuration (default: `{}`) |
| `public_folder` | string | Subfolder to serve as document root (e.g. `public`) |
| `env_file_name` | string | Env file name (default: `.env.hosting`) |
| `env_shell_source` | bool | Auto-source env vars in SSH sessions |
| `service_hostname_enabled` | bool | Enable per-webroot service hostname (default: `true`) |
| `status` | string | Current lifecycle status |
| `status_message` | string | Error message when `failed` |

## API Endpoints

| Method | Path | Response | Description |
|--------|------|----------|-------------|
| `GET` | `/tenants/{tenantID}/webroots` | 200, paginated | List webroots for a tenant |
| `POST` | `/tenants/{tenantID}/webroots` | 202 | Create webroot (async). Supports nested FQDNs |
| `GET` | `/webroots/{id}` | 200 | Get webroot by ID |
| `PUT` | `/webroots/{id}` | 202 | Update runtime, version, config, or public folder (async) |
| `DELETE` | `/webroots/{id}` | 202 | Delete webroot and cascade to FQDNs (async) |
| `POST` | `/webroots/{id}/retry` | 202 | Retry a failed webroot |

### Create Request

```json
{
  "subscription_id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "main",
  "runtime": "php",
  "runtime_version": "8.5",
  "runtime_config": {},
  "public_folder": "public",
  "service_hostname_enabled": true,
  "fqdns": [
    { "fqdn": "example.com", "ssl_enabled": true }
  ]
}
```

### Update Request

All fields are optional. Only provided fields are changed.

```json
{
  "runtime": "node",
  "runtime_version": "20",
  "runtime_config": {"entry_point": "server.js"},
  "public_folder": "dist",
  "service_hostname_enabled": false
}
```

## Supported Runtimes

### PHP

- **App server**: PHP-FPM pool per tenant
- **Socket**: `/run/php/{tenantID}-php{version}.sock`
- **Config**: `/etc/php/{version}/fpm/pool.d/{tenantID}.conf`
- **Process model**: `pm = dynamic`, max 5 children, 2 start servers
- **Reload**: `USR2` signal for graceful reload
- **Default version**: 8.5
- **Security**: `open_basedir` restricted to `/var/www/storage/{tenantID}/:/tmp/`
- **Nginx**: FastCGI pass to the FPM socket, `try_files` falls back to `/index.php?$query_string`

### Node.js

- **App server**: Systemd service unit per webroot
- **Service name**: `node-{tenantID}-{webrootName}`
- **Entry point**: `index.js` (from working directory)
- **Port**: Deterministic hash of `{tenantID}/{webrootName}` mapped to range 3000-9999
- **Env**: `NODE_ENV=production`, `PORT={port}`
- **Nginx**: Reverse proxy to `127.0.0.1:{port}` with WebSocket upgrade support

### Python

- **App server**: Gunicorn via systemd service unit
- **Service name**: `gunicorn-{tenantID}-{webrootName}`
- **Socket**: `/run/gunicorn/{tenantID}-{webrootName}.sock`
- **WSGI module**: `app:application` (default)
- **Workers**: 3, timeout 120s
- **Reload**: HUP signal for graceful reload
- **Nginx**: Reverse proxy to the Gunicorn unix socket

### Ruby

- **App server**: Puma via systemd service unit
- **Service name**: `puma-{tenantID}-{webrootName}`
- **Socket**: `/run/puma/{tenantID}-{webrootName}.sock`
- **Workers**: 2, threads 1-5, production mode
- **Reload**: USR1 signal for graceful restart
- **Nginx**: Reverse proxy to the Puma unix socket

### Static

- **App server**: None. Nginx serves files directly.
- **Nginx**: `try_files $uri $uri/ =404`

## Runtime Manager Interface

All runtimes implement the `Manager` interface:

```go
type Manager interface {
    Configure(ctx context.Context, webroot *WebrootInfo) error
    Start(ctx context.Context, webroot *WebrootInfo) error
    Stop(ctx context.Context, webroot *WebrootInfo) error
    Reload(ctx context.Context, webroot *WebrootInfo) error
    Remove(ctx context.Context, webroot *WebrootInfo) error
}
```

## Nginx Configuration

The node-agent generates nginx server blocks per webroot. Config files are placed in `{nginxConfigDir}/sites-enabled/{tenantID}_{webrootName}.conf`.

Key features:
- **Server names** from bound FQDNs (falls back to `_` if none)
- **Document root**: `/var/www/storage/{tenantID}/webroots/{webrootName}/{publicFolder}`
- **SSL**: Auto-configured when certificate files exist at `{certDir}/{fqdn}/fullchain.pem` and `privkey.pem`. Falls back to HTTP-only if certs are not yet provisioned. HTTP-to-HTTPS redirect when SSL is active.
- **TLS**: TLSv1.2 and TLSv1.3, `HIGH:!aNULL:!MD5` ciphers, server cipher preference
- **Debug headers**: `X-Served-By` (hostname) and `X-Shard` (shard name)
- **Orphan cleanup**: `CleanOrphanedConfigs` removes config files for webroots that no longer exist
- **Logs**: Access and error logs per webroot in `/var/www/storage/{tenantID}/logs/`

## Service Hostnames

Each webroot automatically gets a stable service hostname in the format:

```
{webroot_name}.{tenant_name}.{brand.base_hostname}
```

For example, a webroot named `main` on tenant `t_abc1234567` under a brand with base hostname `hosting.example.com` would get:

```
main.t_abc1234567.hosting.example.com
```

Service hostnames are enabled by default (`service_hostname_enabled: true`) and can be toggled per webroot via the API. When enabled:

1. **DNS A records** are auto-created pointing to the cluster's load balancer IPs
2. **LB map entries** are created (hostname → shard backend)
3. **Nginx server_name** includes the service hostname alongside bound FQDNs

This provides a predictable URL for each webroot without requiring custom FQDN binding. Service hostnames are managed by the create/update/delete webroot workflows and included in shard convergence.

## FQDN Binding

FQDNs are tenant-scoped (`tenant_id NOT NULL`) and can optionally be bound to a webroot (`webroot_id` is nullable). An FQDN can exist independently of any webroot -- for example, to serve email or to reserve a domain before assigning it to a webroot.

When an FQDN is bound to a webroot:
1. Auto-DNS records (A/AAAA) are created pointing to the cluster's load balancer IPs
2. The LB map entry is created (FQDN -> shard backend)
3. Nginx is reloaded on all shard nodes
4. If `ssl_enabled` is true, a Let's Encrypt certificate is provisioned via a child workflow

Unbound FQDNs (no webroot) can be created at the tenant level via `POST /tenants` with a top-level `fqdns` array, or via the FQDN API directly.

## Storage Layout

All webroot files live on CephFS shared storage:

```
/var/www/storage/{tenantID}/
  webroots/
    {webrootName}/
      {publicFolder}/     # Document root (if set)
  logs/
    {webrootName}-access.log
    {webrootName}-error.log
    php-error.log         # PHP-FPM error log
```
