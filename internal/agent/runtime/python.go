package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/rs/zerolog"
)

const pythonServiceTemplate = `[Unit]
Description=Gunicorn app for {{ .TenantName }}/{{ .WebrootName }}
After=network.target

[Service]
Type=notify
User={{ .TenantName }}
Group={{ .TenantName }}
WorkingDirectory={{ .WorkingDir }}
ExecStart=/usr/bin/gunicorn \
    --bind unix:/run/gunicorn/{{ .TenantName }}-{{ .WebrootName }}.sock \
    --workers 3 \
    --timeout 120 \
    {{ .WSGIModule }}
ExecReload=/bin/kill -s HUP $MAINPID
Restart=on-failure
RestartSec=5

StandardOutput=append:/var/www/storage/{{ .TenantName }}/logs/gunicorn-{{ .WebrootName }}.log
StandardError=append:/var/www/storage/{{ .TenantName }}/logs/gunicorn-{{ .WebrootName }}.error.log

[Install]
WantedBy=multi-user.target
`

var pythonServiceTmpl = template.Must(template.New("pythonservice").Parse(pythonServiceTemplate))

// Python manages Python (Gunicorn) application lifecycle via systemd service units.
type Python struct {
	logger zerolog.Logger
	svcMgr ServiceManager
}

// NewPython creates a new Python runtime manager.
func NewPython(logger zerolog.Logger, svcMgr ServiceManager) *Python {
	return &Python{
		logger: logger.With().Str("runtime", "python").Logger(),
		svcMgr: svcMgr,
	}
}

type pythonServiceData struct {
	TenantName  string
	WebrootName string
	WorkingDir  string
	WSGIModule  string
}

func (p *Python) serviceName(webroot *WebrootInfo) string {
	return fmt.Sprintf("gunicorn-%s-%s", webroot.TenantName, webroot.Name)
}

func (p *Python) unitFilePath(webroot *WebrootInfo) string {
	return filepath.Join("/etc/systemd/system", p.serviceName(webroot)+".service")
}

// Configure generates and writes a systemd service unit for the Gunicorn application.
func (p *Python) Configure(ctx context.Context, webroot *WebrootInfo) error {
	wsgiModule := "app:application"
	workingDir := filepath.Join("/var/www/storage", webroot.TenantName, "webroots", webroot.Name)

	data := pythonServiceData{
		TenantName:  webroot.TenantName,
		WebrootName: webroot.Name,
		WorkingDir:  workingDir,
		WSGIModule:  wsgiModule,
	}

	var buf bytes.Buffer
	if err := pythonServiceTmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("render python service template: %w", err)
	}

	unitPath := p.unitFilePath(webroot)

	p.logger.Info().
		Str("tenant", webroot.TenantName).
		Str("webroot", webroot.Name).
		Str("path", unitPath).
		Msg("writing Gunicorn systemd unit")

	if err := os.MkdirAll("/run/gunicorn", 0755); err != nil {
		return fmt.Errorf("create gunicorn socket dir: %w", err)
	}

	if err := os.WriteFile(unitPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write python systemd unit: %w", err)
	}

	return p.svcMgr.DaemonReload(ctx)
}

// Start enables and starts the Gunicorn systemd service.
func (p *Python) Start(ctx context.Context, webroot *WebrootInfo) error {
	service := p.serviceName(webroot)
	p.logger.Info().Str("service", service).Msg("starting Gunicorn service")
	return p.svcMgr.Start(ctx, service)
}

// Stop stops and disables the Gunicorn systemd service.
func (p *Python) Stop(ctx context.Context, webroot *WebrootInfo) error {
	service := p.serviceName(webroot)
	p.logger.Info().Str("service", service).Msg("stopping Gunicorn service")
	return p.svcMgr.Stop(ctx, service)
}

// Reload sends a HUP signal to Gunicorn for graceful reload.
func (p *Python) Reload(ctx context.Context, webroot *WebrootInfo) error {
	service := p.serviceName(webroot)
	p.logger.Info().Str("service", service).Msg("reloading Gunicorn service")
	return p.svcMgr.Reload(ctx, service)
}

// Remove stops the service and removes the systemd unit file.
func (p *Python) Remove(ctx context.Context, webroot *WebrootInfo) error {
	if err := p.Stop(ctx, webroot); err != nil {
		p.logger.Warn().Err(err).Msg("failed to stop gunicorn service during removal, continuing")
	}

	unitPath := p.unitFilePath(webroot)
	p.logger.Info().Str("path", unitPath).Msg("removing Gunicorn systemd unit")

	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove python systemd unit: %w", err)
	}

	sockPath := fmt.Sprintf("/run/gunicorn/%s-%s.sock", webroot.TenantName, webroot.Name)
	_ = os.Remove(sockPath)

	return p.svcMgr.DaemonReload(ctx)
}
