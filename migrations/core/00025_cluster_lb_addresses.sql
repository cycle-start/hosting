-- +goose Up
CREATE TABLE cluster_lb_addresses (
    id         TEXT PRIMARY KEY,
    cluster_id TEXT NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    address    INET NOT NULL,
    family     INTEGER NOT NULL,
    label      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(cluster_id, address)
);

-- +goose Down
DROP TABLE cluster_lb_addresses;
