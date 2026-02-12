package main

import (
	"os"

	"github.com/rs/zerolog"
	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/agent"
	"github.com/edvin/hosting/internal/config"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load config")
	}

	if cfg.NodeID == "" {
		logger.Fatal().Msg("NODE_ID is required")
	}

	agentCfg := agent.Config{
		MySQLDSN:        cfg.MySQLDSN,
		NginxConfigDir:  getEnv("NGINX_CONFIG_DIR", "/etc/nginx"),
		WebStorageDir:   getEnv("WEB_STORAGE_DIR", "/var/www/storage"),
		HomeBaseDir:     getEnv("HOME_BASE_DIR", "/home"),
		CertDir:         getEnv("CERT_DIR", "/etc/ssl/hosting"),
		ValkeyConfigDir: getEnv("VALKEY_CONFIG_DIR", "/etc/valkey"),
		ValkeyDataDir:   getEnv("VALKEY_DATA_DIR", "/var/lib/valkey"),
		InitSystem:      getEnv("INIT_SYSTEM", "direct"),
		ShardName:       getEnv("SHARD_NAME", ""),
	}

	srv := agent.NewServer(logger, agentCfg)

	tc, err := temporalclient.Dial(temporalclient.Options{
		HostPort: cfg.TemporalAddress,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to temporal")
	}
	defer tc.Close()

	taskQueue := "node-" + cfg.NodeID
	w := worker.New(tc, taskQueue, worker.Options{})

	nodeActs := activity.NewNodeLocal(
		logger,
		srv.TenantManager(),
		srv.WebrootManager(),
		srv.NginxManager(),
		srv.DatabaseManager(),
		srv.ValkeyManager(),
		srv.Runtimes(),
	)
	w.RegisterActivity(nodeActs)

	logger.Info().
		Str("nodeID", cfg.NodeID).
		Str("taskQueue", taskQueue).
		Msg("starting node agent temporal worker")

	if err := w.Run(worker.InterruptCh()); err != nil {
		logger.Fatal().Err(err).Msg("worker failed")
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
