# Extended E2E Test Plan

## Current State

Existing E2E tests live in `tests/e2e/` and cover:

| File | What it tests |
|------|--------------|
| `platform_test.go` | Health check, cluster bootstrap, seed tenants, shared CephFS storage, web traffic through HAProxy |
| `tenant_lifecycle_test.go` | Create/update/suspend/unsuspend/delete tenant, list pagination, validation, 404 |
| `webroot_test.go` | Webroot CRUD, FQDN binding/unbinding, multiple runtimes, validation |
| `dns_test.go` | Zone CRUD, zone record CRUD, auto DNS records on FQDN bind, multiple record types |
| `database_test.go` | Database CRUD, database user CRUD, validation |
| `s3_test.go` | S3 bucket CRUD, access key lifecycle, S3 object operations via AWS SDK, public/private toggle, quota, validation |
| `backup_test.go` | Web backup create/restore via SSH, backup list |

### What is NOT covered

| Resource / Scenario | Routes exist | E2E coverage |
|---------------------|-------------|-------------|
| Valkey instances | Yes | None |
| Valkey users | Yes | None |
| Email accounts | Yes | None |
| Email aliases | Yes | None |
| Email forwards | Yes | None |
| Email auto-replies | Yes | None |
| SSH keys | Yes | None |
| Certificates | Yes | None |
| Brand CRUD | Yes | Only `findOrCreateBrand` helper |
| Brand cluster assignment | Yes | None |
| Brand isolation (multi-brand) | Yes | None |
| API key management | Yes | None |
| Tenant migration | Yes | None |
| Database migration | Yes | None |
| Valkey migration | Yes | None |
| Tenant retry / retry-failed | Yes | None |
| FQDN full-stack verification | Partial | FQDN binding tested, but no DNS propagation + LB map + nginx config verification |
| Backup restore verification | Partial | Web backup only; no database backup/restore |
| Database backup/restore | Yes | None |
| Tenant resource summary | Yes | None |
| Search | Yes | None |
| Dashboard stats | Yes | None |
| Audit logs | Yes | None |

---

## Test Infrastructure Patterns

### Helper Functions (existing, in `helpers_test.go`)

```go
// HTTP verbs
httpGet(t, url) (*http.Response, string)
httpPost(t, url, body) (*http.Response, string)
httpPut(t, url, body) (*http.Response, string)
httpDelete(t, url) (*http.Response, string)
httpGetWithHost(url, host) (*http.Response, string, error)

// Parsing
parseJSON(t, body) map[string]interface{}
parseJSONArray(t, body) []map[string]interface{}
parsePaginatedItems(t, body) []map[string]interface{}

// Async polling
waitForStatus(t, url, wantStatus, timeout) map[string]interface{}
waitForHTTP(t, url, host, timeout) (*http.Response, string)

// Infrastructure discovery
findFirstRegionID(t) string
findFirstCluster(t, regionID) map[string]interface{}
findShardByRole(t, clusterID, role) map[string]interface{}
findNodeIPsByRole(t, clusterID, role) []string
findStorageShard(t, clusterID) string

// Resource setup
findOrCreateBrand(t) string
createTestTenant(t, name) (tenantID, regionID, clusterID, webShardID, dbShardID)

// Node access
sshExec(t, ip, cmd) string
```

### New Helpers Needed

The following helpers should be added to `helpers_test.go` to support the extended tests.

```go
// findShardIDByRole returns the shard ID for a given role, or empty string if not found.
// Unlike findShardByRole, this does not fatal on missing shard (for skip logic).
func findShardIDByRole(t *testing.T, clusterID, role string) string

// findValkeyShardID returns the shard ID for the valkey role in the cluster.
func findValkeyShardID(t *testing.T, clusterID string) string

// findEmailShardID returns the shard ID for the email role in the cluster.
func findEmailShardID(t *testing.T, clusterID string) string

// findDNSShardID returns the shard ID for the dns role in the cluster.
func findDNSShardID(t *testing.T, clusterID string) string

// createTestWebroot creates a webroot on a tenant and waits for it to become active.
// Returns the webroot ID. Registers cleanup.
func createTestWebroot(t *testing.T, tenantID, name, runtime, version string) string

// createTestFQDN creates an FQDN on a webroot and waits for it to become active.
// Returns the FQDN ID. Registers cleanup.
func createTestFQDN(t *testing.T, webrootID, fqdn string) string

// createTestZone creates a zone and waits for it to become active.
// Returns the zone ID. Registers cleanup.
func createTestZone(t *testing.T, tenantID, regionID, name string) string

// createTestDatabase creates a database and waits for it to become active.
// Returns the database ID. Registers cleanup.
func createTestDatabase(t *testing.T, tenantID, shardID, name string) string

// createTestValkeyInstance creates a Valkey instance and waits for it to become active.
// Returns the instance ID. Registers cleanup.
func createTestValkeyInstance(t *testing.T, tenantID, shardID, name string) string

// createAPIKeyWithBrands creates an API key scoped to specific brands and scopes.
// Returns the raw key string and the key ID. Registers cleanup.
func createAPIKeyWithBrands(t *testing.T, name string, scopes, brands []string) (rawKey, keyID string)

// httpGetWithKey performs an HTTP GET using a specific API key instead of the default.
func httpGetWithKey(t *testing.T, url, apiKey string) (*http.Response, string)

// httpPostWithKey performs an HTTP POST using a specific API key.
func httpPostWithKey(t *testing.T, url string, body interface{}, apiKey string) (*http.Response, string)

// httpDeleteWithKey performs an HTTP DELETE using a specific API key.
func httpDeleteWithKey(t *testing.T, url, apiKey string) (*http.Response, string)

// generateSSHKeyPair generates an RSA SSH key pair for testing.
// Returns the public key in authorized_keys format.
func generateSSHKeyPair(t *testing.T) string

// generateSelfSignedCert generates a self-signed TLS certificate for testing.
// Returns certPEM, keyPEM.
func generateSelfSignedCert(t *testing.T, cn string) (certPEM, keyPEM string)

// digQuery performs a DNS query against a specific nameserver IP and returns the answer.
func digQuery(t *testing.T, nameserverIP, recordType, name string) string
```

