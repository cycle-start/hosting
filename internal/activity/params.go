package activity

// CreateTenantParams holds parameters for creating a tenant on a node.
type CreateTenantParams struct {
	ID          string
	UID         int
	SFTPEnabled bool
	SSHEnabled  bool
}

// UpdateTenantParams holds parameters for updating a tenant on a node.
type UpdateTenantParams struct {
	ID          string
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
	FQDNs          []FQDNParam
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
	FQDNs          []FQDNParam
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

// SyncSFTPKeysParams holds parameters for syncing SFTP authorized keys on a node.
type SyncSFTPKeysParams struct {
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

// BackupResult holds the result of a backup operation.
type BackupResult struct {
	StoragePath string
	SizeBytes   int64
}
