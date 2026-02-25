# Subscriptions & FQDN Decoupling - Admin UI Handoff

This document describes all API changes from the Subscriptions & FQDN Decoupling feature that need to be reflected in the admin UI (React SPA in `web/admin/`). It is self-contained and assumes no prior context about the hosting platform internals.

## Background

The hosting platform is a Go REST API that manages tenants and their resources (webroots, databases, DNS zones, email accounts, Valkey/Redis instances, S3 buckets, etc.). The admin UI is a React SPA using TanStack Query for data fetching, with types in `web/admin/src/lib/types.ts`, hooks in `web/admin/src/lib/hooks.ts`, and an API client in `web/admin/src/lib/api.ts`.

Two changes were made to the backend:

1. **Subscriptions** -- a new resource type that groups related resources within a tenant (e.g., "a web hosting package"). Every resource (webroot, database, email account, etc.) now belongs to a subscription.
2. **FQDN decoupling** -- FQDNs (domain names) are now tenant-scoped instead of webroot-scoped. An FQDN can exist independently (e.g., for email-only domains) and optionally bind to a webroot.

---

## API Conventions

All endpoints are under `/api/v1`. Auth is via `Authorization: Bearer <api-key>` header.

**Pagination**: List endpoints return:
```json
{
  "items": [...],
  "has_more": true,
  "next_cursor": "last-item-id"
}
```
Query params: `?limit=50&cursor=<id>` for pagination. Some endpoints also support `?search=`, `?status=`, `?sort=`, `?order=asc|desc`.

**Errors**: All errors return:
```json
{ "error": "human-readable message" }
```
Common status codes: 400 (validation), 404 (not found), 409 (conflict/duplicate), 500 (server error).

**Async operations**: Create/delete operations that trigger background provisioning return `202 Accepted`. The resource will have `status: "pending"` or `status: "deleting"` until the background work completes.

---

## New Endpoints

### Subscriptions

#### `GET /api/v1/tenants/{tenantID}/subscriptions`
List subscriptions for a tenant. Paginated.

**Response** (200):
```json
{
  "items": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "tenant_id": "t_abc123",
      "name": "main-hosting",
      "status": "active",
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  ],
  "has_more": false,
  "next_cursor": ""
}
```

#### `GET /api/v1/subscriptions/{id}`
Get a single subscription by ID.

**Response** (200): Single subscription object (same shape as items above).

#### `POST /api/v1/tenants/{tenantID}/subscriptions`
Create a subscription. **Synchronous** -- returns 201 immediately (no background provisioning).

