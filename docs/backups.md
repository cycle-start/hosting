# Backup & Restore

The hosting platform supports on-demand backups of webroots (files) and MySQL databases. Backups are created, restored, and deleted via asynchronous Temporal workflows that run on shard node agents.

## Backup Types

| Type       | Format     | Tool       | Storage Path Pattern                              |
|------------|------------|------------|---------------------------------------------------|
| `web`      | `.tar.gz`  | `tar czf`  | `/var/backups/hosting/{tenantID}/{backupID}.tar.gz`  |
| `database` | `.sql.gz`  | `mysqldump | gzip` | `/var/backups/hosting/{tenantID}/{backupID}.sql.gz` |

Web backups archive the webroot's storage directory. Database backups run `mysqldump` piped through `gzip`.

## Data Model

Each backup record tracks:

- `id` -- unique backup ID
- `tenant_id` -- owning tenant
- `type` -- `web` or `database`
- `source_id` -- ID of the webroot or database being backed up
- `source_name` -- human-readable name resolved at creation time
- `storage_path` -- filesystem path to the backup file (set after completion)
- `size_bytes` -- file size in bytes (set after completion)
- `status` -- lifecycle status (see below)
- `started_at` / `completed_at` -- timing metadata

## Status Lifecycle

```
pending -> provisioning -> active
                       \-> failed (retryable)
active -> provisioning (restore in progress) -> active
active -> deleting -> deleted
```

## API Endpoints

All backup endpoints enforce brand-scoped authorization.

### List backups for a tenant
```
GET /tenants/{tenantID}/backups?limit=50&cursor=...
```
Returns paginated `{items: [...], has_more: bool}`.

### Create a backup
```
POST /tenants/{tenantID}/backups
{
  "type": "web" | "database",
  "source_id": "<webroot or database ID>"
}
```
Returns `202 Accepted` with the backup record. The `source_name` is resolved from the webroot or database at creation time.

### Get a backup
```
GET /backups/{id}
```

### Delete a backup
```
DELETE /backups/{id}
```
Returns `202 Accepted`. Triggers the `DeleteBackupWorkflow` which removes the backup file from disk, then marks the record as `deleted`.

### Restore a backup
```
POST /backups/{id}/restore
```
Returns `202 Accepted`. For web backups, extracts the tar.gz over the webroot directory on all shard nodes. For database backups, pipes `gunzip` into `mysql` on the first node.

### Retry a failed backup
```
POST /backups/{id}/retry
```
Re-triggers the backup workflow for a backup in `failed` status.

## Workflows

All workflows use a 5-minute `StartToCloseTimeout` (30s for delete) and up to 3 retry attempts per activity.

### CreateBackupWorkflow

1. Sets backup status to `provisioning`.
2. Fetches `BackupContext` (backup record, tenant, shard nodes).
3. Validates the tenant has an assigned shard with at least one node.
4. Runs the backup on the first node in the shard:
   - **Web**: calls `CreateWebBackup` which runs `tar czf` on the webroot directory.
   - **Database**: calls `CreateMySQLBackup` which runs `mysqldump | gzip`.
5. Records `storage_path`, `size_bytes`, `started_at`, `completed_at`.
6. Sets status to `active`.

On any failure, the backup is marked `failed` with an error message.

### RestoreBackupWorkflow

1. Fetches `BackupContext` and sets status to `provisioning`.
2. Restores based on type:
   - **Web**: calls `RestoreWebBackup` (`tar xzf`) on all shard nodes (shared CephFS, but all nodes for safety).
   - **Database**: calls `RestoreMySQLBackup` (`gunzip -c | mysql`) on the first node only.
3. Sets status back to `active`.

### DeleteBackupWorkflow

1. Fetches `BackupContext`.
2. Calls `DeleteBackupFile` on the first node to remove the file from disk.
3. Sets status to `deleted`.

## Storage Location

Backup files are stored on the shard nodes at `/var/backups/hosting/{tenantID}/`. The directory is created automatically if it does not exist. On web shards this is on shared CephFS storage; on database shards it is on local SSD.

## Retention & Automatic Cleanup

Old backups are automatically cleaned up by the `CleanupOldBackupsWorkflow`, which runs on a daily cron schedule (`0 5 * * *` -- 5:00 AM UTC).

The retention period is configured via the `BACKUP_RETENTION_DAYS` environment variable (default: **30 days**).

The cleanup workflow:
1. Queries for all active backups older than the retention period (`GetOldBackups` activity).
2. Starts a child `DeleteBackupWorkflow` for each expired backup.
3. Continues processing remaining backups even if individual deletions fail.

## Source Files

- Handler: `internal/api/handler/backup.go`
- Model: `internal/model/backup.go`
- Workflows: `internal/workflow/backup.go`
- Cleanup: `internal/workflow/maintenance.go`
- Node activities: `internal/activity/node_local.go` (backup section)
- Activity params: `internal/activity/params.go`
- Config: `internal/config/config.go` (`BACKUP_RETENTION_DAYS`)
- Cron registration: `cmd/worker/main.go`
