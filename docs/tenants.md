# Tenant Management

A **tenant** is the fundamental unit of hosting. Each tenant maps to a Linux user on web shard nodes, owns webroots, databases, DNS zones, email accounts, Valkey instances, S3 buckets, and SSH keys. Tenants are scoped to a brand and placed on a specific region/cluster/shard.

## Model

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Auto-generated short ID |
| `brand_id` | string | Brand this tenant belongs to |
| `customer_id` | string | Customer identifier (opaque, from control panel or CRM) |
| `region_id` | string | Region ID |
| `cluster_id` | string | Cluster ID |
| `shard_id` | string | Web shard ID (nullable) |
| `uid` | int | Linux UID, auto-assigned from `tenant_uid_seq` |
| `sftp_enabled` | bool | Whether SFTP access is enabled |
| `ssh_enabled` | bool | Whether SSH access is enabled |
| `status` | string | Current lifecycle status |
| `status_message` | string | Error message when status is `failed` |
| `suspend_reason` | string | Reason for suspension (e.g., "abuse", "unpaid", "migration") |

## Lifecycle

```
pending -> provisioning -> active
                |              |
                v              v
             failed       suspended
                              |
                              v
                          pending (unsuspend)

active -> deleting -> deleted
```

Statuses: `pending`, `provisioning`, `active`, `failed`, `suspended`, `deleting`, `deleted`.

Each transition triggers a Temporal workflow that runs on every node in the tenant's shard:
- **CreateTenantWorkflow** -- creates the Linux user and SSH/SFTP config on each node
- **UpdateTenantWorkflow** -- updates user settings and SSH/SFTP config on each node
- **SuspendTenantWorkflow** -- suspends the tenant on each node
- **UnsuspendTenantWorkflow** -- restores the tenant on each node
- **DeleteTenantWorkflow** -- removes SSH config and deletes the tenant from each node

## API Endpoints

All endpoints require `ApiKeyAuth`. Brand access is enforced on every request.

### CRUD

| Method | Path | Response | Description |
|--------|------|----------|-------------|
| `GET` | `/tenants` | 200, paginated | List tenants. Filters: `search`, `status`, `sort`, `order`, `limit`, `cursor` |
| `POST` | `/tenants` | 202 | Create tenant (async). Supports nested resource creation |
| `GET` | `/tenants/{id}` | 200 | Get tenant by ID |
| `PUT` | `/tenants/{id}` | 202 | Update tenant (async). Currently supports `sftp_enabled`, `ssh_enabled` |
| `DELETE` | `/tenants/{id}` | 202 | Delete tenant (async). Cascades to all child resources |

### Actions

| Method | Path | Response | Description |
|--------|------|----------|-------------|
| `POST` | `/tenants/{id}/suspend` | 202 | Suspend tenant with reason, cascades to all child resources |
| `POST` | `/tenants/{id}/unsuspend` | 202 | Unsuspend, restoring tenant and all child resources |
| `POST` | `/tenants/{id}/migrate` | 202 | Migrate to a different web shard |
| `POST` | `/tenants/{id}/retry` | 202 | Retry provisioning for a failed tenant |
| `POST` | `/tenants/{id}/retry-failed` | 202 | Retry all failed child resources |
| `GET` | `/tenants/{id}/resource-summary` | 200 | Resource counts grouped by type and status |
| `POST` | `/tenants/{id}/login-sessions` | 201 | Create an OIDC login session for the tenant |

## Create Request

```json
{
  "brand_id": "acme",
  "region_id": "osl-1",
  "cluster_id": "prod-1",
  "shard_id": "web-1",
  "sftp_enabled": true,
  "ssh_enabled": false,
  "subscriptions": [
    { "id": "550e8400-e29b-41d4-a716-446655440000", "name": "main" }
  ],
  "zones": [{ "subscription_id": "550e8400-e29b-41d4-a716-446655440000", "name": "example.com" }],
  "webroots": [{
    "subscription_id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "main",
    "runtime": "php",
    "runtime_version": "8.5",
    "public_folder": "public",
    "fqdns": [{ "fqdn": "example.com", "ssl_enabled": true }]
  }],
  "databases": [{
    "subscription_id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "main_db",
    "shard_id": "db-1",
    "users": [{ "username": "app", "password": "secret123", "privileges": ["ALL"] }]
  }],
  "valkey_instances": [{
    "subscription_id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "cache",
    "shard_id": "valkey-1",
    "max_memory_mb": 64,
    "users": [{ "username": "app", "password": "secret123", "privileges": ["allcommands"], "key_pattern": "~*" }]
  }],
  "s3_buckets": [{ "subscription_id": "550e8400-e29b-41d4-a716-446655440000", "name": "media", "shard_id": "s3-1", "public": false }],
  "fqdns": [{ "fqdn": "unbound.example.com", "ssl_enabled": true }],
  "ssh_keys": [{ "name": "deploy", "public_key": "ssh-ed25519 AAAA..." }]
}
```

The cluster must be in the brand's allowed cluster list. Subscriptions are created synchronously before other resources. All nested resources require a `subscription_id` and trigger their own provisioning workflows. FQDNs can be created at the top level (unbound to any webroot) or nested inside webroots.

## Migration

```json
POST /tenants/{id}/migrate
{
  "target_shard_id": "web-2",
  "migrate_zones": true,
  "migrate_fqdns": true
}
```

Triggers `MigrateTenantWorkflow` which moves the tenant to a different web shard. Optionally migrates associated zones and FQDNs.

## Suspension

```json
POST /tenants/{id}/suspend
{
  "reason": "abuse"
}
```

Suspending a tenant requires a `reason` (free text, e.g., "abuse", "unpaid", "migration"). The reason is stored in `suspend_reason` on the tenant and all cascaded child resources.

**Cascade behavior:** When a tenant is suspended, all active child resources are also suspended with the same reason:
- Subscriptions
- Webroots
- Databases
- Valkey instances
- S3 buckets
- Zones

Unsuspending (`POST /tenants/{id}/unsuspend`) restores the tenant and all suspended child resources to active, clearing `suspend_reason`.

## Retry Failed Resources

`POST /tenants/{id}/retry-failed` scans all child resource types for `failed` status and re-triggers their provisioning workflows. Returns `{"status": "retrying", "count": N}` with the number of resources being retried. Also retries the tenant itself if it is in `failed` state.

Resource types retried: webroots, FQDNs, certificates, zones, zone records, databases, database users, Valkey instances, Valkey users, email accounts, email aliases, email forwards, email auto-replies, SSH keys, S3 buckets, S3 access keys, backups.

## Resource Summary

`GET /tenants/{id}/resource-summary` returns a synchronous breakdown:

```json
{
  "webroots": {"active": 2, "pending": 1},
  "fqdns": {"active": 3},
  "certificates": {},
  "databases": {"active": 1},
  "database_users": {"active": 1},
  "zones": {"active": 1},
  "zone_records": {"active": 5},
  "valkey_instances": {},
  "valkey_users": {},
  "ssh_keys": {"active": 1},
  "backups": {},
  "email_accounts": {},
  "email_aliases": {},
  "email_forwards": {},
  "email_autoreplies": {},
  "total": 15,
  "pending": 1,
  "provisioning": 0,
  "failed": 0
}
```
