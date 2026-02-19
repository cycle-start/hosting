# Hosting Platform - Development Commands
set dotenv-load

# Control plane VM IP (k3s). Change if using a different Terraform controlplane_ip.
cp := "10.10.10.2"

# LB VM IP (HAProxy for tenant traffic).
lb := "10.10.10.70"

# SSH options for dev VMs — skip host key checking entirely since VMs get new keys on every rebuild.
ssh_opts := "-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR"

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

# Create the well-known dev API key (used by seed configs and e2e tests)
create-dev-key:
    CORE_DATABASE_URL="postgres://hosting:hosting@{{cp}}:5432/hosting_core?sslmode=disable" go run ./cmd/core-api create-api-key --name dev --raw-key hst_dev_e2e_test_key_00000000

# Create the agent API key (used by the LLM incident agent to call the core API)
create-agent-key:
    CORE_DATABASE_URL="postgres://hosting:hosting@{{cp}}:5432/hosting_core?sslmode=disable" go run ./cmd/core-api create-api-key --name agent --raw-key hst_agent_key_000000000000000

# Register cluster topology (regions, clusters, shards, nodes) with the platform
cluster-apply:
    go run ./cmd/hostctl cluster apply -f clusters/vm-generated.yaml

# Full bootstrap after DB reset: migrate, create dev key, create agent key, register cluster, seed tenants
bootstrap: migrate create-dev-key create-agent-key cluster-apply seed

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
    ssh {{ssh_opts}} ubuntu@10.10.10.50 "sudo ceph -s"

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
    @if sudo virsh net-info hosting 2>/dev/null | grep -q 'Active:.*no'; then \
        echo "Starting libvirt network 'hosting'..."; \
        sudo virsh net-start hosting; \
    fi
    cd terraform && terraform apply -auto-approve
    @echo "Waiting for control plane API ({{cp}}:8090)..."
    @for i in $(seq 1 60); do \
        if curl -sf -o /dev/null http://{{cp}}:8090/healthz 2>/dev/null; then \
            echo "Control plane ready."; \
            break; \
        fi; \
        if [ "$i" -eq 60 ]; then echo "Timed out waiting for control plane" && exit 1; fi; \
        sleep 5; \
    done
    go run ./cmd/hostctl cluster apply -f clusters/vm-generated.yaml

# Rebuild everything: new golden images, recreate VMs, deploy control plane
vm-rebuild:
    just packer-all
    just vm-down
    cd terraform && terraform apply -auto-approve
    @echo "Waiting for k3s to be ready..."
    @sleep 30
    just vm-kubeconfig
    just vm-deploy
    just vm-up

# Destroy VMs
vm-down:
    cd terraform && terraform destroy -auto-approve

# SSH into a VM (e.g. just vm-ssh web-1-node-0)
vm-ssh name:
    ssh {{ssh_opts}} ubuntu@$(cd terraform && terraform output -json 2>/dev/null | python3 -c "import sys,json; o=json.load(sys.stdin); d={}; [d.update(v['value']) for k,v in o.items() if k.endswith('_ips')]; print(d['{{name}}'])")

# --- k3s Control Plane ---

# Rebuild and deploy a single image to k3s (e.g. just vm-deploy-one admin-ui)
vm-deploy-one name:
    docker build --no-cache -t hosting-{{name}}:latest -f docker/{{name}}.Dockerfile .
    docker save hosting-{{name}}:latest | ssh {{ssh_opts}} ubuntu@{{cp}} "sudo k3s ctr images import -"
    kubectl --context hosting rollout restart deployment/hosting-{{name}}

# Build all Docker images and deploy to k3s VM
vm-deploy:
    # Build Docker images
    docker build -t hosting-core-api:latest -f docker/core-api.Dockerfile .
    docker build -t hosting-worker:latest -f docker/worker.Dockerfile .
    docker build -t hosting-admin-ui:latest -f docker/admin-ui.Dockerfile .
    docker build -t hosting-mcp-server:latest -f docker/mcp-server.Dockerfile .
    # Import into k3s containerd
    docker save hosting-core-api:latest hosting-worker:latest hosting-admin-ui:latest hosting-mcp-server:latest | \
      ssh {{ssh_opts}} ubuntu@{{cp}} "sudo k3s ctr images import -"
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
    # Create Grafana alerting ConfigMap
    kubectl --context hosting delete configmap grafana-alerting --ignore-not-found
    kubectl --context hosting create configmap grafana-alerting \
      --from-file=contact-points.yaml=docker/grafana/provisioning/alerting/contact-points.yaml \
      --from-file=notification-policies.yaml=docker/grafana/provisioning/alerting/notification-policies.yaml \
      --from-file=alert-rules.yaml=docker/grafana/provisioning/alerting/alert-rules.yaml
    # Install/upgrade Helm chart
    helm --kube-context hosting upgrade --install hosting \
      deploy/helm/hosting -f deploy/helm/hosting/values-dev.yaml

