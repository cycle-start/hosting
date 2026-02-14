#!/bin/bash
set -euo pipefail

# Bootstrap Vector on all running VMs.
# This is a Phase A script — installs Vector without rebuilding golden images.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
VECTOR_DIR="$PROJECT_DIR/deploy/vector"

SSH_OPTS="-o StrictHostKeyChecking=no -o ConnectTimeout=5"

# Node definitions: name ip role
NODES=(
  "controlplane-0    10.10.10.2   controlplane"
  "web-1-node-0      10.10.10.10  web"
  "web-1-node-1      10.10.10.11  web"
  "db-1-node-0       10.10.10.20  db"
  "dns-1-node-0      10.10.10.30  dns"
  "valkey-1-node-0   10.10.10.40  valkey"
  "storage-1-node-0  10.10.10.50  storage"
  "dbadmin-1-node-0  10.10.10.60  dbadmin"
)

install_vector() {
  local name=$1 ip=$2 role=$3

  echo "=== $name ($ip) — role: $role ==="

  # Check connectivity.
  if ! ssh $SSH_OPTS ubuntu@"$ip" "echo ok" >/dev/null 2>&1; then
    echo "  SKIP: cannot reach $ip"
    return 0
  fi

  # Install Vector if not present.
  ssh $SSH_OPTS ubuntu@"$ip" bash -s <<'INSTALL_EOF'
if command -v vector >/dev/null 2>&1; then
  echo "Vector already installed"
else
  echo "Installing Vector..."
  export DEBIAN_FRONTEND=noninteractive
  curl -1sLf https://setup.vector.dev | sudo bash
  sudo apt-get install -y vector
fi
sudo mkdir -p /etc/vector/config.d
# Remove default demo config that ships with the package.
sudo rm -f /etc/vector/vector.yaml
INSTALL_EOF

  # Configure Vector env to use our config file.
  ssh $SSH_OPTS ubuntu@"$ip" bash -s -- "$role" <<'VECENV_EOF'
if [ "$1" = "controlplane" ]; then
  echo 'VECTOR_CONFIG=/etc/vector/vector.toml' | sudo tee /etc/default/vector >/dev/null
else
  printf 'VECTOR_CONFIG=/etc/vector/vector.toml\nVECTOR_CONFIG_DIR=/etc/vector/config.d\n' | sudo tee /etc/default/vector >/dev/null
fi
VECENV_EOF

  # Copy base config.
  if [ "$role" = "controlplane" ]; then
    scp $SSH_OPTS "$VECTOR_DIR/controlplane.toml" ubuntu@"$ip":/tmp/vector.toml
    ssh $SSH_OPTS ubuntu@"$ip" "sudo cp /tmp/vector.toml /etc/vector/vector.toml"
  else
    scp $SSH_OPTS "$VECTOR_DIR/base.toml" ubuntu@"$ip":/tmp/vector.toml
    ssh $SSH_OPTS ubuntu@"$ip" "sudo cp /tmp/vector.toml /etc/vector/vector.toml"

    # Copy role-specific config if it exists.
    if [ -f "$VECTOR_DIR/$role.toml" ]; then
      scp $SSH_OPTS "$VECTOR_DIR/$role.toml" ubuntu@"$ip":/tmp/vector-role.toml
      ssh $SSH_OPTS ubuntu@"$ip" "sudo cp /tmp/vector-role.toml /etc/vector/config.d/$role.toml"
    fi

    # Add observability env vars to /etc/default/node-agent (if not already set).
    ssh $SSH_OPTS ubuntu@"$ip" bash -s -- "$role" <<'ENV_EOF'
ROLE="$1"
ENV_FILE="/etc/default/node-agent"
if [ -f "$ENV_FILE" ]; then
  grep -q "^REGION_ID=" "$ENV_FILE" || echo "REGION_ID=dev" | sudo tee -a "$ENV_FILE" >/dev/null
  grep -q "^CLUSTER_ID=" "$ENV_FILE" || echo "CLUSTER_ID=vm-cluster-1" | sudo tee -a "$ENV_FILE" >/dev/null
  grep -q "^NODE_ROLE=" "$ENV_FILE" || echo "NODE_ROLE=$ROLE" | sudo tee -a "$ENV_FILE" >/dev/null
  grep -q "^SERVICE_NAME=" "$ENV_FILE" || echo "SERVICE_NAME=node-agent" | sudo tee -a "$ENV_FILE" >/dev/null
  grep -q "^METRICS_ADDR=" "$ENV_FILE" || echo "METRICS_ADDR=:9100" | sudo tee -a "$ENV_FILE" >/dev/null
fi
ENV_EOF

    # Restart node-agent to pick up new env vars.
    ssh $SSH_OPTS ubuntu@"$ip" "sudo systemctl restart node-agent" || true
  fi

  # Start Vector.
  ssh $SSH_OPTS ubuntu@"$ip" "sudo systemctl enable vector && sudo systemctl restart vector"

  echo "  Done: $name"
  echo ""
}

echo "Bootstrapping Vector on all VMs..."
echo ""

for entry in "${NODES[@]}"; do
  read -r name ip role <<< "$entry"
  install_vector "$name" "$ip" "$role"
done

echo "=== Bootstrap complete ==="
echo "Check Grafana at http://grafana.hosting.test -> Explore -> Loki"
