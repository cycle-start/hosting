package logging

import (
	"os"

	"github.com/rs/zerolog"

	"github.com/edvin/hosting/internal/config"
)

// NewLogger creates a structured zerolog.Logger with observability context fields
// from the config. Non-empty fields are added automatically.
func NewLogger(cfg *config.Config) zerolog.Logger {
	ctx := zerolog.New(os.Stdout).With().Timestamp()

	if cfg.ServiceName != "" {
		ctx = ctx.Str("service", cfg.ServiceName)
	}
	if cfg.RegionID != "" {
		ctx = ctx.Str("region", cfg.RegionID)
	}
	if cfg.ClusterID != "" {
		ctx = ctx.Str("cluster", cfg.ClusterID)
	}
	if cfg.ShardName != "" {
		ctx = ctx.Str("shard", cfg.ShardName)
	}
	if cfg.NodeID != "" {
		ctx = ctx.Str("node_id", cfg.NodeID)
	}
	if cfg.NodeRole != "" {
		ctx = ctx.Str("node_role", cfg.NodeRole)
	}

	logger := ctx.Logger()

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	return logger.Level(level)
}
