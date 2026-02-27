package setup

// DeployMode describes how the platform will be deployed.
type DeployMode string

const (
	DeployModeSingle DeployMode = "single" // All roles on one machine (localhost)
	DeployModeMulti  DeployMode = "multi"  // Multiple machines with role assignments
	DeployModeK8s    DeployMode = "k8s"    // Existing Kubernetes cluster (Helm)
)

// Config is the in-memory state of the setup wizard.
type Config struct {
	DeployMode DeployMode `json:"deploy_mode"`

	// Region & cluster
	RegionName  string `json:"region_name"`
	ClusterName string `json:"cluster_name"`

	// Brand
	Brand BrandConfig `json:"brand"`

	// Control plane
	ControlPlane ControlPlaneConfig `json:"control_plane"`

	// Nodes (multi-node mode only)
	Nodes []NodeConfig `json:"nodes"`

	// Storage
	Storage StorageConfig `json:"storage"`

	// TLS
	TLS TLSConfig `json:"tls"`
}

// BrandConfig holds brand/domain configuration.
type BrandConfig struct {
	Name            string `json:"name"`
	PlatformDomain  string `json:"platform_domain"`  // e.g. "platform.example.com" — admin UI, API, temporal
	CustomerDomain  string `json:"customer_domain"`   // e.g. "hosting.example.com" — hosted sites base
	HostmasterEmail string `json:"hostmaster_email"`  // SOA hostmaster
	MailHostname    string `json:"mail_hostname"`     // MX target, e.g. "mail.hosting.example.com"
	PrimaryNS       string `json:"primary_ns"`        // e.g. "ns1.hosting.example.com"
	PrimaryNSIP     string `json:"primary_ns_ip"`     // IP for the primary NS glue record
	SecondaryNS     string `json:"secondary_ns"`      // e.g. "ns2.hosting.example.com"
	SecondaryNSIP   string `json:"secondary_ns_ip"`   // IP for the secondary NS glue record
}

// ControlPlaneConfig holds control plane infrastructure choices.
type ControlPlaneConfig struct {
	Database ControlPlaneDB `json:"database"`
}

// ControlPlaneDB controls whether PostgreSQL is managed or external.
type ControlPlaneDB struct {
	Mode     string `json:"mode"`     // "builtin" or "external"
	Host     string `json:"host"`     // External only
	Port     int    `json:"port"`     // External only
	Name     string `json:"name"`     // External only
	User     string `json:"user"`     // External only
	Password string `json:"password"` // External only
	SSLMode  string `json:"ssl_mode"` // External only
}

// NodeRole is a role that can be assigned to a machine.
type NodeRole string

const (
	RoleControlPlane NodeRole = "controlplane"
	RoleWeb          NodeRole = "web"
	RoleDatabase     NodeRole = "database"
	RoleDNS          NodeRole = "dns"
	RoleValkey       NodeRole = "valkey"
	RoleEmail        NodeRole = "email"
	RoleStorage      NodeRole = "storage"
	RoleLB           NodeRole = "lb"
	RoleGateway      NodeRole = "gateway"
	RoleDBAdmin      NodeRole = "dbadmin"
)

// AllRoles in display order.
var AllRoles = []NodeRole{
	RoleControlPlane,
	RoleWeb,
	RoleDatabase,
	RoleDNS,
	RoleValkey,
	RoleEmail,
	RoleStorage,
	RoleLB,
	RoleGateway,
	RoleDBAdmin,
}

// NodeConfig describes a machine in the deployment.
type NodeConfig struct {
	Hostname string     `json:"hostname"`
	IP       string     `json:"ip"`
	Roles    []NodeRole `json:"roles"`
}

// StorageConfig holds object/file storage configuration.
type StorageConfig struct {
	Mode         string `json:"mode"`         // "builtin" (Ceph) or "external"
	S3Endpoint   string `json:"s3_endpoint"`  // External only
	S3AccessKey  string `json:"s3_access_key"`
	S3SecretKey  string `json:"s3_secret_key"`
	S3BucketName string `json:"s3_bucket_name"`
}

// TLSConfig holds TLS/certificate configuration.
type TLSConfig struct {
	Mode  string `json:"mode"`  // "letsencrypt" or "manual"
	Email string `json:"email"` // Let's Encrypt contact email
}

// DefaultConfig returns a config with sensible defaults for exploration.
func DefaultConfig() *Config {
	return &Config{
		DeployMode:  DeployModeSingle,
		RegionName:  "default",
		ClusterName: "cluster-1",
		Brand: BrandConfig{
			HostmasterEmail: "hostmaster@example.com",
		},
		ControlPlane: ControlPlaneConfig{
			Database: ControlPlaneDB{
				Mode:    "builtin",
				Port:    5432,
				Name:    "hosting",
				User:    "hosting",
				SSLMode: "disable",
			},
		},
		Storage: StorageConfig{
			Mode: "builtin",
		},
		TLS: TLSConfig{
			Mode: "letsencrypt",
		},
	}
}
