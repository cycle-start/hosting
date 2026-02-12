-- +goose Up
CREATE TABLE platform_config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT INTO platform_config (key, value) VALUES
    ('base_hostname', 'localhost'),
    ('primary_ns', 'ns1.hosting.localhost'),
    ('secondary_ns', 'ns2.hosting.localhost'),
    ('hostmaster_email', 'hostmaster.hosting.localhost');

-- +goose Down
DROP TABLE platform_config;
