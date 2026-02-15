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

	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
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

