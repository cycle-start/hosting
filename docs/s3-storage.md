# S3 Object Storage

S3 object storage is backed by **Ceph RADOS Gateway (RGW)** running on storage shard nodes (`shard.role = "s3"`). Each S3 node runs a single-node Ceph cluster (mon + mgr + osd + rgw) with a dedicated raw disk. The platform manages buckets and access keys through the core API, with Temporal workflows executing provisioning via `radosgw-admin` CLI and the S3 API on node agents.

## Architecture

```
API request --> Core DB (desired state) --> Temporal workflow --> Node agent (radosgw-admin / S3 API)
```

- RGW is **cluster-wide**, so bucket operations only execute on the **first node** in the shard (unlike database/valkey which execute on every node).
- Each tenant gets an RGW user (auto-created on first bucket creation) with `uid = tenantID`.
- Bucket names are **tenant-scoped**: the internal RGW bucket name is `{tenantID}--{bucketName}`.
- RGW admin credentials are auto-generated during cloud-init and stored in `/etc/default/node-agent`.

## Data Model

### S3 Bucket (`model.S3Bucket`)

| Field            | Type      | JSON                | Description                          |
|------------------|-----------|---------------------|--------------------------------------|
| `ID`             | `string`  | `id`                | Platform-generated unique ID         |
| `TenantID`       | `*string` | `tenant_id`         | Owning tenant (required for create)  |
| `Name`           | `string`  | `name`              | User-facing bucket name (slug)       |
| `ShardID`        | `*string` | `shard_id`          | S3 shard assignment                  |
| `Public`         | `bool`    | `public`            | Public read access enabled           |
| `QuotaBytes`     | `int64`   | `quota_bytes`       | Size quota in bytes (0 = unlimited)  |
| `Status`         | `string`  | `status`            | Lifecycle status                     |
| `StatusMessage`  | `*string` | `status_message`    | Error details when `status=failed`   |
| `CreatedAt`      | `time`    | `created_at`        | Creation timestamp                   |
| `UpdatedAt`      | `time`    | `updated_at`        | Last update timestamp                |
| `ShardName`      | `*string` | `shard_name`        | Resolved shard name (read-only)      |

### S3 Access Key (`model.S3AccessKey`)

| Field              | Type      | JSON                  | Description                         |
|--------------------|-----------|-----------------------|-------------------------------------|
| `ID`               | `string`  | `id`                  | Platform-generated unique ID        |
| `S3BucketID`       | `string`  | `s3_bucket_id`        | Parent bucket                       |
| `AccessKeyID`      | `string`  | `access_key_id`       | S3 access key ID (20 chars)         |
| `SecretAccessKey`   | `string`  | `secret_access_key`   | S3 secret key (40 chars, shown once)|
| `Permissions`      | `string`  | `permissions`         | `read-only` or `read-write`         |
| `Status`           | `string`  | `status`              | Lifecycle status                    |
| `StatusMessage`    | `*string` | `status_message`      | Error details when `status=failed`  |
| `CreatedAt`        | `time`    | `created_at`          | Creation timestamp                  |
| `UpdatedAt`        | `time`    | `updated_at`          | Last update timestamp               |

**Secret key handling:** The `secret_access_key` is **only returned once** in the creation response (HTTP 201). It is redacted in all subsequent GET and LIST responses. There is no way to retrieve it again -- if lost, delete the key and create a new one.

### Bucket Naming

Buckets are unique per tenant, not globally. The internal RGW bucket name is constructed as:

```
{tenantID}--{bucketName}
```

For example, tenant `abc123` creating bucket `media` results in the RGW bucket `abc123--media`. This allows different tenants to use the same user-facing bucket name.

### Status Lifecycle

`pending` --> `provisioning` --> `active`
                             --> `failed` (retryable)
`active`  --> `deleting`     --> `deleted`

## API Endpoints

### S3 Buckets

| Method   | Path                                       | Status | Description                       |
|----------|--------------------------------------------|--------|-----------------------------------|
| `GET`    | `/tenants/{tenantID}/s3-buckets`          | 200    | List buckets for a tenant         |
| `POST`   | `/tenants/{tenantID}/s3-buckets`          | 202    | Create a bucket                   |
| `GET`    | `/s3-buckets/{id}`                        | 200    | Get a bucket                      |
| `PUT`    | `/s3-buckets/{id}`                        | 202    | Update public/quota settings      |
| `DELETE` | `/s3-buckets/{id}`                        | 202    | Delete a bucket and all objects   |
| `POST`   | `/s3-buckets/{id}/retry`                  | 202    | Retry a failed provisioning       |

### S3 Access Keys

