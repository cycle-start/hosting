-- +goose Up
CREATE TABLE s3_buckets (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT REFERENCES tenants(id) ON DELETE SET NULL,
    name        TEXT NOT NULL,
    shard_id    TEXT REFERENCES shards(id),
    public      BOOLEAN NOT NULL DEFAULT false,
    quota_bytes BIGINT NOT NULL DEFAULT 0,
    status      TEXT NOT NULL DEFAULT 'pending',
    status_message TEXT,
    suspend_reason TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name)
);

-- +goose Down
DROP TABLE IF EXISTS s3_buckets;
