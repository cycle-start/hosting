package runtime

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/rs/zerolog"
)

// ServiceManager abstracts system service management so that runtime managers
// work identically regardless of the underlying init system.
//
// Production nodes (VMs / bare metal) use SystemdManager.
// Docker development nodes use DirectManager.
type ServiceManager interface {
	// DaemonReload tells the init system to re-scan service definitions.
	DaemonReload(ctx context.Context) error

	// Start enables and starts a service.
	Start(ctx context.Context, unit string) error

	// Stop disables and stops a service.
	Stop(ctx context.Context, unit string) error

	// Reload asks a service to re-read its configuration (e.g. HUP).
	Reload(ctx context.Context, unit string) error

	// Restart fully stops and starts a service.
	Restart(ctx context.Context, unit string) error

	// Signal sends a specific signal to a process matched by name.
	Signal(ctx context.Context, process, signal string) error
}

// ---------------------------------------------------------------------------
// SystemdManager — production (VMs / bare metal with systemd)
// ---------------------------------------------------------------------------

// SystemdManager implements ServiceManager using systemctl.
type SystemdManager struct {
	logger zerolog.Logger
}

// NewSystemdManager creates a ServiceManager backed by systemd.
func NewSystemdManager(logger zerolog.Logger) *SystemdManager {
	return &SystemdManager{logger: logger.With().Str("svc_mgr", "systemd").Logger()}
}

func (s *SystemdManager) DaemonReload(ctx context.Context) error {
	return sysctl(ctx, "daemon-reload")
}

func (s *SystemdManager) Start(ctx context.Context, unit string) error {
	return sysctl(ctx, "enable", "--now", unit)
}

func (s *SystemdManager) Stop(ctx context.Context, unit string) error {
	return sysctl(ctx, "disable", "--now", unit)
}

func (s *SystemdManager) Reload(ctx context.Context, unit string) error {
	return sysctl(ctx, "reload", unit)
}

func (s *SystemdManager) Restart(ctx context.Context, unit string) error {
	return sysctl(ctx, "restart", unit)
}

func (s *SystemdManager) Signal(ctx context.Context, process, signal string) error {
	return pkillSignal(ctx, process, signal)
}

// ---------------------------------------------------------------------------
// DirectManager — Docker development (no systemd)
// ---------------------------------------------------------------------------

// DirectManager implements ServiceManager using direct process signals.
// Operations that require systemd (DaemonReload, Start with enable) are
// best-effort no-ops that log warnings.
type DirectManager struct {
	logger zerolog.Logger
}

// NewDirectManager creates a ServiceManager for environments without systemd.
func NewDirectManager(logger zerolog.Logger) *DirectManager {
	return &DirectManager{logger: logger.With().Str("svc_mgr", "direct").Logger()}
}

func (d *DirectManager) DaemonReload(_ context.Context) error {
	d.logger.Debug().Msg("daemon-reload: no-op (no systemd)")
	return nil
}

func (d *DirectManager) Start(_ context.Context, unit string) error {
	d.logger.Warn().Str("unit", unit).Msg("start: no-op without systemd (process must be started by entrypoint)")
	return nil
}

func (d *DirectManager) Stop(ctx context.Context, unit string) error {
	d.logger.Debug().Str("unit", unit).Msg("stop: sending SIGTERM")
	if err := pkillSignal(ctx, unit, "TERM"); err != nil {
		d.logger.Warn().Str("unit", unit).Msg("could not stop process (may not be running)")
	}
	return nil
}

func (d *DirectManager) Reload(ctx context.Context, unit string) error {
	d.logger.Debug().Str("unit", unit).Msg("reload: sending SIGHUP")
	if err := pkillSignal(ctx, unit, "HUP"); err != nil {
		d.logger.Warn().Str("unit", unit).Msg("could not reload process (may not be running)")
	}
	return nil
}

func (d *DirectManager) Restart(ctx context.Context, unit string) error {
	_ = d.Stop(ctx, unit)
	return d.Start(ctx, unit)
}

func (d *DirectManager) Signal(ctx context.Context, process, signal string) error {
	if err := pkillSignal(ctx, process, signal); err != nil {
		d.logger.Warn().Str("process", process).Str("signal", signal).Msg("could not signal process (may not be running)")
	}
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func sysctl(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl %v: %s: %w", args, string(output), err)
	}
	return nil
}

func pkillSignal(ctx context.Context, process, signal string) error {
	cmd := exec.CommandContext(ctx, "pkill", "-"+signal, "-f", process)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pkill -%s %s: %s: %w", signal, process, string(output), err)
	}
	return nil
}
