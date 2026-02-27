# Hosting Platform - Status & Roadmap

## Current Status Summary

**Build:** Go 1.26, compiles clean, `go vet` passes, all test packages pass.
**Infrastructure:** k3s control plane (core-api, worker, admin-ui, MCP server, Temporal, PostgreSQL, Loki, Grafana, Prometheus, Alloy). Nodes run on VMs provisioned by Terraform/libvirt with Packer golden images.
**Dev Environment:** 10 VMs (controlplane + 2 web + 1 db + 1 dns + 1 valkey + 1 storage + 1 dbadmin + 1 lb + 1 gateway) on libvirt, accessible at `*.massive-hosting.com`.
**CLI:** `hostctl cluster apply` bootstraps infrastructure; `hostctl seed` populates tenant data; `hostctl converge-shard` triggers convergence. Auto-loads `.env` for API key.

---

## What's Implemented

### Core API (REST)

Full CRUD REST API at `api.massive-hosting.com/api/v1` with OpenAPI docs at `/docs`.

**Authentication & Authorization:**
- API key auth (`X-API-Key` header) with fine-grained scopes (`resource:action` format)
- Brand-based access control (keys authorized for specific brands or `*` for platform admin)
- All mutations audit-logged with sanitized request bodies (passwords/keys redacted)
- Credential hashing: MySQL passwords stored as `mysql_native_password` hashes, Valkey passwords as SHA256 hashes, S3 secrets as SHA256 hashes. Plaintext is never persisted in the control plane DB.

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
| Shards | CRUD `/clusters/{id}/shards`, converge, retry | Yes | Roles: web, database, dns, email, valkey, s3, gateway |
| Nodes | CRUD `/clusters/{id}/nodes` | No | UUID-based Temporal task queue routing |
| Tenants | CRUD, suspend/unsuspend/migrate/retry `/tenants` | Yes | Resource summary, resource usage, login sessions, retry-failed |
| Webroots | CRUD `/tenants/{id}/webroots`, retry | Yes | PHP/Node/Python/Ruby/Static runtimes; service hostnames |
| FQDNs | CRUD `/webroots/{id}/fqdns`, retry | Yes | Auto-DNS + auto-LB-map + optional LE cert |
| Certificates | List/upload `/fqdns/{id}/certificates`, retry | Yes | PEM upload, LE provisioning |
| SSH Keys | CRUD `/tenants/{id}/ssh-keys`, retry | Yes | SSH public keys for SFTP/SSH access |
| Egress Rules | CRUD `/tenants/{id}/egress-rules`, retry | Yes | Per-tenant nftables whitelist (allow CIDRs + reject) |
| Database Access Rules | CRUD `/databases/{id}/access-rules`, retry | Yes | Per-database MySQL host patterns; internal-only default |
| Zones | CRUD `/zones`, tenant reassign, retry | Yes | Brand-scoped DNS zones |
| Zone Records | CRUD `/zones/{id}/records`, retry | Yes | A/AAAA/CNAME/MX/TXT/NS/etc. |
| Databases | CRUD `/tenants/{id}/databases`, migrate, retry | Yes | MySQL; charset, collation |
| Database Users | CRUD `/databases/{id}/users`, retry | Yes | Privileges (all/read-only) |
| Valkey Instances | CRUD `/tenants/{id}/valkey-instances`, migrate, retry | Yes | Managed Redis; eviction, max memory |
| Valkey Users | CRUD `/valkey-instances/{id}/users`, retry | Yes | ACL-based access |
| WireGuard Peers | CRUD `/tenants/{id}/wireguard-peers`, retry | Yes | VPN peers for DB/Valkey access |
| S3 Buckets | CRUD `/tenants/{id}/s3-buckets`, retry | Yes | Ceph RGW; public/private, quotas |
| S3 Access Keys | CRUD `/s3-buckets/{id}/access-keys` | Yes | 20-char ID, 40-char secret; shown once |
| Email Accounts | CRUD `/fqdns/{id}/email-accounts`, retry | Yes | Stalwart SMTP/IMAP/JMAP |
| Email Aliases | CRUD `/email-accounts/{id}/aliases`, retry | Yes | |
| Email Forwards | CRUD `/email-accounts/{id}/forwards`, retry | Yes | External forwarding with keep-copy |
| Email Auto-Replies | GET/PUT/DELETE `/email-accounts/{id}/autoreply`, retry | Yes | Vacation/out-of-office |
| Env Vars | GET/PUT/DELETE `/webroots/{id}/env-vars` | Yes | Webroot-scoped env vars, vaulted secrets |
| Daemons | CRUD `/webroots/{id}/daemons`, enable/disable/retry | Yes | Supervisord processes, optional nginx proxy |
| Backups | CRUD `/tenants/{id}/backups`, restore, retry | Yes | Web (tar.gz) and MySQL (.sql.gz) |
| Logs | GET `/logs` | No | Loki proxy for platform log querying |

