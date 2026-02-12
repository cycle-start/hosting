-- +goose Up
CREATE TABLE domains (
    id              SERIAL PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,
    master          TEXT,
    last_check      INT,
    type            TEXT NOT NULL DEFAULT 'NATIVE',
    notified_serial INT,
    account         TEXT
);

-- +goose Down
DROP TABLE domains;
