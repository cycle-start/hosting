# Hosting Platform - Status & Roadmap

## Current Status Summary

**Build:** Compiles clean, `go vet` passes, all test packages pass.
**Infrastructure:** Docker Compose stack with core-db, service-db, temporal, temporal-ui, platform-valkey, mysql, powerdns, ceph, haproxy, local registry, stalwart, prometheus, grafana, loki, promtail, core-api, worker.
**Migrations:** All core + service DB migrations applied.
**CLI:** `hostctl cluster apply` bootstraps a full cluster; `hostctl seed` populates tenant data; `hostctl converge-shard` triggers manual shard convergence.
**Integration tests:** None exist yet (`tests/e2e/` is empty).

---

## What's Implemented

### Core API (REST)

Full CRUD REST API at `localhost:8090/api/v1`:

| Resource         | Endpoints                                        | Async |
|------------------|--------------------------------------------------|-------|
| Platform Config  | GET/PUT `/platform/config`                       | No    |
| Regions          | CRUD `/regions`, runtimes sub-resource            | No    |
| Clusters         | CRUD `/regions/{id}/clusters`, provision/decommission | Yes |
| Cluster LB Addrs | CRUD `/clusters/{id}/lb-addresses`               | No    |
| Shards           | CRUD `/clusters/{id}/shards`, converge            | Yes   |
| Host Machines    | CRUD `/clusters/{id}/hosts`                      | No    |
| Node Profiles    | CRUD `/node-profiles`                            | No    |
| Nodes            | CRUD `/clusters/{id}/nodes`, provision/decommission | Yes |
| Node Deployments | GET `/nodes/{id}/deployment`, list by host        | No    |
| Tenants          | CRUD + suspend/unsuspend/migrate `/tenants`       | Yes   |
| Webroots         | CRUD `/tenants/{id}/webroots`                    | Yes   |
| FQDNs            | Create/delete `/webroots/{id}/fqdns`             | Yes   |
| Certificates     | List/upload `/fqdns/{id}/certificates`           | Yes   |
| Zones            | CRUD `/zones`                                    | Yes   |
| Zone Records     | CRUD `/zones/{id}/records`                       | Yes   |
| Databases        | CRUD + reassign `/tenants/{id}/databases`        | Yes   |
| Database Users   | CRUD `/databases/{id}/users`                     | Yes   |
| Valkey Instances | CRUD + reassign `/tenants/{id}/valkey-instances` | Yes   |
| Valkey Users     | CRUD `/valkey-instances/{id}/users`              | Yes   |
| Email Accounts   | CRUD `/fqdns/{id}/email-accounts`                | Yes   |

Infrastructure resources (regions, clusters, shards, nodes, hosts, profiles) use synchronous CRUD.
Tenant-level resources are async: API returns 202, Temporal workflow handles provisioning.

### Temporal Workflows

**Resource lifecycle:**
- Tenant: create, update, suspend, unsuspend, delete, migrate
- Webroot: create, update, delete
- FQDN: bind (auto-DNS + auto-LB-map + optional LE cert), unbind
- Zone: create (SOA + NS records), delete
- Zone Record: create, update, delete
- Database: create, delete
- Database User: create, update, delete
- Valkey Instance: create, delete
- Valkey User: create, update, delete
- Certificate: provision LE (stub ACME), upload custom, renew (stub), cleanup (stub)
- Email Account: create, delete (schema-only, Stalwart integration stubbed)

**Infrastructure lifecycle:**
- ProvisionClusterWorkflow: creates shards + nodes from cluster spec, deploys infrastructure containers (HAProxy, service DB, Valkey), provisions all node containers, configures HAProxy backends, runs smoke test
- DecommissionClusterWorkflow: tears down all nodes and infrastructure containers
- ProvisionNodeWorkflow: selects host, pulls image, creates Docker container, waits for health, sets gRPC address, triggers shard convergence
- DecommissionNodeWorkflow: stops + removes container, updates deployment record
- RollingUpdateWorkflow: updates all nodes in a shard to a new image one at a time
- ConvergeShardWorkflow: pushes all existing resources (tenants, webroots, databases, valkey instances + their users) to every node in a shard, role-aware (web/database/valkey)

Status progression: `pending -> provisioning -> active` (or `failed`/`deleted`).

### Node Agent (gRPC)

Runs privileged inside each node container:

