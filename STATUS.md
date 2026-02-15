# Hosting Platform - Status & Roadmap

## Current Status Summary

**Build:** Go 1.26, compiles clean, `go vet` passes, all test packages pass.
**Infrastructure:** k3s control plane (core-api, worker, admin-ui, MCP server, Temporal, PostgreSQL, Loki, Grafana, Prometheus, Alloy). Nodes run on VMs provisioned by Terraform/libvirt with Packer golden images.
**Dev Environment:** 9 VMs (controlplane + 2 web + 1 db + 1 dns + 1 valkey + 1 storage + 1 dbadmin + 1 lb) on libvirt, accessible at `*.hosting.test`.
**CLI:** `hostctl cluster apply` bootstraps infrastructure; `hostctl seed` populates tenant data; `hostctl converge-shard` triggers convergence. Auto-loads `.env` for API key.

---

## What's Implemented

### Core API (REST)

Full CRUD REST API at `api.hosting.test/api/v1` with OpenAPI docs at `/docs`.

**Authentication & Authorization:**
- API key auth (`X-API-Key` header) with fine-grained scopes (`resource:action` format)
- Brand-based access control (keys authorized for specific brands or `*` for platform admin)
- All mutations audit-logged with sanitized request bodies (passwords/keys redacted)

| Resource | Endpoints | Async | Notes |
|---|---|---|---|
| Dashboard | GET `/dashboard/stats` | No | Platform-wide resource counts |
| Search | GET `/search` | No | Cross-resource substring search |
| Audit Logs | GET `/audit-logs` | No | Mutation history with API key tracking |
| Platform Config | GET/PUT `/platform/config` | No | Base domain, NS servers, OIDC issuer |
| API Keys | CRUD `/api-keys` | No | Scopes, brand access; key shown once |
| Brands | CRUD `/brands`, cluster mappings | No | Multi-brand isolation boundary |
| Regions | CRUD `/regions`, runtimes sub-resource | No | |
| Clusters | CRUD `/regions/{id}/clusters` | No | |
| Cluster LB Addrs | CRUD `/clusters/{id}/lb-addresses` | No | |
| Shards | CRUD `/clusters/{id}/shards`, converge, retry | Yes | Roles: web, database, dns, email, valkey, s3 |
| Nodes | CRUD `/clusters/{id}/nodes` | No | UUID-based Temporal task queue routing |
| Tenants | CRUD, suspend/unsuspend/migrate/retry `/tenants` | Yes | Resource summary, login sessions, retry-failed |
| Webroots | CRUD `/tenants/{id}/webroots`, retry | Yes | PHP/Node/Python/Ruby/Static runtimes |
| FQDNs | CRUD `/webroots/{id}/fqdns`, retry | Yes | Auto-DNS + auto-LB-map + optional LE cert |
| Certificates | List/upload `/fqdns/{id}/certificates`, retry | Yes | PEM upload, LE provisioning |
| SSH Keys | CRUD `/tenants/{id}/ssh-keys`, retry | Yes | SSH public keys for SFTP/SSH access |
| Zones | CRUD `/zones`, tenant reassign, retry | Yes | Brand-scoped DNS zones |
| Zone Records | CRUD `/zones/{id}/records`, retry | Yes | A/AAAA/CNAME/MX/TXT/NS/etc. |
| Databases | CRUD `/tenants/{id}/databases`, migrate, retry | Yes | MySQL; charset, collation |
| Database Users | CRUD `/databases/{id}/users`, retry | Yes | Privileges (all/read-only) |
| Valkey Instances | CRUD `/tenants/{id}/valkey-instances`, migrate, retry | Yes | Managed Redis; eviction, max memory |
| Valkey Users | CRUD `/valkey-instances/{id}/users`, retry | Yes | ACL-based access |
| S3 Buckets | CRUD `/tenants/{id}/s3-buckets`, retry | Yes | Ceph RGW; public/private, quotas |
| S3 Access Keys | CRUD `/s3-buckets/{id}/access-keys`, retry | Yes | 20-char ID, 40-char secret; shown once |
| Email Accounts | CRUD `/fqdns/{id}/email-accounts`, retry | Yes | Stalwart SMTP/IMAP/JMAP |
| Email Aliases | CRUD `/email-accounts/{id}/aliases`, retry | Yes | |
| Email Forwards | CRUD `/email-accounts/{id}/forwards`, retry | Yes | External forwarding with keep-copy |
| Email Auto-Replies | GET/PUT/DELETE `/email-accounts/{id}/autoreply`, retry | Yes | Vacation/out-of-office |
| Backups | CRUD `/tenants/{id}/backups`, restore, retry | Yes | Web (tar.gz) and MySQL (.sql.gz) |
| Logs | GET `/logs` | No | Loki proxy for platform log querying |

