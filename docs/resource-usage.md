# Resource Usage Collection

Tracks per-resource disk usage for visibility. Collection-only — no quotas or billing enforcement.

## How It Works

A cron workflow (`CollectResourceUsageWorkflow`) runs every 30 minutes:

1. Lists all active web shards → picks one node per shard → calls `GetResourceUsage(role: "web")`
2. Lists all active database shards → picks primary node per shard → calls `GetResourceUsage(role: "database")`
3. For each result, upserts into the `resource_usage` table via `UpsertResourceUsage`

### Web Node Collection

Walks `/var/www/storage/*/webroots/*/` directories and runs `du -sb` on each to get per-webroot byte counts.

### Database Node Collection

Queries MySQL `information_schema.tables`:

```sql
SELECT table_schema, SUM(data_length + index_length) FROM information_schema.tables GROUP BY table_schema
```

### Email

Not yet collected — requires Stalwart API integration for storage accounting.

## Data Model

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Auto-generated UUID |
| `resource_type` | string | `webroot` or `database` |
| `resource_id` | string | References the resource |
| `tenant_id` | string | Owning tenant |
| `bytes_used` | int64 | Disk usage in bytes |
| `collected_at` | timestamp | When usage was last measured |

Single row per resource, upserted on each collection. `collected_at` tracks freshness.

## API

| Method | Path | Response | Description |
|--------|------|----------|-------------|
| `GET` | `/tenants/{id}/resource-usage` | 200 | List all usage entries for a tenant |

### Response

```json
{
  "items": [
    {
      "id": "abc123",
      "resource_type": "webroot",
      "resource_id": "wr-456",
      "tenant_id": "t-789",
      "bytes_used": 104857600,
      "collected_at": "2026-02-19T10:30:00Z"
    }
  ],
  "has_more": false
}
```

## Architecture

- Cron schedule: `*/30 * * * *` (every 30 minutes)
- Node activity: `GetResourceUsage` runs on node-agent Temporal task queue
- Core activity: `UpsertResourceUsage` writes to PostgreSQL
- Resolution: name-based lookup (tenant+webroot path → resource ID)
