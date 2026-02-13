package runtime

import (
	"context"

	"github.com/rs/zerolog"
)

// Static is a no-op runtime manager for static sites.
// Nginx serves the files directly, so no application server is needed.
type Static struct {
	logger zerolog.Logger
}

// NewStatic creates a new static runtime manager.
func NewStatic(logger zerolog.Logger) *Static {
	return &Static{logger: logger.With().Str("runtime", "static").Logger()}
}

// Configure is a no-op for static sites.
func (s *Static) Configure(_ context.Context, webroot *WebrootInfo) error {
	s.logger.Debug().
		Str("tenant", webroot.TenantName).
		Str("webroot", webroot.Name).
		Msg("static runtime: no configuration needed")
	return nil
}

// Start is a no-op for static sites.
func (s *Static) Start(_ context.Context, webroot *WebrootInfo) error {
	s.logger.Debug().
		Str("tenant", webroot.TenantName).
		Str("webroot", webroot.Name).
		Msg("static runtime: no start needed")
	return nil
}

// Stop is a no-op for static sites.
func (s *Static) Stop(_ context.Context, webroot *WebrootInfo) error {
	s.logger.Debug().
		Str("tenant", webroot.TenantName).
		Str("webroot", webroot.Name).
		Msg("static runtime: no stop needed")
	return nil
}

// Reload is a no-op for static sites.
func (s *Static) Reload(_ context.Context, webroot *WebrootInfo) error {
	s.logger.Debug().
		Str("tenant", webroot.TenantName).
		Str("webroot", webroot.Name).
		Msg("static runtime: no reload needed")
	return nil
}

// Remove is a no-op for static sites.
func (s *Static) Remove(_ context.Context, webroot *WebrootInfo) error {
	s.logger.Debug().
		Str("tenant", webroot.TenantName).
		Str("webroot", webroot.Name).
		Msg("static runtime: no removal needed")
	return nil
}
