package hostctl

type SeedConfig struct {
	APIURL      string          `yaml:"api_url"`
	APIKey      string          `yaml:"api_key"`
	Region      string          `yaml:"region"`
	Cluster     string          `yaml:"cluster"`
	Brands      []BrandDef      `yaml:"brands"`
	Zones       []ZoneDef       `yaml:"zones"`
	Tenants     []TenantDef     `yaml:"tenants"`
	OIDCClients []OIDCClientDef `yaml:"oidc_clients"`
}

type OIDCClientDef struct {
	ID           string   `yaml:"id"`
	Secret       string   `yaml:"secret"`
	Name         string   `yaml:"name"`
	RedirectURIs []string `yaml:"redirect_uris"`
}

type BrandDef struct {
	ID              string   `yaml:"id"`
	Name            string   `yaml:"name"`
	BaseHostname    string   `yaml:"base_hostname"`
	PrimaryNS       string   `yaml:"primary_ns"`
	SecondaryNS     string   `yaml:"secondary_ns"`
	HostmasterEmail string   `yaml:"hostmaster_email"`
	AllowedClusters []string `yaml:"allowed_clusters"`
}

type ZoneDef struct {
	Name   string `yaml:"name"`
	Brand  string `yaml:"brand"`
	Tenant string `yaml:"tenant"`
}

type TenantDef struct {
	Name             string              `yaml:"name"`
	Brand            string              `yaml:"brand"`
	Shard            string              `yaml:"shard"`
	SFTPEnabled      bool                `yaml:"sftp_enabled"`
	SSHEnabled       bool                `yaml:"ssh_enabled"`
	Webroots         []WebrootDef        `yaml:"webroots"`
	Databases        []DatabaseDef       `yaml:"databases"`
	ValkeyInstances  []ValkeyInstanceDef `yaml:"valkey_instances"`
	S3Buckets        []S3BucketDef       `yaml:"s3_buckets"`
	EmailAccounts    []EmailAcctDef      `yaml:"email_accounts"`
}

type WebrootDef struct {
	Name           string         `yaml:"name"`
	Runtime        string         `yaml:"runtime"`
	RuntimeVersion string         `yaml:"runtime_version"`
	RuntimeConfig  map[string]any `yaml:"runtime_config"`
	PublicFolder   string         `yaml:"public_folder"`
	FQDNs          []FQDNDef      `yaml:"fqdns"`
}

type FQDNDef struct {
	FQDN       string `yaml:"fqdn"`
	SSLEnabled bool   `yaml:"ssl_enabled"`
}

type DatabaseDef struct {
	Name  string          `yaml:"name"`
	Shard string          `yaml:"shard"`
	Users []DatabaseUserDef `yaml:"users"`
}

type DatabaseUserDef struct {
	Username   string   `yaml:"username"`
	Password   string   `yaml:"password"`
	Privileges []string `yaml:"privileges"`
}

type ValkeyInstanceDef struct {
	Name        string          `yaml:"name"`
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
	Name       string `yaml:"name"`
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
