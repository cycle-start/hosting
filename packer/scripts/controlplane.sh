#!/bin/bash
set -ex

# Wait for cloud-init to finish (Packer boot cloud-init).
# Exit code 2 = "recoverable errors" which is fine for our minimal cloud-init.
cloud-init status --wait || true

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get upgrade -y
apt-get install -y curl

# Install k3s (don't start — cloud-init will start it on first boot).
curl -sfL https://get.k3s.io | INSTALL_K3S_SKIP_START=true sh -s - \
  --disable=servicelb

# Install Helm.
curl -sfL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Install Vector for log shipping.
curl -1sLf https://setup.vector.dev | bash
apt-get install -y vector
mkdir -p /etc/vector/config.d
rm -f /etc/vector/vector.yaml
cp /tmp/vector-controlplane.toml /etc/vector/vector.toml
# Point Vector to our config instead of the default vector.yaml.
cat > /etc/default/vector <<'EOF'
VECTOR_CONFIG=/etc/vector/vector.toml
EOF
systemctl enable vector

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/*
cloud-init clean
