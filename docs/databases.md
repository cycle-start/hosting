# MySQL Databases

MySQL databases are provisioned on **database shards** (`shard.role = "database"`). Each shard contains one or more nodes running MySQL. The platform manages databases and database users through the core API, with Temporal workflows executing the actual provisioning on node agents via the `mysql` CLI.

> **Note:** MySQL replication between shard nodes is NOT yet implemented. Workflows currently execute `CREATE DATABASE` / `DROP DATABASE` on every node in the shard, but there is no active replication configured between them.

## Architecture

```
API request --> Core DB (desired state) --> Temporal workflow --> Node agent (mysql CLI)
```

- **Core DB** stores the `databases` and `database_users` tables (desired state).
- **Temporal workflows** orchestrate provisioning across all nodes in a shard.
- **Node agents** run `mysql` CLI commands against the local MySQL server.

Database and user names are validated with `^[a-zA-Z0-9_]+$` to prevent SQL injection.

## Data Model

### Database (`model.Database`)

| Field            | Type      | JSON                | Description                          |
|------------------|-----------|---------------------|--------------------------------------|
| `ID`             | `string`  | `id`                | Platform-generated unique ID         |
| `TenantID`       | `*string` | `tenant_id`         | Owning tenant (nullable)             |
| `Name`           | `string`  | `name`              | MySQL database name                  |
| `ShardID`        | `*string` | `shard_id`          | Database shard assignment            |
| `NodeID`         | `*string` | `node_id`           | Specific node (optional)             |
| `Status`         | `string`  | `status`            | Lifecycle status (see below)         |
| `StatusMessage`  | `*string` | `status_message`    | Error details when `status=failed`   |
| `CreatedAt`      | `time`    | `created_at`        | Creation timestamp                   |
| `UpdatedAt`      | `time`    | `updated_at`        | Last update timestamp                |
| `ShardName`      | `*string` | `shard_name`        | Resolved shard name (read-only)      |

### Database User (`model.DatabaseUser`)

| Field            | Type       | JSON                | Description                          |
|------------------|------------|---------------------|--------------------------------------|
| `ID`             | `string`   | `id`                | Platform-generated unique ID         |
| `DatabaseID`     | `string`   | `database_id`       | Parent database                      |
| `Username`       | `string`   | `username`          | MySQL username                       |
| `Password`       | `string`   | `password`          | MySQL password (write-only)          |
| `Privileges`     | `[]string` | `privileges`        | MySQL privilege list                 |
| `Status`         | `string`   | `status`            | Lifecycle status                     |
| `StatusMessage`  | `*string`  | `status_message`    | Error details when `status=failed`   |
| `CreatedAt`      | `time`     | `created_at`        | Creation timestamp                   |
| `UpdatedAt`      | `time`     | `updated_at`        | Last update timestamp                |

**Password handling:** The password is stored in the core DB for workflow execution but is **never returned** in GET or LIST responses. It is only accepted on create/update.

### Allowed Privileges

`ALL`, `ALL PRIVILEGES`, `SELECT`, `INSERT`, `UPDATE`, `DELETE`, `CREATE`, `DROP`, `ALTER`, `INDEX`, `REFERENCES`, `CREATE VIEW`, `SHOW VIEW`, `TRIGGER`, `EXECUTE`, `CREATE ROUTINE`, `ALTER ROUTINE`, `EVENT`, `LOCK TABLES`, `CREATE TEMPORARY TABLES`

If no privileges are specified, defaults to `ALL PRIVILEGES`.

### Status Lifecycle

`pending` --> `provisioning` --> `active`
                             --> `failed` (retryable)
`active`  --> `deleting`     --> `deleted`

## API Endpoints

### Databases

| Method   | Path                                      | Status | Description                      |
|----------|-------------------------------------------|--------|----------------------------------|
| `GET`    | `/tenants/{tenantID}/databases`           | 200    | List databases for a tenant      |
| `POST`   | `/tenants/{tenantID}/databases`           | 202    | Create a database                |
| `GET`    | `/databases/{id}`                         | 200    | Get a database                   |
| `DELETE` | `/databases/{id}`                         | 202    | Delete a database                |
| `POST`   | `/databases/{id}/migrate`                 | 202    | Migrate to a different shard     |
| `PUT`    | `/databases/{id}/tenant`                  | 200    | Reassign to a different tenant   |
| `POST`   | `/databases/{id}/retry`                   | 202    | Retry a failed provisioning      |

