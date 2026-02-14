# Hosting Platform

## Build & Run

```bash
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
go build ./...
go test ./...
go vet ./...
```

E2E tests (requires running control plane at `api.hosting.test`):
```bash
HOSTING_E2E=1 go test ./tests/e2e/... -v
```

## Conventions

- **Migrations**: Edit the original migration file and wipe/restart the DB. Never add ALTER TABLE migrations — this is pre-release software.
- **API responses**: List endpoints return `{items: [...], has_more: bool}`. Async operations return 202 Accepted.
- **Handlers**: parse/validate request → build model → call service → return response. No business logic in handlers.
- **Config**: All config is via environment variables loaded in `internal/config/config.go`.

## Helm Chart Sync Rule

When modifying any of these, update the Helm chart in `deploy/helm/hosting/` to match:

- `internal/config/config.go` — new/changed env vars → update `templates/configmap.yaml` or `templates/secret.yaml`, and add defaults to `values.yaml`
- Health check endpoints (`/healthz`, `/readyz`) — update probe paths in deployment templates

Keep `values.yaml` defaults in sync with `config.go` defaults.
