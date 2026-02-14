package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	temporalclient "go.temporal.io/sdk/client"

	"github.com/edvin/hosting/internal/api"
	"github.com/edvin/hosting/internal/config"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/db"
	"github.com/edvin/hosting/internal/logging"
)

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "create-api-key" {
		createAPIKey(os.Args[2:])
		return
	}

	migrateFlag := flag.Bool("migrate", false, "Run database migrations before starting")
	migrateDirFlag := flag.String("migrate-dir", "migrations/core", "Migration files directory")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.Validate("core-api"); err != nil {
		fmt.Fprintf(os.Stderr, "invalid config: %v\n", err)
		os.Exit(1)
	}

	logger := logging.NewLogger(cfg)

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

	srv := api.NewServer(logger, corePool, tc, cfg)

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

func createAPIKey(args []string) {
	fs := flag.NewFlagSet("create-api-key", flag.ExitOnError)
	name := fs.String("name", "", "Name for the API key (required)")
	fs.Parse(args)

	if *name == "" {
		fmt.Fprintln(os.Stderr, "error: --name is required")
		fmt.Fprintln(os.Stderr, "usage: core-api create-api-key --name <name>")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to load config: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewCorePool(ctx, cfg.CoreDatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	svc := core.NewAPIKeyService(pool)
	key, rawKey, err := svc.Create(ctx, *name, nil, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create API key: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("API key created successfully.\n\n")
	fmt.Printf("  Name:   %s\n", key.Name)
	fmt.Printf("  ID:     %s\n", key.ID)
	fmt.Printf("  Key:    %s\n\n", rawKey)
	fmt.Printf("Save this key — it will not be shown again.\n")
}
