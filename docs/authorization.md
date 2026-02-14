# API Key Authorization

## Overview

API keys have two authorization dimensions:

1. **Scopes** — what operations the key can perform (`resource:action`)
2. **Brands** — which brands the key can access

## Admin Tiers

| Tier | Scopes | Brands | Description |
|------|--------|--------|-------------|
| Platform admin | `["*:*"]` | `["*"]` | Full access to everything |
| Brand admin | `["*:*"]` | `["acme"]` | Full access within specific brands |
| Restricted | `["tenants:read", "databases:read"]` | `["acme"]` | Read-only for tenants/databases in one brand |

## Scope Format

`resource:action` where:

- **Wildcard**: `*:*` grants all scopes

### Resources

| Category | Resources |
|----------|-----------|
| Infrastructure | `brands`, `regions`, `clusters`, `shards`, `nodes` |
| Hosting | `tenants`, `webroots`, `fqdns`, `certificates`, `ssh_keys`, `backups` |
| Databases | `databases`, `database_users` |
| DNS | `zones`, `zone_records` |
| Email | `email` |
| Storage | `s3`, `valkey` |
| Platform | `platform`, `api_keys`, `audit_logs` |

### Actions

| Action | HTTP Methods |
|--------|-------------|
| `read` | GET |
| `write` | POST, PUT, PATCH |
| `delete` | DELETE |

### Examples

- `tenants:read` — list and get tenants
- `databases:write` — create databases
- `*:*` — full access to all resources

## Brand Scoping

- `brands: ["*"]` — platform admin, can access all brands
- `brands: ["acme", "other"]` — can only access resources belonging to those brands

### Platform-Only Endpoints

These endpoints require `brands: ["*"]` (platform admin):

- Dashboard (`/dashboard/stats`)
- Platform config (`/platform/config`)
- API keys (`/api-keys`)
- Audit logs (`/audit-logs`)
- Search (`/search`)
- Infrastructure: regions, clusters, shards, nodes

### Brand-Scoped Resources

Resources trace to a brand through their ownership chain:

| Resource | Brand Resolution |
|----------|-----------------|
| Tenant | `brand_id` (direct) |
| Zone | `brand_id` (direct) |
| Database, Valkey, S3 Bucket | `brand_id` (direct, nullable `tenant_id`) |
| Webroot, FQDN, Certificate, SSH Key, Backup | via tenant → `brand_id` |
| Zone Record | via zone → `brand_id` |
| Database User | via database → `brand_id` |
| Email Account/Alias/Forward/AutoReply | via FQDN → webroot → tenant → `brand_id` |

## API Key Management

### Create

```
POST /api/v1/api-keys
{
  "name": "my-key",
  "scopes": ["tenants:read", "databases:read"],
  "brands": ["acme-brand"]
}
```

Returns the full key (shown once, not stored).

### Update

```
PUT /api/v1/api-keys/{id}
{
  "name": "updated-name",
  "scopes": ["tenants:read", "tenants:write"],
  "brands": ["acme-brand", "other-brand"]
}
```

### Revoke

```
DELETE /api/v1/api-keys/{id}
```

Immediately stops the key from authenticating. Irreversible.

### Bootstrap Key

The `create-api-key` CLI command creates a platform admin key (`*:*` scopes, `*` brands) by default.
