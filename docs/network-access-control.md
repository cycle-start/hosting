# Network Access Control

## Overview

Network access control provides:

1. **Tenant Egress Rules** — Per-tenant outbound network restrictions via nftables

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
| Gateway | 768 | 768–1023 |

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

## WireGuard VPN Gateway

Tenants can create WireGuard VPN peers to access their DB and Valkey ULA addresses from external machines (developer laptops, CI servers, etc.).

### Architecture

```
Developer laptop                 Gateway node (10.10.10.90)            DB/Valkey nodes
┌──────────┐  WireGuard tunnel  ┌───────────────────────┐  ULA routes ┌─────────────┐
│ wg0      ├───────────────────►│ wg0: fd00:…:ffff::0/64│────────────►│ tenant0:    │
│ fd00:…   │                    │ nftables FORWARD per  │             │ fd00:…::uid │
│ :ffff::N │                    │ peer → tenant ULAs    │             │             │
└──────────┘                    └───────────────────────┘             └─────────────┘
```

- Client address: `fd00:{hash}:ffff::{peer_index}/128` (shard index `0xFFFF` reserved for gateway)
- Gateway has transit routes to all DB and Valkey node ULA prefixes
- Gateway's `tenant_service_ingress` allows `fd00::/16` traffic, so WireGuard client traffic is accepted at service nodes

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/tenants/{id}/wireguard-peers` | List peers for a tenant |
| POST | `/tenants/{id}/wireguard-peers` | Create a peer |
| GET | `/wireguard-peers/{id}` | Get a peer |
| DELETE | `/wireguard-peers/{id}` | Delete a peer |
| POST | `/wireguard-peers/{id}/retry` | Retry a failed peer |

### Request Body (Create)

```json
{
  "name": "Edvins laptop",
  "subscription_id": "sub_xxx"
}
```

### Response (Create — 202 Accepted)

```json
{
  "peer": { "id": "...", "name": "Edvins laptop", "assigned_ip": "fd00:…:ffff::1", ... },
  "private_key": "...",
  "client_config": "[Interface]\nPrivateKey = ...\nAddress = fd00:…:ffff::1/128\n\n[Peer]\nPublicKey = ...\n..."
}
```

The `private_key` and `client_config` are returned **once** on creation and never stored. The user must save the `.conf` file.

### Per-Peer Firewall

Each peer gets nftables FORWARD rules allowing traffic only to their tenant's ULA addresses on DB and Valkey nodes. The `wg_forward` table defaults to `policy drop`:

```
table ip6 wg_forward {
    chain forward {
        type filter hook forward priority 0; policy drop;
        ip6 saddr fd00:…:ffff::1 ip6 daddr fd00:…:{db_idx}::5000 accept
        ip6 saddr fd00:…:ffff::1 ip6 daddr fd00:…:{valkey_idx}::5000 accept
    }
}
```

### Convergence

Gateway shard convergence:
1. Ensures WireGuard interface on each gateway node
2. Lists all peers for tenants in the cluster
3. Computes per-peer allowed ULAs (all DB + Valkey ULAs for the peer's tenant)
4. Syncs all peers to each gateway node
5. Sets up transit routes to all DB and Valkey shard nodes

### Service Metadata in Client Config

The `client_config` returned on peer creation includes service metadata comments that enable automatic service discovery by the `hosting-cli` tool:

```ini
[Interface]
PrivateKey = ...
Address = fd00:…:ffff::1/128

[Peer]
PublicKey = ...
PresharedKey = ...
Endpoint = gw.example.com:51820
AllowedIPs = fd00::/16
PersistentKeepalive = 25

# hosting-cli:services
# mysql=fd00:abcd:101::1388
# valkey=fd00:abcd:201::1388
```

Service addresses are computed from the tenant's databases and Valkey instances at creation time using the same ULA scheme (`fd00:{cluster_hash}:{transit_index}::{tenant_uid}`). The `hosting-cli proxy` command parses these comments to automatically set up local port forwarding.

### CLI Tunnel Tool (`hosting-cli`)

A standalone binary that establishes userspace WireGuard tunnels without requiring root or kernel modules (uses `golang.zx2c4.com/wireguard/tun/netstack`).

**Workflow:**
1. Create a WireGuard peer in the control panel
2. Download the `.conf` file
3. `hosting-cli import peer.conf -tenant <tenant-id>` — saves profile with tenant association
4. `hosting-cli proxy` — establishes tunnel and proxies MySQL/Valkey to localhost

**Multi-tenant support:** Profiles are stored with tenant IDs. `hosting-cli use <name>` switches between tenants. All commands default to the active profile but accept `-profile` to override.

See `docs/hosting-cli.md` for full documentation.

### Feature Gate

WireGuard peers require a subscription with the `wireguard` module.

## Authorization

- Egress rules use `network:read/write/delete` scopes
- WireGuard peers use `wireguard:read/write/delete` scopes
