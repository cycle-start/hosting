-- +goose Up
CREATE TABLE cron_jobs (
    id                    TEXT PRIMARY KEY,
    tenant_id             TEXT NOT NULL REFERENCES tenants(id),
    webroot_id            TEXT NOT NULL REFERENCES webroots(id),
    name                  TEXT NOT NULL,
    schedule              TEXT NOT NULL,
    command               TEXT NOT NULL,
    working_directory     TEXT NOT NULL DEFAULT '',
    enabled               BOOLEAN NOT NULL DEFAULT false,
    timeout_seconds       INT NOT NULL DEFAULT 3600,
    max_memory_mb         INT NOT NULL DEFAULT 512,
    consecutive_failures  INT NOT NULL DEFAULT 0,
    max_failures          INT NOT NULL DEFAULT 5,
    status                TEXT NOT NULL DEFAULT 'pending',
    status_message        TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name)
);

CREATE INDEX idx_cron_jobs_tenant_id ON cron_jobs(tenant_id);
CREATE INDEX idx_cron_jobs_webroot_id ON cron_jobs(webroot_id);

CREATE TABLE cron_executions (
    id              TEXT PRIMARY KEY,
    cron_job_id     TEXT NOT NULL REFERENCES cron_jobs(id) ON DELETE CASCADE,
    node_id         TEXT NOT NULL,
    success         BOOLEAN NOT NULL,
    exit_code       INT,
    duration_ms     INT,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_cron_executions_cron_job_id ON cron_executions(cron_job_id);
CREATE INDEX idx_cron_executions_started_at ON cron_executions(started_at DESC);

-- +goose Down
DROP TABLE cron_executions;
DROP TABLE cron_jobs;
