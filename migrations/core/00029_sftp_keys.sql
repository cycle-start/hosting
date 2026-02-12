-- +goose Up
CREATE TABLE sftp_keys (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    public_key TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sftp_keys_tenant_id ON sftp_keys(tenant_id);
CREATE UNIQUE INDEX idx_sftp_keys_tenant_fingerprint ON sftp_keys(tenant_id, fingerprint) WHERE status != 'deleted';

-- +goose Down
DROP TABLE sftp_keys;
