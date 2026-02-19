package activity

import (
	"encoding/json"

	"github.com/edvin/hosting/internal/model"
)

// CreateTenantParams holds parameters for creating a tenant on a node.
type CreateTenantParams struct {
	ID             string
	Name           string
	UID            int
	SFTPEnabled    bool
	SSHEnabled     bool
	DiskQuotaBytes int64
}

// UpdateTenantParams holds parameters for updating a tenant on a node.
type UpdateTenantParams struct {
	ID          string
	Name        string
	UID         int
	SFTPEnabled bool
	SSHEnabled  bool
}

// FQDNParam represents an FQDN for webroot operations.
type FQDNParam struct {
	FQDN       string
	WebrootID  string
	SSLEnabled bool
}

// CreateWebrootParams holds parameters for creating a webroot on a node.
type CreateWebrootParams struct {
	ID             string
	TenantName     string
	Name           string
	Runtime        string
	RuntimeVersion string
	RuntimeConfig  string
	PublicFolder   string
	EnvVars        map[string]string
	EnvFileName    string
	EnvShellSource bool
	FQDNs          []FQDNParam
	Daemons        []DaemonProxyInfo
}

// UpdateWebrootParams holds parameters for updating a webroot on a node.
type UpdateWebrootParams struct {
	ID             string
	TenantName     string
	Name           string
	Runtime        string
	RuntimeVersion string
	RuntimeConfig  string
	PublicFolder   string
	EnvVars        map[string]string
	EnvFileName    string
	EnvShellSource bool
	FQDNs          []FQDNParam
	Daemons        []DaemonProxyInfo
}

// ConfigureRuntimeParams holds parameters for configuring a runtime on a node.
type ConfigureRuntimeParams struct {
	ID             string
	TenantName     string
	Name           string
	Runtime        string
	RuntimeVersion string
	RuntimeConfig  string
	PublicFolder   string
	EnvVars        map[string]string
}

// CreateDatabaseUserParams holds parameters for creating a database user on a node.
type CreateDatabaseUserParams struct {
	DatabaseName string
	Username     string
	Password     string
	Privileges   []string
}

// UpdateDatabaseUserParams holds parameters for updating a database user on a node.
type UpdateDatabaseUserParams struct {
	DatabaseName string
	Username     string
	Password     string
	Privileges   []string
}

// CreateValkeyInstanceParams holds parameters for creating a Valkey instance on a node.
type CreateValkeyInstanceParams struct {
	Name        string
	Port        int
	Password    string
	MaxMemoryMB int
}

// DeleteValkeyInstanceParams holds parameters for deleting a Valkey instance on a node.
type DeleteValkeyInstanceParams struct {
	Name string
	Port int
}

// CreateValkeyUserParams holds parameters for creating a Valkey user on a node.
type CreateValkeyUserParams struct {
	InstanceName string
	Port         int
	Username     string
	Password     string
	Privileges   []string
	KeyPattern   string
}

// UpdateValkeyUserParams holds parameters for updating a Valkey user on a node.
type UpdateValkeyUserParams struct {
	InstanceName string
	Port         int
	Username     string
	Password     string
	Privileges   []string
	KeyPattern   string
}

// DeleteValkeyUserParams holds parameters for deleting a Valkey user on a node.
type DeleteValkeyUserParams struct {
	InstanceName string
	Port         int
	Username     string
}

// InstallCertificateParams holds parameters for installing a certificate on a node.
type InstallCertificateParams struct {
	FQDN     string
	CertPEM  string
	KeyPEM   string
	ChainPEM string
}

// SyncSSHConfigParams holds parameters for syncing SSH/SFTP config on a node.
type SyncSSHConfigParams struct {
	TenantName  string
	SSHEnabled  bool
	SFTPEnabled bool
}

// SyncSSHKeysParams holds parameters for syncing SSH authorized keys on a node.
type SyncSSHKeysParams struct {
	TenantName string
	PublicKeys []string
}

// DumpMySQLDatabaseParams holds parameters for dumping a MySQL database on a node.
type DumpMySQLDatabaseParams struct {
	DatabaseName string
	DumpPath     string
}

// ImportMySQLDatabaseParams holds parameters for importing a MySQL database dump on a node.
type ImportMySQLDatabaseParams struct {
	DatabaseName string
	DumpPath     string
}

// DumpValkeyDataParams holds parameters for dumping Valkey data on a node.
type DumpValkeyDataParams struct {
	Name     string
	Port     int
	Password string
	DumpPath string
}

// ImportValkeyDataParams holds parameters for importing Valkey data on a node.
type ImportValkeyDataParams struct {
	Name     string
	Port     int
	DumpPath string
}

// CreateWebBackupParams holds parameters for creating a web backup on a node.
type CreateWebBackupParams struct {
	TenantName  string
	WebrootName string
	BackupPath  string // e.g. /var/backups/hosting/{tenant}/{backup-id}.tar.gz
}

// RestoreWebBackupParams holds parameters for restoring a web backup on a node.
type RestoreWebBackupParams struct {
	TenantName  string
	WebrootName string
	BackupPath  string
}

// CreateMySQLBackupParams holds parameters for creating a MySQL backup on a node.
type CreateMySQLBackupParams struct {
	DatabaseName string
	BackupPath   string // e.g. /var/backups/hosting/{tenant}/{backup-id}.sql.gz
}

// RestoreMySQLBackupParams holds parameters for restoring a MySQL backup on a node.
type RestoreMySQLBackupParams struct {
	DatabaseName string
	BackupPath   string
}

