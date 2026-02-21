-- +goose Up
CREATE TABLE zones (
    id         TEXT PRIMARY KEY,
    brand_id   TEXT NOT NULL REFERENCES brands(id),
    tenant_id       TEXT NOT NULL REFERENCES tenants(id),
    subscription_id TEXT NOT NULL REFERENCES subscriptions(id),
    name       TEXT NOT NULL UNIQUE,
    region_id  TEXT NOT NULL REFERENCES regions(id),
    status     TEXT NOT NULL DEFAULT 'pending',
    status_message TEXT,
    suspend_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE zones;
