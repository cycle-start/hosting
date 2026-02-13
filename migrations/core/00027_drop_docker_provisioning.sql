-- +goose Up
DROP TABLE IF EXISTS node_deployments;
DROP TABLE IF EXISTS infrastructure_services;
DROP TABLE IF EXISTS node_profiles;
DROP TABLE IF EXISTS host_machines;
ALTER TABLE nodes DROP COLUMN IF EXISTS grpc_address;

-- +goose Down
-- Tables were Docker-provisioning specific and are no longer used.
-- See migrations 00017-00021 for original definitions.
