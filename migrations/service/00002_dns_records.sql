-- +goose Up
CREATE TABLE records (
    id        SERIAL PRIMARY KEY,
    domain_id INT NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    name      TEXT NOT NULL,
    type      TEXT NOT NULL,
    content   TEXT NOT NULL,
    ttl       INT NOT NULL DEFAULT 3600,
    prio      INT,
    disabled  BOOLEAN NOT NULL DEFAULT false,
    ordername TEXT,
    auth      BOOLEAN NOT NULL DEFAULT true
);
CREATE INDEX records_domain_id_idx ON records(domain_id);
CREATE INDEX records_name_type_idx ON records(name, type);

-- +goose Down
DROP TABLE records;
