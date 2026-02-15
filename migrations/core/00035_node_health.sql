-- +goose Up
CREATE TABLE node_health (
    node_id        TEXT PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
    status         TEXT NOT NULL,
    checks         JSONB NOT NULL DEFAULT '{}',
    reconciliation JSONB,
    reported_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE drift_events (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_id    TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL,
    resource   TEXT NOT NULL,
    action     TEXT NOT NULL,
    detail     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_drift_events_node_id ON drift_events(node_id);
CREATE INDEX idx_drift_events_created_at ON drift_events(created_at);

-- +goose Down
DROP TABLE drift_events;
DROP TABLE node_health;
