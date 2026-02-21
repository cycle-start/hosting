# Control Panel Integration

The hosting platform provides infrastructure APIs that the control panel consumes. This document describes how the two systems are connected.

## Architecture

- **Hosting platform** (this repo): Owns all infrastructure — tenants, webroots, databases, DNS, email, etc. Exposes a REST API.
- **Control panel** (`../controlpanel`): Customer-facing UI. Owns users, customers, and access control. Reads/writes hosting resources via the core API.

The control panel does not duplicate hosting data. It queries the core API on every request.

## Shared Identifiers

Both systems use deterministic IDs in dev seeds so they can reference the same entities without runtime discovery.

| Entity | ID | Defined In |
|---|---|---|
| Brand | `brand_acme_dev_000000000001` | `seeds/dev-tenants.yaml` |
| Customer | `cust_acme_dev_000000000001` | `seeds/dev-tenants.yaml` + control panel seed |
| Subscription | `sub_acme_dev_000000000001` | `seeds/dev-tenants.yaml` |

In production, the CRM or control panel generates customer/subscription IDs and passes them when creating tenants.

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

## Dev Seed Workflow

1. Seed the hosting platform: `go run ./cmd/hostctl seed -f seeds/dev-tenants.yaml`
2. Seed the control panel: `bun run seed` (in the controlpanel repo)
3. Both use the same brand/customer/subscription IDs from the table above

## Queries the Control Panel Makes

| Action | Endpoint |
|---|---|
| List customer's tenants | `GET /tenants?customer_id={id}` |
| Get tenant details | `GET /tenants/{id}` |
| List webroots | `GET /tenants/{id}/webroots` |
| List databases | `GET /tenants/{id}/databases` |
| Manage DNS zones | `GET/POST/PUT/DELETE /zones/*` |
| Manage email | `GET/POST/PUT/DELETE /email-accounts/*` |