### Test Naming Convention

All test functions follow the pattern `Test<Resource><Scenario>`:

```
TestValkeyInstanceCRUD
TestValkeyUserLifecycle
TestEmailAccountCRUD
TestSSHKeyCRUD
TestBrandIsolation
TestTenantMigration
...
```

### Constants

```go
const (
    provisionTimeout = 5 * time.Minute   // existing
    seedTimeout      = 5 * time.Minute   // existing
    httpTimeout      = 60 * time.Second   // existing
    clusterName      = "vm-cluster-1"     // existing
    migrationTimeout = 10 * time.Minute   // new: migrations take longer
)
```

---

## 1. Valkey E2E Tests

**File:** `tests/e2e/valkey_test.go`

### TestValkeyInstanceCRUD

Full Valkey instance lifecycle:
create tenant -> create Valkey instance -> wait active -> list by tenant ->
create Valkey user -> wait active -> update user -> list users -> delete user ->
delete instance -> verify deleted.

```go
func TestValkeyInstanceCRUD(t *testing.T) {
    tenantID, _, clusterID, _, _ := createTestTenant(t, "e2e-valkey-crud")
    valkeyShardID := findValkeyShardID(t, clusterID)
    if valkeyShardID == "" {
        t.Skip("no valkey shard found in cluster; skipping valkey tests")
    }

    // Step 1: Create a Valkey instance.
    //   POST /tenants/{tenantID}/valkey-instances
    //   Body: { "name": "e2e-cache", "shard_id": valkeyShardID, "max_memory_mb": 64 }
    //   Expect: 202, status=pending, password redacted in response

    // Step 2: Wait for instance to become active.
    //   Poll GET /valkey-instances/{id} until status=active

    // Step 3: Verify instance in tenant list.
    //   GET /tenants/{tenantID}/valkey-instances
    //   Expect: instance present, password redacted

    // Step 4: Verify port was assigned (non-zero).

    // Step 5: Create a Valkey user.
    //   POST /valkey-instances/{id}/users
    //   Body: { "username": "e2e-app", "password": "TestP@ssw0rd!", "privileges": ["+@all"], "key_pattern": "~*" }
    //   Expect: 202, password redacted in response

    // Step 6: Wait for user to become active.

    // Step 7: Update the user (change password).
    //   PUT /valkey-users/{userID}
    //   Body: { "password": "NewP@ssw0rd123" }
    //   Expect: 202

    // Step 8: List users for the instance.
    //   GET /valkey-instances/{id}/users
    //   Expect: user present, password redacted

    // Step 9: Delete the user.
    //   DELETE /valkey-users/{userID}
    //   Expect: 202

    // Step 10: Delete the instance.
    //   DELETE /valkey-instances/{id}
    //   Expect: 202, wait for deleted/404
}
```

### TestValkeyInstanceWithNestedUsers

Tests creating a Valkey instance with users in a single request.

```go
func TestValkeyInstanceWithNestedUsers(t *testing.T) {
    // Create tenant, find valkey shard.
    // POST /tenants/{tenantID}/valkey-instances with nested users:
    //   { "name": "e2e-nested", "shard_id": "...", "max_memory_mb": 128,
    //     "users": [
    //       { "username": "app1", "password": "...", "privileges": ["+@all"], "key_pattern": "~app1:*" },
    //       { "username": "app2", "password": "...", "privileges": ["+@read"], "key_pattern": "~app2:*" }
    //     ] }
    // Wait for instance active.
    // List users and verify both exist.
}
```

### TestValkeyInstanceDataPersistence

Verifies that data written to a Valkey instance persists through user connections.
This requires SSH access to a valkey node to run `valkey-cli`.

```go
func TestValkeyInstanceDataPersistence(t *testing.T) {
    // Create tenant, instance, user. Wait all active.
    // SSH to valkey node:
    //   valkey-cli -p {port} -a {password} SET e2e-key "e2e-value"
    //   valkey-cli -p {port} -a {password} GET e2e-key
    // Verify returned value matches.
    // Test key expiry: SET with EX, wait, verify expired.
}
```

### TestValkeyInstanceGetNotFound

```go
func TestValkeyInstanceGetNotFound(t *testing.T) {
    // GET /valkey-instances/00000000-0000-0000-0000-000000000000
    // Expect: 404
}
```

### TestValkeyUserCreateValidation

```go
func TestValkeyUserCreateValidation(t *testing.T) {
    // Create instance, try creating user with short password.
    // Expect: 400
}
```

---

## 2. Email E2E Tests

**File:** `tests/e2e/email_test.go`

