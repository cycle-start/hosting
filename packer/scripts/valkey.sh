#!/bin/bash
set -ex

export DEBIAN_FRONTEND=noninteractive

# Valkey is available in the Ubuntu Universe repository since 24.04.
apt-get update
apt-get install -y valkey

# Disable the system-wide service — we run per-instance processes managed by node-agent.
systemctl disable valkey-server

# Create directories for Valkey config and data.
mkdir -p /etc/valkey /var/lib/valkey

# Vector role-specific config.
cp /tmp/vector-valkey.toml /etc/vector/config.d/valkey.toml

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*
