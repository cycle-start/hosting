-- +goose Up

-- Nodes can belong to multiple shards via a join table.
CREATE TABLE node_shard_assignments (
    node_id     TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    shard_id    TEXT NOT NULL REFERENCES shards(id) ON DELETE CASCADE,
    shard_index INT NOT NULL,
    PRIMARY KEY (node_id, shard_id)
);
CREATE UNIQUE INDEX idx_node_shard_index ON node_shard_assignments(shard_id, shard_index);

-- Tenants are assigned to a web shard for workload placement.
ALTER TABLE tenants ADD COLUMN shard_id TEXT REFERENCES shards(id);

-- Databases are assigned to a database shard (replaces direct node assignment).
ALTER TABLE databases ADD COLUMN shard_id TEXT REFERENCES shards(id);
ALTER TABLE databases ALTER COLUMN node_id DROP NOT NULL;
ALTER TABLE databases DROP CONSTRAINT databases_name_key;
ALTER TABLE databases ADD CONSTRAINT databases_shard_id_name_key UNIQUE (shard_id, name);

-- +goose Down
ALTER TABLE databases DROP CONSTRAINT databases_shard_id_name_key;
ALTER TABLE databases ADD CONSTRAINT databases_name_key UNIQUE (name);
ALTER TABLE databases DROP COLUMN shard_id;

ALTER TABLE tenants DROP COLUMN shard_id;
DROP INDEX IF EXISTS idx_node_shard_index;
DROP TABLE node_shard_assignments;
