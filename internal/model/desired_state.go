package model

// DesiredState is the full desired state for a node, returned by the internal API.
type DesiredState struct {
	NodeID string       `json:"node_id"`
	Shards []ShardState `json:"shards"`
}

// ShardState is the desired state for a single shard on a node.
type ShardState struct {
	ShardID   string `json:"shard_id"`
	ShardRole string `json:"shard_role"`

	// Web shard fields
	Tenants []DesiredTenant `json:"tenants,omitempty"`

	// Database shard fields
	Databases []DesiredDatabase `json:"databases,omitempty"`

	// Valkey shard fields
	ValkeyInstances []DesiredValkeyInstance `json:"valkey_instances,omitempty"`

	// LB shard fields
	FQDNMappings []DesiredFQDNMapping `json:"fqdn_mappings,omitempty"`

	// Storage shard fields
	S3Buckets []DesiredS3Bucket `json:"s3_buckets,omitempty"`
}

// DesiredTenant is a tenant in the desired state for web shards.
type DesiredTenant struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	UID         int32            `json:"uid"`
	SFTPEnabled bool             `json:"sftp_enabled"`
	SSHEnabled  bool             `json:"ssh_enabled"`
	Status      string           `json:"status"`
	Webroots    []DesiredWebroot `json:"webroots,omitempty"`
	SSHKeys     []string         `json:"ssh_keys,omitempty"`
}

// DesiredWebroot is a webroot in the desired state.
type DesiredWebroot struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Runtime        string            `json:"runtime"`
	RuntimeVersion string            `json:"runtime_version"`
	RuntimeConfig  string            `json:"runtime_config"`
	PublicFolder   string            `json:"public_folder"`
	EnvVars        map[string]string `json:"env_vars,omitempty"`
	EnvFileName    string            `json:"env_file_name"`
	Status         string            `json:"status"`
	FQDNs          []DesiredFQDN     `json:"fqdns,omitempty"`
	CronJobs       []DesiredCronJob  `json:"cron_jobs,omitempty"`
	Daemons        []DesiredDaemon   `json:"daemons,omitempty"`
}

// DesiredCronJob is a cron job in the desired state.
type DesiredCronJob struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// DesiredDaemon is a daemon in the desired state.
type DesiredDaemon struct {
	ID        string  `json:"id"`
	NodeID    *string `json:"node_id,omitempty"`
	Name      string  `json:"name"`
	Enabled   bool    `json:"enabled"`
	ProxyPath *string `json:"proxy_path,omitempty"`
	ProxyPort *int    `json:"proxy_port,omitempty"`
}

// DesiredFQDN is an FQDN in the desired state.
type DesiredFQDN struct {
	FQDN       string `json:"fqdn"`
	SSLEnabled bool   `json:"ssl_enabled"`
	Status     string `json:"status"`
}

// DesiredDatabase is a database in the desired state for database shards.
type DesiredDatabase struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Status string          `json:"status"`
	Users  []DesiredDBUser `json:"users,omitempty"`
}

// DesiredDBUser is a database user in the desired state.
type DesiredDBUser struct {
	ID         string   `json:"id"`
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	Privileges []string `json:"privileges"`
	Status     string   `json:"status"`
}

// DesiredValkeyInstance is a Valkey instance in the desired state.
type DesiredValkeyInstance struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Port        int                 `json:"port"`
	Password    string              `json:"password"`
	MaxMemoryMB int                 `json:"max_memory_mb"`
	Status      string              `json:"status"`
	Users       []DesiredValkeyUser `json:"users,omitempty"`
}

// DesiredValkeyUser is a Valkey user in the desired state.
type DesiredValkeyUser struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Privileges string `json:"privileges"`
	KeyPattern string `json:"key_pattern"`
	Status     string `json:"status"`
}

// DesiredFQDNMapping is an FQDN-to-backend mapping for LB shards.
type DesiredFQDNMapping struct {
	FQDN      string `json:"fqdn"`
	LBBackend string `json:"lb_backend"`
}

// DesiredS3Bucket is an S3 bucket in the desired state.
type DesiredS3Bucket struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	TenantID string `json:"tenant_id"`
	Status   string `json:"status"`
}
