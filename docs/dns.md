# DNS Zones & Records

DNS is powered by PowerDNS with zones and records managed through the core API. Zones are brand-scoped and backed by a PowerDNS database. The platform automatically creates DNS records when FQDNs are bound to webroots, when email accounts are created, and when tenants are provisioned (service hostnames).

## Zone Model

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | UUID |
| `brand_id` | string | Brand this zone belongs to |
| `tenant_id` | string | Optional tenant association (nullable) |
| `name` | string | Zone name (e.g. `example.com`) |
| `region_id` | string | Region where the DNS shard lives |
| `status` | string | Lifecycle status |
| `status_message` | string | Error message when `failed` |

## Zone Record Model

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | UUID |
| `zone_id` | string | Parent zone ID |
| `type` | string | Record type: `A`, `AAAA`, `CNAME`, `MX`, `TXT`, `SRV`, `NS`, `CAA`, `PTR`, `SOA`, `ALIAS`, `SSHFP`, `TLSA`, `LOC`, `NAPTR`, `HTTPS`, `SVCB`, `DNSKEY`, `DS`, `NSEC`, `NSEC3`, `RRSIG` |
| `name` | string | Record name (FQDN) |
| `content` | string | Record value |
| `ttl` | int | TTL in seconds (default: 3600, range: 60-86400) |
| `priority` | int | Priority (used for MX, SRV) |
| `managed_by` | string | `custom` or `auto` |
| `source_type` | string | For auto records: `fqdn`, `email-mx`, `email-spf`, `email-dkim`, `email-dmarc`, `service-hostname` |
| `source_fqdn_id` | string | FQDN that triggered auto-creation (nullable) |
| `status` | string | Lifecycle status |

## Zone API Endpoints

| Method | Path | Response | Description |
|--------|------|----------|-------------|
| `GET` | `/zones` | 200, paginated | List all zones. Filters: `search`, `status`, `sort`, `order` |
| `POST` | `/zones` | 202 | Create zone (async) |
| `GET` | `/zones/{id}` | 200 | Get zone by ID |
| `PUT` | `/zones/{id}` | 200 | Update zone (sync). Currently only `tenant_id` |
| `DELETE` | `/zones/{id}` | 202 | Delete zone and all records (async) |
| `PUT` | `/zones/{id}/tenant` | 200 | Reassign zone to a different tenant (sync) |
| `POST` | `/zones/{id}/retry` | 202 | Retry a failed zone |

### Create Zone Request

```json
{
  "name": "example.com",
  "brand_id": "acme",
  "region_id": "osl-1",
  "tenant_id": "abc123"
}
```

If `tenant_id` is provided, `brand_id` is derived from the tenant automatically. If `tenant_id` is omitted, `brand_id` is required.

## Zone Record API Endpoints

| Method | Path | Response | Description |
|--------|------|----------|-------------|
| `GET` | `/zones/{zoneID}/records` | 200, paginated | List records in a zone |
| `POST` | `/zones/{zoneID}/records` | 202 | Create record (async) |
| `GET` | `/zone-records/{id}` | 200 | Get record by ID |
| `PUT` | `/zone-records/{id}` | 202 | Update record content/TTL/priority (async) |
| `DELETE` | `/zone-records/{id}` | 202 | Delete record (async) |
| `POST` | `/zone-records/{id}/retry` | 202 | Retry a failed record |

### Create Record Request

```json
{
  "type": "A",
  "name": "www.example.com",
  "content": "10.10.10.50",
  "ttl": 3600,
  "priority": null
}
```

Records created via the API are marked `managed_by: "custom"`. TTL defaults to 3600 if not specified.

## Zone Provisioning (CreateZoneWorkflow)

When a zone is created, the Temporal workflow:

