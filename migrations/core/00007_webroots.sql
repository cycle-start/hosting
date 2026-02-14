-- +goose Up
CREATE TABLE webroots (
    id              TEXT PRIMARY KEY,
    tenant_id       TEXT NOT NULL REFERENCES tenants(id),
    name            TEXT NOT NULL,
    runtime         TEXT NOT NULL,
    runtime_version TEXT NOT NULL,
    runtime_config  JSONB NOT NULL DEFAULT '{}',
    public_folder   TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'pending',
    status_message  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, name)
);

-- +goose Down
DROP TABLE webroots;
