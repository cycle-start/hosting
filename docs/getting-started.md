# Getting Started

## Prerequisites

- Go 1.26+
- Docker Desktop (or Docker Engine with Compose)
- [just](https://github.com/casey/just) command runner
- [goose](https://github.com/pressly/goose) for database migrations
- protoc 3.21+ (only if modifying gRPC definitions)
- Terraform + libvirt provider (for VM-based nodes)

## Quick Start

### 1. Start infrastructure and control plane

```bash
just dev
```

This starts the Docker Compose control plane (PostgreSQL, Temporal, HAProxy, MySQL, PowerDNS, Ceph, etc.), runs database migrations, and starts core-api + worker.

### 2. Create VMs and bootstrap a cluster

```bash
just dev-vm
```

This runs `just dev` then provisions VMs via Terraform/libvirt and registers them with the platform using `hostctl cluster apply`.

### 3. Create an API key

```bash
just create-api-key admin
```

This connects directly to the core database, creates a hashed API key, and prints the plaintext key once. Save it — you'll need it for the admin UI and authenticated API requests.

To enable authentication, set `AUTH_ENABLED=true` on the core-api service. All `/api/v1/*` endpoints will then require the `X-API-Key` header.

### 4. Seed test data

```bash
go run ./cmd/hostctl seed -f seeds/dev-tenants.yaml
```

This creates:
- A DNS zone (`example.com`)
- A tenant (`acme-corp`) on the web shard
- A webroot with PHP 8.5 runtime
- FQDNs (`acme.hosting.localhost`, `www.acme.hosting.localhost`)
- A MySQL database with a user
- A Valkey (Redis) instance with a user
- An email account

### 5. Verify

```bash
# List tenants (with auth)
curl -s -H "X-API-Key: hst_..." http://localhost:8090/api/v1/tenants | jq

# Open admin UI
open http://localhost:3001

# Check Temporal workflows
open http://localhost:8080
```

### 6. Run e2e tests

```bash
just test-e2e
```

## Teardown

```bash
# Destroy VMs
just vm-down

# Stop control plane, remove volumes (clean slate)
just down-clean
```

## Resetting the database

If migrations have changed since your database was created (e.g. columns added to existing migration files), reset and re-migrate:

```bash
just reset-db && just migrate
```

This drops all tables and re-applies all migrations from scratch. All data will be lost.

## Rebuilding after code changes

```bash
just rebuild core-api          # Rebuild and restart core-api
just rebuild worker            # Rebuild and restart worker
```

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
seeds/               Test data seed YAML files
migrations/
  core/              Core database migrations
  service/           Service database migrations
docker/              Dockerfiles and container configs (control plane)
docs/                Documentation
tests/e2e/           End-to-end tests (require VMs)
```

## Architecture overview

The platform uses a desired-state / actual-state model:

1. **core-api** accepts REST requests and writes desired state to the core database
2. **worker** picks up Temporal workflows that converge actual state to match
3. **node-agent** runs on each VM node as a Temporal worker, receiving tasks via node-specific task queues

Hierarchy: Region > Cluster > Shard > Node. Tenants are assigned to shards. Nodes are VMs provisioned by Terraform/libvirt and registered with the platform via `hostctl cluster apply`.

## Useful commands

```bash
just --list                    # Show all available commands
just create-api-key admin      # Create an API key
just test                      # Run unit tests
just lint                      # Run linter
just log core-api              # Tail logs for a service
just ps                        # Show running services
just db-core                   # Connect to core database
just migrate-status            # Check migration status
just dev-admin                 # Start admin UI dev server (hot reload)
just vm-ssh web-1-node-0       # SSH into a VM
just lb-show                   # Show HAProxy map entries
```
