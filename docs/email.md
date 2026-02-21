# Email (Stalwart)

Email hosting is powered by [Stalwart](https://stalwart.org/), an all-in-one mail server providing SMTP, IMAP, and JMAP. Each cluster has a Stalwart instance managed via its REST API and JMAP protocol. The platform manages accounts, aliases, forwards, and auto-replies through the core API, with Temporal workflows handling asynchronous provisioning.

## Architecture

Email accounts are scoped to FQDNs (domains). The resolution chain for any email operation is:

```
FQDN -> Tenant -> Cluster -> Stalwart URL + Token
```

Since FQDNs are now tenant-scoped (not webroot-scoped), the resolution chain goes directly from FQDN to tenant. This is encapsulated in `StalwartContext`, resolved once per workflow via the `GetStalwartContext` activity. The context provides the Stalwart base URL, admin token, mail hostname, and FQDN details.

Two clients are used to talk to Stalwart:
- **REST client** (`stalwart.Client`) -- account/domain CRUD, alias management via principal patch operations
- **JMAP client** (`stalwart.JMAPClient`) -- Sieve script deployment (forwards) and vacation responses (auto-reply)

## Email Accounts

An email account is a mailbox on Stalwart. Each account belongs to an FQDN and has an address, display name, and quota.

**Model fields:** `id`, `fqdn_id`, `subscription_id`, `address`, `display_name`, `quota_bytes`, `status`

The `subscription_id` is required when creating an email account, linking it to a subscription for billing and lifecycle management.

### Create workflow (`CreateEmailAccountWorkflow`)

1. Set status to `provisioning`
2. Look up the email account
3. Resolve `StalwartContext` (FQDN -> cluster)
4. Create domain in Stalwart (idempotent -- safe to call if domain already exists)
5. Create the account principal in Stalwart
6. Auto-create MX and SPF DNS records (if a matching zone exists)
7. Set status to `active`

### Delete workflow (`DeleteEmailAccountWorkflow`)

1. Set status to `deleting`
2. Delete the account from Stalwart
3. Set status to `deleted`
4. Count remaining active accounts for the FQDN
5. If zero remain: delete the Stalwart domain and remove email DNS records

### Nested creation

The account create endpoint supports creating aliases, forwards, and an auto-reply in a single request. Each nested resource is persisted independently and triggers its own workflow.

### API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/fqdns/{fqdnID}/email-accounts` | List accounts for an FQDN |
| POST | `/fqdns/{fqdnID}/email-accounts` | Create account (202 Accepted) |
| GET | `/email-accounts/{id}` | Get account |
| DELETE | `/email-accounts/{id}` | Delete account (202 Accepted) |
| POST | `/email-accounts/{id}/retry` | Retry failed provisioning |

## Email Aliases

An alias is an additional email address that delivers to an existing account. Aliases are implemented by adding/removing addresses from the Stalwart principal's `emails` array via patch operations.

**Model fields:** `id`, `email_account_id`, `address`, `status`

### Create workflow (`CreateEmailAliasWorkflow`)

1. Set status to `provisioning`
2. Look up alias and parent account
3. Resolve Stalwart credentials
4. Add alias address to principal via `addItem` patch on the `emails` field
5. Set status to `active`

### Delete workflow (`DeleteEmailAliasWorkflow`)

1. Set status to `deleting`
2. Remove alias address via `removeItem` patch on the `emails` field
3. Set status to `deleted`

### API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/email-accounts/{id}/aliases` | List aliases |
| POST | `/email-accounts/{id}/aliases` | Create alias (202) |
| GET | `/email-aliases/{aliasID}` | Get alias |
| DELETE | `/email-aliases/{aliasID}` | Delete alias (202) |
| POST | `/email-aliases/{aliasID}/retry` | Retry |

## Email Forwards

Forwards send copies of incoming mail to external destinations. They are implemented via **Sieve scripts** deployed through the JMAP protocol.

**Model fields:** `id`, `email_account_id`, `destination`, `keep_copy`, `status`

The `keep_copy` flag (default `true`) controls whether the original message is retained in the mailbox:
- `keep_copy: true` -- generates `redirect :copy "dest@example.com";` (deliver locally AND forward)
- `keep_copy: false` -- generates `redirect "dest@example.com";` (forward only, no local copy)

### Sieve script generation

The `GenerateForwardScript` function builds a complete Sieve script from all active forwards for an account. If any forward uses `:copy`, the `require ["copy"];` header is included. Example output:

```sieve
require ["copy"];
redirect :copy "alice@external.com";
redirect "bob@other.com";
```

The script is deployed as `hosting-forwards` via JMAP's `SieveScript/set` method. When all forwards are deleted, the script is removed entirely.

### Sync model

Forward create and delete workflows both call `StalwartSyncForwardScript`, which re-reads all active forwards from the database and regenerates the full script. This ensures consistency -- the script always reflects the current database state. On delete, the forward is marked `deleted` before sync so it is excluded.

### API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/email-accounts/{id}/forwards` | List forwards |
| POST | `/email-accounts/{id}/forwards` | Create forward (202) |
| GET | `/email-forwards/{forwardID}` | Get forward |
| DELETE | `/email-forwards/{forwardID}` | Delete forward (202) |
| POST | `/email-forwards/{forwardID}/retry` | Retry |

## Auto-Reply (Vacation)

Auto-reply provides out-of-office / vacation responses. It is implemented via JMAP's `VacationResponse/set` method.

**Model fields:** `id`, `email_account_id`, `subject`, `body`, `start_date`, `end_date`, `enabled`, `status`

- `start_date` / `end_date` are optional (RFC3339). When set, the auto-reply is only active within that window.
- `enabled` toggles the response on or off without deleting the configuration.
- The endpoint is an upsert (PUT) -- creating or replacing the auto-reply in one call.

### Update workflow (`UpdateEmailAutoReplyWorkflow`)

1. Resolve Stalwart credentials
2. Build `VacationParams` with subject, body, date range, and enabled flag
3. Deploy via `StalwartSetVacation` (JMAP `VacationResponse/set`)
4. Set status to `active`

### Delete workflow (`DeleteEmailAutoReplyWorkflow`)

1. Clear the vacation response by calling `StalwartSetVacation` with `nil` params
2. Set status to `deleted`

### API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/email-accounts/{id}/autoreply` | Get auto-reply config |
| PUT | `/email-accounts/{id}/autoreply` | Create/replace auto-reply (202) |
| DELETE | `/email-accounts/{id}/autoreply` | Delete auto-reply (202) |
| POST | `/email-autoreplies/{id}/retry` | Retry |

## Automatic DNS Records

When the first email account is created on an FQDN, the platform automatically creates DNS records in the matching zone (if one exists):

- **MX record** -- `{fqdn} MX 10 {mail_hostname}` (TTL 300). The mail hostname defaults to `mail.{fqdn}` if not set in the cluster config.
- **SPF record** -- `{fqdn} TXT "v=spf1 mx ~all"` (TTL 300)

These records are written to both the PowerDNS database (for live DNS) and the core database (tracked as `managed_by: platform` with `source_fqdn_id`).

When the last email account on an FQDN is deleted, both records are cleaned up from PowerDNS and the core database.

## Status Lifecycle

All email resources follow the standard status lifecycle:

```
pending -> provisioning -> active
                        -> failed -> (retry) -> provisioning
           deleting -> deleted
```

Failed resources can be retried via the `/retry` endpoint, which re-triggers the provisioning workflow.
