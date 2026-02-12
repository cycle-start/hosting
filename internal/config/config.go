package config

import (
	"os"
)

type Config struct {
	CoreDatabaseURL    string
	ServiceDatabaseURL string
	TemporalAddress    string
	HTTPListenAddr     string
	MySQLDSN           string
	Deployer           string
	RegistryURL        string
	LogLevel           string
	StalwartAdminToken string
	// NodeID is the unique identifier for this node when running as a Temporal worker.
	// Used to register on the "node-{id}" task queue.
	NodeID string
}

func Load() (*Config, error) {
	cfg := &Config{
		CoreDatabaseURL:    getEnv("CORE_DATABASE_URL", ""),
		ServiceDatabaseURL: getEnv("SERVICE_DATABASE_URL", ""),
		TemporalAddress:    getEnv("TEMPORAL_ADDRESS", "localhost:7233"),
		HTTPListenAddr:     getEnv("HTTP_LISTEN_ADDR", ":8090"),
		MySQLDSN:           getEnv("MYSQL_DSN", ""),
		Deployer:           getEnv("DEPLOYER", "docker"),
		RegistryURL:        getEnv("REGISTRY_URL", "registry.localhost:5000"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		StalwartAdminToken: getEnv("STALWART_ADMIN_TOKEN", ""),
		NodeID:             getEnv("NODE_ID", ""),
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
