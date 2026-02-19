-- +goose Up

-- Signing keys (RSA-2048)
CREATE TABLE oidc_signing_keys (
    id TEXT PRIMARY KEY,
    algorithm TEXT NOT NULL DEFAULT 'RS256',
    public_key_pem TEXT NOT NULL,
    private_key_pem TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- OIDC clients (e.g., DB admin tools via oauth2-proxy)
CREATE TABLE oidc_clients (
    id TEXT PRIMARY KEY,
    secret_hash TEXT NOT NULL,
    name TEXT NOT NULL,
    redirect_uris TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Authorization codes (short-lived)
CREATE TABLE oidc_auth_codes (
    code TEXT PRIMARY KEY,
    client_id TEXT NOT NULL REFERENCES oidc_clients(id),
    tenant_id TEXT NOT NULL REFERENCES tenants(id),
    redirect_uri TEXT NOT NULL,
    scope TEXT NOT NULL DEFAULT 'openid',
    nonce TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Login sessions (passwordless auth for admin-initiated flows)
CREATE TABLE oidc_login_sessions (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id),
    database_id TEXT REFERENCES databases(id),
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS oidc_login_sessions;
DROP TABLE IF EXISTS oidc_auth_codes;
DROP TABLE IF EXISTS oidc_clients;
DROP TABLE IF EXISTS oidc_signing_keys;