1. Sets status to `provisioning`
2. Fetches the zone and its brand
3. Creates the zone in the PowerDNS `domains` table (type: `NATIVE`)
4. Creates a **SOA record**: `{brand.primary_ns} {brand.hostmaster_email} 1 10800 3600 604800 300` (TTL: 86400)
5. Creates a **primary NS record** pointing to `brand.primary_ns` (TTL: 86400)
6. Creates a **secondary NS record** pointing to `brand.secondary_ns` (TTL: 86400)
7. Sets status to `active`

SOA and NS values come from the brand configuration (`primary_ns`, `secondary_ns`, `hostmaster_email`).

## Zone Deletion (DeleteZoneWorkflow)

1. Sets status to `deleting`
2. Looks up the PowerDNS domain ID
3. Deletes all records for the domain from PowerDNS
4. Deletes the zone from the PowerDNS `domains` table
5. Sets status to `deleted`

Delete is idempotent -- if the zone does not exist in PowerDNS, it skips straight to marking deleted.

## Auto-DNS (Platform-Managed Records)

When an FQDN is bound to a webroot via `BindFQDNWorkflow`, the platform automatically creates DNS records:

1. Walks up the domain hierarchy to find a matching zone (e.g., for `www.example.com` it checks `www.example.com`, then `example.com`, then `com`)
2. If a matching zone exists and no custom A/AAAA records exist for the FQDN:
   - Creates **A records** pointing to the cluster's load balancer IPv4 addresses
   - Creates **AAAA records** pointing to the cluster's load balancer IPv6 addresses
   - TTL: 300 seconds
3. Records are marked `managed_by: "auto"` with `source_type: "fqdn"` and `source_fqdn_id` set to the originating FQDN

When an FQDN is unbound (`UnbindFQDNWorkflow`), the auto-managed A and AAAA records are automatically deleted.

**Custom records take precedence**: if a user has already created A/AAAA records for the FQDN, auto-DNS is skipped.

## Auto-Email DNS

When an email account is created on an FQDN, the platform automatically creates:

- **MX record**: `{mail_hostname}` with priority 10, TTL 300 (`source_type: "email-mx"`)
- **TXT record (SPF)**: `v=spf1 mx ~all`, TTL 300 (`source_type: "email-spf"`)
- **TXT record (DKIM)**: DKIM key record if brand has `dkim_selector` and `dkim_public_key` (`source_type: "email-dkim"`)
- **TXT record (DMARC)**: `_dmarc` TXT record if brand has `dmarc_policy` (`source_type: "email-dmarc"`)

All are marked `managed_by: "auto"`. When email is removed from an FQDN, records are cleaned up.

## Service Hostname DNS

When a tenant is provisioned, the platform creates DNS records for service hostnames:

- `ssh.{tenant}.{base_hostname}` -> web node IPs
- `sftp.{tenant}.{base_hostname}` -> web node IPs
- `mysql.{tenant}.{base_hostname}` -> database node IPs
- `web.{tenant}.{base_hostname}` -> load balancer IPs

These are marked `managed_by: "auto"` with `source_type: "service-hostname"` and stored in both core DB and PowerDNS.

## Custom vs Auto-Managed Records

| Property | Auto-Managed | Custom |
|----------|-------------|--------|
| `managed_by` | `auto` | `custom` |
| `source_type` | `fqdn`, `email-mx`, `email-spf`, `email-dkim`, `email-dmarc`, `service-hostname` | null |
| Created by | FQDN binding / email / tenant provisioning workflows | API (`POST /zones/{id}/records`) |
| Editable via API | No | Yes |
| Deletable via API | No | Yes |
| Auto-cleaned | Yes (on FQDN unbind / email removal) | No |
| `source_fqdn_id` | Set to originating FQDN ID (for FQDN/email records) | null |

## Tenant Reassignment

Zones can be reassigned to a different tenant (or detached from any tenant) without affecting the DNS data in PowerDNS:

```json
PUT /zones/{id}/tenant
{ "tenant_id": "new-tenant-id" }
```

Pass `null` for `tenant_id` to detach the zone. This is a synchronous operation -- no Temporal workflow is triggered.
