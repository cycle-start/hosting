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

const rubyServiceTemplate = `[Unit]
Description=Puma app for {{ .TenantName }}/{{ .WebrootName }}
After=network.target

[Service]
Type=simple
User={{ .TenantName }}
Group={{ .TenantName }}
WorkingDirectory={{ .WorkingDir }}
ExecStart=/usr/local/bin/bundle exec puma \
    -b unix:///run/puma/{{ .TenantName }}-{{ .WebrootName }}.sock \
    -e production \
    --workers 2 \
    --threads 1:5
ExecReload=/bin/kill -s USR1 $MAINPID
Restart=on-failure
RestartSec=5

StandardOutput=append:/home/{{ .TenantName }}/logs/puma-{{ .WebrootName }}.log
StandardError=append:/home/{{ .TenantName }}/logs/puma-{{ .WebrootName }}.error.log

[Install]
WantedBy=multi-user.target
`

var rubyServiceTmpl = template.Must(template.New("rubyservice").Parse(rubyServiceTemplate))

// Ruby manages Ruby (Puma) application lifecycle via systemd service units.
type Ruby struct {
	logger zerolog.Logger
	svcMgr ServiceManager
}

// NewRuby creates a new Ruby runtime manager.
func NewRuby(logger zerolog.Logger, svcMgr ServiceManager) *Ruby {
	return &Ruby{
		logger: logger.With().Str("runtime", "ruby").Logger(),
		svcMgr: svcMgr,
	}
}

type rubyServiceData struct {
	TenantName  string
	WebrootName string
	WorkingDir  string
}

func (r *Ruby) serviceName(webroot *agentv1.WebrootInfo) string {
	return fmt.Sprintf("puma-%s-%s", webroot.GetTenantName(), webroot.GetName())
}

func (r *Ruby) unitFilePath(webroot *agentv1.WebrootInfo) string {
	return filepath.Join("/etc/systemd/system", r.serviceName(webroot)+".service")
}

// Configure generates and writes a systemd service unit for the Puma application.
func (r *Ruby) Configure(ctx context.Context, webroot *agentv1.WebrootInfo) error {
	workingDir := filepath.Join("/var/www/storage", webroot.GetTenantName(), webroot.GetName())

	data := rubyServiceData{
		TenantName:  webroot.GetTenantName(),
		WebrootName: webroot.GetName(),
		WorkingDir:  workingDir,
	}

	var buf bytes.Buffer
	if err := rubyServiceTmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("render ruby service template: %w", err)
	}

	unitPath := r.unitFilePath(webroot)

	r.logger.Info().
		Str("tenant", webroot.GetTenantName()).
		Str("webroot", webroot.GetName()).
		Str("path", unitPath).
		Msg("writing Puma systemd unit")

	if err := os.MkdirAll("/run/puma", 0755); err != nil {
		return fmt.Errorf("create puma socket dir: %w", err)
	}

	if err := os.WriteFile(unitPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write ruby systemd unit: %w", err)
	}

	return r.svcMgr.DaemonReload(ctx)
}

// Start enables and starts the Puma systemd service.
func (r *Ruby) Start(ctx context.Context, webroot *agentv1.WebrootInfo) error {
	service := r.serviceName(webroot)
	r.logger.Info().Str("service", service).Msg("starting Puma service")
	return r.svcMgr.Start(ctx, service)
}

// Stop stops and disables the Puma systemd service.
func (r *Ruby) Stop(ctx context.Context, webroot *agentv1.WebrootInfo) error {
	service := r.serviceName(webroot)
	r.logger.Info().Str("service", service).Msg("stopping Puma service")
	return r.svcMgr.Stop(ctx, service)
}

// Reload sends a USR1 signal to Puma for graceful restart.
func (r *Ruby) Reload(ctx context.Context, webroot *agentv1.WebrootInfo) error {
	service := r.serviceName(webroot)
	r.logger.Info().Str("service", service).Msg("reloading Puma service")
	return r.svcMgr.Reload(ctx, service)
}

// Remove stops the service and removes the systemd unit file.
func (r *Ruby) Remove(ctx context.Context, webroot *agentv1.WebrootInfo) error {
	if err := r.Stop(ctx, webroot); err != nil {
		r.logger.Warn().Err(err).Msg("failed to stop puma service during removal, continuing")
	}

	unitPath := r.unitFilePath(webroot)
	r.logger.Info().Str("path", unitPath).Msg("removing Puma systemd unit")

	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove ruby systemd unit: %w", err)
	}

	sockPath := fmt.Sprintf("/run/puma/%s-%s.sock", webroot.GetTenantName(), webroot.GetName())
	_ = os.Remove(sockPath)

	return r.svcMgr.DaemonReload(ctx)
}
