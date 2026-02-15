-- +goose Up
CREATE TABLE brands (
    id               TEXT PRIMARY KEY,
    name             TEXT NOT NULL,
    base_hostname    TEXT NOT NULL,
    primary_ns       TEXT NOT NULL,
    secondary_ns     TEXT NOT NULL,
    hostmaster_email TEXT NOT NULL,
    mail_hostname    TEXT NOT NULL DEFAULT '',
    spf_includes     TEXT NOT NULL DEFAULT '',
    dkim_selector    TEXT NOT NULL DEFAULT '',
    dkim_public_key  TEXT NOT NULL DEFAULT '',
    dmarc_policy     TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'active',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name)
);

CREATE TABLE brand_clusters (
    brand_id   TEXT NOT NULL REFERENCES brands(id) ON DELETE CASCADE,
    cluster_id TEXT NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    PRIMARY KEY (brand_id, cluster_id)
);

-- +goose Down
DROP TABLE brand_clusters;
DROP TABLE brands;
