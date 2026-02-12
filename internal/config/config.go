package config

import (
	"os"
)

type Config struct {
	CoreDatabaseURL    string
	ServiceDatabaseURL string
	TemporalAddress    string
	GRPCListenAddr     string
	HTTPListenAddr     string
	NodeAgentAddr      string
	MySQLDSN           string
	Deployer           string
	RegistryURL        string
	LogLevel           string
	StalwartAdminToken string
}

func Load() (*Config, error) {
	cfg := &Config{
		CoreDatabaseURL:    getEnv("CORE_DATABASE_URL", ""),
		ServiceDatabaseURL: getEnv("SERVICE_DATABASE_URL", ""),
		TemporalAddress:    getEnv("TEMPORAL_ADDRESS", "localhost:7233"),
		GRPCListenAddr:     getEnv("GRPC_LISTEN_ADDR", ":9090"),
		HTTPListenAddr:     getEnv("HTTP_LISTEN_ADDR", ":8090"),
		NodeAgentAddr:      getEnv("NODE_AGENT_ADDR", "localhost:9090"),
		MySQLDSN:           getEnv("MYSQL_DSN", ""),
		Deployer:           getEnv("DEPLOYER", "docker"),
		RegistryURL:        getEnv("REGISTRY_URL", "registry.localhost:5000"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		StalwartAdminToken: getEnv("STALWART_ADMIN_TOKEN", ""),
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
