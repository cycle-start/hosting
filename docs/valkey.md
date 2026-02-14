# Valkey (Managed Redis)

Valkey instances are provisioned on **valkey shards** (`shard.role = "valkey"`). Each instance runs as a separate `valkey-server` process with its own config, port, and data directory. Users are managed via the Valkey ACL system. The platform handles instance lifecycle, user management, and data migration through Temporal workflows.

## Architecture

```
API request --> Core DB (desired state) --> Temporal workflow --> Node agent (valkey-cli / systemd)
```

- Each Valkey instance gets a **unique port** (auto-assigned) and an **auto-generated password**.
- Instances run as `valkey@{name}.service` systemd units with config at `{configDir}/{name}.conf`.
- Data is persisted with both RDB snapshots and AOF (append-only file).
- The default eviction policy is `allkeys-lru`.

## Data Model

### Valkey Instance (`model.ValkeyInstance`)

| Field            | Type      | JSON                | Description                           |
|------------------|-----------|---------------------|---------------------------------------|
| `ID`             | `string`  | `id`                | Platform-generated unique ID          |
| `TenantID`       | `*string` | `tenant_id`         | Owning tenant (nullable)              |
| `Name`           | `string`  | `name`              | Instance name (slug)                  |
| `ShardID`        | `*string` | `shard_id`          | Valkey shard assignment               |
| `Port`           | `int`     | `port`              | TCP port (auto-assigned)              |
| `MaxMemoryMB`    | `int`     | `max_memory_mb`     | Memory limit in MB (default: 64)      |
| `Password`       | `string`  | `password`          | Instance password (write-only)        |
| `Status`         | `string`  | `status`            | Lifecycle status                      |
| `StatusMessage`  | `*string` | `status_message`    | Error details when `status=failed`    |
| `CreatedAt`      | `time`    | `created_at`        | Creation timestamp                    |
| `UpdatedAt`      | `time`    | `updated_at`        | Last update timestamp                 |
| `ShardName`      | `*string` | `shard_name`        | Resolved shard name (read-only)       |

### Valkey User (`model.ValkeyUser`)

| Field              | Type       | JSON                  | Description                         |
|--------------------|------------|-----------------------|-------------------------------------|
| `ID`               | `string`   | `id`                  | Platform-generated unique ID        |
| `ValkeyInstanceID` | `string`   | `valkey_instance_id`  | Parent instance                     |
| `Username`         | `string`   | `username`            | ACL username (slug)                 |
| `Password`         | `string`   | `password`            | User password (write-only)          |
| `Privileges`       | `[]string` | `privileges`          | ACL command categories              |
| `KeyPattern`       | `string`   | `key_pattern`         | Key access pattern (default: `~*`)  |
| `Status`           | `string`   | `status`              | Lifecycle status                    |
| `StatusMessage`    | `*string`  | `status_message`      | Error details when `status=failed`  |
| `CreatedAt`        | `time`     | `created_at`          | Creation timestamp                  |
| `UpdatedAt`        | `time`     | `updated_at`          | Last update timestamp               |

**Password handling:** Instance and user passwords are **redacted** in all GET and LIST responses. The instance password is auto-generated on creation and never returned to the caller.

### Key Pattern

The `key_pattern` field controls which keys the user can access via Valkey's ACL system. It uses Valkey glob syntax:

- `~*` -- all keys (default)
- `~myapp:*` -- only keys prefixed with `myapp:`
- `~cache:*` -- only keys prefixed with `cache:`

### Privileges

Privileges are passed directly as Valkey ACL command categories/commands (e.g., `+@all`, `+@read`, `+@write`, `+@string`, `+GET`, `-FLUSHALL`). They are applied via `ACL SETUSER`.

### Status Lifecycle

`pending` --> `provisioning` --> `active`
                             --> `failed` (retryable)
`active`  --> `deleting`     --> `deleted`

## API Endpoints

### Valkey Instances

| Method   | Path                                           | Status | Description                      |
|----------|------------------------------------------------|--------|----------------------------------|
| `GET`    | `/tenants/{tenantID}/valkey-instances`         | 200    | List instances for a tenant      |
| `POST`   | `/tenants/{tenantID}/valkey-instances`         | 202    | Create an instance               |
| `GET`    | `/valkey-instances/{id}`                       | 200    | Get an instance                  |
| `DELETE` | `/valkey-instances/{id}`                       | 202    | Delete an instance               |
| `POST`   | `/valkey-instances/{id}/migrate`               | 202    | Migrate to a different shard     |
| `PUT`    | `/valkey-instances/{id}/tenant`                | 200    | Reassign to a different tenant   |
| `POST`   | `/valkey-instances/{id}/retry`                 | 202    | Retry a failed provisioning      |

