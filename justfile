# Hosting Platform - Development Commands
set dotenv-load

# Control plane VM IP (k3s). Change if using a different Terraform controlplane_ip.
cp := "10.10.10.2"

# LB VM IP (HAProxy for tenant traffic).
lb := "10.10.10.70"

# Default: show available commands
default:
    @just --list

# --- Build ---

# Build all Go binaries
build:
    go build ./cmd/...

# Generate protobuf code
proto:
    protoc -Iproto \
        --go_out=. --go_opt=module=github.com/edvin/hosting \
        --go-grpc_out=. --go-grpc_opt=module=github.com/edvin/hosting \
        proto/agent/v1/agent.proto proto/agent/v1/types.proto

# Build admin UI for development
build-admin:
    cd web/admin && npm run build

# Start admin UI dev server (with API proxy to api.hosting.test)
dev-admin:
    cd web/admin && npm run dev

# Generate OpenAPI docs from swag annotations
docs:
    swag init -g internal/api/doc.go -o internal/api/docs --parseDependency --parseInternal

# --- Test ---

# Run unit tests
test:
    go test ./... -short -count=1

# Run unit tests with verbose output
test-v:
    go test ./... -short -count=1 -v

# Run tests for a specific package (e.g. just test-pkg workflow)
test-pkg pkg:
    go test ./internal/{{pkg}}/... -count=1 -v

# Run integration tests
test-integration:
    go test ./... -tags integration -count=1

# Run e2e tests (requires VMs running)
test-e2e:
    HOSTING_E2E=1 go test ./tests/e2e/... -count=1 -timeout 10m -v

# Run all tests
test-all: test test-integration test-e2e

# --- Lint ---

# Run linter
lint:
    golangci-lint run ./...

# Run go vet
vet:
    go vet ./...

# --- Database ---

# Run core DB migrations
migrate-core:
    goose -dir migrations/core postgres "postgres://hosting:hosting@{{cp}}:5432/hosting_core?sslmode=disable" up

# Run PowerDNS DB migrations
migrate-powerdns:
    goose -dir migrations/powerdns postgres "postgres://hosting:hosting@{{cp}}:5433/hosting_powerdns?sslmode=disable" up

# Run all migrations
migrate: migrate-core migrate-powerdns

# Reset core DB (drop all tables and goose version tracking)
reset-core:
    kubectl --context hosting exec statefulset/postgres-core -- psql -U hosting hosting_core -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

# Reset PowerDNS DB (drop all tables and goose version tracking)
reset-powerdns:
    kubectl --context hosting exec statefulset/postgres-powerdns -- psql -U hosting hosting_powerdns -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

# Reset all databases
reset-db: reset-core reset-powerdns

# Migration status
migrate-status:
    @echo "=== Core DB ==="
    goose -dir migrations/core postgres "postgres://hosting:hosting@{{cp}}:5432/hosting_core?sslmode=disable" status
    @echo ""
    @echo "=== PowerDNS DB ==="
    goose -dir migrations/powerdns postgres "postgres://hosting:hosting@{{cp}}:5433/hosting_powerdns?sslmode=disable" status

# --- Auth ---

# Create an API key (requires core-db running)
create-api-key name:
    CORE_DATABASE_URL="postgres://hosting:hosting@{{cp}}:5432/hosting_core?sslmode=disable" go run ./cmd/core-api create-api-key --name {{name}}

# Generate Temporal mTLS certificates
gen-certs:
    bash scripts/gen-temporal-certs.sh

# --- Utilities ---

# Connect to core DB via psql
db-core:
    psql "postgres://hosting:hosting@{{cp}}:5432/hosting_core?sslmode=disable"

# Connect to PowerDNS DB via psql
db-powerdns:
    psql "postgres://hosting:hosting@{{cp}}:5433/hosting_powerdns?sslmode=disable"

# Connect to MySQL on the DB shard VM
db-mysql:
    mysql -h 10.10.10.20 -P 3306 -u root -prootpassword

# Check Ceph cluster health
ceph-status:
    ssh -o StrictHostKeyChecking=no ubuntu@10.10.10.50 "sudo ceph -s"

# Update HAProxy map entry (e.g. just lb-set www.example.com shard-web-a)
lb-set fqdn backend:
    echo "set map /var/lib/haproxy/maps/fqdn-to-shard.map {{fqdn}} {{backend}}" | nc {{lb}} 9999

# Delete HAProxy map entry
lb-del fqdn:
    echo "del map /var/lib/haproxy/maps/fqdn-to-shard.map {{fqdn}}" | nc {{lb}} 9999

# Show HAProxy map
lb-show:
    echo "show map /var/lib/haproxy/maps/fqdn-to-shard.map" | nc {{lb}} 9999

# --- Packer (Golden Images) ---

# Build the node-agent binary for Linux (for VM deployment)
build-node-agent:
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/node-agent ./cmd/node-agent

# Build all golden images (requires node-agent binary)
packer-all: build-node-agent
    cd packer && packer init . && packer build .

