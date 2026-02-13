-- +goose Up
CREATE TABLE infrastructure_services (
    id              TEXT PRIMARY KEY,
    cluster_id      TEXT NOT NULL REFERENCES clusters(id),
    host_machine_id TEXT NOT NULL REFERENCES host_machines(id),
    service_type    TEXT NOT NULL,       -- 'haproxy', 'powerdns', 'valkey'
    container_id    TEXT NOT NULL DEFAULT '',
    container_name  TEXT NOT NULL,
    image           TEXT NOT NULL,
    config          JSONB NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'pending',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(cluster_id, service_type)
);

-- +goose Down
DROP TABLE infrastructure_services;
