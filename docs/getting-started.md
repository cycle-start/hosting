# Getting Started

## Prerequisites

- Go 1.26+
- Docker Desktop (for building images)
- [just](https://github.com/casey/just) command runner
- [goose](https://github.com/pressly/goose) for database migrations
- [Packer](https://www.packer.io/) for building VM golden images
- [Terraform](https://www.terraform.io/) + [libvirt provider](https://github.com/dmacvicar/terraform-provider-libvirt) for VM provisioning
- [Helm](https://helm.sh/) for k3s deployments
- protoc 3.21+ (only if modifying gRPC definitions)

## Quick Start

### 1. Build golden image and create VMs

```bash
just packer-base
just dev-k3s
```

Or as a single command (also rebuilds the base image and destroys existing VMs first):

```bash
just vm-rebuild
```

`packer-base` builds a single QEMU/KVM base golden image (minimal Ubuntu). Ansible handles installing role-specific software during provisioning.

`dev-k3s` creates VMs and sets up the full environment (assumes base image already exists):
1. Creates VMs via Terraform/libvirt
2. Waits for SSH on all VMs
3. Provisions all VMs via Ansible (static inventory, since the API isn't up yet)
4. Re-runs cloud-init (so runcmd succeeds after Ansible installs software)
5. Waits for k3s, then fetches the kubeconfig
6. Builds Docker images and deploys infrastructure + platform to k3s
7. Runs `bootstrap` (migrate, create API keys, register cluster, seed test data)

### 2. Enable Windows access (WSL2 only)

The VMs run on a libvirt network (`10.10.10.0/24`) inside WSL2. Windows needs IP forwarding rules and a route to reach them.

**In WSL2:**

```bash
just forward
```

This enables IP forwarding from Windows through WSL2 to the VM network. It prints the Windows commands you need to run.

**In PowerShell (as Administrator):**

```powershell
# Add route to VM network (use the WSL2 IP printed by `just forward`)
route add 10.10.10.0 mask 255.255.255.0 <WSL2_IP>
```

**In `C:\Windows\System32\drivers\etc\hosts` (as Administrator):**

```
10.10.10.2  admin.massive-hosting.com api.massive-hosting.com mcp.massive-hosting.com temporal.massive-hosting.com dbadmin.massive-hosting.com
```

After this, all services are accessible from the Windows browser. See [Local Networking](local-networking.md) for details.

### 3. Bootstrap (already done by `dev-k3s`)

`dev-k3s` runs `bootstrap` automatically at the end. If you need to re-run it (e.g. after a DB reset):

```bash
just bootstrap
```

This runs the full sequence: wait for Postgres, migrate, create API keys, wait for core-api, register cluster, and seed data:

1. **`wait-db`** — waits for PostgreSQL to accept connections.
2. **`migrate`** — runs goose migrations for both core and PowerDNS databases.
3. **`create-dev-key`** — creates a well-known dev API key (`hst_dev_e2e_test_key_00000000`) used by seed configs, `hostctl`, and e2e tests.
4. **`create-agent-key`** — creates the agent API key (`hst_agent_key_000000000000000`) used by the LLM incident agent.
5. **`wait-api`** — waits for core-api to be healthy.
6. **`cluster-apply`** — registers the cluster topology (regions, clusters, shards, nodes) via `hostctl cluster apply`.
7. **`seed`** — creates test data: a brand ("Acme Hosting"), DNS zone, tenant with webroots, databases, Valkey, S3, email, and a Laravel fixture app.

You can also run each step individually: `just migrate`, `just create-dev-key`, `just cluster-apply`, `just seed`.

### 4. Verify

```bash
# Admin UI
open http://admin.massive-hosting.com

# Core API
curl -s -H "Authorization: Bearer hst_dev_e2e_test_key_00000000" https://api.massive-hosting.com/api/v1/tenants | jq

# Temporal UI
open http://temporal.massive-hosting.com

# Tenant site (after seeding — add host entry for 10.10.10.70)
curl -k https://acme.mhst.io
```

### 5. Run e2e tests

```bash
just test-e2e
```

The e2e tests use the well-known dev API key by default — no `HOSTING_API_KEY` env var needed.

## Teardown

```bash
# Destroy VMs
just vm-down

# Remove forwarding rules
just forward-stop
```

## Resetting the database

If migrations have changed since your database was created (e.g. columns added to existing migration files), reset and re-bootstrap:

```bash
just reset-db
just bootstrap
```

`bootstrap` runs the full sequence: migrate, create API keys, register cluster, and seed test data.

If VMs are in a bad state and `reset-db` fails, do a full rebuild instead (see below).

## Rebuilding after code changes

**Control plane only** (API, worker, admin UI — no node-agent changes):

```bash
just vm-deploy
```

This rebuilds all Docker images, imports them into k3s, and restarts the deployments. If you also reset the database, follow up with `just reset-db && just bootstrap`.

**Node agent only** (anything under `internal/agent/`, `internal/activity/`):

```bash
just deploy-node-agent
```

This builds the node-agent binary and deploys it to all VMs via Ansible (uses the dynamic inventory, so the API must be running with nodes registered).

**Full rebuild** (control plane + node agent + database reset):

```bash
just rebuild
```

This single command handles the full dependency chain: deploy control plane images to k3s, wipe DB, run migrations, create API keys, wait for core-api to be healthy, register cluster topology, deploy updated node-agent to all VMs (via dynamic Ansible inventory), and seed test data.

**Destroying and recreating VMs** (nuclear option):

```bash
just vm-rebuild
```

This rebuilds the base golden image, destroys all VMs, then runs `dev-k3s` (create VMs, Ansible provision, deploy control plane, full bootstrap).

## Project layout

```
cmd/
  core-api/          REST API server (also: create-api-key subcommand)
  worker/            Temporal worker (workflows + activities)
  node-agent/        Temporal worker running on each VM node
  admin-ui/          Admin UI reverse proxy + SPA server
  dbadmin-proxy/     Reverse proxy for database admin UIs
  hostctl/           CLI for cluster bootstrap and test data seeding

internal/
  api/               HTTP handlers, request/response types
  activity/          Temporal activity implementations
  workflow/          Temporal workflow definitions
  agent/             Node agent managers (tenant, webroot, nginx, database, valkey)
  config/            Configuration loading
  core/              Core business logic services
  hostctl/           Cluster bootstrap and seed logic
  model/             Domain models
  platform/          Shared utilities

terraform/           Terraform configs for VM provisioning
packer/              Packer configs for golden images
seeds/               Test data seed YAML files
migrations/
  core/              Core database migrations
  powerdns/          PowerDNS database migrations
deploy/
  k3s/               Kubernetes manifests for infrastructure services
  helm/hosting/      Helm chart for platform services
docker/              Dockerfiles (used to build images for k3s)
docs/                Documentation
tests/e2e/           End-to-end tests (require VMs)
```

## Architecture overview

The platform uses a desired-state / actual-state model:

1. **core-api** accepts REST requests and writes desired state to the core database
2. **worker** picks up Temporal workflows that converge actual state to match
3. **node-agent** runs on each VM node as a Temporal worker, receiving tasks via node-specific task queues

Hierarchy: Region > Cluster > Shard > Node. Tenants are assigned to shards. Nodes are VMs provisioned by Terraform/libvirt and registered with the platform via `hostctl cluster apply`.

The control plane runs on a k3s single-node cluster (controlplane VM at `10.10.10.2`). All pods use `hostNetwork: true` so services bind directly to the VM's IP.

## Useful commands

```bash
just --list                    # Show all available commands
just test                      # Run unit tests
just test-e2e                  # Run e2e tests
just lint                      # Run linter
just vm-deploy                 # Rebuild and redeploy to k3s
just bootstrap                 # Migrate, create keys, register cluster, seed data
just cluster-apply             # Register cluster topology only
just seed                      # Seed test data only
just vm-pods                   # Show k3s pod status
just vm-log hosting-core-api   # Tail logs for a deployment
just vm-ssh web-1-node-0       # SSH into a VM
just lb-show                   # Show HAProxy map entries
just forward                   # Enable Windows -> VM networking
just dev-admin                 # Start admin UI dev server (hot reload)
```