# Build a specific role image (e.g. just packer-role web)
packer-role role: build-node-agent
    cd packer && packer init . && packer build -only="*.{{role}}" .

# --- VM Infrastructure ---

# Create VMs with Terraform and register them with the platform
# (Requires golden images — run `just packer-all` first)
vm-up:
    cd terraform && terraform apply -auto-approve
    go run ./cmd/hostctl cluster apply -f clusters/vm-generated.yaml

# Destroy VMs
vm-down:
    cd terraform && terraform destroy -auto-approve

# SSH into a VM (e.g. just vm-ssh web-1-node-0)
vm-ssh name:
    ssh -o StrictHostKeyChecking=no ubuntu@$(cd terraform && terraform output -json 2>/dev/null | python3 -c "import sys,json; o=json.load(sys.stdin); d={}; [d.update(v['value']) for k,v in o.items() if k.endswith('_ips')]; print(d['{{name}}'])")

# --- k3s Control Plane ---

# Build Docker images and deploy to k3s VM
vm-deploy:
    # Build Docker images
    docker build -t hosting-core-api:latest -f docker/core-api.Dockerfile .
    docker build -t hosting-worker:latest -f docker/worker.Dockerfile .
    docker build -t hosting-admin-ui:latest -f docker/admin-ui.Dockerfile .
    docker build -t hosting-mcp-server:latest -f docker/mcp-server.Dockerfile .
    # Import into k3s containerd
    docker save hosting-core-api:latest hosting-worker:latest hosting-admin-ui:latest hosting-mcp-server:latest | \
      ssh -o StrictHostKeyChecking=no ubuntu@{{cp}} "sudo k3s ctr images import -"
    # Apply infra manifests (includes Traefik config and Ingress)
    kubectl --context hosting apply -f deploy/k3s/
    # Deploy SSL certs (mkcert if available, otherwise self-signed)
    just _ssl-deploy
    # Create Grafana dashboards ConfigMap
    kubectl --context hosting delete configmap grafana-dashboards --ignore-not-found
    kubectl --context hosting create configmap grafana-dashboards \
      --from-file=api-overview.json=docker/grafana/provisioning/dashboards/api-overview.json \
      --from-file=infrastructure.json=docker/grafana/provisioning/dashboards/infrastructure.json \
      --from-file=log-explorer.json=docker/grafana/provisioning/dashboards/log-explorer.json \
      --from-file=tenant.json=docker/grafana/provisioning/dashboards/tenant.json \
      --from-file=workflow.json=docker/grafana/provisioning/dashboards/workflow.json \
      --from-file=database.json=docker/grafana/provisioning/dashboards/database.json \
      --from-file=dns.json=docker/grafana/provisioning/dashboards/dns.json
    # Install/upgrade Helm chart
    helm --kube-context hosting upgrade --install hosting \
      deploy/helm/hosting -f deploy/helm/hosting/values-dev.yaml

# Fetch kubeconfig from controlplane VM and merge into ~/.kube/config
vm-kubeconfig:
    mkdir -p ~/.kube
    ssh -o StrictHostKeyChecking=no ubuntu@{{cp}} "sudo cp /etc/rancher/k3s/k3s.yaml /home/ubuntu/.kube/config && sudo chown ubuntu:ubuntu /home/ubuntu/.kube/config"
    scp -o StrictHostKeyChecking=no ubuntu@{{cp}}:/home/ubuntu/.kube/config /tmp/k3s-config
    sed -i 's/127.0.0.1/{{cp}}/g' /tmp/k3s-config
    sed -i 's/: default$/: hosting/g' /tmp/k3s-config
    KUBECONFIG=~/.kube/config:/tmp/k3s-config kubectl config view --flatten > /tmp/kube-merged && mv /tmp/kube-merged ~/.kube/config
    kubectl config use-context hosting
    @echo "Merged into ~/.kube/config as context 'hosting'"

# Show k3s pod status
vm-pods:
    kubectl --context hosting get pods

# Stream k3s pod logs (e.g. just vm-log hosting-core-api)
vm-log name:
    kubectl --context hosting logs -f deployment/{{name}}

# Full dev setup: build images, create VMs, deploy control plane, seed cluster
dev-k3s: build-node-agent
    just packer-role controlplane
    cd terraform && terraform apply -auto-approve
    @echo "Waiting for k3s to be ready..."
    @sleep 30
    just vm-kubeconfig
    just vm-deploy
    @sleep 10
    go run ./cmd/hostctl cluster apply -f clusters/vm-generated.yaml

# --- SSL ---

