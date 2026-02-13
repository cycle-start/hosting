#!/bin/bash
set -ex

# Wait for cloud-init to finish (Packer boot cloud-init).
# Exit code 2 = "recoverable errors" which is fine for our minimal cloud-init.
cloud-init status --wait || true

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get upgrade -y
apt-get install -y curl

# Create directories and install node-agent binary.
mkdir -p /opt/hosting/bin
install -m 0755 /tmp/node-agent /opt/hosting/bin/node-agent

# Install systemd service (uses EnvironmentFile for per-instance config).
cp /tmp/node-agent.service /etc/systemd/system/node-agent.service
systemctl daemon-reload
systemctl enable node-agent

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*
cloud-init clean
