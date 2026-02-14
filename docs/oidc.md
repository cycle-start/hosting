# OIDC Provider

The hosting platform includes a built-in OpenID Connect (OIDC) provider that enables external applications to authenticate tenants. This is used to integrate third-party tools (e.g., CloudBeaver for database administration) that support OIDC-based single sign-on.

## Overview

The OIDC provider implements the authorization code flow (RFC 6749). It allows external applications (relying parties) to verify tenant identity without managing their own user databases. The platform acts as the identity provider -- tenants are the subjects.

Key characteristics:
- **Algorithm**: RS256 (RSA-SHA256)
- **Signing key**: RSA-2048, auto-generated on first use, persisted in the database
- **Authorization codes**: 32 bytes, URL-safe base64 encoded, expire after 60 seconds, single-use
- **Login sessions**: Short-lived (30 seconds), single-use, act as proof of identity
- **ID tokens**: JWT, 1-hour expiry, tenant ID as subject claim

## Endpoints

### Discovery
```
GET /.well-known/openid-configuration
```
No authentication required. Returns the standard OIDC discovery document with:
- `issuer` -- base API URL
- `authorization_endpoint` -- `{issuer}/oidc/authorize`
- `token_endpoint` -- `{issuer}/oidc/token`
- `jwks_uri` -- `{issuer}/oidc/jwks`
- Supported: `response_types: [code]`, `grant_types: [authorization_code]`, `scopes: [openid]`, `id_token_signing_alg: [RS256]`

### JWKS (JSON Web Key Set)
```
GET /oidc/jwks
```
No authentication required. Returns the public RSA key in JWK format for token signature verification. The signing key is auto-generated and stored in the database on first access.

### Authorize
```
GET /oidc/authorize?client_id=...&redirect_uri=...&login_hint=...&scope=openid&state=...&nonce=...
```
No API key required -- the `login_hint` (a login session ID) acts as proof of identity.

Required parameters:
- `client_id` -- registered OIDC client ID
- `redirect_uri` -- must match a registered redirect URI for the client
- `login_hint` -- login session ID (obtained via `POST /tenants/{id}/login-sessions`)

Optional parameters:
- `scope` -- defaults to `openid`
- `state` -- CSRF protection, returned as-is in the redirect
- `nonce` -- included in the ID token for replay protection

On success, redirects (`302 Found`) to `redirect_uri` with `code` and optionally `state` query parameters.

### Token
```
POST /oidc/token
Content-Type: application/x-www-form-urlencoded

grant_type=authorization_code&code=...&client_id=...&client_secret=...&redirect_uri=...
```
No API key required -- client authenticates via `client_id` + `client_secret`.

Returns a JSON response with `access_token`, `token_type`, and `id_token`. The access token and ID token are the same signed JWT.

## Client Management

### Register a client
```
POST /oidc/clients
Authorization: X-API-Key ...

{
  "id": "cloudbeaver",
  "secret": "my-secret",
  "name": "CloudBeaver",
  "redirect_uris": ["https://dbadmin.hosting.test/api/auth/callback"]
}
```
Returns `201 Created`. The client secret is stored as a bcrypt hash and cannot be retrieved after creation. If a client with the same ID already exists, it is updated (upsert).

## Login Sessions

Login sessions bridge the gap between the platform's API key authentication and the OIDC flow. They provide a secure, short-lived way to initiate authentication for a specific tenant.

### Create a login session
```
POST /tenants/{id}/login-sessions
Authorization: X-API-Key ...
```
Returns `201 Created` with `session_id` and `expires_at`. The session ID is then used as the `login_hint` parameter in the authorize request.

Session properties:
- Expires after 30 seconds
- Single-use (marked as used after the authorize endpoint consumes it)
- Validates that the session has not expired and has not been previously used

## Authorization Code Flow

The complete flow for authenticating a tenant via an external application:

```
1. Platform API creates a login session:
   POST /tenants/{tenantID}/login-sessions -> session_id

2. Platform redirects tenant's browser to the authorize endpoint:
   GET /oidc/authorize?client_id=...&redirect_uri=...&login_hint={session_id}&state=...&nonce=...

3. Authorize endpoint validates the login session, creates an auth code,
   and redirects to the client's redirect_uri:
   302 -> {redirect_uri}?code={auth_code}&state=...

4. The external application exchanges the code for an ID token:
   POST /oidc/token (client_id, client_secret, code, redirect_uri)
   -> { "id_token": "eyJ...", "access_token": "eyJ...", "token_type": "Bearer" }

5. The application verifies the ID token signature using the JWKS endpoint
   and extracts the tenant ID from the "sub" claim.
```

## ID Token Claims

The signed JWT ID token contains:

| Claim                | Value                        |
|----------------------|------------------------------|
| `iss`                | Platform issuer URL          |
| `sub`                | Tenant ID                    |
| `aud`                | Client ID                    |
| `exp`                | 1 hour from issuance         |
| `iat`                | Issuance timestamp           |
| `preferred_username` | Tenant ID                    |
| `groups`             | `[tenantID]`                 |
| `nonce`              | Echo of request nonce (if provided) |

## Signing Key Management

The RSA-2048 signing key is:
- Auto-generated on first request to any endpoint that needs it (JWKS, authorize, login sessions)
- Stored in the `oidc_signing_keys` table with PEM-encoded public and private keys
- Loaded from the database on subsequent startups (only one active key at a time)
- Protected by a mutex for concurrent access safety

## Data Model

- `oidc_signing_keys` -- RSA key pairs (id, algorithm, public/private PEM, active flag)
- `oidc_clients` -- registered relying parties (id, bcrypt secret hash, name, redirect URIs)
- `oidc_auth_codes` -- authorization codes (code, client_id, tenant_id, redirect_uri, scope, nonce, expires_at, used)
- `oidc_login_sessions` -- short-lived login sessions (id, tenant_id, expires_at, used)

## Source Files

- Discovery/JWKS/Authorize/Token: `internal/api/handler/oidc.go`
- Client management: `internal/api/handler/oidc_client.go`
- Login sessions: `internal/api/handler/oidc_login.go`
- Service (all OIDC logic): `internal/core/oidc.go`
- Models: `internal/model/oidc.go`
