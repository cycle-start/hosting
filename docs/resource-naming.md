# Resource Naming

Resources that need system-level identifiers (Linux usernames, MySQL database names, file paths, systemd units, RGW buckets) use auto-generated **prefixed short names** as their primary key (`id`), generated via `platform.NewName(prefix)`. These short names serve double duty as both the database primary key and the system-level identifier — there is no separate UUID.

Other resources (brands, zones, FQDNs, nodes, etc.) use UUIDs or user-supplied identifiers as appropriate.

## Naming Table

| Resource | PK (`id`) | Prefix | System Use |
|---|---|---|---|
| Tenant | short name | `t` | Linux username, CephFS paths, S3 RGW user UID |
| Webroot | short name | `w` | File paths, nginx configs |
| Database | short name | `db` | MySQL database name |
| Valkey Instance | short name | `kv` | systemd unit, config file, data dir |
| S3 Bucket | short name | `s3` | RGW bucket naming |
| Cron Job | short name | `cj` | systemd timer/service unit names |
| Daemon | short name | `d` | supervisord program config |
| Brand | UUID | _(none)_ | User-supplied `name` for display |
| Zone | UUID | _(none)_ | User-supplied `name` |
| Region | user-supplied | _(none)_ | e.g. `osl-1` |
| Cluster | user-supplied | _(none)_ | e.g. `dev` |
| Node | UUID (Terraform) | _(none)_ | Temporal task queue routing |

## Name Format

IDs are generated as `{prefix}{10-char-random}`, using the character set `abcdefghijklmnopqrstuvwxyz0123456789`. Examples:

- Tenant: `t8k3pq7w2m`
- Webroot: `w4n8q1w5r2`
- Database: `dbf4n8q1w5r`
- Valkey: `kvj6t2y8e4h`
- S3 Bucket: `s3b3g7l1v5d`
- Cron Job: `cjk5n9r3w7a`
- Daemon: `dh2q6v0z4b8`

IDs are globally unique per resource type (enforced by primary key constraint).

## System-Level Usage

### Tenant (`t...`)

- **Linux user:** `useradd t8k3pq7w2m`
- **Home directory:** `/home/t8k3pq7w2m/`
- **CephFS storage:** `/var/www/storage/t8k3pq7w2m/`
- **SSH authorized_keys:** `/home/t8k3pq7w2m/.ssh/authorized_keys`
- **S3 RGW user UID:** `t8k3pq7w2m`

### Webroot (`w...`)

- **Document root:** `/var/www/storage/t.../webroots/w4n8q1w5r2/`
- **Nginx config:** `/etc/nginx/sites-available/w4n8q1w5r2.conf`
- **PHP-FPM socket:** `/run/php/w4n8q1w5r2.sock`

### Database (`db...`)

- **MySQL database:** `CREATE DATABASE dbf4n8q1w5r`

### Valkey Instance (`kv...`)

- **systemd unit:** `valkey@kvj6t2y8e4h.service`
- **Config file:** `/etc/valkey/instances/kvj6t2y8e4h.conf`
- **Data directory:** `/var/lib/valkey/kvj6t2y8e4h/`

### S3 Bucket (`s3...`)

- **Internal RGW bucket:** `t8k3pq7w2m--s3b3g7l1v5d` (tenant ID + `--` + bucket ID)

### Cron Job (`cj...`)

- **systemd timer:** `cron-t8k3pq7w2m-cjk5n9r3w7a.timer`
- **systemd service:** `cron-t8k3pq7w2m-cjk5n9r3w7a.service`

### Daemon (`d...`)

- **supervisord config:** `/etc/supervisor/conf.d/daemon-t8k3pq7w2m-dh2q6v0z4b8.conf`
- **supervisord program:** `daemon-t8k3pq7w2m-dh2q6v0z4b8`
- **Working directory:** Inherits from parent webroot

## Database/Valkey User Naming

Usernames for database and valkey users must start with the parent resource's ID. This prevents collisions across resources on the same shard and makes ownership obvious.

**Database users** (parent: `dbf4n8q1w5r`):
- `dbf4n8q1w5r` — single-user shorthand
- `dbf4n8q1w5r_admin` — suffixed for multiple users
- `dbf4n8q1w5r_readonly` — read-only user

**Valkey users** (parent: `kvj6t2y8e4h`):
- `kvj6t2y8e4h` — single-user shorthand
- `kvj6t2y8e4h_reader` — restricted ACL user

The API validates this prefix constraint on user creation. Usernames that don't start with the parent resource ID are rejected.

## API Behavior

- **Creation requests** do not accept an `id` field for short-name resources — IDs are auto-generated server-side.
- **Responses** include `id` (the prefixed short name). There is no separate `name` field.
- **Search** matches against `id`.
- **Brand** is the exception: it has a user-supplied `name` (display name) and UUID `id`.
