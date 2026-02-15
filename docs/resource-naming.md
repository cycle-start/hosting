# Resource Naming

All resources use UUID primary keys for database identity and API references. Resources that need system-level identifiers (Linux usernames, MySQL database names, file paths, systemd units, RGW buckets) get auto-generated **prefixed short names** via `platform.NewName(prefix)`.

## Naming Table

| Resource | PK | Name Prefix | System Use |
|---|---|---|---|
| Tenant | UUID | `t_` | Linux username, CephFS paths, S3 RGW user UID |
| Webroot | UUID | `web_` | File paths, nginx configs |
| Database | UUID | `db_` | MySQL database name |
| Valkey Instance | UUID | `kv_` | systemd unit, config file, data dir |
| S3 Bucket | UUID | `s3_` | RGW bucket naming |
| Cron Job | UUID | `cron_` | systemd timer/service unit names |
| Brand | UUID | _(none)_ | User-supplied `name` for display |

## Name Format

Names are generated as `{prefix}{10-char-random}`, using the character set `abcdefghijklmnopqrstuvwxyz0123456789`. Examples:

- Tenant: `t_a7k3m9x2p1`
- Database: `db_f4n8q1w5r2`
- Valkey: `kv_j6t2y8e4h0`
- S3 Bucket: `s3_b3g7l1v5d9`
- Webroot: `web_c8m2p6s0x4`
- Cron Job: `cron_k5n9r3w7a1`

Names are globally unique per resource type (enforced by `UNIQUE(name)` constraint in the database).

## System-Level Usage

### Tenant (`t_xxx`)

- **Linux user:** `useradd t_a7k3m9x2p1`
- **Home directory:** `/home/t_a7k3m9x2p1/`
- **CephFS quota path:** `/mnt/ceph/tenants/t_a7k3m9x2p1/`
- **SSH authorized_keys:** `/home/t_a7k3m9x2p1/.ssh/authorized_keys`
- **S3 RGW user UID:** `t_a7k3m9x2p1`

### Webroot (`web_xxx`)

- **Document root:** `/home/t_xxx/webroots/web_c8m2p6s0x4/`
- **Nginx config:** `/etc/nginx/sites-available/web_c8m2p6s0x4.conf`
- **PHP-FPM socket:** `/run/php/web_c8m2p6s0x4.sock`

### Database (`db_xxx`)

- **MySQL database:** `CREATE DATABASE db_f4n8q1w5r2`

### Valkey Instance (`kv_xxx`)

- **systemd unit:** `valkey@kv_j6t2y8e4h0.service`
- **Config file:** `/etc/valkey/instances/kv_j6t2y8e4h0.conf`
- **Data directory:** `/var/lib/valkey/kv_j6t2y8e4h0/`

### S3 Bucket (`s3_xxx`)

- **Internal RGW bucket:** `t_a7k3m9x2p1--s3_b3g7l1v5d9` (tenant name + `--` + bucket name)

### Cron Job (`cron_xxx`)

- **systemd timer:** `cron-t_a7k3m9x2p1-cron_k5n9r3w7a1.timer`
- **systemd service:** `cron-t_a7k3m9x2p1-cron_k5n9r3w7a1.service`

## Database/Valkey User Naming

Usernames for database and valkey users must start with the parent resource's name. This prevents collisions across resources on the same shard and makes ownership obvious.

**Database users** (parent: `db_f4n8q1w5r2`):
- `db_f4n8q1w5r2` — single-user shorthand
- `db_f4n8q1w5r2_admin` — suffixed for multiple users
- `db_f4n8q1w5r2_readonly` — read-only user

**Valkey users** (parent: `kv_j6t2y8e4h0`):
- `kv_j6t2y8e4h0` — single-user shorthand
- `kv_j6t2y8e4h0_reader` — restricted ACL user

The API validates this prefix constraint on user creation. Usernames that don't start with the parent resource name are rejected.

## API Behavior

- **Creation requests** do not accept a `name` field — names are auto-generated server-side.
- **Responses** include both `id` (UUID) and `name` (prefixed short name).
- **Search** matches against both `id` and `name`.
- **Brand** is the exception: it has a user-supplied `name` (display name) and UUID `id`. Brand creation does not accept an `id` field.
