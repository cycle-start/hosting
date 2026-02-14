-- +goose Up
CREATE TABLE certificates (
    id         TEXT PRIMARY KEY,
    fqdn_id    TEXT NOT NULL REFERENCES fqdns(id),
    type       TEXT NOT NULL,
    cert_pem   TEXT,
    key_pem    TEXT,
    chain_pem  TEXT,
    issued_at  TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    status     TEXT NOT NULL DEFAULT 'pending',
    status_message TEXT,
    is_active  BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE certificates;
