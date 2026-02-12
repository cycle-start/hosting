-- +goose Up
CREATE TABLE clusters (
    id            TEXT PRIMARY KEY,
    region_id     TEXT NOT NULL REFERENCES regions(id),
    name          TEXT NOT NULL,
    config        JSONB NOT NULL DEFAULT '{}',
    status        TEXT NOT NULL DEFAULT 'pending',
    spec          JSONB NOT NULL DEFAULT '{}',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(region_id, name)
);

-- +goose Down
DROP TABLE clusters;
