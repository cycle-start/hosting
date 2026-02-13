-- +goose Up
CREATE TABLE s3_access_keys (
    id                TEXT PRIMARY KEY,
    s3_bucket_id      TEXT NOT NULL REFERENCES s3_buckets(id) ON DELETE CASCADE,
    access_key_id     TEXT NOT NULL UNIQUE,
    secret_access_key TEXT NOT NULL,
    permissions       TEXT NOT NULL DEFAULT 'read-write',
    status            TEXT NOT NULL DEFAULT 'active',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS s3_access_keys;
