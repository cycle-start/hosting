-- +goose Up
CREATE TABLE wireguard_peers (
    id              TEXT PRIMARY KEY,
    tenant_id       TEXT NOT NULL REFERENCES tenants(id),
    subscription_id TEXT NOT NULL REFERENCES subscriptions(id),
    name            TEXT NOT NULL,
    public_key      TEXT NOT NULL,
    preshared_key   TEXT NOT NULL,
    assigned_ip     TEXT NOT NULL,
    peer_index      INTEGER NOT NULL,
    endpoint        TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'pending',
    status_message  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, name)
);