### Valkey Users

| Method   | Path                                           | Status | Description                      |
|----------|------------------------------------------------|--------|----------------------------------|
| `GET`    | `/valkey-instances/{instanceID}/users`         | 200    | List users for an instance       |
| `POST`   | `/valkey-instances/{instanceID}/users`         | 202    | Create a user                    |
| `GET`    | `/valkey-users/{id}`                           | 200    | Get a user                       |
| `PUT`    | `/valkey-users/{id}`                           | 202    | Update password/privileges/keys  |
| `DELETE` | `/valkey-users/{id}`                           | 202    | Delete a user                    |
| `POST`   | `/valkey-users/{id}/retry`                     | 202    | Retry a failed provisioning      |

All 202 responses indicate an async Temporal workflow has been started.

## Request Bodies

### Create Valkey Instance

```json
{
  "name": "myapp-cache",
  "shard_id": "shard-id-here",
  "max_memory_mb": 128,
  "users": [
    {
      "username": "myapp",
      "password": "secretpass123",
      "privileges": ["+@all"],
      "key_pattern": "~myapp:*"
    }
  ]
}
```

- `name` must be a valid slug.
- `max_memory_mb` defaults to **64 MB** if not specified.
- `users` array is optional. Nested users are created in the same request.
- A port and instance password are auto-generated.

### Create Valkey User

```json
{
  "username": "readonly_user",
  "password": "readonlypass123",
  "privileges": ["+@read", "+@connection"],
  "key_pattern": "~*"
}
```

`key_pattern` defaults to `~*` (all keys) if omitted.

### Migrate Valkey Instance

```json
{
  "target_shard_id": "new-shard-id"
}
```

Migration uses RDB dump/import: `BGSAVE` on the source, copy the RDB file, then stop the target instance, replace its RDB, remove AOF, and restart. This ensures a consistent snapshot transfer.

### Reassign Tenant

```json
{
  "tenant_id": "new-tenant-id"
}
```

Pass `null` for `tenant_id` to detach. This is a synchronous metadata-only operation.

## Temporal Workflows

| Workflow                         | Trigger           | Steps                                                  |
|----------------------------------|-------------------|--------------------------------------------------------|
| `CreateValkeyInstanceWorkflow`   | POST create       | Set provisioning -> lookup shard nodes -> create instance on each node -> set active |
| `DeleteValkeyInstanceWorkflow`   | DELETE             | Set deleting -> lookup shard nodes -> delete instance on each node -> set deleted |
| `CreateValkeyUserWorkflow`       | POST create user  | Set provisioning -> lookup context -> `ACL SETUSER` on each node -> set active |
| `UpdateValkeyUserWorkflow`       | PUT update user   | Set provisioning -> lookup context -> `ACL DELUSER` + `ACL SETUSER` on each node -> set active |
| `DeleteValkeyUserWorkflow`       | DELETE user       | Set deleting -> lookup context -> `ACL DELUSER` on each node -> set deleted |

All workflows retry up to 3 times with a 30-second timeout per activity.

## Node Agent Operations

The `ValkeyManager` on each node agent manages instances via config files, `valkey-server`, `valkey-cli`, and systemd.

### Instance Management

- **CreateInstance**: Write config to `{configDir}/{name}.conf` -> create data dir -> start `valkey-server --daemonize yes` -> enable systemd unit. Idempotent: if the instance exists, config is converged and running config is updated via `CONFIG SET`.
- **DeleteInstance**: `SHUTDOWN NOSAVE` via valkey-cli -> stop systemd unit -> remove config file -> remove data directory.

### Instance Config

Generated config for each instance:

```
port {port}
bind 0.0.0.0
protected-mode yes
requirepass {password}
maxmemory {maxMemoryMB}mb
maxmemory-policy allkeys-lru
dir {dataDir}/{name}
dbfilename dump.rdb
appendonly yes
appendfilename "appendonly.aof"
```

### User Management (ACL)

- **CreateUser**: `ACL SETUSER {username} on >{password} {keyPattern} {privileges...}` -> `ACL SAVE`
- **UpdateUser**: `ACL DELUSER` -> `ACL SETUSER` (delete and recreate) -> `ACL SAVE`
- **DeleteUser**: `ACL DELUSER {username}` -> `ACL SAVE`

### Data Migration

- **DumpData**: `BGSAVE` -> poll `LASTSAVE` until timestamp changes -> copy `dump.rdb` to dump path.
- **ImportData**: `SHUTDOWN NOSAVE` -> stop systemd -> replace `dump.rdb` -> remove AOF files -> restart `valkey-server` -> start systemd.