**Prerequisite:** These tests require a Stalwart mail server in the test environment.
Tests should use `t.Skip()` if the email FQDN binding fails or if the email
endpoints return 404/500 indicating Stalwart is not available.

### TestEmailAccountCRUD

Full email account lifecycle:
create tenant -> create webroot -> create FQDN -> create email account ->
wait active -> list accounts -> create alias -> create forward ->
set auto-reply -> list aliases -> list forwards -> get auto-reply ->
delete alias -> delete forward -> delete auto-reply -> delete account.

```go
func TestEmailAccountCRUD(t *testing.T) {
    tenantID, regionID, _, _, _ := createTestTenant(t, "e2e-email-crud")

    // Create webroot + FQDN (email accounts are scoped to FQDNs).
    webrootID := createTestWebroot(t, tenantID, "email-site", "static", "1")
    fqdnID := createTestFQDN(t, webrootID, "mail.e2e-email.example.com.")

    // Step 1: Create email account.
    //   POST /fqdns/{fqdnID}/email-accounts
    //   Body: { "address": "test@mail.e2e-email.example.com", "display_name": "Test User", "quota_bytes": 1073741824 }
    //   Expect: 202

    // Step 2: Wait for account to become active (or skip if Stalwart unavailable).

    // Step 3: List email accounts for the FQDN.
    //   GET /fqdns/{fqdnID}/email-accounts
    //   Expect: account present

    // Step 4: Get account by ID.

    // Step 5: Create alias.
    //   POST /email-accounts/{id}/aliases
    //   Body: { "address": "contact@mail.e2e-email.example.com" }
    //   Expect: 202

    // Step 6: Create forward.
    //   POST /email-accounts/{id}/forwards
    //   Body: { "destination": "external@example.com", "keep_copy": true }
    //   Expect: 202

    // Step 7: Set auto-reply.
    //   PUT /email-accounts/{id}/autoreply
    //   Body: { "subject": "Out of office", "body": "I am away.", "enabled": true }
    //   Expect: 202 or 200

    // Step 8: List aliases.
    //   GET /email-accounts/{id}/aliases

    // Step 9: List forwards.
    //   GET /email-accounts/{id}/forwards

    // Step 10: Get auto-reply.
    //   GET /email-accounts/{id}/autoreply

    // Step 11: Delete alias.
    //   DELETE /email-aliases/{aliasID}

    // Step 12: Delete forward.
    //   DELETE /email-forwards/{forwardID}

    // Step 13: Delete auto-reply.
    //   DELETE /email-accounts/{id}/autoreply

    // Step 14: Delete account.
    //   DELETE /email-accounts/{id}
    //   Expect: 202
}
```

### TestEmailAccountNestedCreation

Tests creating an email account with aliases, forwards, and auto-reply in a single request.

```go
func TestEmailAccountNestedCreation(t *testing.T) {
    // POST /fqdns/{fqdnID}/email-accounts with nested resources:
    //   { "address": "...", "display_name": "...",
    //     "aliases": [{"address": "..."}],
    //     "forwards": [{"destination": "...", "keep_copy": true}],
    //     "autoreply": {"subject": "...", "body": "...", "enabled": true} }
    // Verify all sub-resources created.
}
```

### TestEmailAccountValidation

```go
func TestEmailAccountValidation(t *testing.T) {
    // Missing address -> 400
    // Invalid email format -> 400
}
```

---

## 3. SSH Key E2E Tests

**File:** `tests/e2e/ssh_key_test.go`

### TestSSHKeyCRUD

Full SSH key lifecycle:
create tenant -> create SSH key -> wait active -> list keys -> get key ->
verify fingerprint computed -> delete key -> verify deleted.

```go
func TestSSHKeyCRUD(t *testing.T) {
    tenantID, _, _, _, _ := createTestTenant(t, "e2e-ssh-crud")

    // Generate a test SSH key pair.
    pubKey := generateSSHKeyPair(t)

    // Step 1: Create SSH key.
    //   POST /tenants/{tenantID}/ssh-keys
    //   Body: { "name": "e2e-deploy-key", "public_key": pubKey }
    //   Expect: 202, fingerprint computed

    // Step 2: Wait for key to become active.

    // Step 3: Verify fingerprint is non-empty and has expected format (SHA256:...).

    // Step 4: List SSH keys for tenant.
    //   GET /tenants/{tenantID}/ssh-keys
    //   Expect: key present

    // Step 5: Get SSH key by ID.
    //   GET /ssh-keys/{id}
    //   Verify public_key matches, fingerprint matches

    // Step 6: Delete SSH key.
    //   DELETE /ssh-keys/{id}
    //   Expect: 202
}
```

### TestSSHKeyNodeSync

Verifies that an SSH key deployed via the API actually appears in the
authorized_keys file on the web nodes.

```go
func TestSSHKeyNodeSync(t *testing.T) {
    tenantID, _, _, _, _ := createTestTenant(t, "e2e-ssh-sync")

    pubKey := generateSSHKeyPair(t)

    // Create SSH key and wait for active.
    // SSH to web node:
    //   cat /var/www/storage/{tenantID}/.ssh/authorized_keys
    // Verify the public key appears in the file.

    // Delete SSH key. Wait for deleted.
    // Verify the key is removed from the file.
}
```

### TestSSHKeyInvalidKey

```go
func TestSSHKeyInvalidKey(t *testing.T) {
    // POST with invalid public key content.
    //   Body: { "name": "bad-key", "public_key": "not-a-valid-ssh-key" }
    //   Expect: 400 "invalid SSH public key"
}
```

