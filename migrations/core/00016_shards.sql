-- +goose Up
CREATE TABLE shards (
    id         TEXT PRIMARY KEY,
    cluster_id TEXT NOT NULL REFERENCES clusters(id),
    name       TEXT NOT NULL,
    role       TEXT NOT NULL,              -- 'web', 'database', 'dns', 'email', 'valkey', 'storage', 'dbadmin', 'lb'
    lb_backend TEXT NOT NULL DEFAULT '',   -- HAProxy backend name
    config     JSONB NOT NULL DEFAULT '{}',
    status     TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(cluster_id, name)
);

-- +goose Down
DROP TABLE shards;
