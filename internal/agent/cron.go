package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/rs/zerolog"
)

// CronJobInfo holds the information needed to manage a cron job on a node.
type CronJobInfo struct {
	ID               string
	TenantID         string
	WebrootName      string
	Name             string
	Schedule         string
	Command          string
	WorkingDirectory string
	TimeoutSeconds   int
	MaxMemoryMB      int
}

// CronManager manages systemd timer units for tenant cron jobs.
type CronManager struct {
	logger        zerolog.Logger
	webStorageDir string
	unitDir       string
}

// NewCronManager creates a new CronManager.
func NewCronManager(logger zerolog.Logger, cfg Config) *CronManager {
	return &CronManager{
		logger:        logger.With().Str("component", "cron-manager").Logger(),
		webStorageDir: cfg.WebStorageDir,
		unitDir:       "/etc/systemd/system",
	}
}

func (m *CronManager) timerName(info *CronJobInfo) string {
	return fmt.Sprintf("cron-%s-%s", info.TenantID, info.ID)
}

func (m *CronManager) servicePath(info *CronJobInfo) string {
	return filepath.Join(m.unitDir, m.timerName(info)+".service")
}

func (m *CronManager) timerPath(info *CronJobInfo) string {
	return filepath.Join(m.unitDir, m.timerName(info)+".timer")
}

func (m *CronManager) workDir(info *CronJobInfo) string {
	base := filepath.Join(m.webStorageDir, info.TenantID, "webroots", info.WebrootName)
	if info.WorkingDirectory != "" {
		return filepath.Join(base, info.WorkingDirectory)
	}
	return base
}

// CreateUnits writes the timer and service unit files and runs daemon-reload.
func (m *CronManager) CreateUnits(ctx context.Context, info *CronJobInfo) error {
	m.logger.Info().
		Str("cron_job", info.ID).
		Str("tenant", info.TenantID).
		Str("name", info.Name).
		Msg("creating cron job units")

	calendar, err := cronToSystemdCalendar(info.Schedule)
	if err != nil {
		return fmt.Errorf("invalid cron schedule %q: %w", info.Schedule, err)
	}

	// Write service unit.
	serviceContent, err := m.renderService(info)
	if err != nil {
		return fmt.Errorf("render service unit: %w", err)
	}
	if err := os.WriteFile(m.servicePath(info), []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("write service unit: %w", err)
	}

	// Write timer unit.
	timerContent, err := m.renderTimer(info, calendar)
	if err != nil {
		return fmt.Errorf("render timer unit: %w", err)
	}
	if err := os.WriteFile(m.timerPath(info), []byte(timerContent), 0644); err != nil {
		return fmt.Errorf("write timer unit: %w", err)
	}

	// Daemon reload.
	return m.daemonReload(ctx)
}

// UpdateUnits rewrites the unit files and reloads.
func (m *CronManager) UpdateUnits(ctx context.Context, info *CronJobInfo) error {
	return m.CreateUnits(ctx, info) // Idempotent — same as create.
}

// DeleteUnits stops the timer, removes unit files, and reloads.
func (m *CronManager) DeleteUnits(ctx context.Context, info *CronJobInfo) error {
	name := m.timerName(info)
	m.logger.Info().Str("unit", name).Msg("deleting cron job units")

	// Stop and disable timer (ignore errors — may not be running).
	_ = exec.CommandContext(ctx, "systemctl", "stop", name+".timer").Run()
	_ = exec.CommandContext(ctx, "systemctl", "disable", name+".timer").Run()

	// Remove unit files.
	os.Remove(m.servicePath(info))
	os.Remove(m.timerPath(info))

	return m.daemonReload(ctx)
}

// EnableTimer starts and enables the timer.
func (m *CronManager) EnableTimer(ctx context.Context, info *CronJobInfo) error {
	name := m.timerName(info)
	m.logger.Info().Str("unit", name).Msg("enabling cron timer")

	cmd := exec.CommandContext(ctx, "systemctl", "enable", "--now", name+".timer")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("enable timer %s: %s: %w", name, string(output), err)
	}
	return nil
}

