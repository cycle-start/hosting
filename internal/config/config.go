package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	CoreDatabaseURL    string
	PowerDNSDatabaseURL string
	TemporalAddress    string
	HTTPListenAddr     string
	MySQLDSN           string
	RegistryURL        string
	LogLevel           string
	StalwartAdminToken string
	// NodeID is the unique identifier for this node when running as a Temporal worker.
	// Used to register on the "node-{id}" task queue.
	NodeID string

	ACMEEmail        string // ACME_EMAIL — contact email for Let's Encrypt
	ACMEDirectoryURL string // ACME_DIRECTORY_URL — defaults to LE production

	// Retention
	AuditLogRetentionDays int // AUDIT_LOG_RETENTION_DAYS — default 90
	BackupRetentionDays   int // BACKUP_RETENTION_DAYS — default 30

	// OIDC
	OIDCIssuerURL string // OIDC_ISSUER_URL — issuer URL for the built-in OIDC provider

	// Temporal mTLS
	TemporalTLSCert       string // TEMPORAL_TLS_CERT — path to client cert
	TemporalTLSKey        string // TEMPORAL_TLS_KEY — path to client key
	TemporalTLSCACert     string // TEMPORAL_TLS_CA_CERT — path to CA cert
	TemporalTLSServerName string // TEMPORAL_TLS_SERVER_NAME — SNI override

	// Observability context
	RegionID    string // REGION_ID
	ClusterID   string // CLUSTER_ID
	ShardName   string // SHARD_NAME
	NodeRole    string // NODE_ROLE
	ServiceName string // SERVICE_NAME
	MetricsAddr string // METRICS_ADDR — listen addr for /metrics (worker + node-agent)

	LokiURL       string // LOKI_URL — Loki query endpoint for platform logs (default: http://127.0.0.1:3100)
	TenantLokiURL string // TENANT_LOKI_URL — Loki query endpoint for tenant logs (default: http://127.0.0.1:3101)

	SecretEncryptionKey string // SECRET_ENCRYPTION_KEY — 32-byte AES-256 key, hex-encoded

	InternalNetworkCIDR string // INTERNAL_NETWORK_CIDR — default 10.0.0.0/8, used for database ingress default

	// LLM Agent
	AgentEnabled            bool   // AGENT_ENABLED — enable the LLM incident agent (default: false)
	AgentMaxConcurrent      int    // AGENT_MAX_CONCURRENT — max parallel group leaders (default: 3)
	AgentFollowerConcurrent int    // AGENT_FOLLOWER_CONCURRENT — max parallel followers per group after leader resolves (default: 5)
	AgentAPIKey             string // AGENT_API_KEY — API key for the agent to call the core API
	LLMBaseURL        string // LLM_BASE_URL — OpenAI-compatible API base URL
	LLMAPIKey         string // LLM_API_KEY — API key for the LLM endpoint
	LLMModel          string // LLM_MODEL — model name (default: Qwen/Qwen2.5-72B-Instruct)
	LLMMaxTurns       int    // LLM_MAX_TURNS — max conversation turns per investigation (default: 10)

	SSHCAPrivateKey string // SSH_CA_PRIVATE_KEY — PEM-encoded SSH CA private key (for web terminal)

	WireGuardEndpoint string // WIREGUARD_ENDPOINT — public endpoint for WireGuard VPN (e.g. "vpn.massive-hosting.com:51820")

	MCPApiURL string // MCP_API_URL — base URL the MCP proxy uses to reach the core API (default: http://127.0.0.1:8090)
}

