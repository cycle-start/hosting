-- +goose Up
CREATE TABLE tenant_services (
    id         TEXT PRIMARY KEY,
    tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    service    TEXT NOT NULL,
    node_id    TEXT NOT NULL REFERENCES nodes(id),
    hostname   TEXT NOT NULL,
    enabled    BOOLEAN NOT NULL DEFAULT true,
    status     TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, service)
);

-- +goose Down
DROP TABLE tenant_services;
