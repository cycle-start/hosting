-- +goose Up
CREATE TABLE nodes (
    id          TEXT PRIMARY KEY,
    cluster_id  TEXT NOT NULL REFERENCES clusters(id),
    hostname    TEXT NOT NULL,
    ip_address  INET,
    ip6_address INET,
    roles       TEXT[] NOT NULL DEFAULT '{}',
    grpc_address TEXT NOT NULL DEFAULT '',
    shard_index INT,
    status      TEXT NOT NULL DEFAULT 'active',
    last_health_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_nodes_shard_index ON nodes(shard_id, shard_index) WHERE shard_id IS NOT NULL AND shard_index IS NOT NULL;

-- +goose Down
DROP TABLE nodes;