- **TenantManager:** Linux user accounts (useradd/userdel/usermod), directory structure, SFTP setup
- **WebrootManager:** Webroot directories under tenant home
- **NginxManager:** Per-webroot nginx server blocks from templates (PHP/Node/Python/Ruby/Static), SSL cert installation, config test + reload
- **DatabaseManager:** MySQL CREATE/DROP DATABASE, CREATE/DROP USER, GRANT
- **ValkeyManager:** Valkey instance lifecycle (config + systemd template units), ACL user management via valkey-cli
- **Runtime managers:** PHP-FPM pool configs + systemctl reload, Node.js proxy, Python (gunicorn), Ruby (puma), Static

### Docker Deployer

Full Docker SDK integration for node container lifecycle:
- Image pulling with digest tracking
- Container creation with env, volumes, ports, resource constraints, health checks, network attachment
- TLS support for remote Docker hosts (client certs, CA cert)
- Ephemeral port mapping (Docker picks host ports)
- Docker network attachment (for development `hosting_default` network)

### Node Container Images

Dockerfiles for each role, built and pushed to local registry:
- `web-node`: Ubuntu 24.04 + nginx + PHP 8.5 + Node/Python/Ruby + node-agent
- `db-node`: Ubuntu 24.04 + MySQL 8.4 LTS + node-agent
- `dns-node`: PowerDNS + node-agent
- `valkey-node`: Valkey 8 + node-agent

### DNS (PowerDNS)

- Service DB stores PowerDNS domains/records tables
- Workflows auto-create A/AAAA records when binding FQDNs (if zone exists)
- Platform-managed vs user-managed record tracking
- SOA + NS records auto-created on zone creation from platform config

### Load Balancing (HAProxy)

- Runtime map file (`fqdn-to-shard.map`) updated via socat without reload
- `SetLBMapEntry`/`DeleteLBMapEntry` activities update mappings when FQDNs are bound/unbound
- Consistent hashing on Host header within shard backends
- HAProxy deployed as infrastructure container during cluster provisioning

### CLI Tooling (`hostctl`)

- `hostctl cluster apply -f <yaml>`: bootstraps full cluster from declarative YAML (region, cluster, LB addresses, node profiles, host machines, provision)
- `hostctl seed -f <yaml>`: seeds tenant data (zones, tenants, webroots, FQDNs, databases, valkey instances, email accounts)
- `hostctl converge-shard <shard-id>`: triggers manual shard convergence

### Unit Tests

Full coverage across all packages:
- Workflow tests (mock activities, verify state transitions)
- Handler tests (HTTP request/response, validation, chi routing)
- Activity tests (mock DB/gRPC calls)
- Agent tests (mock exec commands, template rendering)
- Runtime manager tests
- Request validation, response formatting, config loading, model helpers

---

## What's Missing

### Priority 1: Email (v2)

Email accounts have schema + API + workflow stubs, but no real backend:
- Stalwart integration (SMTP/IMAP/JMAP)
- Email aliases, forwards, auto-reply
- Domain-level email configuration
- Spam filtering policy management

### Priority 2: MySQL Replication Management

DB shards use replication pairs, but there's no automation for:
- Setting up replication between nodes in a shard
- Monitoring replication lag
- Handling failover (ProxySQL VIP switching)
- Promoting a replica to primary

### Priority 3: CephFS Integration

Web nodes need CephFS mounted at `/var/www/storage` for shared tenant files:
- Ceph client configuration distribution (ceph.conf, keyring)
- Mount management in node-agent
- Health monitoring of Ceph mounts

### Priority 4: SSL/ACME

Let's Encrypt certificate provisioning is stubbed:
- Need real ACME challenge implementation (HTTP-01 or DNS-01)
- Certificate storage workflow works but uses placeholder PEM data
- Renewal and cleanup cron workflows are stubs

### Priority 5: Integration & E2E Tests

`tests/e2e/` directory exists but is empty:
- Full lifecycle API integration tests
- Workflow integration tests against real Temporal + databases
- Node-agent integration tests in Docker

### Priority 6: Monitoring & Observability

Prometheus/Grafana/Loki are in docker-compose but not wired up:
- Node health reporting back to core
- Tenant resource usage metrics
- Service health checks (nginx, PHP-FPM, MySQL, CephFS)
- Alerting on failures

### Priority 7: Agent-Side Reconciliation

ConvergeShardWorkflow handles Temporal-driven convergence, but the agent itself has no:
- Startup self-check (query core DB, converge local state)
- Periodic drift detection
- Health status reporting
