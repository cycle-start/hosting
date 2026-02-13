-- +goose Up
CREATE TABLE valkey_users (
    id                 TEXT PRIMARY KEY,
    valkey_instance_id TEXT NOT NULL REFERENCES valkey_instances(id) ON DELETE CASCADE,
    username           TEXT NOT NULL,
    password           TEXT NOT NULL,
    privileges         TEXT[] NOT NULL DEFAULT '{+@all}',
    key_pattern        TEXT NOT NULL DEFAULT '~*',
    status             TEXT NOT NULL DEFAULT 'pending',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE valkey_users;