**OIDC Provider:**
- Discovery (`.well-known/openid-configuration`), JWKS, authorize, token endpoints
- Client management for external auth integrations (e.g., CloudBeaver)
- Tenant login sessions for authorization code flow

**MCP Server:**
- Dynamic tool generation from OpenAPI spec, grouped by domain (infrastructure, tenants, web, databases, dns, email, storage, platform)
- Available at `mcp.massive-hosting.com`

**Response patterns:**
- List endpoints: `{items: [...], next_cursor, has_more}` with search/sort/status filtering
- Async operations: 202 Accepted, Temporal workflow handles provisioning
- Status progression: `pending -> provisioning -> active` (or `failed` with `status_message`, or `suspended` with `suspend_reason`)
- All async resources support `POST /{resource}/{id}/retry` to re-trigger failed provisioning

### Temporal Workflows

**Resource lifecycle (all with retry support):**
- Tenant: create, update, suspend (with reason, cascades to all child resources), unsuspend (cascades), delete, migrate (cross-shard)
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
- Egress Rule: sync (whitelist model — accept CIDRs + final reject; no rules = unrestricted)
- Database Access Rule: sync (internal-only default; rules add external CIDRs on top)
- WireGuard Peer: create (generate keypair + PSK, configure gateway), delete (remove from gateway)
- Backup: create, restore, delete; cron cleanup of old backups

**Infrastructure workflows:**
- Daemon: create, update, delete, enable, disable
- `ConvergeShardWorkflow`: role-aware (web/database/valkey/LB/gateway), cleans orphaned nginx configs before provisioning, collects errors without stopping
- `TenantProvisionWorkflow`: long-running orchestrator, processes provision signals sequentially as child workflows, uses ContinueAsNew after 1000 iterations
- `UpdateServiceHostnamesWorkflow`: auto-generates DNS records for tenant services
- `CollectResourceUsageWorkflow`: cron (every 30 min), fans out to web/DB nodes, collects per-resource disk usage, upserts to `resource_usage` table
- Audit log cleanup cron

**Provisioning callbacks:** optional webhook notifications on task completion with configurable retry.

### Node Agent (Temporal Worker)

Runs on each VM node, connecting to Temporal via `node-{uuid}` task queue:

- **TenantManager:** Linux user accounts, directory structure, UID management
- **WebrootManager:** Webroot directories, storage paths
- **NginxManager:** Per-webroot server blocks from templates, SSL cert installation, config test + reload, orphaned config cleanup
- **SSHManager:** SSH/SFTP configuration, authorized_keys sync across all shard nodes
- **DatabaseManager:** MySQL CREATE/DROP DATABASE/USER, GRANT, dump/import for migrations
- **ValkeyManager:** Instance lifecycle (config + ACL file + systemd units, dual-stack bind, Unix socket auth), ACL user management with hashed passwords, RDB dump/import
- **S3Manager:** Ceph RGW bucket/user management via `radosgw-admin`, tenant-scoped naming (`{tenantID}--{bucketName}`)
- **TenantULAManager:** Per-tenant ULA IPv6 addresses on web/DB/Valkey nodes, nftables UID binding (web), service ingress filtering (DB/Valkey), cross-shard routing
- **WireGuardManager:** WireGuard interface management, per-peer configuration with nftables FORWARD rules, full convergence sync
- **Runtime managers:** PHP-FPM (socket activation, configurable PM/php.ini via runtime_config), Node.js, Python (gunicorn), Ruby (puma), Static

### DNS (PowerDNS)

- Separate PowerDNS PostgreSQL database for zone/record storage
- Brand-aware NS records (each brand defines its own NS hostnames and hostmaster)
- Auto-created A/AAAA records when binding FQDNs (if zone exists)
- Auto-created MX/SPF/DKIM/DMARC records when creating email accounts
- Auto-created service hostname records (ssh/sftp/mysql/web) on tenant provisioning — tracked in core DB
- Auto-created per-webroot service hostname DNS records (`{webroot}.{tenant}.{brand.base_hostname}`)
- Custom records override auto records (auto records preserved in core DB for reactivation)
- Retroactive auto-record creation when zone appears after existing FQDNs
- `managed_by`: `custom` (user) vs `auto` (platform), with `source_type` tracking origin

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

### Tunnel CLI (`hosting-cli`)

Userspace WireGuard tunnel client for accessing tenant MySQL and Valkey services from a local machine (no root required):