**OIDC Provider:**
- Discovery (`.well-known/openid-configuration`), JWKS, authorize, token endpoints
- Client management for external auth integrations (e.g., CloudBeaver)
- Tenant login sessions for authorization code flow

**MCP Server:**
- Dynamic tool generation from OpenAPI spec, grouped by domain (infrastructure, tenants, web, databases, dns, email, storage, platform)
- Available at `mcp.hosting.test`

**Response patterns:**
- List endpoints: `{items: [...], next_cursor, has_more}` with search/sort/status filtering
- Async operations: 202 Accepted, Temporal workflow handles provisioning
- Status progression: `pending -> provisioning -> active` (or `failed` with `status_message`)
- All async resources support `POST /{resource}/{id}/retry` to re-trigger failed provisioning

### Temporal Workflows

**Resource lifecycle (all with retry support):**
- Tenant: create, update, suspend, unsuspend, delete, migrate (cross-shard)
- Webroot: create, update, delete
- FQDN: bind (auto-DNS + auto-LB-map + optional LE cert), unbind
- Zone: create (brand-aware SOA + NS records), delete
- Zone Record: create, update, delete
- Database: create, delete, migrate (dump/restore across shards)
- Database User: create, update, delete
- Valkey Instance: create, delete, migrate (RDB dump/import)
- Valkey User: create, update, delete
- S3 Bucket: create, update (policy/quota), delete
- S3 Access Key: create, delete
- Certificate: provision LE (HTTP-01 ACME), upload custom, cron renewal, cron cleanup
- Email Account: create (auto-creates MX/SPF DNS records), delete (cleanup domain if last account)
- Email Alias: create, delete (via Stalwart JMAP)
- Email Forward: create, delete (Sieve script generation)
- Email Auto-Reply: update, delete (vacation via JMAP)
- SSH Key: add, remove (syncs authorized_keys across all shard nodes)
- Backup: create, restore, delete; cron cleanup of old backups

**Infrastructure workflows:**
- `ConvergeShardWorkflow`: role-aware (web/database/valkey/LB), cleans orphaned nginx configs before provisioning, collects errors without stopping
- `TenantProvisionWorkflow`: long-running orchestrator, processes provision signals sequentially as child workflows, uses ContinueAsNew after 1000 iterations
- `UpdateServiceHostnamesWorkflow`: auto-generates DNS records for tenant services
- Audit log cleanup cron

**Provisioning callbacks:** optional webhook notifications on task completion with configurable retry.

### Node Agent (Temporal Worker)

Runs on each VM node, connecting to Temporal via `node-{uuid}` task queue:

- **TenantManager:** Linux user accounts, directory structure, UID management
- **WebrootManager:** Webroot directories, storage paths
- **NginxManager:** Per-webroot server blocks from templates, SSL cert installation, config test + reload, orphaned config cleanup
- **SSHManager:** SSH/SFTP configuration, authorized_keys sync across all shard nodes
- **DatabaseManager:** MySQL CREATE/DROP DATABASE/USER, GRANT, dump/import for migrations
- **ValkeyManager:** Instance lifecycle (config + systemd units), ACL user management, RDB dump/import
- **S3Manager:** Ceph RGW bucket/user management via `radosgw-admin`, tenant-scoped naming (`{tenantID}--{bucketName}`)
- **Runtime managers:** PHP-FPM (socket activation), Node.js, Python (gunicorn), Ruby (puma), Static

### DNS (PowerDNS)

- Separate PowerDNS PostgreSQL database for zone/record storage
- Brand-aware NS records (each brand defines its own NS hostnames and hostmaster)
- Auto-created A/AAAA records when binding FQDNs (if zone exists)
- Auto-created MX/SPF records when creating email accounts
- Platform-managed vs user-managed record tracking