### TestSSHKeyMultipleKeys

```go
func TestSSHKeyMultipleKeys(t *testing.T) {
    // Create 3 SSH keys for same tenant.
    // List and verify all 3 present.
    // Delete one, verify only 2 remain.
}
```

---

## 4. Certificate E2E Tests

**File:** `tests/e2e/certificate_test.go`

### TestCertificateUpload

Upload a custom TLS certificate for an FQDN.

```go
func TestCertificateUpload(t *testing.T) {
    tenantID, _, _, _, _ := createTestTenant(t, "e2e-cert")
    webrootID := createTestWebroot(t, tenantID, "cert-site", "static", "1")
    fqdnID := createTestFQDN(t, webrootID, "secure.e2e-cert.example.com.")

    // Generate self-signed certificate.
    certPEM, keyPEM := generateSelfSignedCert(t, "secure.e2e-cert.example.com")

    // Step 1: Upload certificate.
    //   POST /fqdns/{fqdnID}/certificates
    //   Body: { "cert_pem": certPEM, "key_pem": keyPEM }
    //   Expect: 202, key_pem redacted in response

    // Step 2: Wait for certificate to become active.

    // Step 3: List certificates for the FQDN.
    //   GET /fqdns/{fqdnID}/certificates
    //   Expect: certificate present, key_pem redacted
}
```

### TestCertificateInvalidPEM

```go
func TestCertificateInvalidPEM(t *testing.T) {
    // Upload with invalid PEM data.
    // Expect: 400
}
```

---

## 5. Brand CRUD and Isolation E2E Tests

**File:** `tests/e2e/brand_test.go`

### TestBrandCRUD

Full brand lifecycle: create -> update -> list clusters -> set clusters -> delete.

```go
func TestBrandCRUD(t *testing.T) {
    brandID := "e2e-brand-crud"

    // Step 1: Create brand.
    //   POST /brands
    //   Body: { "id": brandID, "name": "E2E CRUD Brand",
    //           "base_hostname": "crud.mhst.io",
    //           "primary_ns": "ns1.crud.mhst.io",
    //           "secondary_ns": "ns2.crud.mhst.io",
    //           "hostmaster_email": "hostmaster@crud.mhst.io" }
    //   Expect: 201

    // Step 2: Get brand.
    //   GET /brands/{id}
    //   Verify all fields match

    // Step 3: Update brand.
    //   PUT /brands/{id}
    //   Body: { "name": "E2E CRUD Brand Updated" }
    //   Expect: 200, name updated

    // Step 4: List brands.
    //   GET /brands
    //   Verify brand present in paginated list

    // Step 5: Set allowed clusters.
    //   PUT /brands/{id}/clusters
    //   Body: { "cluster_ids": ["vm-cluster-1"] }
    //   Expect: 200

    // Step 6: List allowed clusters.
    //   GET /brands/{id}/clusters
    //   Expect: ["vm-cluster-1"]

    // Step 7: Clear cluster restrictions.
    //   PUT /brands/{id}/clusters
    //   Body: { "cluster_ids": [] }
    //   Expect: 200

    // Step 8: Delete brand (no tenants -> should succeed).
    //   DELETE /brands/{id}
    //   Expect: 200
}
```

### TestBrandDeleteWithTenants

Verify that deleting a brand with active tenants fails.

```go
func TestBrandDeleteWithTenants(t *testing.T) {
    // Create brand, create tenant under it.
    // DELETE /brands/{id}
    // Expect: 400 or 500 (brand has tenants)
    // Clean up: delete tenant first, then brand.
}
```

### TestBrandIsolation

The core brand isolation test: two brands should not see each other's resources.
This test creates two API keys, each scoped to a different brand, and verifies
that operations are properly isolated.

```go
func TestBrandIsolation(t *testing.T) {
    // --- Setup: Create two brands ---
    brandA := "e2e-brand-a"
    brandB := "e2e-brand-b"
    // POST /brands for each

    // --- Setup: Create two API keys ---
    // POST /api-keys with brands: ["e2e-brand-a"], scopes: ["*:*"]
    //   -> keyA (raw key string)
    // POST /api-keys with brands: ["e2e-brand-b"], scopes: ["*:*"]
    //   -> keyB (raw key string)

    // --- Setup: Set cluster access for both brands ---
    // PUT /brands/e2e-brand-a/clusters with cluster_ids
    // PUT /brands/e2e-brand-b/clusters with cluster_ids

    // --- Setup: Create a tenant under each brand ---
    // POST /tenants (using keyA) with brand_id: brandA -> tenantA
    // POST /tenants (using keyB) with brand_id: brandB -> tenantB

    // --- Test 1: Key A cannot see Brand B's tenant ---
    // GET /tenants/{tenantB} using keyA -> expect 403 or 404

    // --- Test 2: Key A listing tenants only sees Brand A tenants ---
    // GET /tenants using keyA -> verify only tenantA in items

    // --- Test 3: Key B cannot see Brand A's tenant ---
    // GET /tenants/{tenantA} using keyB -> expect 403 or 404

    // --- Test 4: Key A cannot create resources under Brand B tenant ---
    // POST /tenants/{tenantB}/webroots using keyA -> expect 403

    // --- Test 5: Zones are brand-scoped ---
    // POST /zones using keyA with tenant_id from brandA -> 202 (allowed)
    // GET /zones using keyB -> zone not visible

    // --- Test 6: Platform admin key can see everything ---
    // GET /tenants using platform admin key -> both tenantA and tenantB visible

    // --- Cleanup ---
}
```

