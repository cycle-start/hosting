# Hosting Platform — Installation Guide

## Overview

This directory is the **operations root** for your hosting platform deployment. It contains everything needed to provision, deploy, and manage the platform. After setup, keep this directory — you'll come back to it for updates, re-deploys, and configuration changes.

## Prerequisites

- **Target machine**: Ubuntu 24.04 with SSH access (or localhost for single-node)
- **Docker**: Required on the machine running the installer (for building images)
- **Helm**: For deploying the control plane to k3s
- **Ansible**: For provisioning VMs with role-specific software

The setup wizard will guide you through configuration. Run it, fill in the forms, and click deploy.

## Quick Start

```bash
# Run the setup wizard
bin/setup

# Or with remote access (e.g., running on a server, accessing from your laptop):
bin/setup -host 0.0.0.0
# Then SSH tunnel: ssh -L 8400:localhost:8400 user@server
```

Open the URL printed in the terminal. The wizard walks through:

1. **Deployment mode** — Single machine (all-in-one) or multi-node
2. **Region & cluster** — Naming, PHP versions
3. **Brand** — Domain names, nameservers, mail hostname
4. **Database** — Built-in PostgreSQL or external
5. **Machines** — Node IPs and role assignments (multi-node only)
6. **TLS & Security** — Let's Encrypt or manual certs, optional Azure AD SSO
7. **Review** — Verify configuration
8. **Install** — Generate files, then run deploy steps

## What the Wizard Does

1. **Generates configuration files** into `generated/`:
   - Ansible inventory and group vars
   - Cluster topology (`cluster.yaml`)
   - Seed data (`seed.yaml`)
   - Kubeconfig (fetched from the control plane node)

2. **Writes `.env`** with operational secrets (API keys, database URLs, encryption keys, SSO credentials). Merges with any existing `.env` to preserve manually-set values.

3. **Runs deploy steps** (can also be run manually):
   - Generate SSH CA keypair (multi-node only)
   - Ansible provisioning (installs software on all nodes)
   - Deploy control plane (builds Docker images, deploys to k3s via Helm)
   - Register API key
   - Register cluster topology
   - Seed initial brand

## Directory Structure

After setup, your operations root looks like:

```
.
├── .env                          # Operational secrets (auto-generated, DO NOT commit)
├── .gitignore                    # Excludes secrets from version control
├── setup.yaml                    # Setup manifest (your configuration)
├── ssh_ca / ssh_ca.pub           # SSH CA keypair (multi-node only)
│
├── generated/
│   ├── ansible/inventory/        # Ansible inventory and group vars
│   ├── cluster.yaml              # Cluster topology definition
│   ├── seed.yaml                 # Initial brand/data seed
│   └── kubeconfig.yaml           # k3s kubeconfig
│
├── bin/                          # Platform binaries
│   ├── setup                     # Setup wizard
│   ├── hostctl                   # CLI for cluster management
│   ├── core-api                  # API server (also used for key creation)
│   └── ...
│
├── ansible/                      # Ansible playbooks and roles
├── deploy/
│   ├── k3s/                      # Kubernetes manifests (Grafana, Prometheus, etc.)
│   └── helm/hosting/             # Helm chart for the control plane
├── docker/                       # Dockerfiles for control plane services
└── scripts/                      # Deployment scripts
```

## Day-2 Operations

### Re-deploying after code updates

```bash
# Source environment and re-run the deploy script
scripts/deploy-controlplane.sh <target_host> [ssh_user]
```

The deploy script auto-sources `.env` for all required variables.

### Changing configuration

1. Re-run the setup wizard: `bin/setup`
2. Modify your settings
3. Click "Generate Files" to regenerate
4. Re-run the relevant deploy steps

Or edit `setup.yaml` directly and regenerate:
```bash
bin/setup generate -f setup.yaml
```

### Updating node software (Ansible)

```bash
cd ansible && ansible-playbook site.yml -i ../generated/ansible/inventory/static.ini
```

### Managing the cluster

```bash
# Register/update cluster topology
bin/hostctl cluster apply -f generated/cluster.yaml

# Seed brand data
bin/hostctl seed -f generated/seed.yaml

# Create an API key
bin/core-api create-api-key --name admin
```

## Version Control

This operations root can be tracked in a private git repository. The included `.gitignore` ensures secrets stay out of version control while configuration files are tracked. This lets your team:

- Track infrastructure changes over time
- Share the configuration across administrators
- Reproduce deployments from a known state

```bash
git init
git add .
git commit -m "Initial platform setup"
```

## SSO (Azure AD)

If you enabled SSO during setup, all control plane services authenticate via Azure AD:

- **Grafana** — Native OIDC integration
- **Headlamp** — Native OIDC integration
- **Temporal UI** — Native OIDC integration
- **Prometheus** — oauth2-proxy reverse proxy
- **Admin UI** — OIDC login with auto-provisioned API key

You need to register an app in Azure AD (Entra ID) with these redirect URIs:
- `https://admin.<domain>/auth/callback`
- `https://grafana.<domain>/login/generic_oauth`
- `https://headlamp.<domain>/oidc-callback`
- `https://temporal.<domain>/auth/sso/callback`
- `https://prometheus.<domain>/oauth2/callback`

## Troubleshooting

### Setup wizard won't start
Check that `bin/setup` is executable and port 8400 is free.

### Deploy step fails
Check the output in the wizard UI. Common issues:
- Docker not running (for image builds)
- SSH key not configured (for remote targets)
- k3s not installed on the target machine

### Services not accessible after deploy
- Verify HAProxy is running: `ssh <target> sudo systemctl status haproxy`
- Check k3s pods: `KUBECONFIG=generated/kubeconfig.yaml kubectl get pods`
- Check DNS: your `*.platform-domain` should resolve to the LB node IP

### SSO not working
- Verify redirect URIs are registered in Azure AD
- Check that `OIDC_CLIENT_ID` and `OIDC_TENANT_ID` are set in `.env`
- Check pod logs: `kubectl logs deploy/hosting-admin-ui`
