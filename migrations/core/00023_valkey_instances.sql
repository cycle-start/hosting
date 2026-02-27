-- +goose Up
CREATE TABLE valkey_instances (
    id             TEXT PRIMARY KEY,
    tenant_id       TEXT NOT NULL REFERENCES tenants(id),
    subscription_id TEXT NOT NULL REFERENCES subscriptions(id),
    shard_id       TEXT REFERENCES shards(id),
    port           INTEGER NOT NULL,
    max_memory_mb  INTEGER NOT NULL DEFAULT 64,
    password_hash  TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'pending',
    status_message TEXT,
    suspend_reason TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(shard_id, port)
);

-- +goose Down
DROP TABLE valkey_instances;
