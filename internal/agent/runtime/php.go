package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/rs/zerolog"

	agentv1 "github.com/edvin/hosting/proto/agent/v1"
)

const phpPoolTemplate = `[{{ .TenantName }}]
user = {{ .TenantName }}
group = {{ .TenantName }}

listen = /run/php/{{ .TenantName }}-php{{ .Version }}.sock
listen.owner = www-data
listen.group = www-data
listen.mode = 0660

pm = dynamic
pm.max_children = 5
pm.start_servers = 2
pm.min_spare_servers = 1
pm.max_spare_servers = 3
pm.max_requests = 500

php_admin_value[error_log] = /home/{{ .TenantName }}/logs/php-error.log
php_admin_flag[log_errors] = on
`

var phpPoolTmpl = template.Must(template.New("phppool").Parse(phpPoolTemplate))

// PHP manages PHP-FPM pool configuration and lifecycle.
type PHP struct {
	logger zerolog.Logger
	svcMgr ServiceManager
}

// NewPHP creates a new PHP runtime manager.
func NewPHP(logger zerolog.Logger, svcMgr ServiceManager) *PHP {
	return &PHP{
		logger: logger.With().Str("runtime", "php").Logger(),
		svcMgr: svcMgr,
	}
}

type phpPoolData struct {
	TenantName string
	Version    string
}

func (p *PHP) poolConfigPath(webroot *agentv1.WebrootInfo) string {
	version := webroot.GetRuntimeVersion()
	if version == "" {
		version = "8.5"
	}
	return filepath.Join("/etc/php", version, "fpm/pool.d", webroot.GetTenantName()+".conf")
}

func (p *PHP) fpmServiceName(webroot *agentv1.WebrootInfo) string {
	version := webroot.GetRuntimeVersion()
	if version == "" {
		version = "8.5"
	}
	return fmt.Sprintf("php%s-fpm", version)
}

// Configure generates and writes a PHP-FPM pool configuration file for the tenant.
func (p *PHP) Configure(ctx context.Context, webroot *agentv1.WebrootInfo) error {
	version := webroot.GetRuntimeVersion()
	if version == "" {
		version = "8.5"
	}

	data := phpPoolData{
		TenantName: webroot.GetTenantName(),
		Version:    version,
	}

	var buf bytes.Buffer
	if err := phpPoolTmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("render php pool template: %w", err)
	}

	configPath := p.poolConfigPath(webroot)

	p.logger.Info().
		Str("tenant", webroot.GetTenantName()).
		Str("version", version).
		Str("path", configPath).
		Msg("writing PHP-FPM pool config")

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("create php pool config dir: %w", err)
	}

	if err := os.WriteFile(configPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write php pool config: %w", err)
	}

	return nil
}

// Start reloads PHP-FPM to pick up the new pool configuration.
func (p *PHP) Start(ctx context.Context, webroot *agentv1.WebrootInfo) error {
	service := p.fpmServiceName(webroot)
	p.logger.Info().
		Str("tenant", webroot.GetTenantName()).
		Str("service", service).
		Msg("reloading PHP-FPM to start pool")

	// PHP-FPM uses USR2 for graceful reload.
	return p.svcMgr.Signal(ctx, service, "USR2")
}

// Stop removes the pool configuration and reloads PHP-FPM.
func (p *PHP) Stop(ctx context.Context, webroot *agentv1.WebrootInfo) error {
	configPath := p.poolConfigPath(webroot)
	service := p.fpmServiceName(webroot)

	p.logger.Info().
		Str("tenant", webroot.GetTenantName()).
		Str("path", configPath).
		Msg("removing PHP-FPM pool config and reloading")

	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove php pool config: %w", err)
	}

	return p.svcMgr.Signal(ctx, service, "USR2")
}

// Reload triggers a graceful reload of the PHP-FPM service.
func (p *PHP) Reload(ctx context.Context, webroot *agentv1.WebrootInfo) error {
	service := p.fpmServiceName(webroot)
	p.logger.Info().
		Str("tenant", webroot.GetTenantName()).
		Str("service", service).
		Msg("reloading PHP-FPM")

	return p.svcMgr.Signal(ctx, service, "USR2")
}

// Remove removes the pool configuration and reloads PHP-FPM.
func (p *PHP) Remove(ctx context.Context, webroot *agentv1.WebrootInfo) error {
	return p.Stop(ctx, webroot)
}