func Load() (*Config, error) {
	cfg := &Config{
		CoreDatabaseURL:       getEnv("CORE_DATABASE_URL", ""),
		PowerDNSDatabaseURL:   getEnv("POWERDNS_DATABASE_URL", ""),
		TemporalAddress:       getEnv("TEMPORAL_ADDRESS", "localhost:7233"),
		HTTPListenAddr:        getEnv("HTTP_LISTEN_ADDR", ":8090"),
		MySQLDSN:              getEnv("MYSQL_DSN", ""),
		RegistryURL:           getEnv("REGISTRY_URL", "registry.localhost:5000"),
		LogLevel:              getEnv("LOG_LEVEL", "info"),
		StalwartAdminToken:    getEnv("STALWART_ADMIN_TOKEN", ""),
		NodeID:                getEnv("NODE_ID", ""),
		ACMEEmail:             getEnv("ACME_EMAIL", ""),
		ACMEDirectoryURL:      getEnv("ACME_DIRECTORY_URL", "https://acme-v02.api.letsencrypt.org/directory"),
		OIDCIssuerURL:         getEnv("OIDC_ISSUER_URL", "http://api.hosting.localhost"),
		AuditLogRetentionDays: getEnvInt("AUDIT_LOG_RETENTION_DAYS", 90),
		BackupRetentionDays:   getEnvInt("BACKUP_RETENTION_DAYS", 30),
		TemporalTLSCert:       getEnv("TEMPORAL_TLS_CERT", ""),
		TemporalTLSKey:        getEnv("TEMPORAL_TLS_KEY", ""),
		TemporalTLSCACert:     getEnv("TEMPORAL_TLS_CA_CERT", ""),
		TemporalTLSServerName: getEnv("TEMPORAL_TLS_SERVER_NAME", ""),

		RegionID:    getEnv("REGION_ID", ""),
		ClusterID:   getEnv("CLUSTER_ID", ""),
		ShardName:   getEnv("SHARD_NAME", ""),
		NodeRole:    getEnv("NODE_ROLE", ""),
		ServiceName: getEnv("SERVICE_NAME", ""),
		MetricsAddr: getEnv("METRICS_ADDR", ""),

		LokiURL:       getEnv("LOKI_URL", "http://127.0.0.1:3100"),
		TenantLokiURL: getEnv("TENANT_LOKI_URL", "http://127.0.0.1:3101"),

		SecretEncryptionKey: getEnv("SECRET_ENCRYPTION_KEY", ""),

		InternalNetworkCIDR: getEnv("INTERNAL_NETWORK_CIDR", "10.0.0.0/8"),

		AgentEnabled:            getEnvBool("AGENT_ENABLED", false),
		AgentMaxConcurrent:      getEnvInt("AGENT_MAX_CONCURRENT", 3),
		AgentFollowerConcurrent: getEnvInt("AGENT_FOLLOWER_CONCURRENT", 5),
		AgentAPIKey:             getEnv("AGENT_API_KEY", ""),
		LLMBaseURL:         getEnv("LLM_BASE_URL", ""),
		LLMAPIKey:          getEnv("LLM_API_KEY", ""),
		LLMModel:           getEnv("LLM_MODEL", "Qwen/Qwen2.5-72B-Instruct"),
		LLMMaxTurns:        getEnvInt("LLM_MAX_TURNS", 10),

		SSHCAPrivateKey: getEnv("SSH_CA_PRIVATE_KEY", ""),

		WireGuardEndpoint: getEnv("WIREGUARD_ENDPOINT", ""),

		MCPApiURL: getEnv("MCP_API_URL", "http://127.0.0.1:8090"),
	}

	return cfg, nil
}

// Validate checks that all required config fields are set for the given binary.
func (c *Config) Validate(binary string) error {
	var missing []string

	switch binary {
	case "core-api":
		if c.CoreDatabaseURL == "" {
			missing = append(missing, "CORE_DATABASE_URL")
		}
		if c.TemporalAddress == "" {
			missing = append(missing, "TEMPORAL_ADDRESS")
		}
		if c.HTTPListenAddr == "" {
			missing = append(missing, "HTTP_LISTEN_ADDR")
		}
		if c.SecretEncryptionKey == "" {
			missing = append(missing, "SECRET_ENCRYPTION_KEY")
		}
	case "worker":
		if c.CoreDatabaseURL == "" {
			missing = append(missing, "CORE_DATABASE_URL")
		}
		if c.PowerDNSDatabaseURL == "" {
			missing = append(missing, "POWERDNS_DATABASE_URL")
		}
		if c.TemporalAddress == "" {
			missing = append(missing, "TEMPORAL_ADDRESS")
		}
		if c.SecretEncryptionKey == "" {
			missing = append(missing, "SECRET_ENCRYPTION_KEY")
		}
	case "node-agent":
		if c.NodeID == "" {
			missing = append(missing, "NODE_ID")
		}
		if c.TemporalAddress == "" {
			missing = append(missing, "TEMPORAL_ADDRESS")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}

	// Cross-field: cert and key must both be set or both unset.
	if (c.TemporalTLSCert != "") != (c.TemporalTLSKey != "") {
		return fmt.Errorf("TEMPORAL_TLS_CERT and TEMPORAL_TLS_KEY must both be set or both unset")
	}

	// Agent: require LLM_BASE_URL and AGENT_API_KEY when enabled.
	if c.AgentEnabled {
		if c.LLMBaseURL == "" {
			missing = append(missing, "LLM_BASE_URL")
		}
		if c.AgentAPIKey == "" {
			missing = append(missing, "AGENT_API_KEY")
		}
		if len(missing) > 0 {
			return fmt.Errorf("AGENT_ENABLED=true but missing: %s", strings.Join(missing, ", "))
		}
	}

	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "true" || v == "1" || v == "yes"
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