### TestBrandClusterRestriction

Verify that tenant creation on a disallowed cluster is rejected.

```go
func TestBrandClusterRestriction(t *testing.T) {
    // Create brand, set allowed clusters to ["nonexistent-cluster"].
    // Try to create tenant on vm-cluster-1 -> expect 400/403.
    // Update allowed clusters to include vm-cluster-1 -> tenant creation succeeds.
}
```

---

## 6. API Key E2E Tests

**File:** `tests/e2e/api_key_test.go`

### TestAPIKeyCRUD

```go
func TestAPIKeyCRUD(t *testing.T) {
    // Step 1: Create API key.
    //   POST /api-keys
    //   Body: { "name": "e2e-key", "scopes": ["tenants:read", "tenants:write"], "brands": ["e2e-brand"] }
    //   Expect: 201, raw_key returned (only on creation)

    // Step 2: List API keys.
    //   GET /api-keys
    //   Expect: key present, raw_key NOT returned

    // Step 3: Get API key.
    //   GET /api-keys/{id}

    // Step 4: Update API key.
    //   PUT /api-keys/{id}
    //   Body: { "name": "e2e-key-updated", "scopes": ["*:*"], "brands": ["e2e-brand"] }
    //   Expect: 200

    // Step 5: Revoke API key.
    //   DELETE /api-keys/{id}
    //   Expect: 200

    // Step 6: Verify revoked key cannot authenticate.
    //   GET /tenants using revoked key -> 401
}
```

### TestAPIKeyScopeEnforcement

Verify that an API key with limited scopes cannot perform unauthorized operations.

```go
func TestAPIKeyScopeEnforcement(t *testing.T) {
    // Create key with scopes: ["tenants:read"]
    // GET /tenants -> 200 (allowed)
    // POST /tenants -> 403 (insufficient scope: tenants:write)
    // GET /databases/... -> 403 (insufficient scope: databases:read)
}
```

---

## 7. Cross-Shard Migration E2E Tests

**File:** `tests/e2e/migration_test.go`

### TestTenantMigration

Migrate a tenant (web shard) from one shard to another, verify files move.

```go
func TestTenantMigration(t *testing.T) {
    tenantID, _, clusterID, webShardID, _ := createTestTenant(t, "e2e-migrate-tenant")

    // Find a second web shard (if only one, skip).
    // Alternatively, migrate to the same shard and verify the operation completes
    // (useful for testing the workflow machinery even without a second shard).

    // Step 1: Write a test file via SSH to the tenant's webroot directory.
    //   sshExec: echo "migration-test" > /var/www/storage/{tenantID}/testfile.txt

    // Step 2: Trigger migration.
    //   POST /tenants/{tenantID}/migrate
    //   Body: { "target_shard_id": targetShardID }
    //   Expect: 202

    // Step 3: Wait for tenant to return to active.
    //   Poll GET /tenants/{tenantID} until status=active (timeout: migrationTimeout)

    // Step 4: Verify the tenant's shard_id changed.

    // Step 5: Verify the test file exists on the target shard's nodes.
}
```

### TestDatabaseMigration

Migrate a database from one database shard to another.

```go
func TestDatabaseMigration(t *testing.T) {
    tenantID, _, clusterID, _, dbShardID := createTestTenant(t, "e2e-migrate-db")
    if dbShardID == "" {
        t.Skip("no database shard found")
    }

    dbID := createTestDatabase(t, tenantID, dbShardID, "e2e_migratedb")

    // Find a second database shard. If none, skip.

    // Step 1: Write test data to the database via SSH (mysql CLI on db node).

    // Step 2: Trigger migration.
    //   POST /databases/{dbID}/migrate
    //   Body: { "target_shard_id": targetShardID }
    //   Expect: 202

    // Step 3: Wait for database to return to active.

    // Step 4: Verify the data is intact on the new shard.
}
```

### TestValkeyMigration

Migrate a Valkey instance from one shard to another.

```go
func TestValkeyMigration(t *testing.T) {
    tenantID, _, clusterID, _, _ := createTestTenant(t, "e2e-migrate-valkey")
    valkeyShardID := findValkeyShardID(t, clusterID)
    if valkeyShardID == "" {
        t.Skip("no valkey shard found")
    }

    instanceID := createTestValkeyInstance(t, tenantID, valkeyShardID, "e2e-migrate-cache")

    // Find second valkey shard. If none, skip.

    // Step 1: Write test data via valkey-cli.
    // Step 2: Trigger migration.
    //   POST /valkey-instances/{id}/migrate
    //   Body: { "target_shard_id": targetShardID }
    // Step 3: Wait for active.
    // Step 4: Verify data migrated.
}
```

---

## 8. DNS Record Propagation E2E Tests

**File:** `tests/e2e/dns_propagation_test.go`

These tests verify that records created via the API actually appear in PowerDNS
by querying the DNS nodes directly.

### TestDNSRecordPropagation

