-- +goose Up
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    resource_type TEXT,
    resource_id TEXT,
    status_code INT NOT NULL,
    request_body JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_logs_created_at ON audit_logs (created_at);
CREATE INDEX idx_audit_logs_resource_type ON audit_logs (resource_type) WHERE resource_type IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS audit_logs;