### Load Balancing (HAProxy)

- LB VM at 10.10.10.70 with HAProxy
- Runtime map file (`fqdn-to-shard.map`) updated via HAProxy Runtime API (no reload for FQDN changes)
- Consistent hashing on Host header within shard backends
- Convergence pushes all active FQDN mappings to LB nodes

### Email (Stalwart)

- Full Stalwart integration: SMTP, IMAP, JMAP
- Account, alias, forward, auto-reply management via JMAP API
- Domain auto-creation (idempotent), cleanup when last account removed
- Auto-generated MX and SPF DNS records per FQDN
- Sieve script generation for forwards
- Vacation auto-reply with optional date ranges

### S3 Object Storage (Ceph RGW)

- Single-node Ceph cluster per S3 shard (mon + mgr + osd + rgw)
- OSD on dedicated raw disk with LVM + BlueStore
- Bucket policies (public/private read access), quotas
- Access keys: 20-char ID, 40-char secret (shown once on creation)
- RGW admin credentials auto-generated during cloud-init

### Admin UI

React SPA (TypeScript, Vite, Tailwind, shadcn/ui) served by Go binary on port 3001:

- **25 pages:** Dashboard, tenants, webroots, databases, zones, valkey, S3 buckets, FQDNs, email accounts, brands, API keys, audit log, platform config, login
- **Detail pages** with tabs for sub-resources, status badges, retry buttons, status messages for failed resources
- **Log viewer:** real-time log streaming from Loki with time range selection, service filtering, pause/resume, expandable JSON entries, Grafana deep link
- **Forms:** inline creation of nested resources (databases, webroots, zones, email, S3 in tenant creation)
- **Auth:** API key login with error feedback, localStorage persistence

### Observability

- **Prometheus:** 15s scrape interval, targets: core-api, worker, HAProxy, Temporal, node exporters on all VMs
- **Grafana:** 3 dashboards (API overview, infrastructure, log explorer), Prometheus + Loki datasources
- **Loki:** TSDB store with 5 GB PVC
- **Alloy:** DaemonSet tailing all k3s pod logs, extracting `app` label from `app.kubernetes.io/component`, shipping to Loki
- **Log proxy:** core-api `/logs` endpoint proxies LogQL queries to Loki for admin UI consumption
- **Metrics endpoint:** `/metrics` on core-api (request count/latency/status codes)

### CLI Tooling (`hostctl`)

- `hostctl cluster apply -f <yaml>`: bootstraps region, cluster, LB addresses, shards, nodes; triggers convergence
- `hostctl seed -f <yaml>`: seeds brands, zones, tenants with webroots/FQDNs/databases/valkey/S3/email; waits for each resource to reach active
- `hostctl converge-shard <shard-id>`: triggers manual shard convergence
- Auto-loads `.env` file for `HOSTING_API_KEY`

### Infrastructure

- **VM provisioning:** Terraform + libvirt with Packer golden images (9 roles: web, db, dns, valkey, storage, s3, dbadmin, lb, controlplane)
- **k3s control plane:** Helm chart deploys core-api, worker, admin-ui, MCP server; k3s manifests for Temporal, PostgreSQL, Loki, Grafana, Prometheus, Alloy, Traefik ingress
- **Image delivery:** `docker build` -> `docker save` -> SSH pipe -> `k3s ctr images import`
- **Networking:** `*.hosting.test` domain via Traefik ingress, WSL2 -> libvirt routing via `just forward`

### Resource Naming

All resources use UUID primary keys and auto-generated prefixed short names for system-level identifiers:

| Resource | Prefix | System Use |
|---|---|---|
| Tenant | `t_` | Linux user, CephFS paths, S3 RGW user |
| Webroot | `web_` | File paths, nginx configs |
| Database | `db_` | MySQL database name |
| Valkey | `kv_` | systemd unit, config, data dir |
| S3 Bucket | `s3_` | RGW bucket (with tenant prefix) |
| Cron Job | `cron_` | systemd timer/service |

Names are `{prefix}{10-char-random}`, globally unique, auto-generated on creation. Database and valkey usernames must start with the parent resource name (e.g., `db_abc123_admin`). See `docs/resource-naming.md` for details.

