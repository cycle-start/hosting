package runtime

import "context"

// WebrootInfo holds the information needed to configure a runtime for a webroot.
type WebrootInfo struct {
	ID             string
	TenantName     string
	Name           string
	Runtime        string
	RuntimeVersion string
	RuntimeConfig  string
	PublicFolder   string
}

// Manager defines the interface for language-specific runtime management.
// Each runtime implementation handles configuration, lifecycle, and cleanup
// for its specific application server (PHP-FPM, Node.js, Python/Gunicorn, etc.).
type Manager interface {
	// Configure generates and writes the runtime-specific configuration files
	// (e.g., PHP-FPM pool config, systemd unit files).
	Configure(ctx context.Context, webroot *WebrootInfo) error

	// Start activates the runtime for the given webroot.
	Start(ctx context.Context, webroot *WebrootInfo) error

	// Stop deactivates the runtime for the given webroot.
	Stop(ctx context.Context, webroot *WebrootInfo) error

	// Reload triggers a graceful reload of the runtime configuration.
	Reload(ctx context.Context, webroot *WebrootInfo) error

	// Remove cleans up all runtime configuration and stops the service.
	Remove(ctx context.Context, webroot *WebrootInfo) error
}
