# Production Deployment Guide

End-to-end guide for deploying the hosting platform on your own infrastructure.

## Prerequisites

- **VMs**: Ubuntu 24.04 with SSH access (see topology below)
- **Kubernetes**: A cluster for the control plane (k3s, EKS, GKE, etc.)
- **PostgreSQL**: Two databases — one for the core API, one for PowerDNS
- **Temporal**: Self-hosted or Temporal Cloud
- **Domain**: A domain name with DNS control (for the admin UI, API, and tenant sites)
- **Container registry access**: `docker login ghcr.io` with a GitHub PAT that has `read:packages` scope

## Node Topology

The platform requires several node roles. A minimal deployment needs at least one node per role:

| Role | Purpose | Min count |
|------|---------|-----------|
| web | PHP hosting (nginx + PHP-FPM + node-agent) | 1 |
| database | MySQL (managed by node-agent) | 1 |
| dns | PowerDNS authoritative server | 1 |
| valkey | Valkey (Redis-compatible) cache | 1 |
| storage | Ceph object storage | 1 |
| lb | HAProxy load balancer | 1 |
| email | Stalwart mail server | 1 |
| gateway | WireGuard VPN gateway | 1 |
| dbadmin | phpMyAdmin + dbadmin-proxy | 1 |

## Step 1: Pull Container Images

Images are published to GitHub Container Registry:

```bash
docker pull ghcr.io/cycle-start/hosting-core-api:latest
docker pull ghcr.io/cycle-start/hosting-worker:latest
docker pull ghcr.io/cycle-start/hosting-admin-ui:latest
docker pull ghcr.io/cycle-start/controlpanel-api:latest
docker pull ghcr.io/cycle-start/controlpanel-ui:latest
```

Or build from source:
```bash
just images          # builds all 6 images with ghcr.io tags
just images-push     # pushes to ghcr.io
```

## Step 2: Build Node Binaries

Node agents run directly on VMs (not in containers):

```bash
just build-node-agent      # outputs bin/node-agent (linux/amd64)
just build-dbadmin-proxy   # outputs bin/dbadmin-proxy (linux/amd64)
```

## Step 3: Create Configuration Files

### Ansible inventory

Copy the example and fill in your node IPs:

```bash
cp ansible/inventory/production.ini.example ansible/inventory/production.ini
```

Edit `ansible/inventory/production.ini` with your actual IPs, node IDs, and credentials. Key variables in `[all:vars]`:

- `temporal_address` — Temporal server endpoint
- `region_id`, `cluster_id` — must match your cluster definition
- `core_api_url`, `core_api_token` — for dbadmin-proxy to authenticate with the core API

### Cluster topology

Copy the example and fill in your node IPs:

```bash
cp clusters/production.yaml.example clusters/production.yaml
```

This file defines regions, clusters, shards, and nodes. It must match your Ansible inventory.

### Helm values

Copy the example and fill in your environment:

```bash
cp deploy/helm/hosting/values-production.yaml.example deploy/helm/hosting/values-production.yaml
```

Key sections to configure:
- `config.baseDomain` — your domain
- `secrets.existingSecret` — name of a pre-created K8s secret (see Step 5)
- `image.pullSecret` — name of the registry pull secret
- `temporal.address` — Temporal endpoint

## Step 4: Provision Nodes with Ansible

Run the full playbook against your production inventory:

```bash
cd ansible && ansible-playbook site.yml -i inventory/production.ini
```

This installs role-specific software (nginx, PHP, MySQL, HAProxy, etc.), deploys the node-agent binary, writes `/etc/default/node-agent` with the correct identity, and starts all services.

To deploy only node-agent updates:
```bash
cd ansible && ansible-playbook site.yml -i inventory/production.ini --tags node-agent
```

## Step 5: Deploy the Control Plane

### Create Kubernetes secrets

```bash
# Registry pull secret (for private ghcr.io images)
kubectl create secret docker-registry ghcr-pull-secret \
  --docker-server=ghcr.io \
  --docker-username=<github-user> \
  --docker-password=<PAT>

# Application secrets
kubectl create secret generic hosting-secrets \
  --from-literal=CORE_DATABASE_URL='postgres://user:pass@host:5432/hosting_core?sslmode=require' \
  --from-literal=POWERDNS_DATABASE_URL='postgres://user:pass@host:5433/hosting_powerdns?sslmode=require' \
  --from-literal=SECRET_ENCRYPTION_KEY='<32-byte-hex-key>' \
  --from-literal=CONTROLPANEL_DATABASE_URL='postgres://user:pass@host:5432/controlpanel?sslmode=require' \
  --from-literal=CONTROLPANEL_JWT_SECRET='<random-secret>'
```

### Install with Helm

```bash
helm install hosting deploy/helm/hosting \
  -f deploy/helm/hosting/values-production.yaml
```

Wait for all pods to be ready:
```bash
kubectl get pods -w
```

### Run migrations

Migrations run automatically on startup when `coreApi.migrate: true` and `controlpanelApi.migrate: true` are set (default in the production values template).

## Step 6: Register Cluster Topology

Tell the platform about your infrastructure:

```bash
go run ./cmd/hostctl cluster apply -f clusters/production.yaml -f seeds/runtimes.yaml
```

This registers regions, clusters, shards, and nodes so the platform knows which nodes exist and what roles they serve.

## Step 7: Create Initial API Key

```bash
CORE_DATABASE_URL='postgres://...' go run ./cmd/core-api create-api-key --name admin
```

Save the output — this is your admin API key for managing the platform.

## Updating

### Code changes (control plane)

```bash
just images                              # rebuild images
just images-tag $(git rev-parse --short HEAD)  # tag with git SHA
helm upgrade hosting deploy/helm/hosting \
  -f deploy/helm/hosting/values-production.yaml \
  --set image.coreApi.tag=<new-tag> \
  --set image.worker.tag=<new-tag> \
  --set image.adminUi.tag=<new-tag>
```

### Node agent changes

```bash
just build-node-agent
cd ansible && ansible-playbook site.yml -i inventory/production.ini --tags node-agent
```

## Troubleshooting

### Pods stuck in ImagePullBackOff

Check that the `ghcr-pull-secret` exists and has valid credentials:
```bash
kubectl get secret ghcr-pull-secret -o yaml
kubectl describe pod <pod-name>
```

### Node agent not starting

Check the environment file and service status:
```bash
ssh ubuntu@<node-ip> "cat /etc/default/node-agent"
ssh ubuntu@<node-ip> "sudo systemctl status node-agent"
ssh ubuntu@<node-ip> "sudo journalctl -u node-agent --no-pager -n 50"
```

### Core API not healthy

```bash
kubectl logs deploy/hosting-core-api --tail=50
kubectl describe pod -l app.kubernetes.io/component=core-api
```
