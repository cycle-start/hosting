-- +goose Up

-- Nodes belong to a shard within their cluster.
ALTER TABLE nodes ADD COLUMN shard_id TEXT REFERENCES shards(id);

-- Tenants are assigned to a web shard for workload placement.
ALTER TABLE tenants ADD COLUMN shard_id TEXT REFERENCES shards(id);

-- Databases are assigned to a database shard (replaces direct node assignment).
ALTER TABLE databases ADD COLUMN shard_id TEXT REFERENCES shards(id);
ALTER TABLE databases ALTER COLUMN node_id DROP NOT NULL;
ALTER TABLE databases DROP CONSTRAINT databases_node_id_name_key;
ALTER TABLE databases ADD CONSTRAINT databases_shard_id_name_key UNIQUE (shard_id, name);

-- +goose Down
ALTER TABLE databases DROP CONSTRAINT databases_shard_id_name_key;
ALTER TABLE databases ADD CONSTRAINT databases_node_id_name_key UNIQUE (node_id, name);
ALTER TABLE databases ALTER COLUMN node_id SET NOT NULL;
ALTER TABLE databases DROP COLUMN shard_id;

ALTER TABLE tenants DROP COLUMN shard_id;
ALTER TABLE nodes DROP COLUMN shard_id;