// DisableTimer stops and disables the timer.
func (m *CronManager) DisableTimer(ctx context.Context, info *CronJobInfo) error {
	name := m.timerName(info)
	m.logger.Info().Str("unit", name).Msg("disabling cron timer")

	cmd := exec.CommandContext(ctx, "systemctl", "disable", "--now", name+".timer")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("disable timer %s: %s: %w", name, string(output), err)
	}
	return nil
}

func (m *CronManager) daemonReload(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "systemctl", "daemon-reload")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("daemon-reload: %s: %w", string(output), err)
	}
	return nil
}

var serviceTemplate = template.Must(template.New("service").Parse(`[Unit]
Description=Cron job: {{ .Name }} for tenant {{ .TenantID }}
After=network.target

[Service]
Type=oneshot
User={{ .TenantID }}
Group={{ .TenantID }}
WorkingDirectory={{ .WorkDir }}
ExecStart=/bin/bash -c {{ .Command }}
TimeoutStopSec={{ .TimeoutSeconds }}
MemoryMax={{ .MaxMemoryMB }}M
CPUQuota=100%
StandardOutput=journal
StandardError=journal
SyslogIdentifier=cron-{{ .TenantID }}-{{ .ID }}
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths={{ .WebrootPath }}
PrivateTmp=yes
`))

var timerTemplate = template.Must(template.New("timer").Parse(`[Unit]
Description=Timer for cron job: {{ .Name }} for tenant {{ .TenantID }}

[Timer]
OnCalendar={{ .Calendar }}
Persistent=true
RandomizedDelaySec=15

[Install]
WantedBy=timers.target
`))

type serviceData struct {
	CronJobInfo
	WorkDir     string
	WebrootPath string
}

type timerData struct {
	CronJobInfo
	Calendar string
}

func (m *CronManager) renderService(info *CronJobInfo) (string, error) {
	data := serviceData{
		CronJobInfo: *info,
		WorkDir:     m.workDir(info),
		WebrootPath: filepath.Join(m.webStorageDir, info.TenantID, "webroots", info.WebrootName),
	}
	var buf strings.Builder
	if err := serviceTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (m *CronManager) renderTimer(info *CronJobInfo, calendar string) (string, error) {
	data := timerData{
		CronJobInfo: *info,
		Calendar:    calendar,
	}
	var buf strings.Builder
	if err := timerTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// cronToSystemdCalendar converts a 5-field cron expression to systemd OnCalendar format.
func cronToSystemdCalendar(cron string) (string, error) {
	fields := strings.Fields(cron)
	if len(fields) != 5 {
		return "", fmt.Errorf("expected 5 fields, got %d", len(fields))
	}

	minute, hour, dom, month, dow := fields[0], fields[1], fields[2], fields[3], fields[4]

	// Convert day-of-week from cron (0-7, Sun=0 or 7) to systemd format.
	var dowPart string
	if dow != "*" {
		dowMap := map[string]string{
			"0": "Sun", "1": "Mon", "2": "Tue", "3": "Wed",
			"4": "Thu", "5": "Fri", "6": "Sat", "7": "Sun",
		}
		if mapped, ok := dowMap[dow]; ok {
			dowPart = mapped + " "
		} else {
			dowPart = dow + " "
		}
	}

	// Convert step expressions (*/N) to systemd format (0/N).
	convertStep := func(field string) string {
		if strings.HasPrefix(field, "*/") {
			return "0/" + field[2:]
		}
		return field
	}

	monthPart := month
	domPart := dom
	hourPart := convertStep(hour)
	minutePart := convertStep(minute)

	calendar := fmt.Sprintf("%s*-%s-%s %s:%s:00", dowPart, monthPart, domPart, hourPart, minutePart)
	return calendar, nil
}
