# Local Networking

How traffic reaches services and tenant sites in the local development environment.

## Network topology

```
Windows
  |
  | route add 10.10.10.0/24 via <WSL2 IP>
  v
WSL2 (eth0: 172.x.x.x, virbr1: 10.10.10.1)
  |
  | iptables FORWARD + MASQUERADE
  v
libvirt network 10.10.10.0/24
  |
  |-- controlplane-0 (10.10.10.2) — k3s + Traefik
  |     |-- Traefik :80/:443      (routes control plane HTTP by Host header)
  |     |-- core-api :8090
  |     |-- admin-ui :3001
  |     |-- Temporal :7233
  |     |-- Temporal UI :8080
  |     |-- Grafana :3000
  |     |-- Loki :3100
  |     |-- Prometheus :9090
  |     |-- PostgreSQL :5432 (core), :5433 (powerdns)
  |     |-- Valkey :6379
  |     '-- Registry :5000
  |
  |-- lb-1-node-0     (10.10.10.70) — HAProxy + node-agent (tenant traffic)
  |     |-- HAProxy :80/:443       (FQDN→shard map, tenant-only)
  |     |-- HAProxy stats :8404
  |     '-- HAProxy Runtime API :9999
  |
  |-- web-1-node-0  (10.10.10.10)  — nginx + PHP-FPM + node-agent
  |-- web-1-node-1  (10.10.10.11)  — nginx + PHP-FPM + node-agent
  |-- db-1-node-0   (10.10.10.20)  — MySQL + node-agent
  |-- dns-1-node-0  (10.10.10.30)  — PowerDNS + node-agent
  |-- valkey-1-node-0 (10.10.10.40) — Valkey + node-agent
  |-- storage-1-node-0 (10.10.10.50) — Ceph (S3/CephFS) + node-agent
  '-- dbadmin-1-node-0 (10.10.10.60) — CloudBeaver + node-agent
```

Control plane and tenant traffic are split across two separate entry points:

- **Traefik** on the controlplane VM (10.10.10.2) handles all control plane HTTP (admin UI, API, Temporal UI, Grafana, Prometheus, Loki). Configured via k8s Ingress resources.
- **HAProxy** on the LB VM (10.10.10.70) handles all tenant HTTP traffic. Routes requests via a dynamic FQDN→shard map managed by the node-agent through the HAProxy Runtime API.

## How it works

1. Windows hosts file maps control plane hostnames to `10.10.10.2` and tenant FQDNs to `10.10.10.70`
2. Windows route sends `10.10.10.0/24` traffic to WSL2
3. WSL2 iptables forwards the traffic to the libvirt bridge (`virbr1`)
4. Traefik matches the `Host` header and routes to the appropriate k3s Service
5. HAProxy matches tenant FQDNs in its map file and routes to the web shard's node VMs

## Setup

### 1. WSL2 forwarding (once per boot)

```bash
just forward
```

This runs `scripts/wsl-forward.sh` which:
- Enables IP forwarding (`net.ipv4.ip_forward=1`)
- Adds iptables FORWARD rules: `eth0` -> `virbr1` and back
- Adds MASQUERADE so VMs see traffic from `10.10.10.1` (not Windows' `172.x.x.x` which they can't route to)

### 2. Windows route (once per boot)

WSL2's IP changes on each boot. Get it from the forwarding script output or:

```bash
hostname -I | awk '{print $1}'
```

Then in PowerShell as Administrator:

```powershell
route add 10.10.10.0 mask 255.255.255.0 <WSL2_IP>
```

To make it persistent (survives Windows reboots):

```powershell
route -p add 10.10.10.0 mask 255.255.255.0 <WSL2_IP>
```

Note: if WSL2's IP changes, you'll need to update the persistent route.

### 3. Windows hosts file (once)

Edit `C:\Windows\System32\drivers\etc\hosts` as Administrator:

