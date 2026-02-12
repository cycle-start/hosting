-- +goose Up
CREATE TABLE platform_config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT INTO platform_config (key, value) VALUES ('base_hostname', 'localhost');

-- +goose Down
DROP TABLE platform_config;
