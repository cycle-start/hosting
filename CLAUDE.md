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
- **No soft deletes**: Always hard-delete rows (`DELETE FROM`). Never set `status = 'deleted'` and leave rows in the table. `UpdateResourceStatus` with `StatusDeleted` performs a `DELETE FROM`.

## Helm Chart Sync Rule

When modifying any of these, update the Helm chart in `deploy/helm/hosting/` to match:

- `internal/config/config.go` — new/changed env vars → update `templates/configmap.yaml` or `templates/secret.yaml`, and add defaults to `values.yaml`
- Health check endpoints (`/healthz`, `/readyz`) — update probe paths in deployment templates

Keep `values.yaml` defaults in sync with `config.go` defaults.

## Documentation Rule

After implementing any significant feature (new resource type, new runtime, new infrastructure component, new API endpoint group), document it in `docs/`:

- **Feature docs**: One markdown file per feature area (e.g., `docs/s3-storage.md`, `docs/email.md`). Cover: what it does, API endpoints, configuration, architecture decisions.
- **Plans**: Implementation plans go in `docs/plans/` before work starts.
- **STATUS.md**: Update the "What's Implemented" section after completing a feature.
- Keep docs concise and accurate. Remove outdated information rather than letting it accumulate.
