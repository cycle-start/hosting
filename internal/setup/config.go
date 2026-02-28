package setup

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// DeployMode describes how the platform will be deployed.
type DeployMode string

const (
	DeployModeSingle DeployMode = "single" // All roles on one machine (localhost)
	DeployModeMulti  DeployMode = "multi"  // Multiple machines with role assignments
)

// Config is the setup manifest — the single source of truth for a deployment.
// It can be created by the setup wizard UI or written by hand for automated deployments.
type Config struct {
	DeployMode DeployMode `json:"deploy_mode" yaml:"deploy_mode"`

	// Target host for all-in-one mode (default: 127.0.0.1)
	TargetHost string `json:"target_host,omitempty" yaml:"target_host,omitempty"`

	// SSH user for Ansible connections (default: ubuntu)
	SSHUser string `json:"ssh_user,omitempty" yaml:"ssh_user,omitempty"`

	// Region & cluster
	RegionName  string `json:"region_name" yaml:"region_name"`
	ClusterName string `json:"cluster_name" yaml:"cluster_name"`

	// Brand
	Brand BrandConfig `json:"brand" yaml:"brand"`

	// Control plane
	ControlPlane ControlPlaneConfig `json:"control_plane" yaml:"control_plane"`

	// Nodes (multi-node mode only)
	Nodes []NodeConfig `json:"nodes" yaml:"nodes,omitempty"`

	// TLS
	TLS TLSConfig `json:"tls" yaml:"tls"`

	// Email
	Email EmailConfig `json:"email" yaml:"email"`

	// SSO (Authelia)
	SSO SSOConfig `json:"sso" yaml:"sso"`

	// PHP versions to install on web nodes
	PHPVersions []string `json:"php_versions,omitempty" yaml:"php_versions,omitempty"`

	// API key for core-api authentication
	APIKey string `json:"api_key" yaml:"api_key"`
}

// EmailConfig holds email/Stalwart mail server configuration.
type EmailConfig struct {
	StalwartAdminToken string `json:"stalwart_admin_token" yaml:"stalwart_admin_token"`
}

// SSOConfig holds SSO configuration.
// Mode "internal" uses the built-in Authelia IdP; "external" uses a user-provided
// OIDC provider (e.g. Azure AD). Empty mode means SSO is disabled.
type SSOConfig struct {
	Mode string `json:"mode" yaml:"mode"` // "internal", "external", or ""

	// Internal (Authelia) — admin account created during setup
	AdminUsername string `json:"admin_username,omitempty" yaml:"admin_username,omitempty"`
	AdminEmail    string `json:"admin_email,omitempty" yaml:"admin_email,omitempty"`
	AdminPassword string `json:"admin_password,omitempty" yaml:"admin_password,omitempty"`

	// External OIDC provider
	IssuerURL    string `json:"issuer_url,omitempty" yaml:"issuer_url,omitempty"`
	ClientID     string `json:"client_id,omitempty" yaml:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty" yaml:"client_secret,omitempty"`
}

// BrandConfig holds brand/domain configuration.
type BrandConfig struct {
	Name            string `json:"name" yaml:"name"`
	PlatformDomain  string `json:"platform_domain" yaml:"platform_domain"`   // e.g. "platform.example.com" — admin UI, API, temporal
	CustomerDomain  string `json:"customer_domain" yaml:"customer_domain"`   // e.g. "hosting.example.com" — hosted sites base
	HostmasterEmail string `json:"hostmaster_email" yaml:"hostmaster_email"` // SOA hostmaster
	MailHostname    string `json:"mail_hostname" yaml:"mail_hostname"`       // MX target, e.g. "mail.hosting.example.com"
	PrimaryNS       string `json:"primary_ns" yaml:"primary_ns"`             // e.g. "ns1.hosting.example.com"
	PrimaryNSIP     string `json:"primary_ns_ip" yaml:"primary_ns_ip"`       // IP for the primary NS glue record
	SecondaryNS     string `json:"secondary_ns" yaml:"secondary_ns"`         // e.g. "ns2.hosting.example.com"
	SecondaryNSIP   string `json:"secondary_ns_ip" yaml:"secondary_ns_ip"`   // IP for the secondary NS glue record
}

// ControlPlaneConfig holds control plane infrastructure choices.
type ControlPlaneConfig struct {
	Database ControlPlaneDB `json:"database" yaml:"database"`
}

// ControlPlaneDB controls whether PostgreSQL is managed or external.
type ControlPlaneDB struct {
	Mode     string `json:"mode" yaml:"mode"`                           // "builtin" or "external"
	Host     string `json:"host,omitempty" yaml:"host,omitempty"`       // External only
	Port     int    `json:"port,omitempty" yaml:"port,omitempty"`       // External only
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`       // External only
	User     string `json:"user,omitempty" yaml:"user,omitempty"`       // External only
	Password string `json:"password,omitempty" yaml:"password,omitempty"` // External only
	SSLMode  string `json:"ssl_mode,omitempty" yaml:"ssl_mode,omitempty"` // External only
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
	Hostname string     `json:"hostname" yaml:"hostname"`
	IP       string     `json:"ip" yaml:"ip"`
	Roles    []NodeRole `json:"roles" yaml:"roles"`
}

// TLSConfig holds TLS/certificate configuration.
type TLSConfig struct {
	Mode  string `json:"mode" yaml:"mode"`                     // "letsencrypt" or "manual"
	Email string `json:"email,omitempty" yaml:"email,omitempty"` // Let's Encrypt contact email
}

// DefaultConfig returns a config with sensible defaults for exploration.
func DefaultConfig() *Config {
	return &Config{
		DeployMode:  DeployModeSingle,
		TargetHost:  "127.0.0.1",
		SSHUser:     "ubuntu",
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
		TLS: TLSConfig{
			Mode: "letsencrypt",
		},
		Email: EmailConfig{
			StalwartAdminToken: generateRandomToken(),
		},
		PHPVersions: []string{"8.3", "8.5"},
		APIKey:      generateAPIKey(),
	}
}

// singleNodeIP returns the target host IP for all-in-one mode, defaulting to 127.0.0.1.
func (c *Config) singleNodeIP() string {
	if c.TargetHost != "" {
		return c.TargetHost
	}
	return "127.0.0.1"
}

// generateRandomToken returns 32 random hex characters.
func generateRandomToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}

// generateAPIKey returns an API key in the format "hst_" + 32 random hex bytes.
func generateAPIKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return "hst_" + hex.EncodeToString(b)
}