```
# Control plane services (Traefik on controlplane VM)
10.10.10.2  admin.hosting.test api.hosting.test mcp.hosting.test temporal.hosting.test grafana.hosting.test prometheus.hosting.test loki.hosting.test traefik.hosting.test

# DB Admin (CloudBeaver — runs on its own VM, port 8978)
10.10.10.60  dbadmin.hosting.test

# Tenant sites (HAProxy on LB VM) — add seeded + new tenant FQDNs here
10.10.10.70  acme.hosting.test www.acme.hosting.test
```

The seed file (`seeds/dev-tenants.yaml`) creates two tenant FQDNs: `acme.hosting.test` and `www.acme.hosting.test`. These must point to the LB VM (`10.10.10.70`), **not** the controlplane. When adding new tenant FQDNs, always add them to the `10.10.10.70` line.

## SSL/TLS (optional)

HTTPS works out of the box with a self-signed certificate (created during `just vm-deploy` for Traefik, and during cloud-init for the LB VM). Browsers will show a security warning which you can click through.

For trusted certificates with no warnings, use [mkcert](https://github.com/FiloSottile/mkcert):

### Install mkcert

```bash
# On Ubuntu/WSL2
sudo apt install libnss3-tools
curl -L https://dl.filippo.io/mkcert/latest?for=linux/amd64 -o /usr/local/bin/mkcert
sudo chmod +x /usr/local/bin/mkcert

# Install the local CA into system trust stores
mkcert -install
```

If you access sites from Windows browsers, you also need to trust the CA on Windows. WSL2 mounts Windows drives at `/mnt/c/`, so copy the CA cert to your Desktop (using `.crt` extension so Windows recognizes it):

```bash
cp "$(mkcert -CAROOT)/rootCA.pem" /mnt/c/Users/<your-windows-username>/Desktop/rootCA.crt
```

Then on Windows either:
- **GUI**: double-click `rootCA.crt` > "Install Certificate" > "Local Machine" > "Place all certificates in the following store" > "Trusted Root Certification Authorities" > Finish
- **CLI** (PowerShell as Administrator): `certutil -addstore "Root" %USERPROFILE%\Desktop\rootCA.crt`

### Generate and deploy certs

```bash
just ssl-init
```

This generates a wildcard cert for `*.hosting.test` and deploys it to both Traefik (as a k8s TLS Secret) and the LB VM HAProxy. All `https://*.hosting.test` URLs will work without warnings.

### Verify

```bash
curl -v https://api.hosting.test/healthz
```

Both HTTP and HTTPS work simultaneously — no forced redirect.

## Hostname reference

| URL | Service | Routed by | IP |
|-----|---------|-----------|-----|
| `https://admin.hosting.test` | Admin UI | Traefik | 10.10.10.2 |
| `https://api.hosting.test` | Core API | Traefik | 10.10.10.2 |
| `https://mcp.hosting.test` | MCP Server (LLM tool access) | Traefik | 10.10.10.2 |
| `https://temporal.hosting.test` | Temporal UI | Traefik | 10.10.10.2 |
| `https://grafana.hosting.test` | Grafana (logs/metrics) | Traefik | 10.10.10.2 |
| `https://prometheus.hosting.test` | Prometheus | Traefik | 10.10.10.2 |
| `https://loki.hosting.test` | Loki (log aggregation) | Traefik | 10.10.10.2 |
| `https://traefik.hosting.test` | Traefik dashboard | Traefik | 10.10.10.2 |
| `https://dbadmin.hosting.test` | DB Admin (CloudBeaver) | Direct (nginx) | 10.10.10.60 |
| `https://acme.hosting.test` | Tenant site | HAProxy | 10.10.10.70 |

HTTP (`http://`) also works on all URLs — no forced redirect.

Control plane hostnames are configured via k8s Ingress resources in `deploy/k3s/ingress.yaml`. Tenant routing is via the HAProxy FQDN map on the LB VM. The domain is controlled by the `base_domain` Terraform variable (default: `hosting.test`).

### Authentication

Most control plane services are open (no auth) since they're only accessible within the libvirt dev network.

| Service | Auth | Notes |
|---------|------|-------|
| Admin UI | None | Open access |
| Core API | API key | `X-API-Key` header required |
| Grafana | admin / admin | Anonymous users get read-only Viewer role (dashboards only). Log in as `admin`/`admin` for full access including Explore (log queries). |
| Prometheus | None | Open access |
| Loki | None | API-only (no UI). Browse logs via Grafana → Explore → Loki. |
| Temporal UI | None | Open access |
| Traefik | None | Dashboard at `/dashboard/` |
| CloudBeaver | OIDC | Authenticates via core-api as OIDC provider |

## Adding and reaching tenant sites

1. Create a tenant (via admin UI or API)
2. Create a webroot on the tenant
3. Add an FQDN using a `.hosting.test` subdomain, e.g. `mysite.hosting.test`
4. The FQDN binding workflow:
   - Adds the FQDN to the HAProxy map on each LB node via the node-agent
   - Creates DNS records in PowerDNS (if a matching zone exists)
5. Add `10.10.10.70  mysite.hosting.test` to your Windows hosts file
6. Visit `http://mysite.hosting.test` in the browser

The seed file (`seeds/dev-tenants.yaml`) creates a zone `hosting.test` under the `acme-brand`. Any FQDN ending in `.hosting.test` will match this zone and get auto-DNS records created.

### Example: adding a second tenant

```bash
# Create tenant via API
curl -X POST http://api.hosting.test/api/v1/tenants \
  -H "X-API-Key: hst_..." \
  -H "Content-Type: application/json" \
  -d '{"brand_id": "acme-brand", "shard_id": "<web-shard-id>"}'

# Create webroot (use tenant ID from response)
curl -X POST http://api.hosting.test/api/v1/tenants/<id>/webroots \
  -H "X-API-Key: hst_..." \
  -H "Content-Type: application/json" \
  -d '{"name": "main", "runtime": "php", "runtime_version": "8.5"}'

# Add FQDN (use webroot ID from response)
curl -X POST http://api.hosting.test/api/v1/webroots/<id>/fqdns \
  -H "X-API-Key: hst_..." \
  -H "Content-Type: application/json" \
  -d '{"fqdn": "newsite.hosting.test"}'

# Add to Windows hosts file (point to LB VM), then visit
open http://newsite.hosting.test
```

## Verifying DNS records

The platform creates DNS records in PowerDNS when FQDNs are bound. PowerDNS runs on the DNS node VM. You can query it directly from WSL2:

```bash
dig @10.10.10.30 hosting.test SOA
dig @10.10.10.30 acme.hosting.test A
dig @10.10.10.30 hosting.test NS
```

These DNS records aren't used for browser routing in dev (we use hosts file entries), but they confirm the platform's DNS pipeline works correctly and would be used in production.

## Why `.test` instead of `.localhost`?

Browsers hardcode `.localhost` to resolve to `127.0.0.1` per [RFC 6761](https://datatracker.ietf.org/doc/html/rfc6761), ignoring hosts file entries. Since the control plane runs on `10.10.10.2` (not localhost), we use `.test` which is also RFC 6761 reserved but browsers resolve it normally via hosts file / DNS.

## Debugging

### Verify routing from Windows

```powershell
# Check route exists
route print | findstr 10.10.10

# Test connectivity to control plane
ping 10.10.10.2
curl http://10.10.10.2:8090/healthz

# Test connectivity to LB
ping 10.10.10.70
```

### Verify from WSL2

```bash
# Direct API access
curl http://10.10.10.2:8090/healthz

# Via Traefik
curl -H "Host: api.hosting.test" http://10.10.10.2/api/v1/healthz

# Check forwarding rules
just forward-status
```

### Check HAProxy map (on LB VM)

```bash
just lb-show
```

### Check response headers

```bash
curl -v http://acme.hosting.test
```

Look for:
- `X-Served-By` — the node hostname that handled the request
- `X-Shard` — the shard name the node belongs to
