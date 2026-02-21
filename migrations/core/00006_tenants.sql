-- +goose Up
CREATE TABLE tenants (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    brand_id     TEXT NOT NULL REFERENCES brands(id),
    customer_id  TEXT NOT NULL,
    region_id    TEXT NOT NULL REFERENCES regions(id),
    cluster_id   TEXT NOT NULL REFERENCES clusters(id),
    uid          INT NOT NULL UNIQUE,
    sftp_enabled BOOLEAN NOT NULL DEFAULT true,
    ssh_enabled  BOOLEAN NOT NULL DEFAULT false,
    disk_quota_bytes BIGINT NOT NULL DEFAULT 0,
    status       TEXT NOT NULL DEFAULT 'pending',
    status_message TEXT,
    suspend_reason TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_tenants_customer_id ON tenants(customer_id);

CREATE SEQUENCE tenant_uid_seq START 5000;

CREATE TABLE subscriptions (
    id         TEXT PRIMARY KEY,
    tenant_id  TEXT NOT NULL REFERENCES tenants(id),
    name       TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, name)
);

-- +goose Down
DROP TABLE subscriptions;
DROP SEQUENCE tenant_uid_seq;
DROP TABLE tenants;
