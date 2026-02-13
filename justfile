# Hosting Platform - Development Commands

# Default: show available commands
default:
    @just --list

# --- Build ---

# Build all Go binaries
build:
    go build ./cmd/...

# Build Docker images (control plane only)
build-docker:
    docker compose build

# Generate protobuf code
proto:
    protoc -Iproto \
        --go_out=. --go_opt=module=github.com/edvin/hosting \
        --go-grpc_out=. --go-grpc_opt=module=github.com/edvin/hosting \
        proto/agent/v1/agent.proto proto/agent/v1/types.proto

# Build admin UI for development
build-admin:
    cd web/admin && npm run build

# Start admin UI dev server (with API proxy to localhost:8090)
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

# Run integration tests (requires Docker services)
test-integration:
    go test ./... -tags integration -count=1

# Run e2e tests (requires VMs running)
test-e2e:
    go test ./tests/e2e/... -tags e2e -count=1 -timeout 10m -v

# Run all tests
test-all: test test-integration test-e2e

# --- Lint ---

# Run linter
lint:
    golangci-lint run ./...

# Run go vet
vet:
    go vet ./...

# --- Docker ---

# Start all services (infra + monitoring + platform)
up:
    docker compose --profile infra --profile monitoring --profile platform up -d

# Start infrastructure only
up-infra:
    docker compose --profile infra up -d

# Start with Temporal mTLS
up-tls:
    docker compose -f docker-compose.yml -f docker-compose.tls.yml --profile infra --profile platform up -d

# Stop all services
down:
    docker compose --profile infra --profile monitoring --profile platform down

# Stop all services and remove volumes
down-clean:
    docker compose --profile infra --profile monitoring --profile platform down -v

# View logs for all services
logs:
    docker compose logs -f

# View logs for a specific service (e.g. just log core-api)
log svc:
    docker compose logs -f {{svc}}

# Restart a specific service
restart svc:
    docker compose restart {{svc}}

# Rebuild and restart a specific service (picks up code changes)
rebuild svc:
    docker compose up -d --build {{svc}}

# Show service status
ps:
    docker compose ps

# --- Database ---

# Run core DB migrations
migrate-core:
    goose -dir migrations/core postgres "postgres://hosting:hosting@localhost:5432/hosting_core?sslmode=disable" up

# Run PowerDNS DB migrations
migrate-powerdns:
    goose -dir migrations/powerdns postgres "postgres://hosting:hosting@localhost:5433/hosting_powerdns?sslmode=disable" up

# Run all migrations
migrate: migrate-core migrate-powerdns

# Reset core DB (drop all tables and goose version tracking)
reset-core:
    docker compose exec -T core-db psql -U hosting hosting_core -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

# Reset PowerDNS DB (drop all tables and goose version tracking)
reset-powerdns:
    docker compose exec -T powerdns-db psql -U hosting hosting_powerdns -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

# Reset all databases
reset-db: reset-core reset-powerdns

# Migration status
migrate-status:
    @echo "=== Core DB ==="
    goose -dir migrations/core postgres "postgres://hosting:hosting@localhost:5432/hosting_core?sslmode=disable" status
    @echo ""
    @echo "=== PowerDNS DB ==="
    goose -dir migrations/powerdns postgres "postgres://hosting:hosting@localhost:5433/hosting_powerdns?sslmode=disable" status

# --- Local Development ---

# Start Docker infra + control plane, run migrations, then create VMs
dev: up-infra
    @echo "Waiting for databases to be ready..."
    @sleep 5
    just migrate
    @echo "Starting platform services..."
    docker compose --profile platform up -d
    @echo ""
    @echo "Services running:"
    @echo "  Core API:      http://localhost:8090"
    @echo "  Temporal UI:   http://localhost:8080"
    @echo "  HAProxy stats: http://localhost:8404/stats"
    @echo "  PowerDNS API:  http://localhost:8081"
    @echo "  MySQL:         localhost:3306"
    @echo "  Registry:      http://localhost:5000"
    @echo "  Prometheus:    http://localhost:9091"
    @echo "  Grafana:       http://localhost:3000"
    @echo "  Loki:          http://localhost:3100"

# Full dev setup with VMs
dev-vm: dev
    just vm-up

# Full dev setup + e2e tests
dev-e2e: dev-vm
    @echo "Running e2e tests..."
    just test-e2e

# List images in local registry
registry-list:
    curl -s http://localhost:5000/v2/_catalog | python3 -m json.tool

# --- Monitoring ---

# Start monitoring stack only
up-monitoring:
    docker compose --profile monitoring up -d

# Open Grafana dashboard
grafana:
    @echo "Grafana: http://localhost:3000 (admin/admin)"

# Generate Temporal mTLS certificates
gen-certs:
    bash scripts/gen-temporal-certs.sh

# --- Auth ---

# Create an API key (requires core-db running)
create-api-key name:
    CORE_DATABASE_URL="postgres://hosting:hosting@localhost:5432/hosting_core?sslmode=disable" go run ./cmd/core-api create-api-key --name {{name}}

# --- Utilities ---

# Connect to core DB via psql
db-core:
    psql "postgres://hosting:hosting@localhost:5432/hosting_core?sslmode=disable"

# Connect to PowerDNS DB via psql
db-powerdns:
    psql "postgres://hosting:hosting@localhost:5433/hosting_powerdns?sslmode=disable"

# Connect to MySQL
db-mysql:
    mysql -h 127.0.0.1 -P 3306 -u root -prootpassword

# Check Ceph cluster health
ceph-status:
    docker compose exec ceph ceph -s

# Update HAProxy map entry (e.g. just lb-set www.example.com shard-web-a)
lb-set fqdn backend:
    echo "set map /var/lib/haproxy/maps/fqdn-to-shard.map {{fqdn}} {{backend}}" | nc localhost 9999

# Delete HAProxy map entry
lb-del fqdn:
    echo "del map /var/lib/haproxy/maps/fqdn-to-shard.map {{fqdn}}" | nc localhost 9999

# Show HAProxy map
lb-show:
    echo "show map /var/lib/haproxy/maps/fqdn-to-shard.map" | nc localhost 9999

# --- VM Infrastructure ---

# Build the node-agent binary for Linux (for VM deployment)
build-node-agent:
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/node-agent ./cmd/node-agent

# Create VMs with Terraform and register them with the platform
vm-up: build-node-agent
    cd terraform && terraform apply -auto-approve
    go run ./cmd/hostctl cluster apply -f terraform/cluster.yaml

# Destroy VMs
vm-down:
    cd terraform && terraform destroy -auto-approve

# SSH into a VM (e.g. just vm-ssh web-1-node-0)
vm-ssh name:
    ssh -o StrictHostKeyChecking=no ubuntu@$(cd terraform && terraform output -json 2>/dev/null | python3 -c "import sys,json; o=json.load(sys.stdin); d={}; [d.update(v['value']) for k,v in o.items() if k.endswith('_ips')]; print(d['{{name}}'])")

# Expose HAProxy via ngrok for testing real domains
vm-ngrok:
    ngrok http 80
