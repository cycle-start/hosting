package main

import (
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/edvin/hosting/internal/agent"
	"github.com/edvin/hosting/internal/config"
	agentv1 "github.com/edvin/hosting/proto/agent/v1"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load config")
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

	grpcServer := grpc.NewServer()
	agentv1.RegisterNodeAgentServer(grpcServer, srv)

	lis, err := net.Listen("tcp", cfg.GRPCListenAddr)
	if err != nil {
		logger.Fatal().Err(err).Str("addr", cfg.GRPCListenAddr).Msg("failed to listen")
	}

	go func() {
		logger.Info().Str("addr", cfg.GRPCListenAddr).Msg("starting node agent gRPC server")
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal().Err(err).Msg("grpc server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutting down node agent")
	grpcServer.GracefulStop()
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
