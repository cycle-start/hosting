# Subscriptions

A **subscription** is a thin grouping layer that maps hosting resources to commercial orders. It serves as the integration seam between the hosting platform, control panels, and CRM/billing systems. Every billable resource (webroot, database, Valkey instance, S3 bucket, zone, email account) must belong to a subscription.

Subscriptions are scoped to a single tenant -- a subscription cannot span multiple tenants.

## Model

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Caller-provided UUID (CRM generates for idempotency) |
| `tenant_id` | string | Owning tenant |
| `name` | string | Human-readable name (e.g. `main`, `addon-2`) |
| `status` | string | `active` or `deleting` |
| `created_at` | time | Creation timestamp |
| `updated_at` | time | Last update timestamp |

**Constraints:**
- `UNIQUE(tenant_id, name)` -- subscription names are unique per tenant
- `id` is caller-provided (not auto-generated) to support CRM idempotency
- Status defaults to `active` on creation

## API Endpoints

| Method | Path | Response | Description |
|--------|------|----------|-------------|
| `GET` | `/tenants/{tenantID}/subscriptions` | 200, paginated | List subscriptions for a tenant |
| `GET` | `/subscriptions/{id}` | 200 | Get subscription by ID |
| `POST` | `/tenants/{tenantID}/subscriptions` | 201 | Create subscription (synchronous) |
| `DELETE` | `/subscriptions/{id}` | 202 | Delete subscription (async, triggers workflow) |

### Create Request

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "main"
}
```

The `id` is required and must be a valid UUID. It is provided by the caller (typically the CRM) to ensure idempotency. Creation is synchronous and returns 201 -- no Temporal workflow is triggered.

### Delete Cascade

Deleting a subscription triggers `DeleteSubscriptionWorkflow`, which deletes all child resources in the following order:

1. Email accounts (and their aliases, forwards, auto-replies)
2. Webroots (and their FQDNs, certificates)
3. Databases (and their users)
4. Valkey instances (and their users)
5. S3 buckets (and their access keys)
6. Zones (and their records)

Each child resource deletion triggers its own workflow. The subscription is marked `deleting` during this process and `deleted` when all children have been removed.

## Nested Creation

Subscriptions can be created inline when creating a tenant via `POST /tenants`:

```json
{
  "brand_id": "acme",
  "region_id": "osl-1",
  "cluster_id": "prod-1",
  "subscriptions": [
    { "id": "550e8400-e29b-41d4-a716-446655440000", "name": "main" }
  ],
  "webroots": [{
    "subscription_id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "main",
    "runtime": "php",
    "runtime_version": "8.5"
  }]
}
```

## Resource Binding

The following resource types reference a subscription via `subscription_id` (required on create):

- Webroots
- Databases
- Valkey instances
- S3 buckets
- Zones
- Email accounts

This binding is informational for the hosting platform -- it groups resources for billing and lifecycle management. The CRM uses the subscription ID to correlate hosting resources with commercial orders.
