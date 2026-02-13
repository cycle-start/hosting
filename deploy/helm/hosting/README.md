# Hosting Platform Helm Chart

Deploys the hosting platform control plane to Kubernetes:

- **core-api** — REST API server with database migrations
- **worker** — Temporal workflow worker
- **admin-ui** — Admin dashboard (React SPA served by Go binary)

Node agents are **not** included — they run on VMs provisioned via Terraform.

## Prerequisites

The control plane requires:

- **PostgreSQL** — for core database and PowerDNS database
- **Temporal** — workflow orchestration engine

### Production

Use managed services (e.g., AWS RDS, Temporal Cloud) and configure connection
strings via `secrets.coreDatabaseUrl`, `secrets.powerdnsDatabaseUrl`, and
`temporal.address`.

### Quick evaluation

For testing, enable the bundled sub-charts:

```yaml
postgresql:
  enabled: true
  auth:
    username: hosting
    password: hosting
    database: hosting_core

temporal:
  server:
    enabled: true
```

**Do not use bundled sub-charts in production.** They are single-instance
deployments without backups, monitoring, or HA. Use managed services or
operator-managed deployments instead.

## Install

```bash
helm dependency build deploy/helm/hosting/
helm install hosting deploy/helm/hosting/ -f my-values.yaml
```

## Configuration

See [values.yaml](values.yaml) for all options.

| Key | Description | Default |
|-----|-------------|---------|
| `coreApi.replicas` | API server replicas | `1` |
| `coreApi.migrate` | Run DB migrations on startup | `true` |
| `worker.replicas` | Worker replicas | `1` |
| `adminUi.replicas` | Admin UI replicas | `1` |
| `secrets.existingSecret` | Use an existing K8s Secret | `""` |
| `temporal.address` | Temporal frontend address | `temporal:7233` |
| `temporal.tls.enabled` | Enable mTLS to Temporal | `false` |
| `ingress.enabled` | Create an Ingress resource | `false` |
| `postgresql.enabled` | Deploy bundled PostgreSQL | `false` |
| `temporal.server.enabled` | Deploy bundled Temporal | `false` |
