#!/usr/bin/env bash
# Deploy control plane to a k3s node.
# Usage: deploy-controlplane.sh <target_host> [ssh_user]
#
# This script builds Docker images, imports them into k3s, applies k3s
# infrastructure manifests, and runs Helm to deploy the control plane.
# It's called by the setup wizard's "Deploy control plane" step.
set -euo pipefail

TARGET_HOST="${1:?Usage: deploy-controlplane.sh <target_host> [ssh_user]}"
SSH_USER="${2:-ubuntu}"
SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR"

is_local() {
    [[ "$TARGET_HOST" == "127.0.0.1" || "$TARGET_HOST" == "localhost" ]]
}

remote_cmd() {
    if is_local; then
        eval "$@"
    else
        ssh $SSH_OPTS "${SSH_USER}@${TARGET_HOST}" "$@"
    fi
}

echo "==> Building Docker images..."
docker build -t hosting-core-api:latest -f docker/core-api.Dockerfile .
docker build -t hosting-worker:latest -f docker/worker.Dockerfile .
docker build -t hosting-admin-ui:latest -f docker/admin-ui.Dockerfile .
docker build -t hosting-mcp-server:latest -f docker/mcp-server.Dockerfile .
docker build -t controlpanel-api:latest -f docker/controlpanel-api.Dockerfile .
docker build -t controlpanel-ui:latest -f docker/controlpanel-ui.Dockerfile .

echo "==> Importing images into k3s..."
if is_local; then
    docker save \
        hosting-core-api:latest \
        hosting-worker:latest \
        hosting-admin-ui:latest \
        hosting-mcp-server:latest \
        controlpanel-api:latest \
        controlpanel-ui:latest \
        | sudo k3s ctr images import -
else
    docker save \
        hosting-core-api:latest \
        hosting-worker:latest \
        hosting-admin-ui:latest \
        hosting-mcp-server:latest \
        controlpanel-api:latest \
        controlpanel-ui:latest \
        | ssh $SSH_OPTS "${SSH_USER}@${TARGET_HOST}" "sudo k3s ctr images import -"
fi

echo "==> Fetching kubeconfig..."
if is_local; then
    mkdir -p "$HOME/.kube"
    sudo cp /etc/rancher/k3s/k3s.yaml "$HOME/.kube/config"
    sudo chown "$(id -u):$(id -g)" "$HOME/.kube/config"
    export KUBECONFIG="$HOME/.kube/config"
else
    remote_cmd "mkdir -p /home/${SSH_USER}/.kube && sudo cp /etc/rancher/k3s/k3s.yaml /home/${SSH_USER}/.kube/config && sudo chown ${SSH_USER}:${SSH_USER} /home/${SSH_USER}/.kube/config"
    scp $SSH_OPTS "${SSH_USER}@${TARGET_HOST}:/home/${SSH_USER}/.kube/config" /tmp/k3s-config
    # Rewrite the server address to point to the target host
    sed -i "s|https://127.0.0.1:6443|https://${TARGET_HOST}:6443|g" /tmp/k3s-config
    export KUBECONFIG=/tmp/k3s-config
fi

echo "==> Applying k3s infrastructure manifests..."
for f in deploy/k3s/*.yaml; do
    envsubst '$BASE_DOMAIN' < "$f" | kubectl apply -f - 2>/dev/null || true
done

echo "==> Running Helm deployment..."
helm repo add bitnami https://charts.bitnami.com/bitnami 2>/dev/null || true
helm repo add temporal https://go.temporal.io/helm-charts 2>/dev/null || true

if [ ! -f deploy/helm/hosting/charts/postgresql-*.tgz ] 2>/dev/null; then
    helm dependency build deploy/helm/hosting 2>/dev/null || true
fi

# Build Helm --set flags from environment variables (if present)
HELM_SETS=""
[ -n "${BASE_DOMAIN:-}" ] && HELM_SETS="$HELM_SETS --set config.baseDomain=$BASE_DOMAIN"
[ -n "${CORE_DATABASE_URL:-}" ] && HELM_SETS="$HELM_SETS --set secrets.coreDatabaseUrl=$CORE_DATABASE_URL"
[ -n "${POWERDNS_DATABASE_URL:-}" ] && HELM_SETS="$HELM_SETS --set secrets.powerdnsDatabaseUrl=$POWERDNS_DATABASE_URL"
[ -n "${STALWART_ADMIN_TOKEN:-}" ] && HELM_SETS="$HELM_SETS --set secrets.stalwartAdminToken=$STALWART_ADMIN_TOKEN"
[ -n "${SECRET_ENCRYPTION_KEY:-}" ] && HELM_SETS="$HELM_SETS --set secrets.secretEncryptionKey=$SECRET_ENCRYPTION_KEY"
[ -n "${LLM_API_KEY:-}" ] && HELM_SETS="$HELM_SETS --set secrets.llmApiKey=$LLM_API_KEY"
[ -n "${AGENT_API_KEY:-}" ] && HELM_SETS="$HELM_SETS --set secrets.agentApiKey=$AGENT_API_KEY"
[ -n "${CONTROLPANEL_DATABASE_URL:-}" ] && HELM_SETS="$HELM_SETS --set secrets.controlpanelDatabaseUrl=$CONTROLPANEL_DATABASE_URL"
[ -n "${CONTROLPANEL_JWT_SECRET:-}" ] && HELM_SETS="$HELM_SETS --set secrets.controlpanelJwtSecret=$CONTROLPANEL_JWT_SECRET"
[ -f ssh_ca ] && HELM_SETS="$HELM_SETS --set-file secrets.sshCaPrivateKey=ssh_ca"

helm upgrade --install hosting \
    deploy/helm/hosting -f deploy/helm/hosting/values-dev.yaml \
    $HELM_SETS

echo "==> Restarting deployments..."
kubectl rollout restart deployment/hosting-core-api deployment/hosting-worker deployment/hosting-admin-ui deployment/hosting-mcp-server deployment/hosting-controlpanel-api deployment/hosting-controlpanel-ui 2>/dev/null || true

echo "==> Waiting for core-api to be ready..."
for i in $(seq 1 60); do
    if kubectl get pod -l app=hosting-core-api -o jsonpath='{.items[0].status.phase}' 2>/dev/null | grep -q Running; then
        echo "    core-api pod is running"
        break
    fi
    sleep 2
done

echo "==> Waiting for PowerDNS database (port 5433)..."
for i in $(seq 1 90); do
    if remote_cmd "ss -tln | grep -q ':5433 '"; then
        echo "    PowerDNS database is listening"
        break
    fi
    if [ "$i" -eq 90 ]; then
        echo "    WARNING: PowerDNS database not ready after 3 minutes, skipping pdns restart"
    fi
    sleep 2
done

echo "==> Restarting PowerDNS to connect to database..."
remote_cmd "sudo systemctl restart pdns" 2>/dev/null || true

echo "==> Control plane deployed successfully"
