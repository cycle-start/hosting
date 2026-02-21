# Control Panel Integration

The hosting platform provides infrastructure APIs that the control panel consumes. This document describes how the two systems are connected.

## Architecture

- **Hosting platform** (this repo): Owns all infrastructure — tenants, webroots, databases, DNS, email, etc. Exposes a REST API.
- **CRM / billing**: Owns customers, subscriptions, and invoices. Pushes subscription data to the control panel.
- **Control panel** (`../controlpanel`): Customer-facing UI. Owns users, customers, and access control. Reads/writes hosting resources via the core API. Caches subscription data from the CRM.

```
┌─────────┐   push subscriptions    ┌───────────────┐   fetch resources   ┌───────────────────┐
│   CRM   │ ──────────────────────> │ Control Panel │ ─────────────────> │ Hosting Platform  │
│         │   push brand modules    │  (Next.js)    │                    │   (Go REST API)   │
└─────────┘                         └───────────────┘                    └───────────────────┘
```

## Shared Identifiers

Both systems use deterministic IDs in dev seeds so they can reference the same entities without runtime discovery.

| Entity | ID | Defined In |
|---|---|---|
| Brand | `brand_acme_dev_000000000001` | `seeds/dev-tenants.yaml` |
| Customer | `cust_acme_dev_000000000001` | `seeds/dev-tenants.yaml` + control panel seed |
| Subscription | `sub_acme_dev_000000000001` | `seeds/dev-tenants.yaml` + control panel seed |

In production, the CRM generates customer/subscription IDs and passes them when creating tenants.

## Key Fields

### `customer_id` on tenants

Every tenant has a required `customer_id` field (opaque text, not a foreign key). This enables:

- `GET /tenants?customer_id=X` — list all tenants for a customer
- The control panel uses this to show a customer's hosting resources without maintaining a local mapping table

The hosting platform does not validate or interpret `customer_id` — it stores and filters by it.

### `id` on brands (optional)

The brand creation endpoint accepts an optional `id` field. If provided, that ID is used instead of generating a UUID. This allows the control panel and hosting platform to share a deterministic brand ID in dev.

## API Authentication

The control panel authenticates to the core API using a bearer token (`CORE_API_KEY`). In dev, this is a manually created API key. In production, each brand will have its own scoped API key.

## Subscription Cache

The control panel maintains a local cache of subscription data in its `customer_subscriptions` table. This serves two purposes:

1. **Offline product listings** — listing pages show product cards from local DB without calling the hosting API
2. **Graceful degradation** — if the hosting API is unreachable, cached subscription data still renders

### How it works

The CRM pushes subscription data to the control panel via `POST /api/internal/customer-subscriptions`. This is a full-replace sync per customer — all existing subscriptions for that customer are deleted and replaced with the new set.

Each subscription record includes:
- `id` — subscription ID (matches the hosting platform subscription)
- `tenant_id` — which hosting platform tenant this maps to
- `product_name` / `product_description` — display data for the product card
- `modules` — which control panel modules this subscription covers (`["webroots", "dns"]`, etc.)
- `status` — `active`, `suspended`, etc.

### Listing page behavior

When subscriptions are cached for a customer:
1. Listing pages (webroots, DNS, databases) show product cards from local DB
2. Each card links to `?tenant=<tenantId>` for drill-down into hosting API resources
3. The hosting API is only called when the user clicks into a specific product

When no subscriptions are cached (default):
1. Falls back to `GET /tenants?customer_id=X` to discover tenants
2. Fetches resources per tenant from the hosting API
3. Shows a `TenantFilter` dropdown for filtering

### Error handling

All hosting API calls are wrapped in try/catch. When the API is unreachable:
- With cached subscriptions: product cards render, drill-down shows error state
- Without cached subscriptions: listing pages show error state
- Detail pages: show "Unable to load details" error state

## Brand Module Gating

The CRM can disable specific modules per brand via `POST /api/internal/brand-modules`. When a module is disabled:

- The sidebar navigation link is hidden
- The page content is replaced with a "not available" message

Default: all modules (`webroots`, `dns`, `databases`) are enabled.

## Internal API Endpoints (CRM -> Control Panel)

These endpoints are on the control panel, called by the CRM to push data.

**Authentication:** `Authorization: Bearer <INTERNAL_API_KEY>` header.

| Method | Path | Description |
|---|---|---|
| POST | `/api/internal/brand-modules` | Set disabled modules for a brand |
| POST | `/api/internal/customer-subscriptions` | Full-replace subscriptions for a customer |

### POST /api/internal/brand-modules

```json
{
  "brand_id": "brand_acme_dev_000000000001",
  "disabled_modules": ["databases"]
}
```

### POST /api/internal/customer-subscriptions

```json
{
  "customer_id": "cust_123",
  "subscriptions": [
    {
      "id": "sub_abc",
      "tenant_id": "ten_xyz",
      "product_name": "Web Hosting Pro",
      "product_description": "Professional web hosting with SSL and email",
      "modules": ["webroots", "dns"],
      "status": "active"
    }
  ]
}
```

Empty `subscriptions: []` clears all subscriptions for that customer.

## Queries the Control Panel Makes

| Action | Endpoint |
|---|---|
| List customer's tenants | `GET /tenants?customer_id={id}` |
| Get tenant details | `GET /tenants/{id}` |
| List webroots | `GET /tenants/{id}/webroots` |
| Get webroot detail | `GET /webroots/{id}` |
| Get webroot FQDNs | `GET /webroots/{id}/fqdns` |
| Get webroot daemons | `GET /webroots/{id}/daemons` |
| Get webroot cron jobs | `GET /webroots/{id}/cron-jobs` |
| Get webroot env vars | `GET /webroots/{id}/env-vars` |
| List databases | `GET /tenants/{id}/databases` |
| Get database detail | `GET /databases/{id}` |
| Get database users | `GET /databases/{id}/users` |
| Get database access rules | `GET /databases/{id}/access-rules` |
| List DNS zones | `GET /zones` (brand-scoped) |
| Get zone detail | `GET /zones/{id}` |
| Get zone records | `GET /zones/{id}/records` |

## Dev Seed Workflow

1. Seed the hosting platform: `go run ./cmd/hostctl seed -f seeds/dev-tenants.yaml`
2. Seed the control panel: `bun run seed` (in the controlpanel repo)
3. Both use the same brand/customer/subscription IDs from the shared identifiers table above
4. The control panel seed inserts a sample subscription with a placeholder tenant ID — update it from the hosting platform's runtime-generated ID, or call the CRM push endpoint manually
