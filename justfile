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

# Reset Temporal DB (drop and recreate databases, restart Temporal)
reset-temporal:
    # Scale down Temporal to release DB connections
    kubectl --context hosting scale deployment/temporal --replicas=0
    @sleep 5
    kubectl --context hosting exec statefulset/postgres-core -- psql -U hosting postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname IN ('temporal','temporal_visibility') AND pid <> pg_backend_pid();" || true
    kubectl --context hosting exec statefulset/postgres-core -- psql -U hosting postgres -c "DROP DATABASE IF EXISTS temporal;"
    kubectl --context hosting exec statefulset/postgres-core -- psql -U hosting postgres -c "DROP DATABASE IF EXISTS temporal_visibility;"
    # Scale back up — Temporal auto-setup recreates databases on start
    kubectl --context hosting scale deployment/temporal --replicas=1
    @echo "Waiting for Temporal to recreate databases..."
    @sleep 15

# Reset all databases (core + PowerDNS + Temporal)
reset-db: reset-core reset-powerdns reset-temporal

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

# Wait for core-api to be healthy (used between migrate and cluster-apply)
wait-api:
    @echo "Waiting for core-api..."
    @until curl -sf http://{{cp}}:8090/healthz > /dev/null 2>&1; do sleep 2; done
    @echo "core-api is ready."

# Full bootstrap after DB reset: migrate, create dev key, create agent key, register cluster, seed tenants
bootstrap: wait-db migrate create-dev-key create-agent-key wait-api cluster-apply seed

# Full rebuild: deploy control plane + node agents + wipe DB + bootstrap
# Use when both control plane and node-agent code have changed.
rebuild: vm-deploy reset-db migrate create-dev-key create-agent-key wait-api cluster-apply deploy-node-agent seed

# Wait for Postgres to accept connections
wait-db:
    @echo "Waiting for Postgres..."
    @until nc -z {{cp}} 5432 2>/dev/null; do sleep 2; done
    @echo "Postgres is ready."

# Nuclear rebuild: destroy VMs, recreate, deploy everything from scratch
rebuild-all:
    just vm-down
    cd terraform && terraform apply -auto-approve
    @echo "Waiting for VMs to boot..."
    @sleep 30
    just ansible-bootstrap
    just vm-kubeconfig
    just vm-deploy
    just wait-db
    just bootstrap

# Generate Temporal mTLS certificates
gen-certs:
    bash scripts/gen-temporal-certs.sh

# Generate SSH CA key pair for web terminal certificate signing
generate-ssh-ca:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -f ssh_ca ]; then
        echo "SSH CA key already exists (ssh_ca). Delete it first to regenerate."
        exit 0
    fi
    ssh-keygen -t ed25519 -f ssh_ca -C "hosting-platform-ca" -N ""
    echo ""
    echo "Generated ssh_ca (private key) and ssh_ca.pub (public key)"
    echo "The private key will be injected into Helm automatically on vm-deploy."
    echo "Add the public key to Ansible group vars (ssh_ca_public_key):"
    echo ""
    cat ssh_ca.pub

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

# --- Packer (Base Image) ---

# Build the node-agent binary for Linux (for VM deployment)
build-node-agent:
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/node-agent ./cmd/node-agent

# Build the dbadmin-proxy binary for Linux
build-dbadmin-proxy:
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/dbadmin-proxy ./cmd/dbadmin-proxy

# Build the base golden image (Ansible handles role-specific software)
packer-base:
    cd packer && packer init . && packer build .

# --- Ansible (Configuration Management) ---

# Run Ansible against all nodes (full convergence)
ansible-all: build-node-agent build-dbadmin-proxy
    cd ansible && HOSTING_API_KEY=$HOSTING_API_KEY ansible-playbook site.yml

# Run Ansible against a specific role group (e.g. just ansible-role web)
ansible-role role *args: build-node-agent
    cd ansible && HOSTING_API_KEY=$HOSTING_API_KEY ansible-playbook site.yml --limit {{role}} {{args}}

# Run Ansible against a single node by IP (e.g. just ansible-node 10.10.10.10)
ansible-node ip *args: build-node-agent
    cd ansible && HOSTING_API_KEY=$HOSTING_API_KEY ansible-playbook site.yml --limit {{ip}} {{args}}

# Deploy only the node-agent binary to all nodes
deploy-node-agent: build-node-agent
    cd ansible && HOSTING_API_KEY=$HOSTING_API_KEY ansible-playbook site.yml --tags node-agent

# Deploy only Vector config to all nodes
deploy-vector:
    cd ansible && HOSTING_API_KEY=$HOSTING_API_KEY ansible-playbook site.yml --tags vector

