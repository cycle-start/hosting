package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	temporalclient "go.temporal.io/sdk/client"

	"github.com/edvin/hosting/internal/api"
	"github.com/edvin/hosting/internal/config"
	"github.com/edvin/hosting/internal/db"
)

func main() {
	migrateFlag := flag.Bool("migrate", false, "Run database migrations before starting")
	migrateDirFlag := flag.String("migrate-dir", "migrations/core", "Migration files directory")
	flag.Parse()

	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load config")
	}

	if err := cfg.Validate("core-api"); err != nil {
		logger.Fatal().Err(err).Msg("invalid config")
	}

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		logger.Fatal().Str("level", cfg.LogLevel).Msg("invalid log level")
	}
	logger = logger.Level(level)

	if *migrateFlag {
		logger.Info().Str("dir", *migrateDirFlag).Msg("running database migrations")
		if err := db.RunMigrations(cfg.CoreDatabaseURL, *migrateDirFlag); err != nil {
			logger.Fatal().Err(err).Msg("migration failed")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	corePool, err := db.NewCorePool(ctx, cfg.CoreDatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to core database")
	}
	defer corePool.Close()

	tlsConfig, err := cfg.TemporalTLS()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to configure temporal TLS")
	}
	dialOpts := temporalclient.Options{HostPort: cfg.TemporalAddress}
	if tlsConfig != nil {
		dialOpts.ConnectionOptions = temporalclient.ConnectionOptions{TLS: tlsConfig}
		logger.Info().Msg("temporal mTLS enabled")
	}
	tc, err := temporalclient.Dial(dialOpts)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to temporal")
	}
	defer tc.Close()

	srv := api.NewServer(logger, corePool, tc)

	httpServer := &http.Server{
		Addr:         cfg.HTTPListenAddr,
		Handler:      srv,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info().Str("addr", cfg.HTTPListenAddr).Msg("starting core API server")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutting down server")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	httpServer.Shutdown(shutdownCtx)
}
