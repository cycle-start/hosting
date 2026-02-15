-- +goose Up
CREATE TABLE tenant_egress_rules (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id),
    cidr TEXT NOT NULL,
    action TEXT NOT NULL DEFAULT 'deny',
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    status_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_tenant_egress_rules_tenant ON tenant_egress_rules(tenant_id);
CREATE UNIQUE INDEX idx_tenant_egress_rules_tenant_cidr ON tenant_egress_rules(tenant_id, cidr);

-- +goose Down
DROP TABLE IF EXISTS tenant_egress_rules;