```go
func TestDNSRecordPropagation(t *testing.T) {
    tenantID, regionID, clusterID, _, _ := createTestTenant(t, "e2e-dns-prop")

    // Find DNS node IPs.
    dnsNodeIPs := findNodeIPsByRole(t, clusterID, "dns")
    if len(dnsNodeIPs) == 0 {
        t.Skip("no dns nodes found")
    }

    // Step 1: Create a zone.
    zoneID := createTestZone(t, tenantID, regionID, "e2e-prop.example.com.")

    // Step 2: Create an A record.
    //   POST /zones/{zoneID}/records
    //   Body: { "type": "A", "name": "www", "content": "203.0.113.42", "ttl": 300 }
    //   Wait for active.

    // Step 3: Query PowerDNS directly.
    //   dig @{dnsNodeIP} A www.e2e-prop.example.com +short
    //   Expect: "203.0.113.42"
    answer := digQuery(t, dnsNodeIPs[0], "A", "www.e2e-prop.example.com")
    require.Contains(t, answer, "203.0.113.42")

    // Step 4: Update the record.
    //   PUT /zone-records/{id} with content: "203.0.113.100"
    //   Wait for active.

    // Step 5: Re-query and verify new value.
    answer = digQuery(t, dnsNodeIPs[0], "A", "www.e2e-prop.example.com")
    require.Contains(t, answer, "203.0.113.100")

    // Step 6: Delete the record. Wait for deleted.

    // Step 7: Verify record no longer resolves.
    //   dig should return NXDOMAIN or empty answer.
}
```

### TestDNSSOARecordHasBrandValues

Verify that the SOA record for a zone contains the brand's NS and hostmaster values.

```go
func TestDNSSOARecordHasBrandValues(t *testing.T) {
    // Create brand with specific NS values.
    // Create zone under that brand.
    // Query SOA record via dig.
    // Verify primary NS and hostmaster email match brand config.
}
```

### TestDNSMultipleRecordTypes

Query propagation for A, AAAA, CNAME, MX, TXT, SRV records.

```go
func TestDNSMultipleRecordTypePropagation(t *testing.T) {
    // Create zone.
    // Create records of each type.
    // Query each via dig and verify.
}
```

---

## 9. FQDN Full-Stack Binding E2E Tests

**File:** `tests/e2e/fqdn_binding_test.go`

This tests the complete flow: zone creation -> webroot creation -> FQDN binding ->
DNS record auto-creation -> HAProxy LB map update -> nginx config on web nodes ->
HTTP traffic reaches the correct webroot.

### TestFQDNFullStackBinding

```go
func TestFQDNFullStackBinding(t *testing.T) {
    tenantID, regionID, clusterID, _, _ := createTestTenant(t, "e2e-fqdn-full")

    // Step 1: Create a zone for the domain.
    zoneID := createTestZone(t, tenantID, regionID, "e2e-fullstack.example.com.")

    // Step 2: Create a webroot.
    webrootID := createTestWebroot(t, tenantID, "fqdn-site", "php", "8.5")

    // Step 3: Write a PHP test file to the webroot.
    ips := findNodeIPsByRole(t, clusterID, "web")
    sshExec(t, ips[0], fmt.Sprintf(
        "sudo mkdir -p /var/www/storage/%s/fqdn-site/public && "+
        "echo '<?php echo \"fqdn-test-ok\"; ?>' | sudo tee /var/www/storage/%s/fqdn-site/public/index.php",
        tenantID, tenantID))

    // Step 4: Bind FQDN to the webroot.
    fqdnID := createTestFQDN(t, webrootID, "app.e2e-fullstack.example.com.")

    // Step 5: Verify DNS auto-records were created.
    //   GET /zones/{zoneID}/records
    //   Look for platform-managed A/AAAA records with source_fqdn_id = fqdnID

    // Step 6: Verify DNS propagation (if DNS nodes available).
    dnsNodeIPs := findNodeIPsByRole(t, clusterID, "dns")
    if len(dnsNodeIPs) > 0 {
        answer := digQuery(t, dnsNodeIPs[0], "A", "app.e2e-fullstack.example.com")
        // Should resolve to LB addresses.
        require.NotEmpty(t, answer)
    }

    // Step 7: Verify HAProxy LB map includes the FQDN.
    //   SSH to web node, check /etc/haproxy/maps/fqdn-to-shard.map
    //   or check the HAProxy Runtime API.

    // Step 8: Verify nginx config includes a server block for this FQDN.
    //   SSH to web node:
    //   grep "app.e2e-fullstack.example.com" /etc/nginx/sites-enabled/*

    // Step 9: Make HTTP request through HAProxy with Host header.
    //   waitForHTTP(t, webTrafficURL, "app.e2e-fullstack.example.com", httpTimeout)
    //   Expect: body contains "fqdn-test-ok"

    // Step 10: Delete FQDN.
    // Step 11: Verify DNS records removed.
    // Step 12: Verify nginx config removed.
}
```

---

## 10. Backup and Restore E2E Tests (Extended)

**File:** `tests/e2e/backup_extended_test.go`

The existing `backup_test.go` covers web backup/restore. These extended tests
cover database backups and more thorough restoration verification.

### TestBackupDatabaseCycle

```go
func TestBackupDatabaseCycle(t *testing.T) {
    tenantID, _, clusterID, _, dbShardID := createTestTenant(t, "e2e-backup-db")
    if dbShardID == "" {
        t.Skip("no database shard found")
    }

    dbID := createTestDatabase(t, tenantID, dbShardID, "e2e_backupdb")

    // Step 1: Insert test data via SSH (mysql CLI on db node).
    //   CREATE TABLE e2e_test (id INT, val VARCHAR(100));
    //   INSERT INTO e2e_test VALUES (1, 'original');

    // Step 2: Create a database backup.
    //   POST /tenants/{tenantID}/backups
    //   Body: { "type": "database", "source_id": dbID }
    //   Wait for active.

    // Step 3: Modify data.
    //   UPDATE e2e_test SET val = 'modified' WHERE id = 1;
    //   INSERT INTO e2e_test VALUES (2, 'new-row');

    // Step 4: Restore the backup.
    //   POST /backups/{id}/restore
    //   Wait for active.

    // Step 5: Verify original state restored.
    //   SELECT * FROM e2e_test; -> only row (1, 'original')
}
```

