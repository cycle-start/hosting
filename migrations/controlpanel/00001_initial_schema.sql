-- +goose Up
CREATE TABLE partners (
    id            TEXT PRIMARY KEY,
    brand_id      TEXT NOT NULL,
    name          TEXT NOT NULL,
    hostname      TEXT NOT NULL UNIQUE,
    primary_color TEXT NOT NULL DEFAULT '264',
    status        TEXT NOT NULL DEFAULT 'active',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(brand_id, name)
);

CREATE TABLE users (
    id               TEXT PRIMARY KEY,
    partner_id       TEXT NOT NULL REFERENCES partners(id),
    email            TEXT NOT NULL,
    password_hash    TEXT NOT NULL,
    display_name     TEXT,
    locale           TEXT NOT NULL DEFAULT 'en',
    last_customer_id TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (partner_id, email)
);

CREATE TABLE customers (
    id         TEXT PRIMARY KEY,
    partner_id TEXT NOT NULL REFERENCES partners(id),
    name       TEXT NOT NULL,
    email      TEXT,
    status     TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (partner_id, name)
);

CREATE TABLE customer_users (
    customer_id TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permissions TEXT[] NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (customer_id, user_id)
);
CREATE INDEX idx_customer_users_user_id ON customer_users(user_id);

CREATE TABLE brand_modules (
    brand_id         TEXT PRIMARY KEY,
    disabled_modules TEXT[] NOT NULL DEFAULT '{}',
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE customer_subscriptions (
    id                  TEXT PRIMARY KEY,
    customer_id         TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    tenant_id           TEXT NOT NULL,
    product_name        TEXT NOT NULL,
    product_description TEXT,
    modules             TEXT[] NOT NULL,
    status              TEXT NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_customer_subscriptions_customer ON customer_subscriptions(customer_id);

CREATE TABLE user_oidc_connections (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    partner_id TEXT NOT NULL REFERENCES partners(id),
    provider   TEXT NOT NULL,
    subject    TEXT NOT NULL,
    email      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, provider),
    UNIQUE(partner_id, provider, subject)
);

-- +goose Down
DROP TABLE IF EXISTS user_oidc_connections;
DROP TABLE IF EXISTS customer_subscriptions;
DROP TABLE IF EXISTS brand_modules;
DROP TABLE IF EXISTS customer_users;
DROP TABLE IF EXISTS customers;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS partners;
