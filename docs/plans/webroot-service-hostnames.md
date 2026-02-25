# Plan: Per-Webroot Service Hostnames

## Context

Web service hostnames don't belong at the tenant level — they're per-webroot. Each webroot should get its own service hostname (e.g., `{webrootName}.{tenantName}.{brand.base_hostname}`) that works regardless of whether any FQDNs are connected to the webroot. Users should be able to opt out via a setting on the webroot.

## Design

### Service Hostname Format

`{webroot_name}.{tenant_name}.{brand.base_hostname}`

Example: `main.acme-corp.mhst.io`

### Database Changes

**Migration `00008_webroots.sql`** (edit in place):
- Add `service_hostname_enabled BOOLEAN NOT NULL DEFAULT true` to the `webroots` table

### Model Changes

**`internal/model/webroot.go`**:
- Add `ServiceHostnameEnabled bool` field to `Webroot` struct
- JSON tag: `service_hostname_enabled`

### API Changes

**Create webroot request** (`internal/api/request/webroot.go`):
- Add optional `service_hostname_enabled *bool` field (defaults to `true` if omitted)

**Webroot response**: Already returns full model, no change needed.

### Convergence Changes (web shard)

The web shard convergence already configures vhosts per webroot. The service hostname needs to be:

1. **Added to the webroot's desired state** — include `service_hostname` as a computed field in the convergence context (derived from webroot name + tenant name + brand base_hostname)
2. **Nginx/Caddy vhost config** — add the service hostname as an additional `server_name` on the webroot's vhost (alongside any FQDN-based hostnames)
3. **HAProxy FQDN map** — add the service hostname -> shard mapping so the LB routes it correctly

### DNS Changes

When `service_hostname_enabled` is true:
- Create an A record for `{webroot_name}.{tenant_name}.{brand.base_hostname}` pointing to the cluster LB IPs
- This happens during webroot provisioning workflow (similar to how FQDN A records are created)

When `service_hostname_enabled` is false (or toggled off):
- Remove the A record
- Remove from HAProxy FQDN map

### Workflow Changes

**`CreateWebrootWorkflow`**: After creating the webroot, if `service_hostname_enabled`:
- Create DNS A record for the service hostname
- Add to HAProxy FQDN map via Runtime API

**`DeleteWebrootWorkflow`**: Remove the service hostname DNS record and FQDN map entry.

**New `UpdateWebrootWorkflow`** (or extend existing): Handle toggling `service_hostname_enabled` on/off — add/remove DNS record and FQDN map entry.

### Admin UI Changes

**Webroot detail page** (`web/admin/src/pages/webroot-detail.tsx`):
- Show the service hostname with a copy button
- Toggle switch for `service_hostname_enabled`

**Create webroot dialog**:
- Add checkbox for "Enable service hostname" (default: checked)

### LB (HAProxy) Integration

The service hostname needs to be in the FQDN->shard map file so HAProxy routes requests correctly. This uses the same Runtime API mechanism as regular FQDNs — no HAProxy config change needed, just an additional map entry.

## Implementation Order

1. DB migration + model + API request changes
2. Create/delete webroot workflow: add service hostname DNS + FQDN map steps
3. Web shard convergence: add service hostname as additional server_name
4. Admin UI: display + toggle
5. Update webroot workflow: handle toggling the setting

## Dependencies

- Brand must have `base_hostname` set (already implemented)
- Webroot name must be unique per tenant (already enforced)
- HAProxy FQDN map Runtime API must be working (already implemented)
