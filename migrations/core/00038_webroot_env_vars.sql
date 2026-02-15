-- +goose Up
CREATE TABLE tenant_encryption_keys (
    tenant_id     TEXT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    encrypted_dek TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE webroot_env_vars (
    id          TEXT PRIMARY KEY,
    webroot_id  TEXT NOT NULL REFERENCES webroots(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    value       TEXT NOT NULL,
    is_secret   BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(webroot_id, name)
);
CREATE INDEX idx_webroot_env_vars_webroot_id ON webroot_env_vars(webroot_id);

-- +goose Down
DROP TABLE webroot_env_vars;
DROP TABLE tenant_encryption_keys;
