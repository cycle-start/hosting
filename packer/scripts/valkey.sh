#!/bin/bash
set -ex

# Create directories for Valkey config and data.
mkdir -p /etc/valkey /var/lib/valkey

# Vector role-specific config.
cp /tmp/vector-valkey.toml /etc/vector/config.d/valkey.toml
