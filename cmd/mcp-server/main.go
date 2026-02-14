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

	"github.com/rs/zerolog"

	"github.com/edvin/hosting/internal/mcpserver"
)

func main() {
	var (
		configPath = flag.String("config", "mcp.yaml", "Path to mcp.yaml configuration file")
		specFile   = flag.String("spec", "", "Path to swagger.json file (overrides fetching from API)")
		addr       = flag.String("addr", ":8090", "Listen address")
		logLevel   = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	)
	flag.Parse()

	// Setup logging
	level, err := zerolog.ParseLevel(*logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger().Level(level)

	// Load config
	cfg, err := mcpserver.LoadConfig(*configPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load config")
	}

	// Override API URL from environment
	if apiURL := os.Getenv("MCP_API_URL"); apiURL != "" {
		cfg.APIURL = apiURL
	}

	// Load swagger spec
	var specData []byte
	if *specFile != "" {
		specData, err = os.ReadFile(*specFile)
		if err != nil {
			logger.Fatal().Err(err).Str("path", *specFile).Msg("failed to read spec file")
		}
		logger.Info().Str("path", *specFile).Msg("loaded spec from file")
	} else {
		specData, err = mcpserver.FetchSpec(cfg.APIURL, cfg.SpecPath)
		if err != nil {
			logger.Fatal().Err(err).Str("url", cfg.APIURL+cfg.SpecPath).Msg("failed to fetch spec from API")
		}
		logger.Info().Str("url", cfg.APIURL+cfg.SpecPath).Msg("fetched spec from API")
	}

	// Create server
	srv, err := mcpserver.New(cfg, specData, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create MCP server")
	}

	// Override listen address from environment
	if envAddr := os.Getenv("MCP_ADDR"); envAddr != "" {
		*addr = envAddr
	}

	httpSrv := &http.Server{
		Addr:         *addr,
		Handler:      srv,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info().Str("addr", *addr).Msg("MCP server starting")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("server error")
		}
	}()

	<-done
	logger.Info().Msg("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("shutdown error")
	}

	fmt.Println("MCP server stopped")
}
