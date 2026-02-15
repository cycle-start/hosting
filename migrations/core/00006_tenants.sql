-- +goose Up
CREATE TABLE tenants (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    brand_id     TEXT NOT NULL REFERENCES brands(id),
    region_id    TEXT NOT NULL REFERENCES regions(id),
    cluster_id   TEXT NOT NULL REFERENCES clusters(id),
    uid          INT NOT NULL UNIQUE,
    sftp_enabled BOOLEAN NOT NULL DEFAULT true,
    ssh_enabled  BOOLEAN NOT NULL DEFAULT false,
    disk_quota_bytes BIGINT NOT NULL DEFAULT 0,
    status       TEXT NOT NULL DEFAULT 'pending',
    status_message TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE SEQUENCE tenant_uid_seq START 5000;

-- +goose Down
DROP SEQUENCE tenant_uid_seq;
DROP TABLE tenants;