# Run Ansible with static inventory (for bootstrap before API is up)
ansible-bootstrap: build-node-agent build-dbadmin-proxy
    cd ansible && ansible-playbook site.yml -i inventory/static.ini

# --- VM Infrastructure ---

# Create VMs with Terraform and register them with the platform
# (Requires golden images — run `just packer-all` first)
vm-up:
    @if sudo virsh net-info hosting 2>/dev/null | grep -q 'Active:.*no'; then \
        echo "Starting libvirt network 'hosting'..."; \
        sudo virsh net-start hosting; \
    fi
    cd terraform && terraform apply -auto-approve
    @sudo virsh list --all --name | xargs -I{} sudo virsh start {} 2>/dev/null; true
    just _wait-api
    go run ./cmd/hostctl cluster apply -f clusters/vm-generated.yaml

# Rebuild everything: base image, recreate VMs, Ansible provision, deploy control plane
vm-rebuild:
    just packer-base
    just vm-down
    cd terraform && terraform apply -auto-approve
    @if sudo virsh net-info hosting 2>/dev/null | grep -q 'Active:.*no'; then \
        echo "Starting libvirt network 'hosting'..."; \
        sudo virsh net-start hosting; \
    fi
    just _wait-ssh
    just ansible-bootstrap
    just _rerun-cloudinit
    just _wait-k3s
    just vm-kubeconfig
    just vm-deploy
    just _wait-postgres
    just migrate
    just create-dev-key
    just create-agent-key
    just _wait-api
    go run ./cmd/hostctl cluster apply -f clusters/vm-generated.yaml

# Wait for k3s API to be reachable on the control plane VM
[private]
_wait-k3s:
    @echo "Waiting for k3s to be ready on {{cp}}..."
    @for i in $(seq 1 60); do \
        if ssh {{ssh_opts}} -o ConnectTimeout=2 ubuntu@{{cp}} "sudo k3s kubectl get nodes" >/dev/null 2>&1; then \
            echo "k3s is ready."; \
            exit 0; \
        fi; \
        sleep 5; \
    done; \
    echo "ERROR: k3s did not become ready after 5 minutes."; \
    echo "  Try: ssh ubuntu@{{cp}} 'sudo systemctl status k3s'"; \
    exit 1

# Wait for both PostgreSQL databases to accept connections
[private]
_wait-postgres:
    #!/usr/bin/env bash
    echo "Waiting for PostgreSQL (core on :5432, powerdns on :5433)..."
    for i in $(seq 1 60); do
        if (echo > /dev/tcp/{{cp}}/5432) 2>/dev/null && (echo > /dev/tcp/{{cp}}/5433) 2>/dev/null; then
            echo "Both PostgreSQL ports are accepting connections."
            exit 0
        fi
        sleep 3
    done
    echo "ERROR: PostgreSQL did not become ready after 3 minutes."
    echo "  Core DB (:5432):"
    (echo > /dev/tcp/{{cp}}/5432) 2>&1 && echo "    port open" || echo "    port closed"
    echo "  PowerDNS DB (:5433):"
    (echo > /dev/tcp/{{cp}}/5433) 2>&1 && echo "    port open" || echo "    port closed"
    echo "  Check pods: kubectl --context hosting get pods | grep postgres"
    exit 1

# Wait for the core API to be healthy
[private]
_wait-api:
    @echo "Waiting for control plane API ({{cp}}:8090)..."
    @for i in $(seq 1 60); do \
        if curl -sf -o /dev/null http://{{cp}}:8090/healthz 2>/dev/null; then \
            echo "Control plane API is ready."; \
            exit 0; \
        fi; \
        sleep 5; \
    done; \
    echo "ERROR: Core API did not become healthy after 5 minutes."; \
    echo "  Check pods: kubectl --context hosting get pods"; \
    echo "  Check logs: kubectl --context hosting logs deploy/hosting-core-api --tail=50"; \
    exit 1

# Wait for SSH on all VMs (after terraform creates them)
[private]
_wait-ssh:
    #!/usr/bin/env bash
    echo "Waiting for SSH on all VMs..."
    ALL_IPS="10.10.10.2 10.10.10.10 10.10.10.11 10.10.10.20 10.10.10.30 10.10.10.40 10.10.10.50 10.10.10.60 10.10.10.70 10.10.10.80"
    for ip in $ALL_IPS; do
        printf "  %s: " "$ip"
        for i in $(seq 1 60); do
            if ssh {{ssh_opts}} -o ConnectTimeout=2 ubuntu@$ip "true" 2>/dev/null; then
                echo "ready"
                break
            fi
            if [ $i -eq 60 ]; then
                echo "TIMEOUT"
                echo "ERROR: VM $ip did not accept SSH after 5 minutes."
                exit 1
            fi
            sleep 5
        done
    done
    echo "All VMs accepting SSH."

