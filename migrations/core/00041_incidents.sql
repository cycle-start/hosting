-- +goose Up

CREATE TABLE incidents (
    id              TEXT PRIMARY KEY,
    dedupe_key      TEXT NOT NULL,
    type            TEXT NOT NULL,
    severity        TEXT NOT NULL DEFAULT 'warning',
    status          TEXT NOT NULL DEFAULT 'open',
    title           TEXT NOT NULL,
    detail          TEXT NOT NULL DEFAULT '',
    resource_type   TEXT,
    resource_id     TEXT,
    source          TEXT NOT NULL,
    assigned_to     TEXT,
    resolution      TEXT,
    detected_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at     TIMESTAMPTZ,
    escalated_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_incidents_dedupe_open
    ON incidents (dedupe_key) WHERE status NOT IN ('resolved', 'cancelled');
CREATE INDEX idx_incidents_status ON incidents(status);
CREATE INDEX idx_incidents_severity ON incidents(severity);
CREATE INDEX idx_incidents_resource ON incidents(resource_type, resource_id);
CREATE INDEX idx_incidents_created_at ON incidents(created_at);

CREATE TABLE incident_events (
    id          TEXT PRIMARY KEY,
    incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    actor       TEXT NOT NULL,
    action      TEXT NOT NULL,
    detail      TEXT NOT NULL DEFAULT '',
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_incident_events_incident_id ON incident_events(incident_id);

CREATE TABLE capability_gaps (
    id              TEXT PRIMARY KEY,
    tool_name       TEXT NOT NULL UNIQUE,
    description     TEXT NOT NULL,
    category        TEXT NOT NULL DEFAULT 'remediation',
    occurrences     INT NOT NULL DEFAULT 1,
    status          TEXT NOT NULL DEFAULT 'open',
    implemented_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE incident_capability_gaps (
    incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    gap_id      TEXT NOT NULL REFERENCES capability_gaps(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (incident_id, gap_id)
);

-- +goose Down
DROP TABLE IF EXISTS incident_capability_gaps;
DROP TABLE IF EXISTS capability_gaps;
DROP TABLE IF EXISTS incident_events;
DROP TABLE IF EXISTS incidents;