**Request body**:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "main-hosting"
}
```

Both fields are required. The `id` is **caller-provided** (the CRM generates it for idempotency). If a subscription with the same ID already exists, the API returns 409 Conflict.

**Response** (201): The created subscription object.

#### `DELETE /api/v1/subscriptions/{id}`
Delete a subscription and cascade-delete all child resources. **Async** -- returns 202 Accepted.

The subscription transitions to `status: "deleting"`, then all resources with this `subscription_id` (webroots, databases, email accounts, zones, valkey instances, s3 buckets) are deleted via background workflows. Once all children are gone, the subscription itself is deleted from the database.

**Response**: 202 (no body).

### Subscription Model

```typescript
interface Subscription {
  id: string          // UUID, caller-provided
  tenant_id: string   // Parent tenant ID
  name: string        // Human-readable name (e.g., "main-hosting")
  status: string      // "active" | "deleting"
  created_at: string  // ISO 8601
  updated_at: string  // ISO 8601
}
```

Constraints: `UNIQUE(tenant_id, name)` -- subscription names must be unique within a tenant. A subscription cannot span multiple tenants.

---

## Changed FQDN Endpoints

FQDNs are now **tenant-scoped** with an optional webroot binding, rather than being webroot-scoped.

### `GET /api/v1/tenants/{tenantID}/fqdns` (NEW)
List all FQDNs for a tenant. Paginated.

### `POST /api/v1/tenants/{tenantID}/fqdns` (MOVED)
Create an FQDN under a tenant. Previously this was `POST /webroots/{webrootID}/fqdns` (now removed).

**Request body**:
```json
{
  "fqdn": "example.com",
  "webroot_id": "wr_xxx",
  "ssl_enabled": true,
  "email_accounts": [
    {
      "subscription_id": "sub-uuid",
      "address": "info@example.com",
      "display_name": "Info",
      "quota_bytes": 1073741824
    }
  ]
}
```

- `fqdn` (required): The domain name.
- `webroot_id` (optional, nullable): Bind to a webroot on creation. Can be `null` or omitted for unbound FQDNs (e.g., email-only domains).
- `ssl_enabled` (optional, default false): Whether SSL is enabled.
- `email_accounts` (optional): Nested email account creation.

**Response** (202): The created FQDN object.

### `GET /api/v1/fqdns/{id}` (UNCHANGED)
Get a single FQDN by ID.

### `PUT /api/v1/fqdns/{id}` (NEW)
Update an FQDN. Used to bind/unbind webroot and toggle SSL.

**Request body**:
```json
{
  "webroot_id": "wr_xxx",
  "ssl_enabled": true
}
```

To unbind from a webroot, send `"webroot_id": null`. Both fields are optional -- only provided fields are updated.

**Response** (200): The updated FQDN object.

### `GET /api/v1/webroots/{webrootID}/fqdns` (KEPT)
Convenience filter to list FQDNs bound to a specific webroot. Still works as before.

### `DELETE /api/v1/fqdns/{id}` (UNCHANGED)
Delete an FQDN. Returns 202.

### `POST /api/v1/fqdns/{id}/retry` (UNCHANGED)
Retry a failed FQDN provisioning.

### Removed FQDN Endpoint
- `POST /api/v1/webroots/{webrootID}/fqdns` -- **REMOVED**. Replaced by `POST /api/v1/tenants/{tenantID}/fqdns`.

### FQDN Model

```typescript
interface FQDN {
  id: string
  tenant_id: string           // NEW: always present
  fqdn: string
  webroot_id: string | null   // CHANGED: now nullable (was required)
  ssl_enabled: boolean
  status: string              // "pending" | "provisioning" | "active" | "failed" | "deleting"
  status_message?: string
  created_at: string
  updated_at: string
}
```

---

## Changed Request Bodies

### `POST /api/v1/tenants` (Create Tenant)

New optional nested arrays added to the request body:

```json
{
  "brand_id": "brand-1",
  "region_id": "osl-1",
  "cluster_id": "cluster-1",
  "shard_id": "shard-web-1",
  "sftp_enabled": true,

  "subscriptions": [
    { "id": "sub-uuid-1", "name": "main-hosting" }
  ],
  "fqdns": [
    { "fqdn": "example.com", "ssl_enabled": true }
  ],
  "webroots": [
    {
      "subscription_id": "sub-uuid-1",
      "runtime": "php",
      "runtime_version": "8.5",
      "public_folder": "public",
      "fqdns": [
        { "fqdn": "www.example.com", "ssl_enabled": true }
      ]
    }
  ],
  "databases": [
    {
      "subscription_id": "sub-uuid-1",
      "shard_id": "shard-db-1",
      "users": [
        { "username": "app", "password": "secret123", "privileges": ["ALL"] }
      ]
    }
  ],
  "valkey_instances": [
    {
      "subscription_id": "sub-uuid-1",
      "shard_id": "shard-valkey-1",
      "max_memory_mb": 128
    }
  ],
  "s3_buckets": [
    {
      "subscription_id": "sub-uuid-1",
      "shard_id": "shard-s3-1"
    }
  ],
  "zones": [
    {
      "subscription_id": "sub-uuid-1",
      "name": "example.com"
    }
  ],
  "ssh_keys": [
    { "name": "deploy", "public_key": "ssh-ed25519 AAAA..." }
  ],
  "egress_rules": [
    { "cidr": "0.0.0.0/0", "description": "Allow all" }
  ]
}
```

Key changes:
- `subscriptions` array is new -- creates subscriptions inline during tenant creation.
- `fqdns` array is new at the top level -- creates tenant-scoped FQDNs not bound to any webroot.
- `subscription_id` is now required in `webroots`, `databases`, `valkey_instances`, `s3_buckets`, `zones`, and nested `email_accounts`.
- FQDNs nested inside webroots (`webroots[].fqdns[]`) are automatically bound to that webroot.

### All Standalone Resource Create Endpoints

These endpoints now require `subscription_id` in the request body:

| Endpoint | New Required Field |
|---|---|
| `POST /api/v1/tenants/{tenantID}/webroots` | `"subscription_id": "..."` |
| `POST /api/v1/tenants/{tenantID}/databases` | `"subscription_id": "..."` |
| `POST /api/v1/tenants/{tenantID}/valkey-instances` | `"subscription_id": "..."` |
| `POST /api/v1/tenants/{tenantID}/s3-buckets` | `"subscription_id": "..."` |
| `POST /api/v1/zones` | `"subscription_id": "..."` |
| `POST /api/v1/fqdns/{fqdnID}/email-accounts` | `"subscription_id": "..."` |

Example -- creating a webroot now requires:
```json
{
  "subscription_id": "550e8400-e29b-41d4-a716-446655440000",
  "runtime": "php",
  "runtime_version": "8.5"
}
```

### Zone Create Endpoint (`POST /api/v1/zones`)
Now requires both `tenant_id` and `subscription_id`:
```json
{
  "name": "example.com",
  "brand_id": "brand-1",
  "tenant_id": "t_abc123",
  "subscription_id": "sub-uuid",
  "region_id": "osl-1"
}
```

---

## Removed Endpoints

These endpoints no longer exist:

| Removed Endpoint | Reason |
|---|---|
| `PUT /api/v1/databases/{id}/tenant` | Resource reassignment removed. Resources belong to subscriptions now. |
| `PUT /api/v1/zones/{id}/tenant` | Same as above. |
| `PUT /api/v1/valkey-instances/{id}/tenant` | Same as above. |
| `POST /api/v1/webroots/{webrootID}/fqdns` | Replaced by `POST /api/v1/tenants/{tenantID}/fqdns`. |

---

## Changed Response Bodies

### All Resources with subscription_id

The following resource types now include `"subscription_id"` in their JSON responses:

- **Webroot**: `subscription_id: string`
- **Database**: `subscription_id: string`
- **ValkeyInstance**: `subscription_id: string`
- **S3Bucket**: `subscription_id: string`
- **Zone**: `subscription_id: string`
- **EmailAccount**: `subscription_id: string`

### Database, ValkeyInstance, S3Bucket, Zone

- `tenant_id` is now always present and required (was nullable/optional for some of these).
- `subscription_id` added.
- `suspend_reason` field added (string, empty when not suspended).

### FQDN

- `tenant_id` is now always present in responses.
- `webroot_id` is now nullable (`null` for unbound FQDNs).

---

## Complete Model Reference

### Subscription
```typescript
interface Subscription {
  id: string
  tenant_id: string
  name: string
  status: string        // "active" | "deleting"
  created_at: string
  updated_at: string
}
```

### Webroot
```typescript
interface Webroot {
  id: string
  tenant_id: string
  subscription_id: string     // NEW
  name: string
  runtime: string             // "php" | "node" | "python" | "ruby" | "static"
  runtime_version: string
  runtime_config: Record<string, unknown> | null
  public_folder: string
  env_file_name: string
  service_hostname_enabled: boolean
  status: string
  status_message?: string
  suspend_reason: string
  created_at: string
  updated_at: string
}
```

### FQDN
```typescript
interface FQDN {
  id: string
  tenant_id: string           // Always present
  fqdn: string
  webroot_id: string | null   // Nullable -- null means unbound
  ssl_enabled: boolean
  status: string
  status_message?: string
  created_at: string
  updated_at: string
}
```

### Database
```typescript
interface Database {
  id: string
  tenant_id: string
  subscription_id: string     // NEW
  name: string
  shard_id?: string | null
  node_id?: string | null
  status: string
  status_message?: string
  suspend_reason: string
  created_at: string
  updated_at: string
  shard_name?: string         // Computed, read-only
}
```

### ValkeyInstance
```typescript
interface ValkeyInstance {
  id: string
  tenant_id: string
  subscription_id: string     // NEW
  name: string
  shard_id?: string | null
  port: number
  max_memory_mb: number
  password?: string
  status: string
  status_message?: string
  suspend_reason: string
  created_at: string
  updated_at: string
  shard_name?: string         // Computed, read-only
}
```

### S3Bucket
```typescript
interface S3Bucket {
  id: string
  tenant_id: string
  subscription_id: string     // NEW
  name: string
  shard_id?: string | null
  public: boolean
  quota_bytes: number
  status: string
  status_message?: string
  suspend_reason: string
  created_at: string
  updated_at: string
  shard_name?: string         // Computed, read-only
}
```

### Zone
```typescript
interface Zone {
  id: string
  brand_id: string
  tenant_id: string           // Now always present (was nullable)
  subscription_id: string     // NEW
  tenant_name?: string | null
  name: string
  region_id: string
  status: string
  status_message?: string
  suspend_reason: string
  created_at: string
  updated_at: string
  region_name?: string        // Computed, read-only
}
```

### EmailAccount
```typescript
interface EmailAccount {
  id: string
  fqdn_id: string
  subscription_id: string     // NEW
  address: string
  display_name: string
  quota_bytes: number
  status: string
  status_message?: string
  created_at: string
  updated_at: string
}
```

---

## Resource Statuses

All resources use the same status set:

| Status | Meaning |
|---|---|
| `pending` | Created, waiting for provisioning to start |
| `provisioning` | Background workflow is running |
| `converging` | Node agent is applying configuration |
| `active` | Fully provisioned and operational |
| `failed` | Provisioning or convergence failed (retryable) |
| `suspended` | Administratively suspended |
| `deleting` | Deletion in progress |
| `auto_disabled` | Automatically disabled by the system |

Subscriptions only use `active` and `deleting`.

---

## Admin UI Changes Required

The existing admin UI code is in `web/admin/src/`. Here is a summary of all files that need updates.

### 1. Types (`web/admin/src/lib/types.ts`)

The `Subscription` type already exists in types.ts. The following types already have `subscription_id`:
- `Webroot`, `Database`, `ValkeyInstance`, `S3Bucket`, `Zone`, `EmailAccount`

The `FQDN` type already has `tenant_id` and nullable `webroot_id`.

**Verify these types match the models above.** Specifically check that `suspend_reason` is present on Webroot, Database, ValkeyInstance, S3Bucket, and Zone if it's not already there.

The form data types (`SubscriptionFormData`, `WebrootFormData`, `DatabaseFormData`, etc.) already include `subscription_id`. The `CreateTenantRequest` already includes `subscriptions` and `fqdns` arrays.

### 2. Hooks (`web/admin/src/lib/hooks.ts`)

**Missing hooks that need to be added:**

```typescript
// Subscriptions
export function useSubscriptions(tenantId: string) {
  return useQuery({
    queryKey: ['subscriptions', tenantId],
    queryFn: () => api.get<PaginatedResponse<Subscription>>(
      `/tenants/${tenantId}/subscriptions`
    ),
    enabled: !!tenantId,
  })
}