### TestBackupListAndMetadata

```go
func TestBackupListAndMetadata(t *testing.T) {
    // Create tenant, webroot, database.
    // Create one web backup and one database backup.
    // List backups for tenant.
    // Verify both appear with correct type, source_id, source_name, size_bytes > 0.
    // Verify paginated response structure.
}
```

### TestBackupDeletePermissions

```go
func TestBackupDeletePermissions(t *testing.T) {
    // Create backup, delete it.
    //   DELETE /backups/{id}
    //   Expect: 202
    // Verify backup no longer in list.
}
```

---

## 11. Tenant Extended E2E Tests

**File:** `tests/e2e/tenant_extended_test.go`

### TestTenantResourceSummary

```go
func TestTenantResourceSummary(t *testing.T) {
    tenantID, _, clusterID, _, dbShardID := createTestTenant(t, "e2e-summary")

    // Create various resources: webroot, database, SSH key, zone.
    // Wait for all active.

    // GET /tenants/{tenantID}/resource-summary
    // Verify counts: webroots.active=1, databases.active=1, ssh_keys.active=1, zones.active=1
    // Verify total > 0
}
```

### TestTenantRetryFailed

Verify the retry and retry-failed endpoints work for tenants with failed resources.

```go
func TestTenantRetryFailed(t *testing.T) {
    // This test is difficult to set up deterministically since we need a
    // resource in "failed" state. Options:
    //   a) Create a resource that will fail (e.g., webroot on invalid shard)
    //   b) Use the retry endpoint on an active tenant (should be a no-op or ignored)
    //
    // Minimal test: verify the endpoints accept the request and return 202.

    tenantID, _, _, _, _ := createTestTenant(t, "e2e-retry")

    // POST /tenants/{tenantID}/retry -> 202
    // POST /tenants/{tenantID}/retry-failed -> 202
}
```

### TestTenantSuspendedResourceAccess

Verify that a suspended tenant's resources are still visible but marked accordingly.

```go
func TestTenantSuspendedResourceAccess(t *testing.T) {
    tenantID, _, _, _, _ := createTestTenant(t, "e2e-suspend-access")
    webrootID := createTestWebroot(t, tenantID, "suspend-site", "static", "1")

    // Suspend tenant.
    //   POST /tenants/{tenantID}/suspend
    //   Wait for status=suspended

    // Verify webroot still readable via API.
    //   GET /webroots/{webrootID} -> 200

    // Unsuspend.
    //   POST /tenants/{tenantID}/unsuspend
}
```

---

## 12. Error Recovery E2E Tests

**File:** `tests/e2e/error_recovery_test.go`

These tests verify that the system's retry mechanisms work correctly when
operations initially fail.

### TestWebrootRetry

```go
func TestWebrootRetry(t *testing.T) {
    tenantID, _, _, _, _ := createTestTenant(t, "e2e-wr-retry")

    // Create a webroot. If it reaches active, we test retry on a
    // successfully provisioned resource (should be a no-op or re-converge).
    webrootID := createTestWebroot(t, tenantID, "retry-site", "php", "8.5")

    // POST /webroots/{webrootID}/retry
    // Expect: 202
    // Wait for active (should return quickly if already converged).
}
```

### TestDatabaseRetry

```go
func TestDatabaseRetry(t *testing.T) {
    tenantID, _, _, _, dbShardID := createTestTenant(t, "e2e-db-retry")
    if dbShardID == "" {
        t.Skip("no database shard found")
    }

    dbID := createTestDatabase(t, tenantID, dbShardID, "e2e_retrydb")

    // POST /databases/{dbID}/retry -> 202
    // Wait for active.
}
```

### TestZoneRecordRetry

```go
func TestZoneRecordRetry(t *testing.T) {
    tenantID, regionID, _, _, _ := createTestTenant(t, "e2e-zr-retry")
    zoneID := createTestZone(t, tenantID, regionID, "e2e-retry.example.com.")

    // Create record, wait active.
    // POST /zone-records/{id}/retry -> 202
    // Wait for active.
}
```

### TestFQDNRetry

```go
func TestFQDNRetry(t *testing.T) {
    tenantID, _, _, _, _ := createTestTenant(t, "e2e-fqdn-retry")
    webrootID := createTestWebroot(t, tenantID, "retry-fqdn-site", "static", "1")
    fqdnID := createTestFQDN(t, webrootID, "retry.e2e-fqdn.example.com.")

    // POST /fqdns/{fqdnID}/retry -> 202
    // Wait for active.
}
```

### TestSSHKeyRetry

```go
func TestSSHKeyRetry(t *testing.T) {
    // Create SSH key, wait active.
    // POST /ssh-keys/{id}/retry -> 202
    // Wait for active.
}
```

### TestS3BucketRetry

```go
func TestS3BucketRetry(t *testing.T) {
    // Create bucket, wait active.
    // POST /s3-buckets/{id}/retry -> 202
    // Wait for active.
}
```

