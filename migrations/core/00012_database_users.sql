-- +goose Up
CREATE TABLE database_users (
    id          TEXT PRIMARY KEY,
    database_id TEXT NOT NULL REFERENCES databases(id) ON DELETE CASCADE,
    username    TEXT NOT NULL,
    password    TEXT NOT NULL,
    privileges  TEXT[] NOT NULL DEFAULT '{ALL}',
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE database_users;