// CreateS3BucketParams holds parameters for creating an S3 bucket on a node.
type CreateS3BucketParams struct {
	TenantID   string
	Name       string
	QuotaBytes int64
}

// DeleteS3BucketParams holds parameters for deleting an S3 bucket on a node.
type DeleteS3BucketParams struct {
	TenantID string
	Name     string
}

// UpdateS3BucketPolicyParams holds parameters for updating an S3 bucket policy on a node.
type UpdateS3BucketPolicyParams struct {
	TenantID string
	Name     string
	Public   bool
}

// CreateS3AccessKeyParams holds parameters for creating an S3 access key on a node.
type CreateS3AccessKeyParams struct {
	TenantID        string
	AccessKeyID     string
	SecretAccessKey string
}

// DeleteS3AccessKeyParams holds parameters for deleting an S3 access key on a node.
type DeleteS3AccessKeyParams struct {
	TenantID    string
	AccessKeyID string
}

// CleanOrphanedConfigsInput holds parameters for cleaning orphaned nginx configs.
type CleanOrphanedConfigsInput struct {
	ExpectedConfigs map[string]bool `json:"expected_configs"`
}

// CleanOrphanedConfigsResult holds the result of cleaning orphaned nginx configs.
type CleanOrphanedConfigsResult struct {
	Removed []string `json:"removed"`
}

// CleanOrphanedFPMPoolsInput holds parameters for cleaning orphaned PHP-FPM pool configs.
type CleanOrphanedFPMPoolsInput struct {
	ExpectedPools map[string]bool `json:"expected_pools"`
}

// CleanOrphanedFPMPoolsResult holds the result of cleaning orphaned PHP-FPM pool configs.
type CleanOrphanedFPMPoolsResult struct {
	Removed []string `json:"removed"`
}

// CleanOrphanedDaemonConfigsInput holds parameters for cleaning orphaned supervisor daemon configs.
type CleanOrphanedDaemonConfigsInput struct {
	ExpectedConfigs map[string]bool `json:"expected_configs"`
}

// CleanOrphanedDaemonConfigsResult holds the result of cleaning orphaned supervisor daemon configs.
type CleanOrphanedDaemonConfigsResult struct {
	Removed []string `json:"removed"`
}

// CreateCronJobParams holds parameters for creating a cron job on a node.
type CreateCronJobParams struct {
	ID               string
	TenantName       string
	WebrootName      string
	Name             string
	Schedule         string
	Command          string
	WorkingDirectory string
	TimeoutSeconds   int
	MaxMemoryMB      int
}

// UpdateCronJobParams holds parameters for updating a cron job on a node.
type UpdateCronJobParams = CreateCronJobParams

// DeleteCronJobParams holds parameters for deleting a cron job on a node.
type DeleteCronJobParams struct {
	ID         string
	TenantName string
}

// CronJobTimerParams holds parameters for enabling/disabling a cron job timer on a node.
type CronJobTimerParams struct {
	ID         string
	TenantName string
}

// CreateDaemonParams holds parameters for creating a daemon on a node.
type CreateDaemonParams struct {
	ID           string
	NodeID       *string
	TenantName   string
	WebrootName  string
	Name         string
	Command      string
	ProxyPort    *int
	HostIP       string // Tenant's ULA address on the daemon's assigned node
	NumProcs     int
	StopSignal   string
	StopWaitSecs int
	MaxMemoryMB  int
	Environment  map[string]string
}

// UpdateDaemonParams holds parameters for updating a daemon on a node.
type UpdateDaemonParams = CreateDaemonParams

// DeleteDaemonParams holds parameters for deleting a daemon on a node.
type DeleteDaemonParams struct {
	ID          string
	TenantName  string
	WebrootName string
	Name        string
}

// DaemonEnableParams holds parameters for enabling/disabling a daemon on a node.
type DaemonEnableParams struct {
	ID          string
	TenantName  string
	WebrootName string
	Name        string
}

// DaemonProxyInfo holds proxy info for a daemon needed during nginx config generation.
type DaemonProxyInfo struct {
	ProxyPath string
	Port      int
	TargetIP  string // IPv6 ULA or 127.0.0.1 fallback
	ProxyURL  string // Pre-formatted proxy_pass URL (e.g. "http://[fd00:1:2::a]:14523")
}

// BackupResult holds the result of a backup operation.
type BackupResult struct {
	StoragePath string
	SizeBytes   int64
}

// ConfigureReplicationParams holds parameters for configuring MySQL replication.
type ConfigureReplicationParams struct {
	PrimaryHost  string
	ReplUser     string
	ReplPassword string
}

// ConfigureTenantAddressesParams holds parameters for configuring tenant ULA addresses on a node.
type ConfigureTenantAddressesParams struct {
	TenantName   string
	TenantUID    int
	ClusterID    string
	NodeShardIdx int
}

// ConfigureULARoutesParams holds parameters for setting up cross-node ULA routes.
type ConfigureULARoutesParams struct {
	ClusterID        string
	ThisNodeIndex    int
	OtherNodeIndices []int
}

// UpdateShardConfigParams holds parameters for updating a shard's config JSON.
type UpdateShardConfigParams struct {
	ShardID string
	Config  json.RawMessage
}

// SyncEgressRulesParams holds parameters for syncing nftables egress rules on a node.
type SyncEgressRulesParams struct {
	TenantUID int
	Rules     []model.TenantEgressRule
}

// SyncDatabaseUserHostsParams holds parameters for rebuilding MySQL user host patterns.
type SyncDatabaseUserHostsParams struct {
	DatabaseName    string
	Users           []model.DatabaseUser
	AccessRules     []model.DatabaseAccessRule
	InternalNetworkCIDR string
}
