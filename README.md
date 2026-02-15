# Hosting Platform

A distributed, multi-tenant web hosting platform built in Go. Manages the full stack — web hosting, DNS, MySQL databases, Valkey (Redis), S3 object storage, email, backups, and SSL certificates — across fleets of Linux VMs with automated provisioning and convergence.

Designed for scale: millions of tenants across multiple regions, clusters, and brands, all driven by a desired-state model and Temporal workflows.

## Architecture

```
                 REST API
                    |
               +----+----+
               | core-api |  ── writes desired state to PostgreSQL
               +----+----+
                    |
              +-----+------+
              |   worker   |  ── Temporal workflows converge actual state
              +-----+------+
                    |
     +--------------+--------------+
     |              |              |
 +---+---+     +---+---+     +---+---+
 | node  |     | node  |     | node  |   ── Temporal workers on each VM
 | agent |     | agent |     | agent |      (task queue: node-{uuid})
 +-------+     +-------+     +-------+
```

**Hierarchy:** Region > Cluster > Shard > Node. Tenants are assigned to shards. Shards have roles (web, database, dns, email, valkey, s3). Nodes are VMs provisioned by Terraform/libvirt with Packer golden images.

**Control plane:** k3s single-node cluster running core-api, worker, admin UI, Temporal, PostgreSQL, Loki, Grafana, Prometheus.

## What It Does

**Web Hosting** — PHP, Node.js, Python, Ruby, and static sites with nginx, per-tenant Linux users, CephFS shared storage, PHP-FPM socket activation, and long-running worker processes via supervisord.

**DNS** — PowerDNS with brand-aware NS/SOA records, auto-created A/AAAA records on FQDN binding, and auto MX/SPF for email.

**MySQL Databases** — Per-tenant databases with GTID-based replication, cross-shard migration (dump/restore), and granular user privileges.

**Valkey (Redis)** — Managed instances with ACL-based users, configurable memory limits and eviction policies, RDB dump/import for migration.

**S3 Object Storage** — Ceph RADOS Gateway with per-tenant buckets, public/private access policies, quotas, and access key management.

**Email** — Stalwart (SMTP, IMAP, JMAP) with accounts, aliases, forwards (Sieve), and vacation auto-replies. Auto DNS record creation.

**SSL Certificates** — Let's Encrypt via HTTP-01 ACME with automatic renewal, plus custom certificate upload.

**Backups** — Web (tar.gz) and MySQL (.sql.gz) backups with restore and automated cleanup.

**Multi-Brand Isolation** — Brands scope all resources. Each brand defines its own NS hostnames, base domain, and hostmaster. API keys are brand-scoped.

**Load Balancing** — HAProxy with runtime map updates (no reload for FQDN changes), consistent hashing on Host header within shards.

**Observability** — Prometheus metrics, Grafana dashboards, Loki log aggregation, Vector/Alloy log shipping, alerting with runbooks.

**Admin UI** — React SPA with 25+ pages: dashboard, resource management, log viewer, forms with inline sub-resource creation.

## Quick Start

```bash
just packer-all                                        # Build VM golden images
just dev-k3s                                           # Create VMs + deploy control plane
just migrate                                           # Run database migrations
just create-api-key admin                              # Create API key (save to .env)
go run ./cmd/hostctl seed -f seeds/dev-tenants.yaml    # Seed test data
just test-e2e                                          # Verify everything works
```

See [Getting Started](docs/getting-started.md) for the full walkthrough including Windows/WSL2 networking setup.

## Documentation

### Setup & Operations

| | |
|---|---|
| [Getting Started](docs/getting-started.md) | Prerequisites, setup, teardown, rebuilding |
| [Local Networking](docs/local-networking.md) | WSL2 routing, hosts file, `just forward` |
| [Observability](docs/observability.md) | Prometheus, Grafana, Loki, alerting |

### Platform Concepts

| | |
|---|---|
| [Tenants](docs/tenants.md) | Tenant lifecycle, suspension, migration |
| [Brands](docs/brands.md) | Multi-brand isolation, API key scoping |
| [Resource Naming](docs/resource-naming.md) | Auto-generated prefixed names (`t_`, `db_`, `kv_`, ...) |
| [Authorization](docs/authorization.md) | API keys, scopes, brand-based access control |
| [Convergence](docs/convergence.md) | Desired-state model, shard convergence, reconciliation |
| [OIDC](docs/oidc.md) | OpenID Connect provider for external auth |

### Services

| | |
|---|---|
| [Webroots](docs/webroots.md) | Web hosting, runtimes, FQDNs, nginx |
| [DNS](docs/dns.md) | PowerDNS, zones, records, auto-DNS |
| [Databases](docs/databases.md) | MySQL, replication, migration, users |
| [Valkey](docs/valkey.md) | Managed Redis, ACL users, migration |
| [S3 Storage](docs/s3-storage.md) | Ceph RGW, buckets, access keys, quotas |
| [Email](docs/email.md) | Stalwart, accounts, aliases, forwards |
| [Cron Jobs](docs/cron-jobs.md) | Scheduled tasks, distributed locking, logging |
| [Backups](docs/backups.md) | Web and database backups, restore |
| [Load Balancing](docs/load-balancing.md) | HAProxy, runtime map, consistent hashing |

### Reference

| | |
|---|---|
| [STATUS.md](STATUS.md) | Full feature inventory and roadmap |
| [API Docs](http://api.hosting.test/docs) | OpenAPI / Swagger (requires running cluster) |

## Tech Stack

| Component | Technology |
|---|---|
| API + Worker + Agent | Go 1.26 |
| Orchestration | Temporal |
| Core Database | PostgreSQL |
| Admin UI | React, TypeScript, Vite, Tailwind, shadcn/ui |
| DNS | PowerDNS |
| Web Server | nginx + PHP-FPM / Node.js / Python / Ruby |
| Databases | MySQL (GTID replication) |
| Cache | Valkey (Redis fork) |
| Object Storage | Ceph RADOS Gateway |
| Email | Stalwart |
| Load Balancer | HAProxy |
| Shared Storage | CephFS |
| Monitoring | Prometheus, Grafana, Loki, Vector, Alloy |
| Infrastructure | Terraform, Packer, libvirt/KVM, k3s, Helm |
