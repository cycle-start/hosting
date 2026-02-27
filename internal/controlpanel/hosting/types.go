package hosting

import "time"

type Webroot struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	Runtime        string    `json:"runtime"`
	RuntimeVersion string    `json:"runtime_version"`
	PublicFolder   string    `json:"public_folder"`
	Status         string    `json:"status"`
	StatusMessage  *string   `json:"status_message"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type FQDN struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	WebrootID  *string   `json:"webroot_id"`
	FQDN       string    `json:"fqdn"`
	SSLEnabled bool      `json:"ssl_enabled"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Daemon struct {
	ID             string    `json:"id"`
	WebrootID      string    `json:"webroot_id"`
	Command        string    `json:"command"`
	ProxyPath      string    `json:"proxy_path"`
	ProxyPort      int       `json:"proxy_port"`
	NumProcs       int       `json:"num_procs"`
	StopSignal     string    `json:"stop_signal"`
	StopWaitSecs   int       `json:"stop_wait_secs"`
	MaxMemoryMB    int       `json:"max_memory_mb"`
	Enabled        bool      `json:"enabled"`
	Status         string    `json:"status"`
	StatusMessage  *string   `json:"status_message"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CronJob struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenant_id"`
	WebrootID        string    `json:"webroot_id"`
	Schedule         string    `json:"schedule"`
	Command          string    `json:"command"`
	WorkingDirectory string    `json:"working_directory"`
	Enabled          bool      `json:"enabled"`
	TimeoutSeconds   int       `json:"timeout_seconds"`
	MaxMemoryMB      int       `json:"max_memory_mb"`
	Status           string    `json:"status"`
	StatusMessage    *string   `json:"status_message"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Database resources

type Database struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	SubscriptionID string    `json:"subscription_id"`
	ShardID        string    `json:"shard_id"`
	NodeID         string    `json:"node_id"`
	Status         string    `json:"status"`
	StatusMessage  *string   `json:"status_message"`
	SuspendReason  *string   `json:"suspend_reason"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type DatabaseUser struct {
	ID            string    `json:"id"`
	DatabaseID    string    `json:"database_id"`
	Username      string    `json:"username"`
	Privileges    []string  `json:"privileges"`
	Status        string    `json:"status"`
	StatusMessage *string   `json:"status_message"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Login sessions

type LoginSession struct {
	SessionID string `json:"session_id"`
	ExpiresAt string `json:"expires_at"`
}

// Valkey resources

type ValkeyInstance struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	SubscriptionID string    `json:"subscription_id"`
	Port           int       `json:"port"`
	MaxMemoryMB    int       `json:"max_memory_mb"`
	Password       string    `json:"password,omitempty"`
	Status         string    `json:"status"`
	StatusMessage  *string   `json:"status_message"`
	SuspendReason  *string   `json:"suspend_reason"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ValkeyUser struct {
	ID               string    `json:"id"`
	ValkeyInstanceID string    `json:"valkey_instance_id"`
	Username         string    `json:"username"`
	Password         string    `json:"password,omitempty"`
	Privileges       []string  `json:"privileges"`
	KeyPattern       string    `json:"key_pattern"`
	Status           string    `json:"status"`
	StatusMessage    *string   `json:"status_message"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// S3 resources

type S3Bucket struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	SubscriptionID string    `json:"subscription_id"`
	Public         bool      `json:"public"`
	QuotaBytes     int64     `json:"quota_bytes"`
	Status         string    `json:"status"`
	StatusMessage  *string   `json:"status_message"`
	SuspendReason  *string   `json:"suspend_reason"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type S3AccessKey struct {
	ID              string    `json:"id"`
	S3BucketID      string    `json:"s3_bucket_id"`
	AccessKeyID     string    `json:"access_key_id"`
	SecretAccessKey  string   `json:"secret_access_key,omitempty"`
	Permissions     []string  `json:"permissions"`
	Status          string    `json:"status"`
	StatusMessage   *string   `json:"status_message"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Email resources

type EmailAccount struct {
	ID             string    `json:"id"`
	FQDNId         string    `json:"fqdn_id"`
	SubscriptionID string    `json:"subscription_id"`
	Address        string    `json:"address"`
	DisplayName    string    `json:"display_name"`
	QuotaBytes     int64     `json:"quota_bytes"`
	Status         string    `json:"status"`
	StatusMessage  *string   `json:"status_message"`
	TenantID       string    `json:"tenant_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type EmailAlias struct {
	ID             string    `json:"id"`
	EmailAccountID string    `json:"email_account_id"`
	Address        string    `json:"address"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type EmailForward struct {
	ID             string    `json:"id"`
	EmailAccountID string    `json:"email_account_id"`
	Destination    string    `json:"destination"`
	KeepCopy       bool      `json:"keep_copy"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type EmailAutoreply struct {
	ID             string    `json:"id"`
	EmailAccountID string    `json:"email_account_id"`
	Subject        string    `json:"subject"`
	Body           string    `json:"body"`
	StartDate      string    `json:"start_date"`
	EndDate        string    `json:"end_date"`
	Enabled        bool      `json:"enabled"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// DNS resources

type Zone struct {
	ID             string    `json:"id"`
	BrandID        string    `json:"brand_id"`
	TenantID       string    `json:"tenant_id"`
	SubscriptionID string    `json:"subscription_id"`
	Name           string    `json:"name"`
	RegionID       string    `json:"region_id"`
	Status         string    `json:"status"`
	StatusMessage  *string   `json:"status_message"`
	SuspendReason  *string   `json:"suspend_reason"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ZoneRecord struct {
	ID            string    `json:"id"`
	ZoneID        string    `json:"zone_id"`
	Type          string    `json:"type"`
	Name          string    `json:"name"`
	Content       string    `json:"content"`
	TTL           int       `json:"ttl"`
	Priority      *int      `json:"priority,omitempty"`
	ManagedBy     string    `json:"managed_by"`
	SourceType    string    `json:"source_type"`
	Status        string    `json:"status"`
	StatusMessage *string   `json:"status_message"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SSH Key resources

type SSHKey struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	Name          string    `json:"name"`
	PublicKey     string    `json:"public_key"`
	Fingerprint   string    `json:"fingerprint"`
	Status        string    `json:"status"`
	StatusMessage *string   `json:"status_message"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Backup resources

type Backup struct {
	ID            string     `json:"id"`
	TenantID      string     `json:"tenant_id"`
	Type          string     `json:"type"`
	SourceID      string     `json:"source_id"`
	SourceName    string     `json:"source_name"`
	StoragePath   string     `json:"storage_path"`
	SizeBytes     int64      `json:"size_bytes"`
	Status        string     `json:"status"`
	StatusMessage *string    `json:"status_message"`
	StartedAt     *time.Time `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

type EnvVar struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret"`
}

type SetEnvVarEntry struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

type VaultEncryptResponse struct {
	Token string `json:"token"`
}

type VaultDecryptResponse struct {
	Plaintext string `json:"plaintext"`
}

type Tenant struct {
	ID        string `json:"id"`
	RegionID  string `json:"region_id"`
	ClusterID string `json:"cluster_id"`
}

type ClusterRuntime struct {
	ClusterID string `json:"cluster_id"`
	Runtime   string `json:"runtime"`
	Version   string `json:"version"`
	Available bool   `json:"available"`
}

type RuntimeGroup struct {
	Runtime  string   `json:"runtime"`
	Versions []string `json:"versions"`
}

// WireGuard resources

type WireGuardPeer struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	SubscriptionID string    `json:"subscription_id"`
	Name           string    `json:"name"`
	PublicKey      string    `json:"public_key"`
	AssignedIP     string    `json:"assigned_ip"`
	PeerIndex      int       `json:"peer_index"`
	Endpoint       string    `json:"endpoint"`
	Status         string    `json:"status"`
	StatusMessage  *string   `json:"status_message"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type WireGuardPeerCreateResult struct {
	Peer         WireGuardPeer `json:"peer"`
	PrivateKey   string        `json:"private_key"`
	ClientConfig string        `json:"client_config"`
}

type PaginatedResponse[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}
