#!/usr/bin/env bash
# Deploy control plane to a k3s node.
# Usage: deploy-controlplane.sh <target_host> [ssh_user]
#
# This script builds Docker images, imports them into k3s, applies k3s
# infrastructure manifests, and runs Helm to deploy the control plane.
# It's called by the setup wizard's "Deploy control plane" step.
set -euo pipefail

TARGET_HOST="${1:?Usage: deploy-controlplane.sh <target_host> [ssh_user] [output_dir]}"
SSH_USER="${2:-ubuntu}"
OUTPUT_DIR="${3:-.}"
SSH_OPTS="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR"

# Source .env (from output dir first, then project root)
for envfile in "${OUTPUT_DIR}/.env" ".env"; do
    if [ -f "$envfile" ]; then
        set -a
        source "$envfile"
        set +a
        break
    fi
done

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

echo "==> Fetching kubeconfig..."
KUBECONFIG_OUT="${OUTPUT_DIR}/generated/kubeconfig.yaml"
if is_local; then
    sudo cp /etc/rancher/k3s/k3s.yaml "$KUBECONFIG_OUT"
    sudo chown "$(id -u):$(id -g)" "$KUBECONFIG_OUT"
else
    remote_cmd "sudo cat /etc/rancher/k3s/k3s.yaml" > "$KUBECONFIG_OUT"
    sed -i "s|https://127.0.0.1:6443|https://${TARGET_HOST}:6443|g" "$KUBECONFIG_OUT"
fi
chmod 600 "$KUBECONFIG_OUT"
export KUBECONFIG="$KUBECONFIG_OUT"
echo "    Saved to ${KUBECONFIG_OUT}"

echo "==> Applying k3s infrastructure manifests..."
ENVSUBST_VARS='$BASE_DOMAIN $OIDC_ISSUER_URL $OIDC_AUTH_URL $OIDC_TOKEN_URL $OIDC_USERINFO_URL $OIDC_PROVIDER_NAME $OIDC_CLIENT_SECRET $OAUTH2_PROXY_COOKIE_SECRET $GRAFANA_CLIENT_ID $HEADLAMP_CLIENT_ID $TEMPORAL_CLIENT_ID $PROMETHEUS_CLIENT_ID $ADMIN_CLIENT_ID'

# Create authelia-config secret from generated files (internal SSO only)
if [[ -f "${OUTPUT_DIR}/generated/authelia/configuration.yml" ]]; then
    echo "    Creating authelia-config secret..."
    kubectl create secret generic authelia-config \
        --from-file=configuration.yml="${OUTPUT_DIR}/generated/authelia/configuration.yml" \
        --from-file=users_database.yml="${OUTPUT_DIR}/generated/authelia/users_database.yml" \
        --dry-run=client -o yaml | kubectl apply -f -
fi

for f in deploy/k3s/*.yaml; do
    # Skip SSO-related manifests if OIDC is not configured
    if [[ -z "${OIDC_CLIENT_SECRET:-}" ]]; then
        case "$(basename "$f")" in
            oauth2-proxy.yaml|oidc-secret.yaml|authelia.yaml) continue ;;
        esac
    fi
    # Skip authelia.yaml for external SSO mode (no generated config)
    if [[ "$(basename "$f")" == "authelia.yaml" ]] && [[ ! -f "generated/authelia/configuration.yml" ]]; then
        continue
    fi
    envsubst "$ENVSUBST_VARS" < "$f" | kubectl apply -f - 2>/dev/null || true
done

echo "==> Running Helm deployment..."
helm repo add bitnami https://charts.bitnami.com/bitnami 2>/dev/null || true
helm repo add temporal https://go.temporal.io/helm-charts 2>/dev/null || true

if [ ! -f deploy/helm/hosting/charts/postgresql-*.tgz ] 2>/dev/null; then
    helm dependency build deploy/helm/hosting 2>/dev/null || true
fi

HELM_VALUES="${OUTPUT_DIR}/generated/helm-values.yaml"
if [ ! -f "$HELM_VALUES" ]; then
    echo "ERROR: ${HELM_VALUES} not found. Run the setup wizard's Generate step first."
    exit 1
fi

helm upgrade --install hosting deploy/helm/hosting \
    -f "$HELM_VALUES"

echo "==> Restarting deployments..."
kubectl rollout restart deployment/hosting-core-api deployment/hosting-worker deployment/hosting-admin-ui deployment/hosting-controlpanel-api deployment/hosting-controlpanel-ui 2>/dev/null || true
kubectl rollout restart deployment/authelia 2>/dev/null || true

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
