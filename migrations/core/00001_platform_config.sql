-- +goose Up
CREATE TABLE platform_config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- +goose Down
DROP TABLE platform_config;
