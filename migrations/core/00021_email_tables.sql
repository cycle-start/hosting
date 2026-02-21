-- +goose Up
CREATE TABLE email_accounts (
    id           TEXT PRIMARY KEY,
    fqdn_id      TEXT NOT NULL REFERENCES fqdns(id),
    subscription_id TEXT NOT NULL REFERENCES subscriptions(id),
    address      TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    quota_bytes  BIGINT NOT NULL DEFAULT 0,
    status       TEXT NOT NULL DEFAULT 'pending',
    status_message TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE email_accounts;
