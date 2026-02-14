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

### 1. Build golden images

```bash
just packer-all
```

This builds QEMU/KVM golden images for all VM roles (controlplane, web, db, dns, valkey, storage, dbadmin) using Packer. Each image is a minimal Ubuntu with role-specific software pre-installed.

### 2. Create VMs and deploy the control plane

```bash
just dev-k3s
```

This runs the full bootstrap:
1. Creates all VMs via Terraform/libvirt (controlplane + node agents)
2. Fetches the k3s kubeconfig
3. Builds Docker images and imports them into k3s
4. Deploys infrastructure (PostgreSQL, Temporal, HAProxy, Valkey, registry) and the platform (core-api, worker, admin-ui) to k3s
5. Registers the cluster and nodes with the platform via `hostctl cluster apply`

### 3. Enable Windows access

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
10.10.10.2  admin.hosting.test api.hosting.test mcp.hosting.test temporal.hosting.test dbadmin.hosting.test
```

After this, all services are accessible from the Windows browser. See [Local Networking](local-networking.md) for details.

### 4. Create an API key

```bash
KUBECONFIG=~/.kube/k3s-config kubectl exec deploy/hosting-core-api -- \
  /core-api create-api-key --name admin
```

This creates a hashed API key in the database and prints the plaintext key once. Save it — you'll need it for the admin UI and API requests.

### 5. Seed test data

```bash
go run ./cmd/hostctl seed -f seeds/dev-tenants.yaml
```

This creates:
- A brand (`acme-brand`) with DNS nameservers under `hosting.test`
- A DNS zone (`hosting.test`)
- A tenant (`acme-corp`) on the web shard
- A webroot with PHP 8.5 runtime
- FQDNs (`acme.hosting.test`, `www.acme.hosting.test`)
- A MySQL database with a user
- A Valkey (Redis) instance with a user
- S3 bucket and email accounts

### 6. Verify

```bash
# Admin UI
open http://admin.hosting.test

# Core API
curl -s -H "X-API-Key: hst_..." http://api.hosting.test/api/v1/tenants | jq

# Temporal UI
open http://temporal.hosting.test

# Tenant site (after seeding — add host entry for 10.10.10.2)
curl http://acme.hosting.test
```

### 7. Run e2e tests

```bash
just test-e2e
```

## Teardown

```bash
# Destroy VMs
just vm-down

# Remove forwarding rules
just forward-stop
```

## Resetting the database

If migrations have changed since your database was created (e.g. columns added to existing migration files), reset and re-migrate. The easiest way is to delete the controlplane VM's persistent volumes:

```bash
KUBECONFIG=~/.kube/k3s-config kubectl delete pvc --all
KUBECONFIG=~/.kube/k3s-config kubectl rollout restart statefulset
```

Wait for pods to restart, then re-seed.

## Rebuilding after code changes

```bash
just vm-deploy
```

This rebuilds all Docker images, imports them into k3s, and restarts the deployments.

## Project layout

```
cmd/
  core-api/          REST API server (also: create-api-key subcommand)
  worker/            Temporal worker (workflows + activities)
  node-agent/        Temporal worker running on each VM node
  admin-ui/          Admin UI reverse proxy + SPA server
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
  service/           Service database migrations
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
just lint                      # Run linter
just vm-deploy                 # Rebuild and redeploy to k3s
just vm-pods                   # Show k3s pod status
just vm-log hosting-core-api   # Tail logs for a deployment
just vm-ssh web-1-node-0       # SSH into a VM
just lb-show                   # Show HAProxy map entries
just forward                   # Enable Windows -> VM networking
just dev-admin                 # Start admin UI dev server (hot reload)
```
