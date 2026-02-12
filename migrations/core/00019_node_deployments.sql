-- +goose Up
CREATE TABLE node_deployments (
    id              TEXT PRIMARY KEY,
    node_id         TEXT NOT NULL UNIQUE REFERENCES nodes(id),
    host_machine_id TEXT NOT NULL REFERENCES host_machines(id),
    profile_id      TEXT NOT NULL REFERENCES node_profiles(id),
    container_id    TEXT NOT NULL DEFAULT '',
    container_name  TEXT NOT NULL,
    image_digest    TEXT NOT NULL DEFAULT '',
    env_overrides   JSONB NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'pending',
    deployed_at     TIMESTAMPTZ,
    last_health_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE node_deployments;
