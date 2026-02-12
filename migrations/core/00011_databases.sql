-- +goose Up
CREATE TABLE databases (
    id         TEXT PRIMARY KEY,
    tenant_id  TEXT REFERENCES tenants(id) ON DELETE SET NULL,
    name       TEXT NOT NULL,
    node_id    TEXT REFERENCES nodes(id),
    status     TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(node_id, name)
);

-- +goose Down
DROP TABLE databases;