- `hosting-cli import <config> [-tenant ID]`: import WireGuard config, associate with tenant
- `hosting-cli profiles`: list saved profiles with active indicator
- `hosting-cli use <name>`: switch active profile (context switch between tenants)
- `hosting-cli active`: show active profile details and available services
- `hosting-cli tunnel [name]`: establish WireGuard tunnel via netstack (userspace)
- `hosting-cli proxy [-mysql-port 3306] [-valkey-port 6379]`: tunnel + auto-proxy services to localhost
- `hosting-cli proxy -target [addr]:port -port <local-port>`: manual target proxy
- `hosting-cli status`: show profile and service info
- Multi-tenant profiles: each profile stored with tenant ID, context switchable via `use`
- Service auto-discovery: parses `# hosting-cli:services` metadata comments from WireGuard config
- Client config includes service ULA addresses (MySQL, Valkey) embedded as comments at creation time

### Infrastructure

- **VM provisioning:** Terraform + libvirt with single Packer base image (Ubuntu + HWE kernel), Ansible for role-specific software
- **Ansible configuration management:** Dynamic API-backed inventory, 17 roles covering all VM types, tag-based selective deployment (`just deploy-node-agent`, `just ansible-role web --tags php`)
- **k3s control plane:** Helm chart deploys core-api, worker, admin-ui, MCP server; k3s manifests for Temporal, PostgreSQL, Loki, Grafana, Prometheus, Alloy, Traefik ingress
- **Image delivery:** `docker build` -> `docker save` -> SSH pipe -> `k3s ctr images import`
- **Networking:** `*.massive-hosting.com` domain via Traefik ingress, WSL2 -> libvirt routing via `just forward`

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
| Daemon | `daemon_` | supervisord program |

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

### Daemons

- Long-running processes attached to webroots (WebSocket servers, queue workers, custom background services)
- Supervisord-based process management with configurable `numprocs` (1-8), stop signal, stop wait, memory limit
- Optional `proxy_path` (e.g., `/app`, `/ws`) auto-allocates a port (FNV hash into 10000-19999) and adds nginx `location` block with WebSocket Upgrade headers
- Daemons without `proxy_path` run as pure background processes (no nginx integration)
- Enable/disable lifecycle, convergence writes supervisord configs to all shard nodes
- Nginx proxy locations support WebSocket connections (HTTP Upgrade headers + 24-hour timeout)

### Environment Variables & Vaulted Secrets

- Webroot-scoped environment variables available to PHP-FPM, cron jobs, daemons, and SSH sessions
- Per-tenant envelope encryption: AES-256-GCM with platform master key (KEK) and per-tenant data encryption keys (DEK)
- Secret values encrypted at rest, redacted in API responses (`***`)
- Configurable env file name (`.env`, `.env.hosting`, etc.) and optional auto-sourcing in SSH sessions via `.bashrc`
- Injection targets: PHP-FPM `env[KEY]=value`, dot-env file on disk, `EnvironmentFile=` for cron systemd units
- PHP runtime config: configurable PM settings (max_children, start_servers, etc.), php_values, php_admin_values with blocklist
- API: `GET/PUT /webroots/{id}/env-vars`, `DELETE /webroots/{id}/env-vars/{name}`

### Tenant Log Access

- Two Loki instances: Platform Loki (30-day, core services) + Tenant Loki (7-day, nginx/PHP-FPM/cron/daemon)
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

### Incident Management & LLM Agent

- **Incident tracking:** Create, update, resolve, escalate, cancel incidents with full event timeline
- **Auto-detection:** 7 health crons create incidents automatically:
  - Replication health (every minute): replication broken, replication lag
  - Convergence health (every 5 min): shards stuck in converging state
  - Node health (every 2 min): nodes not reporting health
  - Disk pressure (every 5 min): disk usage >90% (warning) or >95% (critical)
  - Cert expiry (daily): certificates expiring within 14 days
  - CephFS health (every 10 min): CephFS unmounted on web nodes
  - Incident escalation (every 5 min): stale incidents not being addressed
- **Auto-resolution:** When health checks pass, matching incidents are auto-resolved
- **Capability gaps:** Track missing tools/capabilities reported by agents, sorted by occurrence; linked to incidents
- **Webhook notifications:** Configurable webhooks for critical incidents and escalations (generic JSON or Slack Block Kit)
- **Dashboard integration:** Incident stats (open, critical, escalated, MTTR) on admin dashboard
- **LLM Investigation Agent:** Autonomous incident responder powered by self-hosted LLM (vLLM + Qwen 72B)
  - **Smart scheduling (leader-follower):** Incidents grouped by type; leader investigated first, resolution hints passed to followers to avoid redundant investigation
  - **Per-type concurrency:** Configurable via `platform_config` (`agent.concurrency.<type>`) — e.g., disk_pressure can fan out widely while replication_lag stays conservative
  - **Live admin chat:** Send messages to the agent during active investigation — injected into LLM conversation between turns, auto-polling UI with visual distinction (admin blue, agent orange)
  - Multi-turn conversation loop with 11 tools (read infrastructure, trigger convergence, resolve/escalate)
  - Tool calls execute via HTTP to core API (same auth as external users)
  - Every step recorded as `incident_event` for full observability
  - Configurable system prompt via `platform_config`
  - Feature-flagged (`AGENT_ENABLED`), concurrency-capped (`AGENT_MAX_CONCURRENT`, `AGENT_FOLLOWER_CONCURRENT`)
  - See `docs/incident-management.md` for full documentation

