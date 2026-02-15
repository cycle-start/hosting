# Network Access Control

## Overview

Network access control provides two complementary features:

1. **Tenant Egress Rules** — Per-tenant outbound network restrictions via nftables
2. **Database Access Rules** — Per-database inbound connection restrictions via MySQL host patterns

## Tenant Egress Rules

Control which destination networks a tenant's processes can reach. Uses nftables with per-tenant chains.

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/tenants/{id}/egress-rules` | List rules for a tenant |
| POST | `/tenants/{id}/egress-rules` | Create a rule |
| GET | `/egress-rules/{id}` | Get a rule |
| DELETE | `/egress-rules/{id}` | Delete a rule |
| POST | `/egress-rules/{id}/retry` | Retry a failed rule |

### Request Body (Create)

```json
{
  "cidr": "10.0.0.0/8",
  "action": "deny",
  "description": "Block internal network"
}
```

- `cidr` — IPv4 or IPv6 CIDR (required)
- `action` — `deny` or `allow` (required)
- `description` — Human-readable description (optional)

### How It Works

1. Rules are stored in the `tenant_egress_rules` table
2. On create/delete, the `SyncEgressRulesWorkflow` runs
3. The workflow fetches all active rules and applies them to every node in the tenant's web shard
4. Each tenant gets a per-UID nftables chain (`tenant_{uid}`) in the `inet tenant_egress` table
5. A jump rule in the output chain routes traffic through the tenant's chain based on UID

### nftables Structure

```
table inet tenant_egress {
    chain output {
        type filter hook output priority 1; policy accept;
        meta skuid 5000 jump tenant_5000
        meta skuid 5001 jump tenant_5001
    }
    chain tenant_5000 {
        ip daddr 10.0.0.0/8 reject
        ip daddr 172.16.0.0/12 reject
    }
}
```

Default policy is `accept` — tenants without rules have unrestricted egress. Rules add specific deny or allow entries.

### Convergence

Egress rules are synced during shard convergence. The node agent rebuilds per-tenant chains from the desired state.

## Database Access Rules

Control which source networks can connect to a database. When rules exist, MySQL users are recreated with host patterns matching the allowed CIDRs.

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/databases/{id}/access-rules` | List rules for a database |
| POST | `/databases/{id}/access-rules` | Create a rule |
| GET | `/database-access-rules/{id}` | Get a rule |
| DELETE | `/database-access-rules/{id}` | Delete a rule |
| POST | `/database-access-rules/{id}/retry` | Retry a failed rule |

### Request Body (Create)

```json
{
  "cidr": "192.168.1.0/24",
  "description": "Application server subnet"
}
```

- `cidr` — IPv4 or IPv6 CIDR (required)
- `description` — Human-readable description (optional)

### How It Works

1. Rules are stored in the `database_access_rules` table
2. On create/delete, the `SyncDatabaseAccessWorkflow` runs
3. The workflow fetches all active rules and all active users for the database
4. On the primary MySQL node, each user is dropped and recreated with host patterns matching the allowed CIDRs
5. When no rules exist, users are created with `'%'` (any host)

### CIDR to MySQL Host Pattern Conversion

| CIDR | MySQL Host Pattern |
|------|--------------------|
| `10.0.0.5/32` | `10.0.0.5` |
| `192.168.1.0/24` | `192.168.1.%` |
| `10.0.0.0/16` | `10.0.%.%` |
| `10.0.0.0/8` | `10.%.%.%` |
| `0.0.0.0/0` | `%` |
| IPv6 CIDRs | Passed as-is (MySQL 8.0.23+ native support) |

### Behavior

- **No rules**: All users have host `'%'` (connections from any IP)
- **Rules exist**: Each user gets one MySQL account per allowed CIDR
- **Rule deleted (last one)**: Users revert to host `'%'`
- Changes apply on the primary node only (MySQL replication handles secondaries)

## Authorization

- Egress rules use `network:read/write/delete` scopes
- Database access rules use `databases:read/write/delete` scopes (shared with database management)
