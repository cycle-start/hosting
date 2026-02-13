-- +goose Up
CREATE TABLE node_profiles (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    role         TEXT NOT NULL,
    image        TEXT NOT NULL,
    env          JSONB NOT NULL DEFAULT '{}',
    volumes      JSONB NOT NULL DEFAULT '[]',
    ports        JSONB NOT NULL DEFAULT '[]',
    resources    JSONB NOT NULL DEFAULT '{"memory_mb": 2048, "cpu_shares": 1024}',
    health_check JSONB NOT NULL DEFAULT '{}',
    privileged   BOOLEAN NOT NULL DEFAULT false,
    network_mode TEXT NOT NULL DEFAULT 'bridge',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed default profiles
INSERT INTO node_profiles (id, name, role, image, env, volumes, ports, resources, health_check, privileged, network_mode) VALUES
('profile-web-default', 'web-default', 'web', 'registry.localhost:5000/hosting/web-node:latest',
 '{"POWERDNS_DATABASE_URL": ""}',
 '["/ceph/hosting:/srv/hosting:rw"]',
 '[{"host": 0, "container": 80}, {"host": 0, "container": 443}, {"host": 0, "container": 9090}]',
 '{"memory_mb": 2048, "cpu_shares": 1024}',
 '{"test": ["CMD", "curl", "-f", "http://localhost:9090/health"], "interval": "10s", "timeout": "5s", "retries": 3}',
 false, 'bridge'),
('profile-database-default', 'database-default', 'database', 'registry.localhost:5000/hosting/db-node:latest',
 '{"MYSQL_ROOT_PASSWORD": "rootpassword"}',
 '[]',
 '[{"host": 0, "container": 3306}, {"host": 0, "container": 9090}]',
 '{"memory_mb": 4096, "cpu_shares": 2048}',
 '{"test": ["CMD", "mysqladmin", "ping", "-h", "localhost"], "interval": "10s", "timeout": "5s", "retries": 3}',
 false, 'bridge'),
('profile-dns-default', 'dns-default', 'dns', 'registry.localhost:5000/hosting/dns-node:latest',
 '{"PDNS_API_KEY": "secret", "PDNS_GPGSQL_HOST": "hosting-powerdns-db", "PDNS_GPGSQL_PORT": "5432", "PDNS_GPGSQL_DBNAME": "hosting_powerdns", "PDNS_GPGSQL_USER": "hosting", "PDNS_GPGSQL_PASSWORD": "hosting"}',
 '[]',
 '[{"host": 0, "container": 53}, {"host": 0, "container": 9090}]',
 '{"memory_mb": 1024, "cpu_shares": 512}',
 '{"test": ["CMD", "pdns_control", "rping"], "interval": "10s", "timeout": "5s", "retries": 3}',
 false, 'bridge'),
('profile-email-default', 'email-default', 'email', 'registry.localhost:5000/hosting/email-node:latest',
 '{}',
 '[]',
 '[{"host": 0, "container": 25}, {"host": 0, "container": 587}, {"host": 0, "container": 993}, {"host": 0, "container": 443}, {"host": 0, "container": 8080}, {"host": 0, "container": 9090}]',
 '{"memory_mb": 2048, "cpu_shares": 1024}',
 '{"test": ["CMD", "curl", "-f", "http://localhost:8080/healthz"], "interval": "10s", "timeout": "5s", "retries": 3}',
 false, 'bridge');

-- +goose Down
DROP TABLE node_profiles;
