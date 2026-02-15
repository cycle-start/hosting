-- +goose Up
CREATE TABLE databases (
    id         TEXT PRIMARY KEY,
    tenant_id  TEXT REFERENCES tenants(id) ON DELETE SET NULL,
    name       TEXT NOT NULL,
    node_id    TEXT REFERENCES nodes(id),
    status     TEXT NOT NULL DEFAULT 'pending',
    status_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name)
);

-- +goose Down
DROP TABLE databases;