# Re-run cloud-init on all VMs (installs software first via Ansible, then runcmd succeeds)
[private]
_rerun-cloudinit:
    #!/usr/bin/env bash
    echo "Re-running cloud-init on all VMs (clean + reboot)..."
    ALL_IPS="10.10.10.2 10.10.10.10 10.10.10.11 10.10.10.20 10.10.10.30 10.10.10.40 10.10.10.50 10.10.10.60 10.10.10.70 10.10.10.80"
    for ip in $ALL_IPS; do
        echo "  Rebooting $ip..."
        ssh {{ssh_opts}} ubuntu@$ip "sudo cloud-init clean --logs && sudo reboot" 2>/dev/null || true
    done
    echo "Waiting 15s for VMs to go down..."
    sleep 15
    for ip in $ALL_IPS; do
        printf "  %s: " "$ip"
        for i in $(seq 1 90); do
            if ssh {{ssh_opts}} -o ConnectTimeout=2 ubuntu@$ip "true" 2>/dev/null; then
                echo "up"
                break
            fi
            if [ $i -eq 90 ]; then
                echo "TIMEOUT"
                echo "ERROR: VM $ip did not come back after reboot."
                exit 1
            fi
            sleep 5
        done
    done
    echo "All VMs back up. Waiting for cloud-init to finish..."
    for ip in $ALL_IPS; do
        printf "  %s cloud-init: " "$ip"
        if ssh {{ssh_opts}} ubuntu@$ip "sudo cloud-init status --wait" 2>/dev/null; then
            echo "done"
        else
            echo "warning (may be OK)"
        fi
    done
    echo "Cloud-init complete on all VMs."

# Destroy VMs
vm-down:
    -cd terraform && terraform destroy -auto-approve
    @# Clean up leftover files that prevent pool deletion
    @if [ -d /var/lib/libvirt/hosting-pool ]; then \
        sudo rm -rf /var/lib/libvirt/hosting-pool; \
    fi
    @# Ensure pool is undefined if terraform couldn't delete it
    @if sudo virsh pool-info hosting >/dev/null 2>&1; then \
        sudo virsh pool-destroy hosting 2>/dev/null || true; \
        sudo virsh pool-undefine hosting 2>/dev/null || true; \
    fi
    @# Ensure network is cleaned up
    @if sudo virsh net-info hosting >/dev/null 2>&1; then \
        sudo virsh net-destroy hosting 2>/dev/null || true; \
        sudo virsh net-undefine hosting 2>/dev/null || true; \
    fi

# Resolve VM name to IP (includes controlplane)
_vm-ip name:
    @cd terraform && terraform output -json 2>/dev/null | python3 -c "import sys,json; o=json.load(sys.stdin); d={'controlplane-0': o.get('controlplane_ip',{}).get('value','')}; [d.update(v['value']) for k,v in o.items() if k.endswith('_ips')]; print(d['{{name}}'])"

# SSH into a VM (e.g. just vm-ssh controlplane-0, just vm-ssh web-1-node-0)
vm-ssh name:
    ssh {{ssh_opts}} ubuntu@$(just _vm-ip {{name}})

# Run a command on a VM (e.g. just vm-exec controlplane-0 "sudo systemctl start k3s")
vm-exec name *cmd:
    ssh {{ssh_opts}} ubuntu@$(just _vm-ip {{name}}) {{cmd}}

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
    # Install/upgrade Helm chart (secrets from .env, SSH CA key from file)
    helm --kube-context hosting upgrade --install hosting \
      deploy/helm/hosting -f deploy/helm/hosting/values-dev.yaml \
      --set secrets.coreDatabaseUrl="$CORE_DATABASE_URL" \
      --set secrets.powerdnsDatabaseUrl="$POWERDNS_DATABASE_URL" \
      --set secrets.stalwartAdminToken="$STALWART_ADMIN_TOKEN" \
      --set secrets.secretEncryptionKey="$SECRET_ENCRYPTION_KEY" \
      --set secrets.llmApiKey="$LLM_API_KEY" \
      --set secrets.agentApiKey="$AGENT_API_KEY" \
      {{ if path_exists("ssh_ca") == "true" { "--set-file secrets.sshCaPrivateKey=ssh_ca" } else { "" } }}
    # Restart app pods to pick up new images (tag is always `latest`)
    kubectl --context hosting rollout restart deployment/hosting-core-api deployment/hosting-worker deployment/hosting-admin-ui deployment/hosting-mcp-server

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

# Deploy Vector config to all running VMs
vm-bootstrap-vector:
    just deploy-vector

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
