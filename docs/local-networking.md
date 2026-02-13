# Local Networking

How traffic reaches services and tenant sites in the local development environment.

## How it works

HAProxy runs with `network_mode: host` so it binds directly on the WSL2/host network. It listens on port 80 and routes requests based on the `Host` header:

1. **Control plane hostnames** are matched first (hardcoded in HAProxy config)
2. **Tenant FQDNs** are looked up in the dynamic `fqdn-to-shard.map` file, which is updated via the HAProxy Runtime API when FQDNs are bound

All development hostnames use the `.localhost` TLD, which browsers resolve to `127.0.0.1` automatically per [RFC 6761](https://datatracker.ietf.org/doc/html/rfc6761). No `/etc/hosts` entries or DNS configuration needed.

## Hostname reference

| URL | Service |
|-----|---------|
| `http://admin.hosting.localhost` | Admin UI |
| `http://api.hosting.localhost` | Core API (`/api/v1/...`) |
| `http://temporal.hosting.localhost` | Temporal UI |
| `http://<fqdn>.hosting.localhost` | Tenant site (via shard map) |

These are configured in the HAProxy config template at `terraform/templates/haproxy.cfg.tpl`, which Terraform generates with real VM IPs for shard backends.

## Adding and reaching tenant sites

1. Create a tenant (via admin UI or API)
2. Create a webroot on the tenant
3. Add an FQDN using a `.hosting.localhost` subdomain, e.g. `mysite.hosting.localhost`
4. The FQDN binding workflow:
   - Adds the FQDN to the HAProxy map via Runtime API
   - Creates DNS records in PowerDNS (if a matching zone exists)
5. Visit `http://mysite.hosting.localhost` in the browser

The seed file (`seeds/dev-tenants.yaml`) creates a zone `hosting.localhost` under the `acme-brand`. Any FQDN ending in `.hosting.localhost` will match this zone and get auto-DNS records created.

### Example: adding a second tenant

```bash
# Create tenant via API
curl -X POST http://api.hosting.localhost/api/v1/tenants \
  -H "X-API-Key: hst_..." \
  -H "Content-Type: application/json" \
  -d '{"brand_id": "acme-brand", "shard_id": "<web-shard-id>"}'

# Create webroot (use tenant ID from response)
curl -X POST http://api.hosting.localhost/api/v1/tenants/<id>/webroots \
  -H "X-API-Key: hst_..." \
  -H "Content-Type: application/json" \
  -d '{"name": "main", "runtime": "php", "runtime_version": "8.5"}'

# Add FQDN (use webroot ID from response)
curl -X POST http://api.hosting.localhost/api/v1/webroots/<id>/fqdns \
  -H "X-API-Key: hst_..." \
  -H "Content-Type: application/json" \
  -d '{"fqdn": "newsite.hosting.localhost"}'

# Visit in browser
open http://newsite.hosting.localhost
```

## Verifying DNS records

The platform creates DNS records in PowerDNS when FQDNs are bound. PowerDNS is exposed on port 5300. You can query it directly:

```bash
# Check zone SOA
dig @127.0.0.1 -p 5300 hosting.localhost SOA

# Check FQDN A record
dig @127.0.0.1 -p 5300 acme.hosting.localhost A

# Check NS records
dig @127.0.0.1 -p 5300 hosting.localhost NS
```

These DNS records aren't used for browser routing (`.localhost` bypasses DNS), but they confirm the platform's DNS pipeline works correctly and would be used in production.

## Network topology

```
Browser (Windows)
  |
  | http://mysite.hosting.localhost
  | (.localhost -> 127.0.0.1, RFC 6761)
  v
HAProxy :80 (network_mode: host)
  |
  |-- Host: admin.hosting.localhost  --> localhost:3001 (admin-ui container)
  |-- Host: api.hosting.localhost    --> localhost:8090 (core-api container)
  |-- Host: temporal.hosting.localhost -> localhost:8080 (temporal-ui container)
  |-- Host: mysite.hosting.localhost --> fqdn-to-shard.map lookup
  |       |
  |       v
  |     shard-web-1 backend
  |       |-- 10.10.10.10:80 (web-1-node-0 VM)
  |       |-- 10.10.10.11:80 (web-1-node-1 VM)
  |
  |-- No match --> 503
```

The worker also runs with `network_mode: host`, so it can reach both Docker services (via `localhost:<port>`) and libvirt VMs (via `10.10.10.x`) for HAProxy Runtime API calls.

## Using custom (non-`.localhost`) domains

For testing with real-looking domain names (e.g. `mysite.test`), you need the browser to resolve them to `127.0.0.1`. Two options:

### Option A: Windows hosts file (simple)

Edit `C:\Windows\System32\drivers\etc\hosts` as Administrator:

```
127.0.0.1  mysite.test
127.0.0.1  www.mysite.test
```

Then create a matching zone in the platform:

```bash
curl -X POST http://api.hosting.localhost/api/v1/zones \
  -H "X-API-Key: hst_..." \
  -H "Content-Type: application/json" \
  -d '{"name": "mysite.test", "brand_id": "acme-brand"}'
```

Downside: each new FQDN needs a manual hosts file entry.

### Option B: Forward DNS to PowerDNS via NRPT (automatic)

This makes Windows forward DNS queries for a specific domain to PowerDNS, so new FQDNs work automatically.

**Step 1**: Expose PowerDNS on port 53. Add a second port mapping in `docker-compose.yml`:

```yaml
powerdns:
  ports:
    - "5300:53/udp"
    - "5300:53/tcp"
    - "8081:8081"
    - "127.0.0.2:53:53/udp"  # For NRPT forwarding
    - "127.0.0.2:53:53/tcp"
```

Port 53 on `127.0.0.1` is typically used by the Windows DNS Client service, so we bind to `127.0.0.2` instead (loopback aliases always work on Windows).

**Step 2**: Add an NRPT rule in PowerShell (as Administrator):

```powershell
# Forward all *.mysite.test queries to PowerDNS
Add-DnsClientNrptRule -Namespace ".mysite.test" -NameServers "127.0.0.2"

# Verify the rule
Get-DnsClientNrptRule
```

**Step 3**: Create the zone and FQDN in the platform. PowerDNS will respond with the correct A record (the cluster LB address), and the browser will route to HAProxy.

To remove the rule later:

```powershell
Get-DnsClientNrptRule | Where-Object Namespace -eq ".mysite.test" | Remove-DnsClientNrptRule
```

### Verifying from WSL

Inside WSL2, `.localhost` resolution works the same way. To query PowerDNS directly:

```bash
dig @127.0.0.1 -p 5300 mysite.test A
```

WSL2 shares the Windows host network, so all port mappings are accessible from both Windows and WSL.
