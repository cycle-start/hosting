package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
)

// SSHManager manages OpenSSH per-tenant configuration files for chrooted SSH/SFTP access.
type SSHManager struct {
	logger       zerolog.Logger
	configDir    string // /etc/ssh/sshd_config.d
	webStorageDir string
}

// NewSSHManager creates a new SSHManager.
func NewSSHManager(logger zerolog.Logger, configDir, webStorageDir string) *SSHManager {
	return &SSHManager{
		logger:       logger.With().Str("component", "ssh-manager").Logger(),
		configDir:    configDir,
		webStorageDir: webStorageDir,
	}
}

// SyncConfig writes the per-tenant sshd config and reloads sshd.
//
// Depending on the tenant's settings:
//   - SSHEnabled: full SSH access with ChrootDirectory
//   - SFTPEnabled (only): SFTP-only access with ForceCommand internal-sftp
//   - Neither: deny the user entirely
func (m *SSHManager) SyncConfig(ctx context.Context, info *TenantInfo) error {
	name := info.Name

	m.logger.Info().
		Str("tenant", name).
		Bool("ssh_enabled", info.SSHEnabled).
		Bool("sftp_enabled", info.SFTPEnabled).
		Msg("syncing SSH config")

	if err := os.MkdirAll(m.configDir, 0755); err != nil {
		return fmt.Errorf("create ssh config dir: %w", err)
	}

	confPath := filepath.Join(m.configDir, fmt.Sprintf("tenant-%s.conf", name))
	chrootDir := filepath.Join(m.webStorageDir, name)

	var config string
	switch {
	case info.SSHEnabled:
		// Full SSH access with chroot.
		config = fmt.Sprintf(`Match User %s
    ChrootDirectory %s
    AllowTcpForwarding no
    X11Forwarding no
`, name, chrootDir)

	case info.SFTPEnabled:
		// SFTP-only access with chroot.
		config = fmt.Sprintf(`Match User %s
    ChrootDirectory %s
    ForceCommand internal-sftp
    AllowTcpForwarding no
    X11Forwarding no
`, name, chrootDir)

	default:
		// Neither SSH nor SFTP — deny the user.
		config = fmt.Sprintf(`Match User %s
    DenyUsers %s
`, name, name)
	}

	m.logger.Debug().
		Str("tenant", name).
		Str("path", confPath).
		Msg("writing sshd per-tenant config")

	if err := os.WriteFile(confPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("write ssh config for %s: %w", name, err)
	}

	// Set up chroot bind mounts for full SSH (not needed for SFTP-only).
	if info.SSHEnabled {
		if err := m.setupChrootBindMounts(ctx, name, chrootDir); err != nil {
			return fmt.Errorf("setup chroot bind mounts for %s: %w", name, err)
		}
	}

	return m.reloadSSHD(ctx)
}

// RemoveConfig removes the per-tenant sshd config and reloads sshd.
func (m *SSHManager) RemoveConfig(ctx context.Context, name string) error {
	confPath := filepath.Join(m.configDir, fmt.Sprintf("tenant-%s.conf", name))

	m.logger.Info().
		Str("tenant", name).
		Str("path", confPath).
		Msg("removing SSH config")

	if err := os.Remove(confPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove ssh config for %s: %w", name, err)
	}

	return m.reloadSSHD(ctx)
}

// setupChrootBindMounts creates read-only bind mounts for system binaries
// inside the tenant's chroot directory, enabling full SSH shell access.
func (m *SSHManager) setupChrootBindMounts(ctx context.Context, name, chrootDir string) error {
	// Directories to bind-mount read-only into the chroot.
	bindDirs := []string{"/bin", "/lib", "/lib64", "/usr"}

	for _, dir := range bindDirs {
		// Skip if the source doesn't exist (e.g., /lib64 on some systems).
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		target := filepath.Join(chrootDir, dir)
		if err := os.MkdirAll(target, 0755); err != nil {
			return fmt.Errorf("mkdir bind mount target %s: %w", target, err)
		}

		// Check if already mounted.
		if m.isMounted(target) {
			continue
		}

		cmd := exec.CommandContext(ctx, "mount", "--bind", "-o", "ro", dir, target)
		m.logger.Debug().Strs("cmd", cmd.Args).Msg("bind mounting")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("mount --bind %s %s: %s: %w", dir, target, string(output), err)
		}
	}

	// Create minimal /dev inside the chroot.
	devDir := filepath.Join(chrootDir, "dev")
	if err := os.MkdirAll(devDir, 0755); err != nil {
		return fmt.Errorf("mkdir dev: %w", err)
	}

	// Create /dev/null and /dev/zero via mknod if they don't exist.
	devNodes := []struct {
		name  string
		major int
		minor int
	}{
		{"null", 1, 3},
		{"zero", 1, 5},
		{"urandom", 1, 9},
	}

	for _, dn := range devNodes {
		path := filepath.Join(devDir, dn.name)
		if _, err := os.Stat(path); err == nil {
			continue // Already exists.
		}
		cmd := exec.CommandContext(ctx, "mknod", path, "c",
			fmt.Sprintf("%d", dn.major), fmt.Sprintf("%d", dn.minor))
		m.logger.Debug().Strs("cmd", cmd.Args).Msg("creating dev node")
		if output, err := cmd.CombinedOutput(); err != nil {
			m.logger.Warn().Str("path", path).Str("output", string(output)).Err(err).Msg("mknod failed, continuing")
		} else {
			_ = os.Chmod(path, 0666)
		}
	}

	// Create minimal /etc inside the chroot (for shell utilities).
	etcDir := filepath.Join(chrootDir, "etc")
	if err := os.MkdirAll(etcDir, 0755); err != nil {
		return fmt.Errorf("mkdir etc: %w", err)
	}

	// Copy /etc/passwd and /etc/group with just the tenant's entry.
	passwdPath := filepath.Join(etcDir, "passwd")
	if _, err := os.Stat(passwdPath); os.IsNotExist(err) {
		content := fmt.Sprintf("root:x:0:0:root:/root:/bin/bash\n%s:x:%d:%d::%s:/bin/bash\n",
			name, int(0), int(0), "/home") // uid/gid will be resolved by NSS, these are placeholders
		_ = os.WriteFile(passwdPath, []byte(content), 0644)
	}

	return nil
}

// isMounted checks if a path is a mount point by reading /proc/mounts.
func (m *SSHManager) isMounted(target string) bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == target {
			return true
		}
	}
	return false
}

// reloadSSHD reloads the OpenSSH server to pick up config changes.
func (m *SSHManager) reloadSSHD(ctx context.Context) error {
	m.logger.Info().Msg("reloading sshd")

	cmd := exec.CommandContext(ctx, "systemctl", "reload", "sshd")
	m.logger.Debug().Strs("cmd", cmd.Args).Msg("executing systemctl reload sshd")
	if output, err := cmd.CombinedOutput(); err != nil {
		// sshd might not be running in dev/Docker — log a warning instead of failing.
		m.logger.Warn().Str("output", string(output)).Err(err).Msg("sshd reload failed (may not be running)")
	}

	return nil
}
