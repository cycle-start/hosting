-- +goose Up
CREATE TABLE nodes (
    id          TEXT PRIMARY KEY,
    cluster_id  TEXT NOT NULL REFERENCES clusters(id),
    hostname    TEXT NOT NULL,
    ip_address  INET,
    ip6_address INET,
    roles       TEXT[] NOT NULL DEFAULT '{}',
    grpc_address TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE nodes;
