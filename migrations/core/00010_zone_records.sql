-- +goose Up
CREATE TABLE zone_records (
    id             TEXT PRIMARY KEY,
    zone_id        TEXT NOT NULL REFERENCES zones(id) ON DELETE CASCADE,
    type           TEXT NOT NULL,
    name           TEXT NOT NULL,
    content        TEXT NOT NULL,
    ttl            INT NOT NULL DEFAULT 3600,
    priority       INT,
    managed_by     TEXT NOT NULL DEFAULT 'user',
    source_fqdn_id TEXT REFERENCES fqdns(id) ON DELETE SET NULL,
    status         TEXT NOT NULL DEFAULT 'pending',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE zone_records;