---

## 13. Search and Dashboard E2E Tests

**File:** `tests/e2e/admin_test.go`

### TestDashboardStats

```go
func TestDashboardStats(t *testing.T) {
    // GET /dashboard/stats
    // Expect: 200, JSON with counts (tenants, webroots, databases, etc.)
    // Verify response has expected keys.
}
```

### TestSearch

```go
func TestSearch(t *testing.T) {
    tenantID, _, _, _, _ := createTestTenant(t, "e2e-search")

    // GET /search?q={tenantID}
    // Expect: 200, results contain the tenant.
}
```

### TestAuditLogs

```go
func TestAuditLogs(t *testing.T) {
    // Perform an action (e.g., create a tenant).
    // GET /audit-logs
    // Expect: 200, paginated list, recent entry references the create action.
}
```

---

## 14. OIDC E2E Tests

**File:** `tests/e2e/oidc_test.go`

### TestOIDCDiscovery

```go
func TestOIDCDiscovery(t *testing.T) {
    // GET /.well-known/openid-configuration (no auth required)
    // Expect: 200, JSON with issuer, jwks_uri, authorization_endpoint, token_endpoint
}
```

### TestOIDCJWKS

```go
func TestOIDCJWKS(t *testing.T) {
    // GET /oidc/jwks (no auth required)
    // Expect: 200, JSON with keys array
}
```

---

## Implementation Priority

The tests should be implemented in this order, based on risk and coverage value:

| Priority | Test file | Rationale |
|----------|-----------|-----------|
| P0 | `valkey_test.go` | Core resource with zero coverage |
| P0 | `ssh_key_test.go` | Simple lifecycle, quick win |
| P0 | `brand_test.go` | Brand isolation is a fundamental correctness requirement |
| P1 | `email_test.go` | Depends on Stalwart being in test env; may need skips |
| P1 | `api_key_test.go` | Authorization enforcement is security-critical |
| P1 | `certificate_test.go` | Simple lifecycle |
| P1 | `dns_propagation_test.go` | Verifies end-to-end DNS correctness |
| P2 | `fqdn_binding_test.go` | Full-stack integration (DNS + LB + nginx) |
| P2 | `migration_test.go` | Requires second shard of each type |
| P2 | `backup_extended_test.go` | Database backup requires DB CLI access |
| P2 | `tenant_extended_test.go` | Resource summary, retry, suspend |
| P3 | `error_recovery_test.go` | Retry endpoints are mostly idempotent re-converge |
| P3 | `admin_test.go` | Dashboard, search, audit logs |
| P3 | `oidc_test.go` | Public endpoints, low risk |

---

## Test Environment Requirements

| Component | Required for | Notes |
|-----------|-------------|-------|
| Core API at `api.massive-hosting.com` | All tests | Already required |
| Web shard (2+ nodes) | Shared storage, FQDN binding | Already available |
| Database shard | Database tests, DB migration, DB backup | Already available |
| Valkey shard | Valkey tests, Valkey migration | Must exist in cluster |
| DNS shard (PowerDNS) | DNS propagation tests | Must exist in cluster |
| Storage shard (Ceph RGW) | S3 tests | Already available |
| Email shard (Stalwart) | Email tests | May not be available yet; tests should skip gracefully |
| Second web shard | Tenant migration | Optional; skip if unavailable |
| Second DB shard | Database migration | Optional; skip if unavailable |
| Second valkey shard | Valkey migration | Optional; skip if unavailable |
| SSH access to nodes | File verification, CLI checks | Already available |

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HOSTING_E2E` | (unset) | Set to `1` to enable E2E tests |
| `CORE_API_URL` | `https://api.massive-hosting.com/api/v1` | Core API base URL |
| `WEB_TRAFFIC_URL` | `https://10.10.10.2` | HAProxy URL for web traffic |
| `HOSTING_API_KEY` | `hst_dev_e2e_test_key_00000000` | Default API key |
| `SSH_KEY_PATH` | `~/.ssh/id_rsa` | SSH private key for node access |
| `CLUSTER_CONFIG` | `../../clusters/vm-generated.yaml` | Cluster config path |

---

## Test Execution

```bash
# Run all E2E tests
HOSTING_E2E=1 go test ./tests/e2e/... -v -timeout 30m

# Run a specific test
HOSTING_E2E=1 go test ./tests/e2e/... -v -run TestValkeyInstanceCRUD -timeout 10m

# Run a category (all Valkey tests)
HOSTING_E2E=1 go test ./tests/e2e/... -v -run TestValkey -timeout 15m

# Run with verbose logging for debugging
HOSTING_E2E=1 go test ./tests/e2e/... -v -count=1 -timeout 30m 2>&1 | tee e2e.log
```

The default `go test` timeout of 10 minutes is too short for E2E tests that
create multiple resources with async provisioning. Always use `-timeout 30m`
or longer.

---

## Cleanup Strategy

Every test follows the cleanup-on-exit pattern:

1. `createTestTenant` registers `t.Cleanup(func() { httpDelete(...) })`.
2. Individual resource creation helpers also register cleanup.
3. Cleanup runs in LIFO order (Go's testing behavior), so dependent resources
   are deleted before their parents.
4. Cleanup is best-effort: errors during deletion are logged but do not fail
   the test.

For tests that create brands or API keys (persistent resources), cleanup is
registered via `t.Cleanup` with the platform admin key.
