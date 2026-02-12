-- +goose Up
ALTER TABLE api_keys ADD COLUMN key_prefix TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE api_keys DROP COLUMN IF EXISTS key_prefix;
