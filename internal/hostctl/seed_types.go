package hostctl

type SeedConfig struct {
	APIURL       string          `yaml:"api_url"`
	APIKey       string          `yaml:"api_key"`
	LBTrafficURL string          `yaml:"lb_traffic_url"`
	Region       string          `yaml:"region"`
	Cluster      string          `yaml:"cluster"`
	Brands       []BrandDef      `yaml:"brands"`
	Zones        []ZoneDef       `yaml:"zones"`
	Tenants      []TenantDef     `yaml:"tenants"`
	OIDCClients  []OIDCClientDef `yaml:"oidc_clients"`
}

type OIDCClientDef struct {
	ID           string   `yaml:"id"`
	Secret       string   `yaml:"secret"`
	Name         string   `yaml:"name"`
	RedirectURIs []string `yaml:"redirect_uris"`
}

type BrandDef struct {
	Name            string   `yaml:"name"`
	BaseHostname    string   `yaml:"base_hostname"`
	PrimaryNS       string   `yaml:"primary_ns"`
	SecondaryNS     string   `yaml:"secondary_ns"`
	HostmasterEmail string   `yaml:"hostmaster_email"`
	MailHostname    string   `yaml:"mail_hostname"`
	SPFIncludes     string   `yaml:"spf_includes"`
	DKIMSelector    string   `yaml:"dkim_selector"`
	DKIMPublicKey   string   `yaml:"dkim_public_key"`
	DMARCPolicy     string   `yaml:"dmarc_policy"`
	AllowedClusters []string `yaml:"allowed_clusters"`
}

type ZoneDef struct {
	Name   string `yaml:"name"`
	Brand  string `yaml:"brand"`
	Tenant string `yaml:"tenant"`
}

type TenantDef struct {
	Name            string              `yaml:"name"`
	Brand           string              `yaml:"brand"`
	Shard           string              `yaml:"shard"`
	SFTPEnabled     bool                `yaml:"sftp_enabled"`
	SSHEnabled      bool                `yaml:"ssh_enabled"`
	SSHKeys         []SSHKeyDef         `yaml:"ssh_keys"`
	Webroots        []WebrootDef        `yaml:"webroots"`
	Databases       []DatabaseDef       `yaml:"databases"`
	ValkeyInstances []ValkeyInstanceDef `yaml:"valkey_instances"`
	S3Buckets       []S3BucketDef       `yaml:"s3_buckets"`
	EmailAccounts   []EmailAcctDef      `yaml:"email_accounts"`
}

type SSHKeyDef struct {
	Name      string `yaml:"name"`
	PublicKey string `yaml:"public_key"` // Supports "${SSH_PUBLIC_KEY}" → reads SSH_PUBLIC_KEY env var or ~/.ssh/id_rsa.pub
}

type WebrootDef struct {
	Runtime        string            `yaml:"runtime"`
	RuntimeVersion string            `yaml:"runtime_version"`
	RuntimeConfig  map[string]any    `yaml:"runtime_config"`
	PublicFolder   string            `yaml:"public_folder"`
	EnvFileName    string            `yaml:"env_file_name"`
	EnvShellSource *bool             `yaml:"env_shell_source"`
	EnvVars        []EnvVarDef       `yaml:"env_vars"`
	FQDNs          []FQDNDef         `yaml:"fqdns"`
	Fixture        *FixtureDef       `yaml:"fixture"`
	Daemons        []DaemonDef       `yaml:"daemons"`
}

type EnvVarDef struct {
	Name   string `yaml:"name"`
	Value  string `yaml:"value"`
	Secret bool   `yaml:"secret"`
}

type FQDNDef struct {
	FQDN       string `yaml:"fqdn"`
	SSLEnabled bool   `yaml:"ssl_enabled"`
}

type FixtureDef struct {
	Tarball    string            `yaml:"tarball"`       // Path to .tar.gz
	EnvVars    map[string]string `yaml:"env_vars"`      // .env key=value (supports ${VAR} templates)
	SetupPath  string            `yaml:"setup_path"`    // POST here for migrations (e.g., "/api/setup")
	HostsEntry bool              `yaml:"hosts_entry"`   // Add 127.0.0.1 FQDN to /etc/hosts on web nodes
}

type DaemonDef struct {
	Command      string            `yaml:"command"`
	ProxyPath    string            `yaml:"proxy_path,omitempty"`
	NumProcs     int               `yaml:"num_procs,omitempty"`
	StopSignal   string            `yaml:"stop_signal,omitempty"`
	StopWaitSecs int               `yaml:"stop_wait_secs,omitempty"`
	MaxMemoryMB  int               `yaml:"max_memory_mb,omitempty"`
	Environment  map[string]string `yaml:"environment,omitempty"`
}

type DatabaseDef struct {
	Shard string            `yaml:"shard"`
	Users []DatabaseUserDef `yaml:"users"`
}

type DatabaseUserDef struct {
	Suffix     string   `yaml:"suffix"`
	Password   string   `yaml:"password"`
	Privileges []string `yaml:"privileges"`
}

type ValkeyInstanceDef struct {
	Shard       string          `yaml:"shard"`
	MaxMemoryMB int             `yaml:"max_memory_mb"`
	Users       []ValkeyUserDef `yaml:"users"`
}

type ValkeyUserDef struct {
	Username   string   `yaml:"username"`
	Password   string   `yaml:"password"`
	Privileges []string `yaml:"privileges"`
	KeyPattern string   `yaml:"key_pattern"`
}

type S3BucketDef struct {
	Shard      string `yaml:"shard"`
	Public     *bool  `yaml:"public"`
	QuotaBytes *int64 `yaml:"quota_bytes"`
}

type EmailAcctDef struct {
	FQDN        string              `yaml:"fqdn"`
	Address     string              `yaml:"address"`
	DisplayName string              `yaml:"display_name"`
	QuotaBytes  int64               `yaml:"quota_bytes"`
	Aliases     []EmailAliasDef     `yaml:"aliases"`
	Forwards    []EmailForwardDef   `yaml:"forwards"`
	AutoReply   *EmailAutoReplyDef  `yaml:"autoreply"`
}

type EmailAliasDef struct {
	Address string `yaml:"address"`
}

type EmailForwardDef struct {
	Destination string `yaml:"destination"`
	KeepCopy    *bool  `yaml:"keep_copy"`
}

type EmailAutoReplyDef struct {
	Subject string `yaml:"subject"`
	Body    string `yaml:"body"`
	Enabled bool   `yaml:"enabled"`
}
