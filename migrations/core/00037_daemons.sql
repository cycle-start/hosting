-- +goose Up
CREATE TABLE daemons (
    id              TEXT PRIMARY KEY,
    tenant_id       TEXT NOT NULL REFERENCES tenants(id),
    node_id         TEXT REFERENCES nodes(id),
    webroot_id      TEXT NOT NULL REFERENCES webroots(id),
    command         TEXT NOT NULL,
    proxy_path      TEXT,
    proxy_port      INT,
    num_procs       INT NOT NULL DEFAULT 1,
    stop_signal     TEXT NOT NULL DEFAULT 'TERM',
    stop_wait_secs  INT NOT NULL DEFAULT 30,
    max_memory_mb   INT NOT NULL DEFAULT 512,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    status          TEXT NOT NULL DEFAULT 'pending',
    status_message  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_daemons_tenant_id ON daemons(tenant_id);
CREATE INDEX idx_daemons_node_id ON daemons(node_id);
CREATE INDEX idx_daemons_webroot_id ON daemons(webroot_id);

-- +goose Down
DROP TABLE IF EXISTS daemons;