# Fetch kubeconfig from controlplane VM and merge into ~/.kube/config
vm-kubeconfig:
    mkdir -p ~/.kube
    ssh {{ssh_opts}} ubuntu@{{cp}} "mkdir -p /home/ubuntu/.kube && sudo cp /etc/rancher/k3s/k3s.yaml /home/ubuntu/.kube/config && sudo chown ubuntu:ubuntu /home/ubuntu/.kube/config"
    scp {{ssh_opts}} ubuntu@{{cp}}:/home/ubuntu/.kube/config /tmp/k3s-config
    sed -i 's/127.0.0.1/{{cp}}/g' /tmp/k3s-config
    sed -i 's/: default$/: hosting/g' /tmp/k3s-config
    -kubectl config delete-context hosting 2>/dev/null
    -kubectl config delete-cluster hosting 2>/dev/null
    -kubectl config delete-user hosting 2>/dev/null
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
    scp {{ssh_opts}} /tmp/hosting.pem ubuntu@{{lb}}:/tmp/hosting.pem
    ssh {{ssh_opts}} ubuntu@{{lb}} "sudo cp /tmp/hosting.pem /etc/haproxy/certs/hosting.pem && sudo systemctl reload haproxy"
    # Deploy to DB Admin VM nginx
    scp {{ssh_opts}} /tmp/hosting-cert.pem ubuntu@10.10.10.60:/tmp/dbadmin.pem
    scp {{ssh_opts}} /tmp/hosting-key.pem ubuntu@10.10.10.60:/tmp/dbadmin-key.pem
    ssh {{ssh_opts}} ubuntu@10.10.10.60 "sudo mkdir -p /etc/nginx/certs && sudo cp /tmp/dbadmin.pem /etc/nginx/certs/dbadmin.pem && sudo cp /tmp/dbadmin-key.pem /etc/nginx/certs/dbadmin-key.pem && sudo systemctl reload nginx"
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
    scp {{ssh_opts}} -o ConnectTimeout=3 /tmp/hosting.pem ubuntu@{{lb}}:/tmp/hosting.pem && \
      ssh {{ssh_opts}} -o ConnectTimeout=3 ubuntu@{{lb}} "sudo cp /tmp/hosting.pem /etc/haproxy/certs/hosting.pem && sudo systemctl reload haproxy" || true
    # Deploy to DB Admin VM nginx (skip if not reachable)
    scp {{ssh_opts}} -o ConnectTimeout=3 /tmp/hosting-cert.pem ubuntu@10.10.10.60:/tmp/dbadmin.pem && \
      scp {{ssh_opts}} -o ConnectTimeout=3 /tmp/hosting-key.pem ubuntu@10.10.10.60:/tmp/dbadmin-key.pem && \
      ssh {{ssh_opts}} -o ConnectTimeout=3 ubuntu@10.10.10.60 "sudo mkdir -p /etc/nginx/certs && sudo cp /tmp/dbadmin.pem /etc/nginx/certs/dbadmin.pem && sudo cp /tmp/dbadmin-key.pem /etc/nginx/certs/dbadmin-key.pem && sudo systemctl reload nginx" || true
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

# --- Test Fixtures ---

# Build the Laravel Reverb E2E test fixture (requires Docker)
build-laravel-fixture:
    #!/usr/bin/env bash
    set -e
    if [ -f .build/laravel-reverb.tar.gz ]; then
      echo "Fixture already exists at .build/laravel-reverb.tar.gz — delete to rebuild"
      exit 0
    fi
    mkdir -p .build/laravel-reverb
    echo "Creating Laravel project..."
    docker run --rm --user "$(id -u):$(id -g)" -v "$(pwd)/.build/laravel-reverb:/app" composer:2 \
      create-project laravel/laravel . --prefer-dist --no-interaction
    echo "Installing Laravel Reverb..."
    docker run --rm --user "$(id -u):$(id -g)" -v "$(pwd)/.build/laravel-reverb:/app" composer:2 \
      require laravel/reverb --no-interaction
    echo "Applying overlay files..."
    cp -r tests/e2e/fixtures/laravel-reverb/overlay/* .build/laravel-reverb/
    echo "Creating tarball..."
    tar -czf .build/laravel-reverb.tar.gz -C .build/laravel-reverb .
    echo "Done: .build/laravel-reverb.tar.gz"

# Seed dev tenants (builds fixture if needed)
seed: build-laravel-fixture
    go run ./cmd/hostctl seed -f seeds/dev-tenants.yaml -timeout 5m
