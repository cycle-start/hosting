# Multi-Brand Isolation

Brands are the fundamental isolation boundary in the hosting platform. Every tenant, zone, FQDN, and email resource is scoped to a brand. A single platform installation can serve multiple independent hosting brands, each with its own DNS configuration, cluster access, and API keys.

## What is a Brand?

A brand represents an independent hosting identity -- for example, a reseller or white-label hosting product. Each brand defines:

| Field | Purpose |
|-------|---------|
| `id` | Caller-provided slug (e.g. `"acme"`, `"myhost"`). Immutable after creation. |
| `name` | Human-readable display name |
| `base_hostname` | Base domain for generating tenant hostnames (e.g. `hosting.test`) |
| `primary_ns` | Primary nameserver hostname for SOA records (e.g. `ns1.acme.com`) |
| `secondary_ns` | Secondary nameserver hostname for SOA records |
| `hostmaster_email` | Hostmaster contact for SOA records (e.g. `hostmaster@acme.com`) |

These fields drive DNS zone SOA records and default hostname generation for all resources under the brand. Updating NS hostnames or hostmaster email on a brand affects all zones under it on the next convergence cycle.

## Brand Model

```go
type Brand struct {
    ID              string    `json:"id"`
    Name            string    `json:"name"`
    BaseHostname    string    `json:"base_hostname"`
    PrimaryNS       string    `json:"primary_ns"`
    SecondaryNS     string    `json:"secondary_ns"`
    HostmasterEmail string    `json:"hostmaster_email"`
    Status          string    `json:"status"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}
```

## Cluster Access Control

Brands can be restricted to specific clusters. This controls where tenants under the brand can be provisioned.

- **No restriction (default):** An empty cluster list means the brand can use any cluster.
- **Restricted:** When cluster IDs are set, tenant creation is rejected if the target cluster is not in the allowed list.

The cluster allowlist is managed via dedicated endpoints:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/brands/{id}/clusters` | List allowed cluster IDs |
| PUT | `/brands/{id}/clusters` | Replace the allowed cluster list |

## API Key Brand Scoping

Every API key carries a `brands` array that controls which brands it can access. This is enforced at the middleware level on every authenticated request.

```go
type APIKey struct {
    ID      string   `json:"id"`
    Name    string   `json:"name"`
    Scopes  []string `json:"scopes"`
    Brands  []string `json:"brands"`
    // ...
}
```

### Brand access rules

- **Wildcard `*`** -- Platform admin. Can access all brands and platform-level resources.
- **Specific brands** (e.g. `["acme", "myhost"]`) -- Can only access resources under those brands.

The `HasBrandAccess` function checks whether an identity can access a given brand:

```go
func HasBrandAccess(identity *APIKeyIdentity, brandID string) bool {
    for _, b := range identity.Brands {
        if b == "*" || b == brandID {
            return true
        }
    }
    return false
}
```

### Platform admin detection

The `IsPlatformAdmin` function checks for wildcard brand access. Platform admins can manage brands themselves, access infrastructure resources (regions, clusters, nodes), and perform cross-brand operations.

```go
func IsPlatformAdmin(identity *APIKeyIdentity) bool {
    for _, b := range identity.Brands {
        if b == "*" {
            return true
        }
    }
    return false
}
```

The `RequirePlatformAdmin` middleware enforces this on routes that require platform-level access.

### Brand-scoped queries

The `BrandIDs` helper extracts the identity's brand list for use in database query filtering. It returns `nil` for platform admins (no filtering), allowing queries to skip the brand filter for wildcard keys:

```go
func BrandIDs(ctx context.Context) []string {
    identity := GetIdentity(ctx)
    for _, b := range identity.Brands {
        if b == "*" {
            return nil  // no filter for platform admins
        }
    }
    return identity.Brands
}
```

## Scope-Based Authorization

In addition to brand scoping, API keys have fine-grained scopes in the format `resource:action`. The `RequireScope` middleware checks that the key has the required scope before allowing access.

- **Wildcard `*:*`** -- Full access to all resources and actions.
- **Specific scopes** (e.g. `tenants:read`, `zones:write`) -- Grants access to individual resource types.

Brand access and scope checks are independent -- a request must pass both to succeed.

## Authentication Flow

1. Client sends `X-API-Key` header
2. Auth middleware hashes the key (SHA-256) and looks up `api_keys` table
3. Key must exist and not be revoked (`revoked_at IS NULL`)
4. The identity (ID, scopes, brands) is stored in the request context
5. Downstream middleware checks scopes via `RequireScope`
6. Handlers check brand access via `HasBrandAccess` or filter results via `BrandIDs`

## Brand API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/brands` | List brands (paginated, searchable, sortable) |
| POST | `/brands` | Create brand (201 Created) |
| GET | `/brands/{id}` | Get brand by slug ID |
| PUT | `/brands/{id}` | Partial update (only provided fields change) |
| DELETE | `/brands/{id}` | Delete brand (must have no tenants or zones) |
| GET | `/brands/{id}/clusters` | List allowed clusters |
| PUT | `/brands/{id}/clusters` | Set allowed clusters |

## Isolation Guarantees

- Uniqueness constraints (e.g. zone names, FQDN hostnames) are per-brand, not global. Two brands can host the same domain name independently.
- DNS requires separate PowerDNS databases per brand because zone name is the primary lookup key and cannot be duplicated within a single database.
- Brand deletion is guarded -- a brand cannot be deleted while it has tenants or zones.
- API keys scoped to one brand cannot read, modify, or even discover resources belonging to another brand.
