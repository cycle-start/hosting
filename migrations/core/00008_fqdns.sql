-- +goose Up
CREATE TABLE fqdns (
    id          TEXT PRIMARY KEY,
    fqdn        TEXT NOT NULL,
    webroot_id  TEXT NOT NULL REFERENCES webroots(id),
    ssl_enabled BOOLEAN NOT NULL DEFAULT true,
    status      TEXT NOT NULL DEFAULT 'pending',
    status_message TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX fqdns_fqdn_key ON fqdns (fqdn) WHERE status NOT IN ('deleted', 'deleting');

-- +goose Down
DROP TABLE fqdns;
