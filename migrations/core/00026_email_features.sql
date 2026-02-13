-- +goose Up

CREATE TABLE email_aliases (
    id               TEXT PRIMARY KEY,
    email_account_id TEXT NOT NULL REFERENCES email_accounts(id),
    address          TEXT NOT NULL UNIQUE,
    status           TEXT NOT NULL DEFAULT 'pending',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE email_forwards (
    id               TEXT PRIMARY KEY,
    email_account_id TEXT NOT NULL REFERENCES email_accounts(id),
    destination      TEXT NOT NULL,
    keep_copy        BOOLEAN NOT NULL DEFAULT true,
    status           TEXT NOT NULL DEFAULT 'pending',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(email_account_id, destination)
);

CREATE TABLE email_autoreplies (
    id               TEXT PRIMARY KEY,
    email_account_id TEXT NOT NULL REFERENCES email_accounts(id) UNIQUE,
    subject          TEXT NOT NULL DEFAULT '',
    body             TEXT NOT NULL DEFAULT '',
    start_date       TIMESTAMPTZ,
    end_date         TIMESTAMPTZ,
    enabled          BOOLEAN NOT NULL DEFAULT false,
    status           TEXT NOT NULL DEFAULT 'pending',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down

DROP TABLE IF EXISTS email_autoreplies;
DROP TABLE IF EXISTS email_forwards;
DROP TABLE IF EXISTS email_aliases;
