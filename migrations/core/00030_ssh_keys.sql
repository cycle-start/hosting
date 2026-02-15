-- +goose Up
CREATE TABLE ssh_keys (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    public_key TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    status_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_ssh_keys_tenant_id ON ssh_keys(tenant_id);
CREATE UNIQUE INDEX idx_ssh_keys_tenant_fingerprint ON ssh_keys(tenant_id, fingerprint);

-- +goose Down
DROP TABLE ssh_keys;
