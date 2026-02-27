package hosting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) get(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("hosting API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hosting API %s: status %d", path, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, result any) error {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("hosting API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("hosting API %s %s: status %d", method, path, resp.StatusCode)
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func (c *Client) doNoBody(ctx context.Context, method, path string) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("hosting API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("hosting API %s %s: status %d", method, path, resp.StatusCode)
	}
	return nil
}

// ListEnvVars returns all env vars for a webroot.
func (c *Client) ListEnvVars(ctx context.Context, webrootID string) ([]EnvVar, error) {
	var resp PaginatedResponse[EnvVar]
	if err := c.get(ctx, fmt.Sprintf("/webroots/%s/env-vars", webrootID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// SetEnvVars replaces all env vars for a webroot.
func (c *Client) SetEnvVars(ctx context.Context, webrootID string, vars []SetEnvVarEntry) error {
	return c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/webroots/%s/env-vars", webrootID),
		map[string]any{"vars": vars}, nil)
}

// DeleteEnvVar deletes a single env var by name.
func (c *Client) DeleteEnvVar(ctx context.Context, webrootID, name string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/webroots/%s/env-vars/%s", webrootID, name))
}

// VaultEncrypt encrypts a plaintext value and returns a vault token.
func (c *Client) VaultEncrypt(ctx context.Context, webrootID, plaintext string) (string, error) {
	var resp VaultEncryptResponse
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/webroots/%s/vault/encrypt", webrootID),
		map[string]string{"plaintext": plaintext}, &resp); err != nil {
		return "", err
	}
	return resp.Token, nil
}

// VaultDecrypt decrypts a vault token and returns the plaintext.
func (c *Client) VaultDecrypt(ctx context.Context, webrootID, token string) (string, error) {
	var resp VaultDecryptResponse
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/webroots/%s/vault/decrypt", webrootID),
		map[string]string{"token": token}, &resp); err != nil {
		return "", err
	}
	return resp.Plaintext, nil
}

// ListWebrootsByTenant returns all webroots for a tenant.
func (c *Client) ListWebrootsByTenant(ctx context.Context, tenantID string) ([]Webroot, error) {
	var resp PaginatedResponse[Webroot]
	if err := c.get(ctx, fmt.Sprintf("/tenants/%s/webroots", tenantID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetWebroot returns a single webroot by ID.
func (c *Client) GetWebroot(ctx context.Context, id string) (*Webroot, error) {
	var w Webroot
	if err := c.get(ctx, fmt.Sprintf("/webroots/%s", id), &w); err != nil {
		return nil, err
	}
	return &w, nil
}

// UpdateWebroot updates a webroot's runtime, runtime_version, and public_folder.
func (c *Client) UpdateWebroot(ctx context.Context, id string, body any) (*Webroot, error) {
	var w Webroot
	if err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/webroots/%s", id), body, &w); err != nil {
		return nil, err
	}
	return &w, nil
}

// GetTenant returns a tenant by ID.
func (c *Client) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	var t Tenant
	if err := c.get(ctx, fmt.Sprintf("/tenants/%s", id), &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// ListClusterRuntimes returns the available runtimes for a cluster.
func (c *Client) ListClusterRuntimes(ctx context.Context, clusterID string) ([]ClusterRuntime, error) {
	var resp PaginatedResponse[ClusterRuntime]
	if err := c.get(ctx, fmt.Sprintf("/clusters/%s/runtimes", clusterID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// ListFQDNsByWebroot returns all FQDNs for a webroot.
func (c *Client) ListFQDNsByWebroot(ctx context.Context, webrootID string) ([]FQDN, error) {
	var resp PaginatedResponse[FQDN]
	if err := c.get(ctx, fmt.Sprintf("/webroots/%s/fqdns", webrootID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// ListFQDNsByTenant returns all FQDNs for a tenant (including unattached).
func (c *Client) ListFQDNsByTenant(ctx context.Context, tenantID string) ([]FQDN, error) {
	var resp PaginatedResponse[FQDN]
	if err := c.get(ctx, fmt.Sprintf("/tenants/%s/fqdns", tenantID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// UpdateFQDN updates an FQDN (e.g. to attach/detach a webroot).
func (c *Client) UpdateFQDN(ctx context.Context, id string, body any) (*FQDN, error) {
	var f FQDN
	if err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/fqdns/%s", id), body, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// GetFQDN returns a single FQDN by ID.
func (c *Client) GetFQDN(ctx context.Context, id string) (*FQDN, error) {
	var f FQDN
	if err := c.get(ctx, fmt.Sprintf("/fqdns/%s", id), &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// ListDaemonsByWebroot returns all daemons for a webroot.
func (c *Client) ListDaemonsByWebroot(ctx context.Context, webrootID string) ([]Daemon, error) {
	var resp PaginatedResponse[Daemon]
	if err := c.get(ctx, fmt.Sprintf("/webroots/%s/daemons", webrootID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetDaemon returns a single daemon by ID.
func (c *Client) GetDaemon(ctx context.Context, id string) (*Daemon, error) {
	var d Daemon
	if err := c.get(ctx, fmt.Sprintf("/daemons/%s", id), &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// CreateDaemon creates a daemon for a webroot.
func (c *Client) CreateDaemon(ctx context.Context, webrootID string, body any) (*Daemon, error) {
	var d Daemon
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/webroots/%s/daemons", webrootID), body, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// UpdateDaemon updates a daemon.
func (c *Client) UpdateDaemon(ctx context.Context, id string, body any) (*Daemon, error) {
	var d Daemon
	if err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/daemons/%s", id), body, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// EnableDaemon enables a daemon.
func (c *Client) EnableDaemon(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/daemons/%s/enable", id), nil, nil)
}

// DisableDaemon disables a daemon.
func (c *Client) DisableDaemon(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/daemons/%s/disable", id), nil, nil)
}

// DeleteDaemon deletes a daemon.
func (c *Client) DeleteDaemon(ctx context.Context, id string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/daemons/%s", id))
}

// ListCronJobsByWebroot returns all cron jobs for a webroot.
func (c *Client) ListCronJobsByWebroot(ctx context.Context, webrootID string) ([]CronJob, error) {
	var resp PaginatedResponse[CronJob]
	if err := c.get(ctx, fmt.Sprintf("/webroots/%s/cron-jobs", webrootID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetCronJob returns a single cron job by ID.
func (c *Client) GetCronJob(ctx context.Context, id string) (*CronJob, error) {
	var j CronJob
	if err := c.get(ctx, fmt.Sprintf("/cron-jobs/%s", id), &j); err != nil {
		return nil, err
	}
	return &j, nil
}

// CreateCronJob creates a cron job for a webroot.
func (c *Client) CreateCronJob(ctx context.Context, webrootID string, body any) (*CronJob, error) {
	var j CronJob
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/webroots/%s/cron-jobs", webrootID), body, &j); err != nil {
		return nil, err
	}
	return &j, nil
}

// UpdateCronJob updates a cron job.
func (c *Client) UpdateCronJob(ctx context.Context, id string, body any) (*CronJob, error) {
	var j CronJob
	if err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/cron-jobs/%s", id), body, &j); err != nil {
		return nil, err
	}
	return &j, nil
}

// EnableCronJob enables a cron job.
func (c *Client) EnableCronJob(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/cron-jobs/%s/enable", id), nil, nil)
}

// DisableCronJob disables a cron job.
func (c *Client) DisableCronJob(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/cron-jobs/%s/disable", id), nil, nil)
}

// DeleteCronJob deletes a cron job.
func (c *Client) DeleteCronJob(ctx context.Context, id string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/cron-jobs/%s", id))
}

// --- Database methods ---

// ListDatabasesByTenant returns all databases for a tenant.
func (c *Client) ListDatabasesByTenant(ctx context.Context, tenantID string) ([]Database, error) {
	var resp PaginatedResponse[Database]
	if err := c.get(ctx, fmt.Sprintf("/tenants/%s/databases", tenantID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetDatabase returns a single database by ID.
func (c *Client) GetDatabase(ctx context.Context, id string) (*Database, error) {
	var d Database
	if err := c.get(ctx, fmt.Sprintf("/databases/%s", id), &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// DeleteDatabase deletes a database.
func (c *Client) DeleteDatabase(ctx context.Context, id string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/databases/%s", id))
}

// ListDatabaseUsers returns all users for a database.
func (c *Client) ListDatabaseUsers(ctx context.Context, databaseID string) ([]DatabaseUser, error) {
	var resp PaginatedResponse[DatabaseUser]
	if err := c.get(ctx, fmt.Sprintf("/databases/%s/users", databaseID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// CreateDatabaseUser creates a user for a database.
func (c *Client) CreateDatabaseUser(ctx context.Context, databaseID string, body any) (*DatabaseUser, error) {
	var u DatabaseUser
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/databases/%s/users", databaseID), body, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateDatabaseUser updates a database user.
func (c *Client) UpdateDatabaseUser(ctx context.Context, userID string, body any) (*DatabaseUser, error) {
	var u DatabaseUser
	if err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/database-users/%s", userID), body, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// DeleteDatabaseUser deletes a database user.
func (c *Client) DeleteDatabaseUser(ctx context.Context, userID string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/database-users/%s", userID))
}

// CreateLoginSession creates a short-lived login session for a tenant, optionally scoped to a database.
func (c *Client) CreateLoginSession(ctx context.Context, tenantID string, databaseID string) (*LoginSession, error) {
	path := fmt.Sprintf("/tenants/%s/login-sessions", tenantID)
	if databaseID != "" {
		path += "?database_id=" + databaseID
	}
	var s LoginSession
	if err := c.doJSON(ctx, http.MethodPost, path, nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// --- Valkey methods ---

// ListValkeyInstancesByTenant returns all Valkey instances for a tenant.
func (c *Client) ListValkeyInstancesByTenant(ctx context.Context, tenantID string) ([]ValkeyInstance, error) {
	var resp PaginatedResponse[ValkeyInstance]
	if err := c.get(ctx, fmt.Sprintf("/tenants/%s/valkey-instances", tenantID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetValkeyInstance returns a single Valkey instance.
func (c *Client) GetValkeyInstance(ctx context.Context, id string) (*ValkeyInstance, error) {
	var v ValkeyInstance
	if err := c.get(ctx, fmt.Sprintf("/valkey-instances/%s", id), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// DeleteValkeyInstance deletes a Valkey instance.
func (c *Client) DeleteValkeyInstance(ctx context.Context, id string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/valkey-instances/%s", id))
}

// ListValkeyUsers returns all users for a Valkey instance.
func (c *Client) ListValkeyUsers(ctx context.Context, instanceID string) ([]ValkeyUser, error) {
	var resp PaginatedResponse[ValkeyUser]
	if err := c.get(ctx, fmt.Sprintf("/valkey-instances/%s/users", instanceID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// CreateValkeyUser creates a user for a Valkey instance.
func (c *Client) CreateValkeyUser(ctx context.Context, instanceID string, body any) (*ValkeyUser, error) {
	var u ValkeyUser
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/valkey-instances/%s/users", instanceID), body, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateValkeyUser updates a Valkey user.
func (c *Client) UpdateValkeyUser(ctx context.Context, userID string, body any) (*ValkeyUser, error) {
	var u ValkeyUser
	if err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/valkey-users/%s", userID), body, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// DeleteValkeyUser deletes a Valkey user.
func (c *Client) DeleteValkeyUser(ctx context.Context, userID string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/valkey-users/%s", userID))
}

// --- S3 methods ---

// ListS3BucketsByTenant returns all S3 buckets for a tenant.
func (c *Client) ListS3BucketsByTenant(ctx context.Context, tenantID string) ([]S3Bucket, error) {
	var resp PaginatedResponse[S3Bucket]
	if err := c.get(ctx, fmt.Sprintf("/tenants/%s/s3-buckets", tenantID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetS3Bucket returns a single S3 bucket.
func (c *Client) GetS3Bucket(ctx context.Context, id string) (*S3Bucket, error) {
	var b S3Bucket
	if err := c.get(ctx, fmt.Sprintf("/s3-buckets/%s", id), &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// UpdateS3Bucket updates an S3 bucket (public/quota).
func (c *Client) UpdateS3Bucket(ctx context.Context, id string, body any) (*S3Bucket, error) {
	var b S3Bucket
	if err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/s3-buckets/%s", id), body, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// DeleteS3Bucket deletes an S3 bucket.
func (c *Client) DeleteS3Bucket(ctx context.Context, id string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/s3-buckets/%s", id))
}

// ListS3AccessKeys returns all access keys for an S3 bucket.
func (c *Client) ListS3AccessKeys(ctx context.Context, bucketID string) ([]S3AccessKey, error) {
	var resp PaginatedResponse[S3AccessKey]
	if err := c.get(ctx, fmt.Sprintf("/s3-buckets/%s/access-keys", bucketID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// CreateS3AccessKey creates an access key for an S3 bucket.
func (c *Client) CreateS3AccessKey(ctx context.Context, bucketID string, body any) (*S3AccessKey, error) {
	var k S3AccessKey
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/s3-buckets/%s/access-keys", bucketID), body, &k); err != nil {
		return nil, err
	}
	return &k, nil
}

// DeleteS3AccessKey deletes an S3 access key.
func (c *Client) DeleteS3AccessKey(ctx context.Context, keyID string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/s3-access-keys/%s", keyID))
}

// --- Email methods ---

// ListEmailAccountsByTenant returns all email accounts for a tenant.
func (c *Client) ListEmailAccountsByTenant(ctx context.Context, tenantID string) ([]EmailAccount, error) {
	var resp PaginatedResponse[EmailAccount]
	if err := c.get(ctx, fmt.Sprintf("/tenants/%s/email-accounts", tenantID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetEmailAccount returns a single email account.
func (c *Client) GetEmailAccount(ctx context.Context, id string) (*EmailAccount, error) {
	var a EmailAccount
	if err := c.get(ctx, fmt.Sprintf("/email-accounts/%s", id), &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// DeleteEmailAccount deletes an email account.
func (c *Client) DeleteEmailAccount(ctx context.Context, id string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/email-accounts/%s", id))
}

// ListEmailAliases returns all aliases for an email account.
func (c *Client) ListEmailAliases(ctx context.Context, accountID string) ([]EmailAlias, error) {
	var resp PaginatedResponse[EmailAlias]
	if err := c.get(ctx, fmt.Sprintf("/email-accounts/%s/aliases", accountID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// CreateEmailAlias creates an alias for an email account.
func (c *Client) CreateEmailAlias(ctx context.Context, accountID string, body any) (*EmailAlias, error) {
	var a EmailAlias
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/email-accounts/%s/aliases", accountID), body, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// DeleteEmailAlias deletes an email alias.
func (c *Client) DeleteEmailAlias(ctx context.Context, aliasID string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/email-aliases/%s", aliasID))
}

// ListEmailForwards returns all forwards for an email account.
func (c *Client) ListEmailForwards(ctx context.Context, accountID string) ([]EmailForward, error) {
	var resp PaginatedResponse[EmailForward]
	if err := c.get(ctx, fmt.Sprintf("/email-accounts/%s/forwards", accountID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// CreateEmailForward creates a forward for an email account.
func (c *Client) CreateEmailForward(ctx context.Context, accountID string, body any) (*EmailForward, error) {
	var f EmailForward
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/email-accounts/%s/forwards", accountID), body, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// DeleteEmailForward deletes an email forward.
func (c *Client) DeleteEmailForward(ctx context.Context, forwardID string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/email-forwards/%s", forwardID))
}

// GetEmailAutoreply returns the autoreply for an email account.
func (c *Client) GetEmailAutoreply(ctx context.Context, accountID string) (*EmailAutoreply, error) {
	var a EmailAutoreply
	if err := c.get(ctx, fmt.Sprintf("/email-accounts/%s/autoreply", accountID), &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// SetEmailAutoreply sets the autoreply for an email account.
func (c *Client) SetEmailAutoreply(ctx context.Context, accountID string, body any) (*EmailAutoreply, error) {
	var a EmailAutoreply
	if err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/email-accounts/%s/autoreply", accountID), body, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// DeleteEmailAutoreply deletes the autoreply for an email account.
func (c *Client) DeleteEmailAutoreply(ctx context.Context, accountID string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/email-accounts/%s/autoreply", accountID))
}

// --- DNS methods ---

// ListAllZones returns all DNS zones (the hosting API has no tenant-scoped endpoint for zones).
func (c *Client) ListAllZones(ctx context.Context) ([]Zone, error) {
	var resp PaginatedResponse[Zone]
	if err := c.get(ctx, "/zones", &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetZone returns a single DNS zone.
func (c *Client) GetZone(ctx context.Context, id string) (*Zone, error) {
	var z Zone
	if err := c.get(ctx, fmt.Sprintf("/zones/%s", id), &z); err != nil {
		return nil, err
	}
	return &z, nil
}

// UpdateZone updates a DNS zone.
func (c *Client) UpdateZone(ctx context.Context, id string, body any) (*Zone, error) {
	var z Zone
	if err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/zones/%s", id), body, &z); err != nil {
		return nil, err
	}
	return &z, nil
}

// DeleteZone deletes a DNS zone.
func (c *Client) DeleteZone(ctx context.Context, id string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/zones/%s", id))
}

// ListZoneRecords returns all records for a DNS zone.
func (c *Client) ListZoneRecords(ctx context.Context, zoneID string) ([]ZoneRecord, error) {
	var resp PaginatedResponse[ZoneRecord]
	if err := c.get(ctx, fmt.Sprintf("/zones/%s/records", zoneID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// CreateZoneRecord creates a record in a DNS zone.
func (c *Client) CreateZoneRecord(ctx context.Context, zoneID string, body any) (*ZoneRecord, error) {
	var rec ZoneRecord
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/zones/%s/records", zoneID), body, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// UpdateZoneRecord updates a DNS record.
func (c *Client) UpdateZoneRecord(ctx context.Context, recordID string, body any) (*ZoneRecord, error) {
	var rec ZoneRecord
	if err := c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/zone-records/%s", recordID), body, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

// DeleteZoneRecord deletes a DNS record.
func (c *Client) DeleteZoneRecord(ctx context.Context, recordID string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/zone-records/%s", recordID))
}

// --- SSH Key methods ---

// ListSSHKeysByTenant returns all SSH keys for a tenant.
func (c *Client) ListSSHKeysByTenant(ctx context.Context, tenantID string) ([]SSHKey, error) {
	var resp PaginatedResponse[SSHKey]
	if err := c.get(ctx, fmt.Sprintf("/tenants/%s/ssh-keys", tenantID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetSSHKey returns a single SSH key.
func (c *Client) GetSSHKey(ctx context.Context, id string) (*SSHKey, error) {
	var k SSHKey
	if err := c.get(ctx, fmt.Sprintf("/ssh-keys/%s", id), &k); err != nil {
		return nil, err
	}
	return &k, nil
}

// CreateSSHKey creates an SSH key for a tenant.
func (c *Client) CreateSSHKey(ctx context.Context, tenantID string, body any) (*SSHKey, error) {
	var k SSHKey
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/tenants/%s/ssh-keys", tenantID), body, &k); err != nil {
		return nil, err
	}
	return &k, nil
}

// DeleteSSHKey deletes an SSH key.
func (c *Client) DeleteSSHKey(ctx context.Context, id string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/ssh-keys/%s", id))
}

// --- Backup methods ---

// ListBackupsByTenant returns all backups for a tenant.
func (c *Client) ListBackupsByTenant(ctx context.Context, tenantID string) ([]Backup, error) {
	var resp PaginatedResponse[Backup]
	if err := c.get(ctx, fmt.Sprintf("/tenants/%s/backups", tenantID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetBackup returns a single backup.
func (c *Client) GetBackup(ctx context.Context, id string) (*Backup, error) {
	var b Backup
	if err := c.get(ctx, fmt.Sprintf("/backups/%s", id), &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// CreateBackup triggers a new backup for a tenant.
func (c *Client) CreateBackup(ctx context.Context, tenantID string, body any) (*Backup, error) {
	var b Backup
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/tenants/%s/backups", tenantID), body, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// RestoreBackup triggers a restore from a backup.
func (c *Client) RestoreBackup(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/backups/%s/restore", id), nil, nil)
}

// DeleteBackup deletes a backup.
func (c *Client) DeleteBackup(ctx context.Context, id string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/backups/%s", id))
}

// --- WireGuard methods ---

// ListWireGuardPeersByTenant returns all WireGuard peers for a tenant.
func (c *Client) ListWireGuardPeersByTenant(ctx context.Context, tenantID string) ([]WireGuardPeer, error) {
	var resp PaginatedResponse[WireGuardPeer]
	if err := c.get(ctx, fmt.Sprintf("/tenants/%s/wireguard-peers", tenantID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetWireGuardPeer returns a single WireGuard peer by ID.
func (c *Client) GetWireGuardPeer(ctx context.Context, id string) (*WireGuardPeer, error) {
	var peer WireGuardPeer
	if err := c.get(ctx, fmt.Sprintf("/wireguard-peers/%s", id), &peer); err != nil {
		return nil, err
	}
	return &peer, nil
}

// CreateWireGuardPeer creates a WireGuard peer for a tenant.
func (c *Client) CreateWireGuardPeer(ctx context.Context, tenantID string, body any) (*WireGuardPeerCreateResult, error) {
	var result WireGuardPeerCreateResult
	if err := c.doJSON(ctx, http.MethodPost, fmt.Sprintf("/tenants/%s/wireguard-peers", tenantID), body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteWireGuardPeer deletes a WireGuard peer.
func (c *Client) DeleteWireGuardPeer(ctx context.Context, id string) error {
	return c.doNoBody(ctx, http.MethodDelete, fmt.Sprintf("/wireguard-peers/%s", id))
}
