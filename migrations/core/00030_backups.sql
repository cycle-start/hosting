-- +goose Up
CREATE TABLE backups (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    type TEXT NOT NULL, -- 'web' or 'database'
    source_id TEXT NOT NULL, -- webroot_id or database_id
    source_name TEXT NOT NULL, -- human-readable name for display
    storage_path TEXT NOT NULL DEFAULT '',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_backups_tenant_id ON backups(tenant_id);

-- +goose Down
DROP TABLE backups;