### Database Users

| Method   | Path                                      | Status | Description                      |
|----------|-------------------------------------------|--------|----------------------------------|
| `GET`    | `/databases/{databaseID}/users`           | 200    | List users for a database        |
| `POST`   | `/databases/{databaseID}/users`           | 202    | Create a database user           |
| `GET`    | `/database-users/{id}`                    | 200    | Get a database user              |
| `PUT`    | `/database-users/{id}`                    | 202    | Update password/privileges       |
| `DELETE` | `/database-users/{id}`                    | 202    | Delete a database user           |
| `POST`   | `/database-users/{id}/retry`              | 202    | Retry a failed provisioning      |

All 202 responses indicate an async Temporal workflow has been started.

## Request Bodies

### Create Database

```json
{
  "name": "myapp_prod",
  "shard_id": "shard-id-here",
  "users": [
    {
      "username": "myapp",
      "password": "secretpass123",
      "privileges": ["ALL"]
    },
    {
      "username": "myapp_readonly",
      "password": "readonlypass123",
      "privileges": ["SELECT"]
    }
  ]
}
```

The `users` array is optional. Nested users are created in the same request as the database. `name` must match `mysql_name` validation (alphanumeric + underscore).

### Migrate Database

```json
{
  "target_shard_id": "new-shard-id"
}
```

Migration uses `mysqldump --single-transaction --routines --triggers` on the source, pipes through `gzip`, then `gunzip | mysql` on the target. This is a multi-step Temporal workflow.

### Reassign Tenant

```json
{
  "tenant_id": "new-tenant-id"
}
```

Pass `null` for `tenant_id` to detach the database from any tenant. This is a synchronous metadata-only operation -- no data is moved.

## Temporal Workflows

| Workflow                       | Trigger           | Steps                                                  |
|--------------------------------|-------------------|--------------------------------------------------------|
| `CreateDatabaseWorkflow`       | POST create       | Set provisioning -> lookup shard nodes -> `CREATE DATABASE` on each node -> set active |
| `DeleteDatabaseWorkflow`       | DELETE             | Set deleting -> lookup shard nodes -> `DROP DATABASE` on each node -> set deleted |
| `CreateDatabaseUserWorkflow`   | POST create user  | Set provisioning -> lookup context -> `CREATE USER` + `GRANT` on each node -> set active |
| `UpdateDatabaseUserWorkflow`   | PUT update user   | Set provisioning -> lookup context -> `ALTER USER` + `REVOKE` + `GRANT` on each node -> set active |
| `DeleteDatabaseUserWorkflow`   | DELETE user       | Set deleting -> lookup context -> `DROP USER` on each node -> set deleted |

All workflows retry up to 3 times with a 30-second timeout per activity. On failure, the resource status is set to `failed` with the error message.

## Node Agent Operations

The `DatabaseManager` on each node agent executes MySQL commands via the `mysql` CLI, authenticating with the DSN from the `MYSQL_DSN` environment variable.

- **CreateDatabase**: `CREATE DATABASE IF NOT EXISTS \`name\``
- **DeleteDatabase**: `DROP DATABASE IF EXISTS \`name\``
- **CreateUser**: `DROP USER IF EXISTS` -> `CREATE USER` -> `GRANT` -> `FLUSH PRIVILEGES`
- **UpdateUser**: `ALTER USER` (password) -> `REVOKE ALL` -> `GRANT` -> `FLUSH PRIVILEGES`
- **DeleteUser**: `DROP USER IF EXISTS`
- **DumpDatabase**: `mysqldump --single-transaction --routines --triggers | gzip > path`
- **ImportDatabase**: `gunzip -c path | mysql dbname`

Users are created with host `'%'` (any host) to allow connections from any source within the network.
