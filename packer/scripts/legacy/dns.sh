#!/bin/bash
set -ex

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y pdns-server pdns-backend-pgsql

# Vector role-specific config.
cp /tmp/vector-dns.toml /etc/vector/config.d/dns.toml

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*

# Final cloud-init clean — must be last to prevent package triggers from
# recreating /var/lib/cloud state.
cloud-init clean