### Tests

- **Unit tests:** workflows (mock activities, state transitions), handlers (HTTP req/res, validation), activities (mock DB/TCP), agent managers (mock exec), runtime managers, request/response/config/model packages
- **E2E tests:** tenant lifecycle, webroot, database, DNS, S3, backup, platform config (require `HOSTING_E2E=1` and running cluster)

### MySQL Replication

- GTID-based async replication for DB shard primary-replica pairs
- Explicit primary election via shard config (`primary_node_id` in config JSON)
- Convergence routes writes to primary, sets up replication to replicas automatically
- Periodic health check workflow detects replication lag/breakage
- Manual failover workflow (API-triggered) promotes replica and updates shard config

### CephFS Integration

- Deterministic Ceph FSID via Terraform (no runtime config distribution)
- Systemd mount units on web nodes with proper dependencies
- Node-agent mount verification via syscall + periodic write probes
- Per-tenant CephFS quotas via extended attributes

### Cron Jobs

- Systemd timer + service units per cron job (`OnCalendar=` from cron syntax)
- Distributed locking via CephFS `flock` — timers fire on all nodes, only one executes
- Instant failover: surviving nodes acquire the lock on next timer fire
- Auto-disable after configurable consecutive failures (default 5), status `auto_disabled`
- Outcome reporting via `ExecStopPost=+/usr/local/bin/cron-outcome` (systemd-native, no polling)
- Runs as tenant user with systemd security hardening (ProtectSystem, MemoryMax, CPUQuota)
- Output captured in journald, shipped to Tenant Loki via Vector with `log_type=cron` label
- Queryable via `GET /tenants/{id}/logs?log_type=cron&cron_job_id={id}`

### Worker Runtime

- Generic "worker" runtime type for long-lived background processes (e.g., Laravel queue workers)
- Supervisord-based process management with configurable `numprocs` (1-8)
- No nginx config generation for worker runtimes
- Versioned PHP binary, systemd security hardening (NoNewPrivileges, ProtectSystem, PrivateTmp)

### Tenant Log Access

- Two Loki instances: Platform Loki (30-day, core services) + Tenant Loki (7-day, nginx/PHP-FPM/cron/worker)
- Logs on local SSD (`/var/log/hosting/{tenantID}/`), shipped by Vector with structured metadata
- Tenant-scoped `GET /tenants/{id}/logs` proxies to Tenant Loki with tenant/webroot/log_type filtering

### Alerting

- Grafana Unified Alerting (no separate Alertmanager) with severity-based routing
- 15+ core alerts: NodeDown, CoreApiDown, HighDiskUsage, HighCpuUsage, 5xxRate, HAProxyBackendDown, etc.
- Every alert links to a runbook in `docs/runbooks/`
- Repeat intervals: critical 1h, warning 4h, info 12h

### Grafana Dashboards

- Alloy JSON extraction pipeline for structured metadata (tenant, webroot, status, method, path)
- Enhanced dashboards: Log Explorer, API Overview, Infrastructure
- New dashboards: Tenant activity, Workflow stats, Database, DNS

### Extended E2E Tests

- 14 test files covering 50+ scenarios: Valkey, Email, SSH Key, Certificate, Brand isolation, API Key scopes
- Full-stack FQDN binding: zone → webroot → FQDN → DNS → HAProxy → nginx → HTTP verification
- Cross-shard migration tests (tenant, database, valkey)
- Backup/restore with data verification, retry on failed resources, DNS propagation via dig

---

## What's Next

### Production Hardening

- **MySQL auto-failover:** Phase 2 of replication — automatic failover via ProxySQL VIP switching (manual failover works today)
- **CephFS HA:** MDS standby for high availability (single MDS today)
- **Service exporters:** mysqld_exporter, PowerDNS exporter, nginx-exporter, redis_exporter for deeper Grafana metrics
- **TLS everywhere:** HTTPS on control plane services, mTLS between nodes

### Platform Features

- **Per-brand DNS isolation:** Separate PowerDNS databases per brand (currently shared)
- **Resource metering/billing:** Usage tracking, quotas enforcement, billing integration
- **Rate limiting:** API rate limiting per API key/brand
- **Multi-region:** Cross-region failover and data replication
