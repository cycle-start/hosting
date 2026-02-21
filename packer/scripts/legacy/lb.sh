#!/bin/bash
set -ex

export DEBIAN_FRONTEND=noninteractive

apt-get update

# Install HAProxy and socat (for Runtime API debugging).
apt-get install -y haproxy socat

# Create map directory used by HAProxy for FQDN-to-shard routing.
mkdir -p /var/lib/haproxy/maps
touch /var/lib/haproxy/maps/fqdn-to-shard.map
chown -R haproxy:haproxy /var/lib/haproxy

# Create HAProxy run directory.
mkdir -p /var/run/haproxy
chown haproxy:haproxy /var/run/haproxy

# Create certs directory for self-signed TLS.
mkdir -p /etc/haproxy/certs

# Enable HAProxy.
systemctl enable haproxy

# Install role-specific Vector config.
cp /tmp/vector-lb.toml /etc/vector/config.d/lb.toml

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*

# Final cloud-init clean — must be last to prevent package triggers from
# recreating /var/lib/cloud state.
cloud-init clean
