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
  |-- controlplane-0 (10.10.10.2) — k3s
  |     |-- HAProxy :80/:443     (routes all HTTP/HTTPS by Host header)
  |     |-- HAProxy stats :8404
  |     |-- core-api :8090
  |     |-- admin-ui :3001
  |     |-- Temporal :7233
  |     |-- Temporal UI :8080
  |     |-- PostgreSQL :5432 (core), :5433 (powerdns)
  |     |-- Valkey :6379
  |     '-- Registry :5000
  |
  |-- web-1-node-0  (10.10.10.10)  — nginx + PHP-FPM + node-agent
  |-- web-1-node-1  (10.10.10.11)  — nginx + PHP-FPM + node-agent
  |-- db-1-node-0   (10.10.10.20)  — MySQL + node-agent
  |-- dns-1-node-0  (10.10.10.30)  — PowerDNS + node-agent
  |-- valkey-1-node-0 (10.10.10.40) — Valkey + node-agent
  |-- storage-1-node-0 (10.10.10.50) — Ceph (S3/CephFS) + node-agent
  '-- dbadmin-1-node-0 (10.10.10.60) — CloudBeaver + node-agent
```

All k3s pods use `hostNetwork: true`, so services bind directly to `10.10.10.2`. HAProxy on port 80 routes requests based on the `Host` header — this is the single entry point for all HTTP traffic.

## How it works

1. Windows hosts file maps `*.hosting.test` hostnames to `10.10.10.2`
2. Windows route sends `10.10.10.0/24` traffic to WSL2
3. WSL2 iptables forwards the traffic to the libvirt bridge (`virbr1`)
4. HAProxy on the controlplane VM matches the `Host` header and routes to the appropriate backend

For control plane services (admin UI, API, Temporal UI), HAProxy forwards to `127.0.0.1:<port>` on the same VM. For tenant sites, HAProxy looks up the FQDN in a dynamic map file and forwards to the web shard's node VMs.

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
10.10.10.2  admin.hosting.test api.hosting.test temporal.hosting.test dbadmin.hosting.test
```

Add tenant FQDNs as needed:

```
10.10.10.2  acme.hosting.test www.acme.hosting.test
```

## SSL/TLS (optional)

HTTPS works out of the box with a self-signed certificate (created during `just vm-deploy`). Browsers will show a security warning which you can click through.

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

If you access sites from Windows browsers, you also need to trust the CA on Windows. Copy the CA cert to Windows and install it:

```bash
# Find where mkcert stores the CA
mkcert -CAROOT
# e.g. /home/user/.local/share/mkcert

# Copy to Windows (adjust path)
cp "$(mkcert -CAROOT)/rootCA.pem" /mnt/c/Users/<you>/rootCA.pem
```

Then on Windows: double-click `rootCA.pem` > "Install Certificate" > "Local Machine" > "Place all certificates in the following store" > "Trusted Root Certification Authorities" > Finish.

### Generate and deploy certs

```bash
just ssl-init
```

This generates a wildcard cert for `*.hosting.test`, creates a k8s Secret, and restarts HAProxy. All `https://*.hosting.test` URLs will work without warnings.

### Verify

```bash
curl -v https://api.hosting.test/healthz
```

Both HTTP and HTTPS work simultaneously — no forced redirect.

## Hostname reference

| URL | Service | Backend |
|-----|---------|---------|
| `https://admin.hosting.test` | Admin UI | `127.0.0.1:3001` |
| `https://api.hosting.test` | Core API | `127.0.0.1:8090` |
| `https://temporal.hosting.test` | Temporal UI | `127.0.0.1:8080` |
| `https://dbadmin.hosting.test` | DB Admin (CloudBeaver) | `127.0.0.1:4180` |
| `https://acme.hosting.test` | Tenant site | web shard VMs |

HTTP (`http://`) also works on all URLs — no forced redirect.

These hostnames are configured in the HAProxy config template at `terraform/templates/haproxy.cfg.tpl`. The domain is controlled by the `base_domain` Terraform variable (default: `hosting.test`).

## Adding and reaching tenant sites

1. Create a tenant (via admin UI or API)
2. Create a webroot on the tenant
3. Add an FQDN using a `.hosting.test` subdomain, e.g. `mysite.hosting.test`
4. The FQDN binding workflow:
   - Adds the FQDN to the HAProxy map via Runtime API
   - Creates DNS records in PowerDNS (if a matching zone exists)
5. Add `10.10.10.2  mysite.hosting.test` to your Windows hosts file
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

# Add to Windows hosts file, then visit
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

# Test connectivity
ping 10.10.10.2
curl http://10.10.10.2:8090/healthz
```

### Verify from WSL2

```bash
# Direct API access
curl http://10.10.10.2:8090/healthz

# Via HAProxy
curl -H "Host: api.hosting.test" http://10.10.10.2/api/v1/healthz

# Check forwarding rules
just forward-status
```

### Check HAProxy map

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
