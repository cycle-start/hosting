-- +goose Up
CREATE TABLE cluster_runtimes (
    cluster_id TEXT NOT NULL REFERENCES clusters(id),
    runtime    TEXT NOT NULL,
    version    TEXT NOT NULL,
    available  BOOLEAN NOT NULL DEFAULT true,
    PRIMARY KEY (cluster_id, runtime, version)
);

CREATE TABLE tenant_runtime_configs (
    id         TEXT PRIMARY KEY,
    tenant_id  TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    runtime    TEXT NOT NULL,
    pool_config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, runtime)
);

-- +goose Down
DROP TABLE tenant_runtime_configs;
DROP TABLE cluster_runtimes;
