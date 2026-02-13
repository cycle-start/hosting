-- +goose Up
CREATE TABLE fqdns (
    id          TEXT PRIMARY KEY,
    fqdn        TEXT NOT NULL UNIQUE,
    webroot_id  TEXT NOT NULL REFERENCES webroots(id),
    ssl_enabled BOOLEAN NOT NULL DEFAULT true,
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE fqdns;
