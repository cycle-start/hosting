-- +goose Up
CREATE TABLE host_machines (
    id              TEXT PRIMARY KEY,
    cluster_id      TEXT NOT NULL REFERENCES clusters(id),
    hostname        TEXT NOT NULL UNIQUE,
    ip_address      INET NOT NULL,
    docker_host     TEXT NOT NULL,
    ca_cert_pem     TEXT NOT NULL DEFAULT '',
    client_cert_pem TEXT NOT NULL DEFAULT '',
    client_key_pem  TEXT NOT NULL DEFAULT '',
    capacity        JSONB NOT NULL DEFAULT '{"max_nodes": 10}',
    roles           TEXT[] NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE host_machines;
