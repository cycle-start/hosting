-- +goose Up

CREATE TABLE products (
    id          TEXT PRIMARY KEY,
    brand_id    TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    modules     TEXT[] NOT NULL DEFAULT '{}',
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(brand_id, name)
);
CREATE INDEX idx_products_brand ON products(brand_id);

ALTER TABLE customer_subscriptions
    ADD COLUMN product_id TEXT NOT NULL REFERENCES products(id);

ALTER TABLE customer_subscriptions
    DROP COLUMN product_name,
    DROP COLUMN product_description,
    DROP COLUMN modules;

CREATE INDEX idx_customer_subscriptions_product ON customer_subscriptions(product_id);

-- +goose Down

ALTER TABLE customer_subscriptions
    DROP COLUMN product_id;

ALTER TABLE customer_subscriptions
    ADD COLUMN product_name        TEXT NOT NULL DEFAULT '',
    ADD COLUMN product_description TEXT,
    ADD COLUMN modules             TEXT[] NOT NULL DEFAULT '{}';

DROP INDEX IF EXISTS idx_products_brand;
DROP TABLE IF EXISTS products;