# Generate trusted SSL certs with mkcert and deploy to Traefik + LB VM
ssl-init:
    mkcert -cert-file /tmp/hosting-cert.pem -key-file /tmp/hosting-key.pem "*.hosting.test" "hosting.test"
    # Deploy to Traefik (k3s)
    kubectl --context hosting -n kube-system create secret tls traefik-default-cert \
      --cert=/tmp/hosting-cert.pem --key=/tmp/hosting-key.pem \
      --dry-run=client -o yaml | kubectl --context hosting -n kube-system apply -f -
    # Deploy to LB VM HAProxy
    cat /tmp/hosting-cert.pem /tmp/hosting-key.pem > /tmp/hosting.pem
    scp -o StrictHostKeyChecking=no /tmp/hosting.pem ubuntu@{{lb}}:/tmp/hosting.pem
    ssh -o StrictHostKeyChecking=no ubuntu@{{lb}} "sudo cp /tmp/hosting.pem /etc/haproxy/certs/hosting.pem && sudo systemctl reload haproxy"
    # Deploy to DB Admin VM nginx
    scp -o StrictHostKeyChecking=no /tmp/hosting-cert.pem ubuntu@10.10.10.60:/tmp/dbadmin.pem
    scp -o StrictHostKeyChecking=no /tmp/hosting-key.pem ubuntu@10.10.10.60:/tmp/dbadmin-key.pem
    ssh -o StrictHostKeyChecking=no ubuntu@10.10.10.60 "sudo mkdir -p /etc/nginx/certs && sudo cp /tmp/dbadmin.pem /etc/nginx/certs/dbadmin.pem && sudo cp /tmp/dbadmin-key.pem /etc/nginx/certs/dbadmin-key.pem && sudo systemctl reload nginx"
    rm /tmp/hosting-cert.pem /tmp/hosting-key.pem /tmp/hosting.pem
    @echo "Trusted SSL certs installed on Traefik, LB VM, and DB Admin VM. Visit https://admin.hosting.test"

# Deploy SSL certs: uses mkcert (trusted) if available, otherwise self-signed
_ssl-deploy:
    #!/usr/bin/env bash
    set -e
    if command -v mkcert &>/dev/null; then
      echo "Using mkcert for trusted SSL certs..."
      mkcert -cert-file /tmp/hosting-cert.pem -key-file /tmp/hosting-key.pem "*.hosting.test" "hosting.test"
    else
      echo "mkcert not found, using self-signed certs (browsers will show warnings)..."
      openssl req -x509 -newkey rsa:2048 \
        -keyout /tmp/hosting-key.pem -out /tmp/hosting-cert.pem \
        -days 365 -nodes -subj '/CN=*.hosting.test' \
        -addext 'subjectAltName=DNS:*.hosting.test,DNS:hosting.test' 2>/dev/null
    fi
    # Deploy to Traefik (k3s)
    kubectl --context hosting -n kube-system create secret tls traefik-default-cert \
      --cert=/tmp/hosting-cert.pem --key=/tmp/hosting-key.pem \
      --dry-run=client -o yaml | kubectl --context hosting -n kube-system apply -f -
    # Deploy to LB VM HAProxy (skip if not reachable)
    cat /tmp/hosting-cert.pem /tmp/hosting-key.pem > /tmp/hosting.pem
    scp -o StrictHostKeyChecking=no -o ConnectTimeout=3 /tmp/hosting.pem ubuntu@{{lb}}:/tmp/hosting.pem && \
      ssh -o StrictHostKeyChecking=no -o ConnectTimeout=3 ubuntu@{{lb}} "sudo cp /tmp/hosting.pem /etc/haproxy/certs/hosting.pem && sudo systemctl reload haproxy" || true
    # Deploy to DB Admin VM nginx (skip if not reachable)
    scp -o StrictHostKeyChecking=no -o ConnectTimeout=3 /tmp/hosting-cert.pem ubuntu@10.10.10.60:/tmp/dbadmin.pem && \
      scp -o StrictHostKeyChecking=no -o ConnectTimeout=3 /tmp/hosting-key.pem ubuntu@10.10.10.60:/tmp/dbadmin-key.pem && \
      ssh -o StrictHostKeyChecking=no -o ConnectTimeout=3 ubuntu@10.10.10.60 "sudo mkdir -p /etc/nginx/certs && sudo cp /tmp/dbadmin.pem /etc/nginx/certs/dbadmin.pem && sudo cp /tmp/dbadmin-key.pem /etc/nginx/certs/dbadmin-key.pem && sudo systemctl reload nginx" || true
    rm -f /tmp/hosting-cert.pem /tmp/hosting-key.pem /tmp/hosting.pem

# --- Networking ---

# Enable port forwarding from Windows to VMs (requires sudo)
forward:
    sudo ./scripts/wsl-forward.sh start

# Disable port forwarding
forward-stop:
    sudo ./scripts/wsl-forward.sh stop

# Check forwarding status
forward-status:
    ./scripts/wsl-forward.sh status

# --- Monitoring ---

# Open Grafana UI
vm-grafana:
    @echo "http://grafana.hosting.test (admin/admin)"

# Open Prometheus UI
vm-prometheus:
    @echo "http://prometheus.hosting.test"

# Bootstrap Vector on all running VMs (Phase A — no Packer rebuild needed)
vm-bootstrap-vector:
    bash scripts/bootstrap-vector.sh
