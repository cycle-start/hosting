# Getting Started

## Prerequisites

- Go 1.26+
- Docker Desktop (or Docker Engine with Compose)
- [just](https://github.com/casey/just) command runner
- [goose](https://github.com/pressly/goose) for database migrations
- protoc 3.21+ (only if modifying gRPC definitions)

## Quick Start

### 1. Start infrastructure and build node images

```bash
just dev-up
```

This starts PostgreSQL, Temporal, the local container registry, and other infrastructure services. It then runs database migrations and builds + pushes node images to the local registry.

### 2. Bootstrap a cluster

```bash
go run ./cmd/hostctl cluster apply -f clusters/dev.yaml
```

This creates:
- A `dev` region
- Node profiles for each role (web, database, dns, valkey)
- A `dev-cluster-1` cluster with one shard per role
- A localhost host machine using the local Docker socket
- Provisions all nodes as Docker containers via the local registry images

The tool waits for the cluster to reach `active` status before printing a summary of shards and nodes.

### 3. Seed test data

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

### 4. Verify

```bash
# List tenants
curl -s http://localhost:8090/api/v1/tenants | jq

# List zones
curl -s http://localhost:8090/api/v1/zones | jq

# Check Temporal workflows
open http://localhost:8080
```

## Teardown

```bash
# Stop everything, remove volumes (clean slate)
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
just build-push-node-images    # Rebuild and push node images to local registry
```

## Project layout

```
cmd/
  core-api/          REST API server
  worker/            Temporal worker (workflows + activities)
  node-agent/        gRPC agent running inside each node
  hostctl/           CLI for cluster bootstrap and test data seeding

internal/
  api/               HTTP handlers, request/response types
  activity/          Temporal activity implementations
  workflow/          Temporal workflow definitions
  agent/             Node agent managers (tenant, webroot, nginx, database, valkey)
  config/            Configuration loading
  core/              Core business logic services
  deployer/          Container deployment (Docker implementation)
  model/             Domain models
  platform/          Shared utilities

clusters/            Cluster definition YAML files
seeds/               Test data seed YAML files
migrations/
  core/              Core database migrations
  service/           Service database migrations
docker/              Dockerfiles and container configs
docs/                Documentation
```

## Architecture overview

The platform uses a desired-state / actual-state model:

1. **core-api** accepts REST requests and writes desired state to the core database
2. **worker** picks up Temporal workflows that converge actual state to match
3. **node-agent** runs inside each node and manages workloads (tenants, webroots, runtimes, databases)

Hierarchy: Region > Cluster > Shard > Node. Tenants are assigned to shards. The deployer (currently Docker) creates node containers on host machines; the node agent is agnostic to how it was deployed.

## Useful commands

```bash
just --list           # Show all available commands
just test             # Run unit tests
just lint             # Run linter
just log core-api     # Tail logs for a service
just ps               # Show running services
just db-core          # Connect to core database
just migrate-status   # Check migration status
```