| Method   | Path                                       | Status | Description                       |
|----------|--------------------------------------------|--------|-----------------------------------|
| `GET`    | `/s3-buckets/{bucketID}/access-keys`      | 200    | List access keys for a bucket     |
| `POST`   | `/s3-buckets/{bucketID}/access-keys`      | 201    | Create an access key              |
| `DELETE` | `/s3-access-keys/{id}`                    | 202    | Delete an access key              |
| `POST`   | `/s3-access-keys/{id}/retry`              | 202    | Retry a failed provisioning       |

Note: access key creation returns **201** (not 202) because the secret is included in the response body.

## Request Bodies

### Create S3 Bucket

```json
{
  "name": "media-uploads",
  "shard_id": "shard-id-here",
  "public": false,
  "quota_bytes": 5368709120
}
```

- `name` must be a valid slug.
- `public` defaults to `false`. When `true`, a public-read bucket policy is applied allowing anonymous `s3:GetObject`.
- `quota_bytes` is optional. Set to `0` or omit for unlimited. Value is in bytes (example above is 5 GB).

### Update S3 Bucket

```json
{
  "public": true,
  "quota_bytes": 10737418240
}
```

Both fields are optional. Only provided fields are applied.

### Create S3 Access Key

```json
{
  "permissions": "read-write"
}
```

- `permissions` defaults to `read-write`. Valid values: `read-only`, `read-write`.
- The `access_key_id` and `secret_access_key` are auto-generated and returned in the 201 response.

### Example Creation Response

```json
{
  "id": "abc123",
  "s3_bucket_id": "bucket456",
  "access_key_id": "AKIAIOSFODNN7EXAMPLE",
  "secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
  "permissions": "read-write",
  "status": "pending",
  "created_at": "2025-01-15T10:30:00Z",
  "updated_at": "2025-01-15T10:30:00Z"
}
```

Save the `secret_access_key` immediately -- it will not be shown again.

## Temporal Workflows

| Workflow                       | Trigger            | Steps                                                    |
|--------------------------------|--------------------|----------------------------------------------------------|
| `CreateS3BucketWorkflow`       | POST create bucket | Set provisioning -> ensure RGW user -> create bucket via S3 API -> set tenant bucket policy -> set quota -> set active |
| `UpdateS3BucketWorkflow`       | PUT update bucket  | Lookup bucket -> update bucket policy (public/private)   |
| `DeleteS3BucketWorkflow`       | DELETE bucket      | Set deleting -> delete all objects (paginated) -> delete bucket -> set deleted |
| `CreateS3AccessKeyWorkflow`    | POST create key    | Set provisioning -> lookup context -> `radosgw-admin key create` -> set active |
| `DeleteS3AccessKeyWorkflow`    | DELETE key         | Set deleting -> lookup context -> `radosgw-admin key rm` -> set deleted |

Bucket create/delete workflows have a 2-minute timeout (deleting all objects can take time). Access key workflows use the standard 30-second timeout. All retry up to 3 times.

## Node Agent Operations

The `S3Manager` on each node agent uses two tools:

### radosgw-admin CLI

- **Ensure RGW user**: `radosgw-admin user info --uid={tenantID}` (check existence) -> `radosgw-admin user create --uid={tenantID} --display-name={tenantID}` (create if missing).
- **Set bucket quota**: `radosgw-admin quota set --uid={tenantID} --bucket={internalName} --max-size={bytes}` -> `radosgw-admin quota enable --uid={tenantID} --bucket={internalName} --quota-scope=bucket`.
- **Create access key**: `radosgw-admin key create --uid={tenantID} --access-key={key} --secret-key={secret} --key-type=s3`. Idempotent: treats `KeyExists` as success on retry.
- **Delete access key**: `radosgw-admin key rm --uid={tenantID} --access-key={key}`. Idempotent: treats `NoSuchKey` / `NoSuchUser` as success.

### S3 API (AWS SDK)

- **Create bucket**: `s3.CreateBucket` (idempotent: `BucketAlreadyExists` is OK).
- **Set bucket policy**: `s3.PutBucketPolicy` with a JSON policy document.
- **Delete all objects**: `s3.ListObjectsV2` (paginated) -> `s3.DeleteObjects` (batch).
- **Delete bucket**: `s3.DeleteBucket` (idempotent: `NoSuchBucket` is OK).

### Bucket Policies

Every bucket gets a **tenant access policy** granting full S3 access to the tenant's RGW user:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "TenantFullAccess",
      "Effect": "Allow",
      "Principal": {"AWS": ["arn:aws:iam:::user/{tenantID}"]},
      "Action": "s3:*",
      "Resource": [
        "arn:aws:s3:::{internalName}",
        "arn:aws:s3:::{internalName}/*"
      ]
    }
  ]
}
```

When `public = true`, an additional statement is added:

```json
{
  "Sid": "PublicRead",
  "Effect": "Allow",
  "Principal": "*",
  "Action": "s3:GetObject",
  "Resource": "arn:aws:s3:::{internalName}/*"
}
```

Setting `public = false` removes the public read statement while preserving tenant access.
