# Hosting Platform

## Build & Run

```bash
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
go build ./...
go test ./...
go vet ./...
```

Docker Compose (local dev):
```bash
docker compose --profile platform up -d          # start everything
docker compose --profile platform build admin-ui && docker compose --profile platform up -d admin-ui  # rebuild admin-ui after frontend changes
```

## Conventions

- **Migrations**: Edit the original migration file and wipe/restart the DB. Never add ALTER TABLE migrations — this is pre-release software.
- **API responses**: List endpoints return `{items: [...], has_more: bool}`. Async operations return 202 Accepted.
- **Handlers**: parse/validate request → build model → call service → return response. No business logic in handlers.
- **Config**: All config is via environment variables loaded in `internal/config/config.go`.

## Helm Chart Sync Rule

When modifying any of these, update the Helm chart in `deploy/helm/hosting/` to match:

- `internal/config/config.go` — new/changed env vars → update `templates/configmap.yaml` or `templates/secret.yaml`, and add defaults to `values.yaml`
- `docker-compose.yml` — new services, changed ports, health checks → update corresponding deployment/service templates
- Health check endpoints (`/healthz`, `/readyz`) — update probe paths in deployment templates

Keep `values.yaml` defaults in sync with `config.go` defaults and `docker-compose.yml` values.
