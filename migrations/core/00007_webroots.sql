-- +goose Up
CREATE TABLE webroots (
    id                       TEXT PRIMARY KEY,
    tenant_id                TEXT NOT NULL REFERENCES tenants(id),
    subscription_id          TEXT NOT NULL REFERENCES subscriptions(id),
    runtime                  TEXT NOT NULL,
    runtime_version          TEXT NOT NULL,
    runtime_config           JSONB NOT NULL DEFAULT '{}',
    public_folder            TEXT NOT NULL DEFAULT '',
    env_file_name            TEXT NOT NULL DEFAULT '.env.hosting',
    service_hostname_enabled BOOLEAN NOT NULL DEFAULT true,
    status                   TEXT NOT NULL DEFAULT 'pending',
    status_message           TEXT,
    suspend_reason           TEXT NOT NULL DEFAULT '',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE resource_usage (
    id            TEXT PRIMARY KEY,
    resource_type TEXT NOT NULL,
    resource_id   TEXT NOT NULL,
    tenant_id     TEXT NOT NULL REFERENCES tenants(id),
    bytes_used    BIGINT NOT NULL DEFAULT 0,
    collected_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(resource_type, resource_id)
);

-- +goose Down
DROP TABLE resource_usage;
DROP TABLE webroots;