export function useSubscription(id: string) {
  return useQuery({
    queryKey: ['subscription', id],
    queryFn: () => api.get<Subscription>(`/subscriptions/${id}`),
    enabled: !!id,
  })
}

export function useCreateSubscription() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: { tenant_id: string; id: string; name: string }) =>
      api.post<Subscription>(`/tenants/${data.tenant_id}/subscriptions`, {
        id: data.id,
        name: data.name,
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['subscriptions'] }),
  })
}

export function useDeleteSubscription() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => api.delete(`/subscriptions/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['subscriptions'] }),
  })
}

// Tenant-scoped FQDNs
export function useTenantFQDNs(tenantId: string) {
  return useQuery({
    queryKey: ['tenant-fqdns', tenantId],
    queryFn: () => api.get<PaginatedResponse<FQDN>>(
      `/tenants/${tenantId}/fqdns`
    ),
    enabled: !!tenantId,
  })
}

export function useUpdateFQDN() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (data: {
      id: string;
      webroot_id?: string | null;
      ssl_enabled?: boolean
    }) => api.put<FQDN>(`/fqdns/${data.id}`, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['fqdns'] })
      qc.invalidateQueries({ queryKey: ['tenant-fqdns'] })
      qc.invalidateQueries({ queryKey: ['fqdn'] })
    },
  })
}
```

**Hooks that need updating:**

- `useCreateFQDN` -- currently posts to `/webroots/{webrootID}/fqdns`. Must change to `/tenants/{tenantID}/fqdns`:
  ```typescript
  // OLD (BROKEN):
  mutationFn: (data: { webroot_id: string; fqdn: string; ssl_enabled?: boolean }) =>
    api.post<FQDN>(`/webroots/${data.webroot_id}/fqdns`, data)

  // NEW:
  mutationFn: (data: { tenant_id: string; fqdn: string; webroot_id?: string; ssl_enabled?: boolean }) =>
    api.post<FQDN>(`/tenants/${data.tenant_id}/fqdns`, data)
  ```

- `useCreateWebroot` -- must include `subscription_id` in the mutation data type:
  ```typescript
  mutationFn: (data: {
    tenant_id: string;
    subscription_id: string;  // ADD THIS
    runtime: string;
    runtime_version: string;
    // ...rest
  }) => api.post<Webroot>(`/tenants/${data.tenant_id}/webroots`, data)
  ```

- `useCreateDatabase` -- must include `subscription_id`:
  ```typescript
  mutationFn: (data: {
    tenant_id: string;
    subscription_id: string;  // ADD THIS
    shard_id: string;
    users?: DatabaseUserFormData[]
  }) => api.post<Database>(`/tenants/${data.tenant_id}/databases`, data)
  ```

- `useCreateValkeyInstance` -- must include `subscription_id`:
  ```typescript
  mutationFn: (data: {
    tenant_id: string;
    subscription_id: string;  // ADD THIS
    shard_id: string;
    max_memory_mb?: number;
    users?: ValkeyUserFormData[]
  }) => api.post<ValkeyInstance>(`/tenants/${data.tenant_id}/valkey-instances`, data)
  ```

- `useCreateS3Bucket` -- must include `subscription_id`:
  ```typescript
  mutationFn: (data: {
    tenant_id: string;
    subscription_id: string;  // ADD THIS
    shard_id: string;
    public?: boolean;
    quota_bytes?: number
  }) => api.post<S3Bucket>(`/tenants/${data.tenant_id}/s3-buckets`, data)
  ```

- `useCreateZone` -- must include `subscription_id` and `tenant_id`:
  ```typescript
  mutationFn: (data: {
    name: string;
    region_id: string;
    brand_id?: string;
    tenant_id: string;          // NOW REQUIRED
    subscription_id: string;    // ADD THIS
  }) => api.post<Zone>('/zones', data)
  ```

- `useCreateEmailAccount` -- must include `subscription_id`:
  ```typescript
  mutationFn: (data: {
    fqdn_id: string;
    subscription_id: string;   // ADD THIS
    address: string;
    display_name?: string;
    quota_bytes?: number
  }) => api.post<EmailAccount>(`/fqdns/${data.fqdn_id}/email-accounts`, data)
  ```

### 3. Tenant Detail Page (`web/admin/src/pages/tenant-detail.tsx`)

Add a **Subscriptions** tab that:
- Lists all subscriptions for the tenant using `useSubscriptions(tenantId)`
- Shows subscription name, status, created date
- Provides a "Create Subscription" button (dialog with ID + name fields)
- Provides a "Delete Subscription" button with confirmation (warns about cascade deletion of all child resources)
- Optionally shows a count or list of resources grouped by subscription

Add a **FQDNs** tab that:
- Lists all FQDNs for the tenant using `useTenantFQDNs(tenantId)`
- Shows FQDN name, bound webroot (if any), SSL status, status
- Provides "Create FQDN" with optional webroot binding
- Provides "Edit FQDN" to bind/unbind webroot (using `useUpdateFQDN`)
- Provides "Delete FQDN" button

### 4. Tenant Creation Form (`web/admin/src/pages/create-tenant.tsx`)

The form already supports nested subscriptions, webroots, databases, etc. Verify that:
- The subscriptions section generates a UUID for the `id` field (use `crypto.randomUUID()`)
- The `subscription_id` dropdown/selector on webroots, databases, valkey, s3, zones references subscriptions defined earlier in the same form
- Top-level `fqdns` array is supported (FQDNs not bound to any webroot)

### 5. Resource Detail Pages and Lists

On every resource detail/list view, show the `subscription_id` field. Ideally display it as a link to the subscription detail, or at minimum show the subscription name by fetching it.

Resources affected:
- Webroot detail/list
- Database detail/list
- Valkey Instance detail/list
- S3 Bucket detail/list
- Zone detail/list
- Email Account detail/list

### 6. Resource Creation Forms

Every standalone resource creation form must now include a `subscription_id` field. This should be a required dropdown that lists the tenant's subscriptions (fetched via `useSubscriptions(tenantId)`).

Affected forms:
- Create Webroot
- Create Database
- Create Valkey Instance
- Create S3 Bucket
- Create Zone
- Create Email Account

### 7. FQDN Management on Webroot Detail

The webroot detail page currently shows FQDNs via `useFQDNs(webrootId)` which calls `GET /webroots/{webrootID}/fqdns`. This endpoint **still works** and returns FQDNs bound to that webroot. Keep this view, but:
- The "Create FQDN" button should now call `POST /tenants/{tenantID}/fqdns` with `webroot_id` set to the current webroot
- Add an "Unbind" action that calls `PUT /fqdns/{id}` with `{"webroot_id": null}`
- Add a "Bind existing FQDN" action that shows unbound FQDNs (where `webroot_id` is null) from `useTenantFQDNs(tenantId)` and calls `PUT /fqdns/{id}` with `{"webroot_id": "current-webroot-id"}`

---

## Testing Notes

- The dev API runs at `https://api.massive-hosting.com/api/v1`
- Create a subscription before creating resources -- all resource creation will fail validation without a valid `subscription_id`
- Subscription IDs must be valid UUIDs (use `crypto.randomUUID()` in the browser)
- Deleting a subscription cascades to ALL child resources -- the UI should show a strong warning
- FQDNs can now exist without a webroot -- test creating email-only domains
- The `useFQDNs(webrootId)` hook (webroot-scoped) and `useTenantFQDNs(tenantId)` hook (tenant-scoped) serve different purposes and should coexist
