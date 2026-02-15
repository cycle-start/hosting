-- +goose Up
CREATE TABLE database_access_rules (
    id TEXT PRIMARY KEY,
    database_id TEXT NOT NULL REFERENCES databases(id),
    cidr TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    status_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_database_access_rules_database ON database_access_rules(database_id);
CREATE UNIQUE INDEX idx_database_access_rules_database_cidr ON database_access_rules(database_id, cidr);

-- +goose Down
DROP TABLE IF EXISTS database_access_rules;
