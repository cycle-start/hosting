package runtime

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"text/template"

	"github.com/rs/zerolog"

	agentv1 "github.com/edvin/hosting/proto/agent/v1"
)

const nodeServiceTemplate = `[Unit]
Description=Node.js app for {{ .TenantName }}/{{ .WebrootName }}
After=network.target

[Service]
Type=simple
User={{ .TenantName }}
Group={{ .TenantName }}
WorkingDirectory={{ .WorkingDir }}
ExecStart=/usr/bin/node {{ .EntryPoint }}
Restart=on-failure
RestartSec=5
Environment=NODE_ENV=production
Environment=PORT={{ .Port }}

StandardOutput=append:/home/{{ .TenantName }}/logs/node-{{ .WebrootName }}.log
StandardError=append:/home/{{ .TenantName }}/logs/node-{{ .WebrootName }}.error.log

[Install]
WantedBy=multi-user.target
`

var nodeServiceTmpl = template.Must(template.New("nodeservice").Parse(nodeServiceTemplate))

// Node manages Node.js application lifecycle via systemd service units.
type Node struct {
	logger zerolog.Logger
	svcMgr ServiceManager
}

// NewNode creates a new Node.js runtime manager.
func NewNode(logger zerolog.Logger, svcMgr ServiceManager) *Node {
	return &Node{
		logger: logger.With().Str("runtime", "node").Logger(),
		svcMgr: svcMgr,
	}
}

type nodeServiceData struct {
	TenantName  string
	WebrootName string
	WorkingDir  string
	EntryPoint  string
	Port        uint32
}

func (n *Node) serviceName(webroot *agentv1.WebrootInfo) string {
	return fmt.Sprintf("node-%s-%s", webroot.GetTenantName(), webroot.GetName())
}

func (n *Node) unitFilePath(webroot *agentv1.WebrootInfo) string {
	return filepath.Join("/etc/systemd/system", n.serviceName(webroot)+".service")
}

// computePort derives a deterministic port from the tenant and webroot name.
// Ports are mapped into the range 3000-9999.
func computePort(tenant, webroot string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(tenant + "/" + webroot))
	return 3000 + (h.Sum32() % 7000)
}

// Configure generates and writes a systemd service unit for the Node.js application.
func (n *Node) Configure(ctx context.Context, webroot *agentv1.WebrootInfo) error {
	port := computePort(webroot.GetTenantName(), webroot.GetName())
	entryPoint := "index.js"
	workingDir := filepath.Join("/var/www/storage", webroot.GetTenantName(), webroot.GetName())

	data := nodeServiceData{
		TenantName:  webroot.GetTenantName(),
		WebrootName: webroot.GetName(),
		WorkingDir:  workingDir,
		EntryPoint:  entryPoint,
		Port:        port,
	}

	var buf bytes.Buffer
	if err := nodeServiceTmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("render node service template: %w", err)
	}

	unitPath := n.unitFilePath(webroot)

	n.logger.Info().
		Str("tenant", webroot.GetTenantName()).
		Str("webroot", webroot.GetName()).
		Uint32("port", port).
		Str("path", unitPath).
		Msg("writing Node.js systemd unit")

	if err := os.WriteFile(unitPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write node systemd unit: %w", err)
	}

	return n.svcMgr.DaemonReload(ctx)
}

// Start enables and starts the Node.js systemd service.
func (n *Node) Start(ctx context.Context, webroot *agentv1.WebrootInfo) error {
	service := n.serviceName(webroot)
	n.logger.Info().Str("service", service).Msg("starting Node.js service")
	return n.svcMgr.Start(ctx, service)
}

// Stop stops and disables the Node.js systemd service.
func (n *Node) Stop(ctx context.Context, webroot *agentv1.WebrootInfo) error {
	service := n.serviceName(webroot)
	n.logger.Info().Str("service", service).Msg("stopping Node.js service")
	return n.svcMgr.Stop(ctx, service)
}

// Reload restarts the Node.js service to pick up changes.
func (n *Node) Reload(ctx context.Context, webroot *agentv1.WebrootInfo) error {
	service := n.serviceName(webroot)
	n.logger.Info().Str("service", service).Msg("restarting Node.js service")
	return n.svcMgr.Restart(ctx, service)
}

// Remove stops the service and removes the systemd unit file.
func (n *Node) Remove(ctx context.Context, webroot *agentv1.WebrootInfo) error {
	if err := n.Stop(ctx, webroot); err != nil {
		n.logger.Warn().Err(err).Msg("failed to stop node service during removal, continuing")
	}

	unitPath := n.unitFilePath(webroot)
	n.logger.Info().Str("path", unitPath).Msg("removing Node.js systemd unit")

	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove node systemd unit: %w", err)
	}

	return n.svcMgr.DaemonReload(ctx)
}