### Resource Usage Collection

- Per-resource disk usage tracking (webroots via `du -sb`, databases via MySQL `information_schema`)
- `resource_usage` table: one row per resource, upserted every 30 minutes by cron workflow
- API: `GET /tenants/{id}/resource-usage` returns all usage entries for a tenant
- Collection-only — no quotas or billing enforcement

### Per-Webroot Service Hostnames

- Every webroot gets a stable service hostname: `{webroot}.{tenant}.{brand.base_hostname}`
- Enabled by default (`service_hostname_enabled: true`), toggleable per webroot
- Create/update/delete workflows auto-manage DNS A records and LB map entries
- Convergence includes service hostnames as additional nginx `server_name` entries
- FQDN bind/unbind workflows include service hostname in nginx config regeneration

### Extended E2E Tests

- 28 test files covering 60+ scenarios: Valkey, Email, SSH Key, Certificate, Brand isolation, API Key scopes
- Full-stack FQDN binding: zone → webroot → FQDN → DNS → HAProxy → nginx → HTTP verification
- Cross-shard migration tests (tenant, database, valkey)
- Backup/restore with data verification, retry on failed resources, DNS propagation via dig
- Incident CRUD with event timeline, dedupe, status transitions (open → escalated → resolved/cancelled)
- Resource usage collection verification (cron-based, per-webroot/database byte counts)
- Service hostname tests (enable/disable toggle, DNS record creation, HTTP traffic via LB)
- Cron job lifecycle (create → systemd timer verification → update → disable/enable → delete)
- Shard convergence trigger with idempotency test
- Suspend/resume with HTTP 503 verification and restore

### Per-Tenant ULA on Service Nodes

- Per-tenant ULA IPv6 addresses (`fd00:{hash}:{shard_index}::{uid}`) on DB and Valkey nodes (extends existing web-node ULA)
- `tenant0` dummy interface provisioned via cloud-init on DB and Valkey VMs (same as web nodes)
- MySQL dual-stack binding (`bind-address = *`) and Valkey dual-stack (`bind 0.0.0.0 ::`)
- Service ingress nftables table (`ip6 tenant_service_ingress`) restricts ULA-destined traffic to web-node ULAs and localhost
- Role-based transit address offsets (web 0-255, DB 256-511, Valkey 512-767) prevent collisions in cross-shard routing
- `ConfigureRoutesV2` supports cross-shard peers — web nodes route to DB/Valkey and vice versa
- Creation workflows (`CreateDatabaseWorkflow`, `CreateValkeyInstanceWorkflow`) configure ULA on shard nodes (non-fatal)
- Convergence workflows set up ULA addresses and cross-shard routes for DB and Valkey shards

### WireGuard VPN Gateway

- Dedicated gateway shard role for WireGuard VPN access to tenant DB and Valkey ULA addresses
- Server-generated client keypair + PSK; private key and full `.conf` returned once on creation, never stored
- Client assigned address: `fd00:{hash}:ffff::{peer_index}/128` (shard index `0xFFFF` reserved for gateway)
- Per-peer nftables FORWARD rules restrict each peer to only their tenant's ULA addresses (policy drop)
- Gateway convergence: syncs all peers, computes allowed ULAs per peer from DB/Valkey shards, sets up transit routes
- Gateway nodes have transit routes to all DB and Valkey shard ULA prefixes
- Feature gated behind `wireguard` subscription module
- Terraform + cloud-init + Ansible role for gateway VM provisioning (WireGuard packages, IPv6 forwarding, server keypair)
- Transit offset 768-1023 for gateway role
- Service metadata embedded in client config (`# hosting-cli:services` comments with MySQL/Valkey ULA addresses)
- Full admin UI: WireGuard tab on tenant detail page (create peer, view config with download/copy, delete, retry failed)
- Full control panel UI: list page with card grid, create form, config display with download/copy/CLI hint, detail page with info + delete

---

## What's Next

### Production Hardening

- **MySQL auto-failover:** Phase 2 of replication — automatic failover via ProxySQL VIP switching (manual failover works today)
- **CephFS HA:** MDS standby for high availability (single MDS today)
- **Service exporters:** mysqld_exporter, PowerDNS exporter, nginx-exporter, redis_exporter for deeper Grafana metrics
- **TLS everywhere:** HTTPS on control plane services, mTLS between nodes
