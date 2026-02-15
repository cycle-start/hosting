-- +goose Up
CREATE TABLE cron_jobs (
    id                TEXT PRIMARY KEY,
    tenant_id         TEXT NOT NULL REFERENCES tenants(id),
    webroot_id        TEXT NOT NULL REFERENCES webroots(id),
    name              TEXT NOT NULL,
    schedule          TEXT NOT NULL,
    command           TEXT NOT NULL,
    working_directory TEXT NOT NULL DEFAULT '',
    enabled           BOOLEAN NOT NULL DEFAULT false,
    timeout_seconds   INT NOT NULL DEFAULT 3600,
    max_memory_mb     INT NOT NULL DEFAULT 512,
    status            TEXT NOT NULL DEFAULT 'pending',
    status_message    TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(webroot_id, name)
);

CREATE INDEX idx_cron_jobs_tenant_id ON cron_jobs(tenant_id);
CREATE INDEX idx_cron_jobs_webroot_id ON cron_jobs(webroot_id);

-- +goose Down
DROP TABLE cron_jobs;
