package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/rs/zerolog"
)

// DaemonInfo holds the information needed to manage a daemon on a node.
type DaemonInfo struct {
	ID           string
	TenantName   string
	WebrootName  string
	Name         string
	Command      string
	ProxyPort    *int
	HostIP       string // Tenant's ULA address on the daemon's assigned node (e.g. "fd00:1:2::a")
	NumProcs     int
	StopSignal   string
	StopWaitSecs int
	MaxMemoryMB  int
	Environment  map[string]string
}

// DaemonManager manages supervisord program configs for tenant daemons.
type DaemonManager struct {
	logger        zerolog.Logger
	webStorageDir string
}

// NewDaemonManager creates a new DaemonManager.
func NewDaemonManager(logger zerolog.Logger, cfg Config) *DaemonManager {
	return &DaemonManager{
		logger:        logger.With().Str("component", "daemon-manager").Logger(),
		webStorageDir: cfg.WebStorageDir,
	}
}

func (m *DaemonManager) configPath(info *DaemonInfo) string {
	return filepath.Join("/etc/supervisor/conf.d", fmt.Sprintf("daemon-%s-%s.conf", info.TenantName, info.Name))
}

func (m *DaemonManager) programName(info *DaemonInfo) string {
	return fmt.Sprintf("daemon-%s-%s", info.TenantName, info.Name)
}

// Configure generates and writes a supervisord program configuration file for the daemon.
func (m *DaemonManager) Configure(ctx context.Context, info *DaemonInfo) error {
	m.logger.Info().
		Str("daemon", info.ID).
		Str("tenant", info.TenantName).
		Str("name", info.Name).
		Msg("configuring daemon")

	workDir := filepath.Join(m.webStorageDir, info.TenantName, "webroots", info.WebrootName)

	// Build environment string with PORT if proxy_port is set.
	env := make(map[string]string)
	for k, v := range info.Environment {
		env[k] = v
	}
	if info.ProxyPort != nil {
		env["PORT"] = fmt.Sprintf("%d", *info.ProxyPort)
		if info.HostIP != "" {
			env["HOST"] = info.HostIP
		}
	}

	data := daemonConfigData{
		TenantName:   info.TenantName,
		WebrootName:  info.WebrootName,
		DaemonName:   info.Name,
		Command:      info.Command,
		WorkingDir:   workDir,
		NumProcs:     info.NumProcs,
		StopSignal:   info.StopSignal,
		StopWaitSecs: info.StopWaitSecs,
		MaxMemoryMB:  info.MaxMemoryMB,
		Environment:  formatDaemonEnvironment(env),
	}

	var buf bytes.Buffer
	if err := daemonConfigTmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("render daemon config template: %w", err)
	}

	configPath := m.configPath(info)

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("create supervisor conf dir: %w", err)
	}

	if err := os.WriteFile(configPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write supervisor config: %w", err)
	}

	return m.supervisorctl(ctx, "reread")
}

// Start updates supervisord and starts the daemon program.
func (m *DaemonManager) Start(ctx context.Context, info *DaemonInfo) error {
	program := m.programName(info)

	m.logger.Info().
		Str("tenant", info.TenantName).
		Str("daemon", info.Name).
		Str("program", program).
		Msg("starting daemon")

	if err := m.supervisorctl(ctx, "update"); err != nil {
		return err
	}
	return m.supervisorctl(ctx, "start", program+":*")
}

// Stop stops the daemon program.
func (m *DaemonManager) Stop(ctx context.Context, info *DaemonInfo) error {
	program := m.programName(info)

	m.logger.Info().
		Str("tenant", info.TenantName).
		Str("daemon", info.Name).
		Str("program", program).
		Msg("stopping daemon")

	// Ignore errors — the program may not be running.
	_ = m.supervisorctl(ctx, "stop", program+":*")
	return nil
}

// Reload triggers a full re-read, update, and restart of the daemon program.
func (m *DaemonManager) Reload(ctx context.Context, info *DaemonInfo) error {
	program := m.programName(info)

	m.logger.Info().
		Str("tenant", info.TenantName).
		Str("daemon", info.Name).
		Str("program", program).
		Msg("reloading daemon")

	if err := m.supervisorctl(ctx, "reread"); err != nil {
		return err
	}
	if err := m.supervisorctl(ctx, "update"); err != nil {
		return err
	}
	return m.supervisorctl(ctx, "restart", program+":*")
}

// Remove stops the daemon, removes its config file, and cleans up supervisord.
func (m *DaemonManager) Remove(ctx context.Context, info *DaemonInfo) error {
	m.Stop(ctx, info)

	configPath := m.configPath(info)
	m.logger.Info().
		Str("tenant", info.TenantName).
		Str("daemon", info.Name).
		Str("path", configPath).
		Msg("removing supervisord daemon config")

	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove supervisor config: %w", err)
	}

	if err := m.supervisorctl(ctx, "reread"); err != nil {
		return err
	}
	return m.supervisorctl(ctx, "update")
}

// supervisorctl executes a supervisorctl command.
func (m *DaemonManager) supervisorctl(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "supervisorctl", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("supervisorctl %v: %s: %w", args, string(output), err)
	}
	return nil
}

const daemonConfigTemplate = `; Auto-generated by node-agent for daemon {{ .TenantName }}/{{ .DaemonName }}
; DO NOT EDIT MANUALLY

[program:daemon-{{ .TenantName }}-{{ .DaemonName }}]
command={{ .Command }}
directory={{ .WorkingDir }}
user={{ .TenantName }}
numprocs={{ .NumProcs }}
{{- if gt .NumProcs 1 }}
process_name=%(program_name)s_%(process_num)02d
{{- end }}
autostart=true
autorestart=unexpected
stopsignal={{ .StopSignal }}
stopwaitsecs={{ .StopWaitSecs }}
stdout_logfile=/var/www/storage/{{ .TenantName }}/logs/daemon-{{ .DaemonName }}.log
stdout_logfile_maxbytes=10MB
stdout_logfile_backups=3
stderr_logfile=/var/www/storage/{{ .TenantName }}/logs/daemon-{{ .DaemonName }}.error.log
stderr_logfile_maxbytes=10MB
stderr_logfile_backups=3
{{- if .Environment }}
environment={{ .Environment }}
{{- end }}
`

var daemonConfigTmpl = template.Must(template.New("daemonconfig").Parse(daemonConfigTemplate))

type daemonConfigData struct {
	TenantName   string
	WebrootName  string
	DaemonName   string
	Command      string
	WorkingDir   string
	NumProcs     int
	StopSignal   string
	StopWaitSecs int
	MaxMemoryMB  int
	Environment  string
}

// formatDaemonEnvironment formats the environment map as a supervisord environment string.
func formatDaemonEnvironment(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", k, env[k]))
	}
	return strings.Join(parts, ",")
}
