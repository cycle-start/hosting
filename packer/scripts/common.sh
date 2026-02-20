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

# Install Vector for log shipping.
curl -1sLf https://setup.vector.dev | bash
apt-get install -y vector
mkdir -p /etc/vector/config.d
rm -f /etc/vector/vector.yaml
cp /tmp/vector.toml /etc/vector/vector.toml
# Point Vector to our config instead of the default vector.yaml.
cat > /etc/default/vector <<'EOF'
VECTOR_CONFIG=/etc/vector/vector.toml
VECTOR_CONFIG_DIR=/etc/vector/config.d
EOF
systemctl enable vector

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*
# NOTE: cloud-init clean is called at the end of each role-specific script,
# AFTER all packages are installed. This ensures no package triggers
# recreate /var/lib/cloud state after the clean.
