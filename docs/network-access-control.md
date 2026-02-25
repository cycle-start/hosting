# Network Access Control

## Overview

Network access control provides two complementary features:

1. **Tenant Egress Rules** — Per-tenant outbound network restrictions via nftables
2. **Database Access Rules** — Per-database inbound connection restrictions via MySQL host patterns

## Tenant Egress Rules

Control which destination networks a tenant's processes can reach. Uses nftables with per-tenant chains and a **whitelist model**.

### Behavior

- **No rules**: Tenant has unrestricted egress (default)
- **Rules exist**: Only the specified CIDRs are allowed; all other egress is blocked

Rules are always "allow" CIDRs. When any rules exist, a final `reject` entry is added at the end of the tenant's chain, blocking everything not explicitly allowed.

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
  "cidr": "93.184.216.0/24",
  "description": "Allow CDN subnet"
}
```

- `cidr` — IPv4 or IPv6 CIDR (required)
- `description` — Human-readable description (optional)

### How It Works

1. Rules are stored in the `tenant_egress_rules` table
2. On create/delete, the `SyncEgressRulesWorkflow` runs
3. The workflow fetches all active rules and applies them to every node in the tenant's web shard
4. Each tenant gets a per-UID nftables chain (`tenant_{uid}`) in the `inet tenant_egress` table
5. A jump rule in the output chain routes traffic through the tenant's chain based on UID

### nftables Structure

With rules (whitelist — only specified CIDRs allowed):

```
table inet tenant_egress {
    chain output {
        type filter hook output priority 1; policy accept;
        meta skuid 5000 jump tenant_5000
    }
    chain tenant_5000 {
        ip daddr 93.184.216.0/24 accept
        ip6 daddr 2001:db8::/32 accept
        reject
    }
}
```

Without rules (unrestricted — chain removed):

```
table inet tenant_egress {
    chain output {
        type filter hook output priority 1; policy accept;
        # No jump rule for tenant 5000 — unrestricted
    }
}
```

### Convergence

Egress rules are synced during shard convergence. The node agent rebuilds per-tenant chains from the desired state.

## Database Access Rules

Control which source networks can connect to a database. The default is **internal-only** — databases are only accessible from within the hosting network.

### Default Behavior

When no access rules exist, MySQL users are created with host patterns matching the internal network CIDR (default: `10.0.0.0/8`, configurable via `INTERNAL_NETWORK_CIDR`). This means databases are accessible from hosting infrastructure but not from the public internet.

When rules are added, MySQL users get the internal host pattern **plus** each rule's CIDR pattern. Internal access is always preserved.

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
5. Internal network access (`10.%.%.%` by default) is always included

### CIDR to MySQL Host Pattern Conversion

| CIDR | MySQL Host Pattern |
|------|--------------------|
| `10.0.0.5/32` | `10.0.0.5` |
| `192.168.1.0/24` | `192.168.1.%` |
| `10.0.0.0/16` | `10.0.%.%` |
| `10.0.0.0/8` | `10.%.%.%` |
| `0.0.0.0/0` | `%` |
| IPv6 CIDRs | Passed as-is (MySQL 8.0.23+ native support) |

### Behavior Summary

- **No rules**: Users have internal-only host pattern (e.g. `10.%.%.%`)
- **Rules exist**: Users get internal host pattern + each rule's CIDR pattern
- **Rule deleted (last one)**: Users revert to internal-only
- Changes apply on the primary node only (MySQL replication handles secondaries)

### Configuration

| Env Var | Default | Description |
|---------|---------|-------------|
| `INTERNAL_NETWORK_CIDR` | `10.0.0.0/8` | CIDR for internal network access. Converted to MySQL host pattern for default database access. |

## Per-Tenant ULA Isolation on Service Nodes

Every tenant gets a unique ULA IPv6 address per node across all node types — web, DB, and Valkey. The address format is `fd00:{cluster_hash}:{shard_index}::{tenant_uid}`.

### Web Nodes

Web nodes use nftables UID-based binding (`ip6 tenant_binding` table) so each tenant's processes can only use their own ULA as source address. This is enforced via the `skuid` match.

### DB and Valkey Nodes

Service nodes (DB/Valkey) don't have per-tenant Linux users, so they use a different model:

- **`tenant0` dummy interface**: Same as web nodes — all tenant ULA addresses are assigned to this interface
- **`ip6 tenant_service_ingress` table**: An nftables set `ula_addrs` tracks all tenant ULAs on the node. The input chain allows traffic to these addresses only from `fd00::/16` (other hosting nodes) and `::1` (localhost), dropping all other ULA-destined traffic
- Non-ULA traffic (node's regular IPv4/IPv6) is unaffected (policy accept)

### nftables Structure (Service Nodes)

```
table ip6 tenant_service_ingress {
    set ula_addrs {
        type ipv6_addr
        elements = { fd00:abcd:1::1388, fd00:abcd:1::1389, ... }
    }
    chain input {
        type filter hook input priority 0; policy accept;
        ip6 daddr @ula_addrs ip6 saddr fd00::/16 accept
        ip6 daddr @ula_addrs ip6 saddr ::1 accept
        ip6 daddr @ula_addrs drop
    }
}
```

### Cross-Shard Routing

Nodes in different shards need to reach each other's ULA addresses (e.g., a web node connecting to `[fd00:hash:db_idx::uid]:3306`). This is achieved via transit addresses in the `fd00:{hash}:0::/48` prefix.

Each node role gets a separate range of transit indices to avoid collisions:

| Role | Transit offset | Range |
|------|---------------|-------|
| Web | 0 | 0–255 |
| Database | 256 | 256–511 |
| Valkey | 512 | 512–767 |

Each node adds its transit address (`fd00:{hash}:0::{transit_index}/64`) to its primary interface, then adds routes to peer nodes' ULA prefixes (`fd00:{hash}:{peer_shard_index}::/48`) via their transit addresses.

### Provisioning

- **Cloud-init**: DB and Valkey node templates include the `tenant0` dummy interface (module, netdev, network files) and `modprobe dummy && systemctl restart systemd-networkd` in runcmd
- **Creation workflows**: `CreateDatabaseWorkflow` and `CreateValkeyInstanceWorkflow` configure tenant ULA on shard nodes after resource creation (non-fatal on failure — convergence catches up)
- **Convergence**: `convergeDatabaseShard` and `convergeValkeyShard` set up ULA addresses for all tenants with resources on the shard, plus cross-shard transit routes

### Dual-Stack Binding

- MySQL: `bind-address = *` (MySQL 8.0.13+ dual-stack syntax)
- Valkey: `bind 0.0.0.0 ::` (listens on both IPv4 and IPv6)

### Verification

```bash
# On DB node: check tenant ULA addresses
ip -6 addr show dev tenant0

# On DB node: check nftables ingress rules
nft list table ip6 tenant_service_ingress

# From web node: reach a tenant's DB ULA
ping6 fd00:{hash}:{db_idx}::{uid}
mysql -h fd00:{hash}:{db_idx}::{uid} -P 3306 -u <user> -p
```

## Authorization

- Egress rules use `network:read/write/delete` scopes
- Database access rules use `databases:read/write/delete` scopes (shared with database management)
