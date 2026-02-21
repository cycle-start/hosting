# Ansible Configuration Management

## Overview

Ansible handles all role-specific software installation and system configuration on VMs. This replaces the previous approach of baking everything into 9 separate Packer golden images.

**Provisioning pipeline:**

```
Packer (once)  → single base image (Ubuntu + HWE kernel)
Terraform      → creates VMs from base image (COW clone)
Cloud-init     → node identity, secrets, per-node config
Ansible        → role-specific software + system config
Node-agent     → tenant lifecycle
```

**What each layer does:**

| Layer | Responsibility | When it runs |
|-------|---------------|--------------|
| Packer | Minimal Ubuntu + HWE kernel + python3 | Once, on image rebuild |
| Terraform | VM creation, networking, cloud-init ISOs | On infrastructure changes |
| Cloud-init | Node identity (`/etc/default/node-agent`), SSH keys, secrets, service-specific bootstrap | First boot only |
| Ansible | All packages, configs, systemd units, binaries | On demand (`just ansible-*`) |
| Node-agent | Tenant users, nginx configs, PHP-FPM, databases, etc. | Continuously via Temporal |

## Quick Reference

```bash
# Deploy node-agent to all nodes (fastest daily workflow)
just deploy-node-agent

# Deploy Vector config changes
just deploy-vector

# Full convergence of all nodes
just ansible-all

# Target a specific role group
just ansible-role web
just ansible-role web --tags php

# Target a single node
just ansible-node 10.10.10.10

# Bootstrap (before API is running, uses static inventory)
just ansible-bootstrap

# Full rebuild from scratch
just vm-rebuild
```

## Directory Structure

```
ansible/
  ansible.cfg                    # SSH, pipelining, fact caching
  site.yml                       # Master playbook (role → host group mapping)
  inventory/
    hosting.py                   # Dynamic inventory (API-backed)
    static.ini                   # Static fallback for bootstrap
    group_vars/
      all.yml                    # Shared vars (Loki endpoints, binary paths)
      web.yml                    # PHP versions and extensions
  roles/
    node_agent/                  # Binary + systemd unit (all node VMs)
    vector/                      # Log shipper + per-role configs
    ssh_hardening/               # SSH config (PasswordAuth=no, tenant includes)
    php/                         # PHP versions + extensions (web)
    nginx/                       # Web server (web, dbadmin)
    composer/                    # PHP package manager (web)
    supervisor/                  # Process manager for daemons (web)
    ceph_client/                 # CephFS mount deps (web)
    mysql/                       # MySQL server + replication config (db)
    powerdns/                    # DNS server (dns)
    valkey_server/               # Valkey + per-instance dirs (valkey)
    stalwart/                    # Mail server binary + systemd (email)
    ceph_server/                 # Full Ceph stack (storage)
    haproxy/                     # Load balancer + map dirs (lb)
    docker/                      # Docker engine (dbadmin)
    cloudbeaver/                 # DB admin UI + proxy (dbadmin)
    k3s/                         # k3s + Helm (controlplane)
```

## Dynamic Inventory

The `hosting.py` script queries the hosting API to discover all nodes:

1. `GET /regions` → for each region
2. `GET /regions/{id}/clusters` → for each cluster
3. `GET /clusters/{id}/nodes` → all nodes with roles

**Groups produced:**
- By role: `web`, `db`, `dns`, `valkey`, `email`, `storage`, `dbadmin`, `lb`
- By cluster: `cluster_{id}`
- By region: `region_{id}`

**Environment variables:**
- `HOSTING_API_KEY` (required) — loaded automatically from `.env` via justfile
- `HOSTING_API_URL` (default: `http://10.10.10.2:8090/api/v1`)

**Testing:**
```bash
cd ansible && HOSTING_API_KEY=$HOSTING_API_KEY python3 inventory/hosting.py --list | jq .
```

The controlplane node is always included from `static.ini` since it doesn't run a node-agent and doesn't appear in the API.

## Common Tasks

### Adding a New PHP Version

1. Edit `ansible/inventory/group_vars/web.yml`:
   ```yaml
   php_versions:
     - "8.3"
     - "8.5"
     - "8.6"   # new
   ```

2. Run:
   ```bash
   just ansible-role web --tags php
   ```

### Updating Node-Agent

```bash
just deploy-node-agent
```

This builds the binary, copies it to all nodes, and restarts the service. Takes ~10 seconds.

### Updating Vector Config

Edit the relevant template in `ansible/roles/vector/templates/`, then:

```bash
just deploy-vector
```

### Patching Software

To update any package (e.g., MySQL, HAProxy, Stalwart):

```bash
# Update a single role group
just ansible-role db --tags mysql

# Or all nodes
just ansible-all
```

### Adding a New Role

1. Create `ansible/roles/<name>/` with `tasks/main.yml`
2. Add the role to the appropriate host group in `ansible/site.yml`
3. Add a Vector config template if the service has logs to ship
4. Update `ansible/inventory/static.ini` if the role maps to a new host group

## Bootstrap vs Day-2

**Bootstrap** (`just ansible-bootstrap`): Used during `vm-rebuild` when the API isn't running yet. Uses `static.ini` which has hardcoded IPs matching Terraform.

**Day-2** (`just ansible-all`, `just deploy-node-agent`, etc.): Uses the dynamic inventory which queries the live API. This is the normal operational mode.
