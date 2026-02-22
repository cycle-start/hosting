package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
)

const (
	groupSSH      = "hosting-ssh"
	groupSFTP     = "hosting-sftp"
	groupNoAccess = "hosting-noaccess"
)

// SSHManager manages SSH/SFTP access for tenants via Linux group membership.
// Instead of per-tenant sshd config files, tenants are added to one of three groups:
//   - hosting-ssh: full shell access with chroot
//   - hosting-sftp: SFTP-only access with chroot
//   - hosting-noaccess: explicitly denied
//
// sshd's Match Group directives (deployed by Ansible) handle the rest.
// Group membership is resolved by NSS at connection time — no sshd reload needed.
type SSHManager struct {
	logger        zerolog.Logger
	webStorageDir string
}

// NewSSHManager creates a new SSHManager.
func NewSSHManager(logger zerolog.Logger, webStorageDir string) *SSHManager {
	return &SSHManager{
		logger:        logger.With().Str("component", "ssh-manager").Logger(),
		webStorageDir: webStorageDir,
	}
}

// SyncConfig sets group membership for the tenant based on SSH/SFTP flags
// and sets up chroot bind mounts for full SSH access.
func (m *SSHManager) SyncConfig(ctx context.Context, info *TenantInfo) error {
	name := info.Name

	m.logger.Info().
		Str("tenant", name).
		Bool("ssh_enabled", info.SSHEnabled).
		Bool("sftp_enabled", info.SFTPEnabled).
		Msg("syncing SSH config")

	// Remove from all groups first, then add to the correct one.
	for _, group := range []string{groupSSH, groupSFTP, groupNoAccess} {
		m.removeFromGroup(ctx, name, group)
	}

	switch {
	case info.SSHEnabled:
		if err := m.addToGroup(ctx, name, groupSSH); err != nil {
			return err
		}
	case info.SFTPEnabled:
		if err := m.addToGroup(ctx, name, groupSFTP); err != nil {
			return err
		}
	default:
		if err := m.addToGroup(ctx, name, groupNoAccess); err != nil {
			return err
		}
	}

	// Set up chroot bind mounts for full SSH (not needed for SFTP-only).
	if info.SSHEnabled {
		chrootDir := filepath.Join(m.webStorageDir, name)
		if err := m.setupChrootBindMounts(ctx, name, chrootDir); err != nil {
			return fmt.Errorf("setup chroot bind mounts for %s: %w", name, err)
		}
	}

	return nil
}

// RemoveConfig removes the tenant from all SSH groups and cleans up legacy config.
func (m *SSHManager) RemoveConfig(ctx context.Context, name string) error {
	m.logger.Info().
		Str("tenant", name).
		Msg("removing SSH config")

	for _, group := range []string{groupSSH, groupSFTP, groupNoAccess} {
		m.removeFromGroup(ctx, name, group)
	}

	return nil
}

// addToGroup adds a user to a Linux group.
func (m *SSHManager) addToGroup(ctx context.Context, user, group string) error {
	cmd := exec.CommandContext(ctx, "gpasswd", "-a", user, group)
	m.logger.Debug().Strs("cmd", cmd.Args).Msg("adding user to group")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("add %s to group %s: %s: %w", user, group, string(output), err)
	}
	return nil
}

// removeFromGroup removes a user from a Linux group. Errors are ignored
// (the user may not be in the group).
func (m *SSHManager) removeFromGroup(ctx context.Context, user, group string) {
	cmd := exec.CommandContext(ctx, "gpasswd", "-d", user, group)
	_ = cmd.Run()
}

// loadMountState reads /proc/mounts once and returns a set of mounted paths.
func (m *SSHManager) loadMountState() map[string]bool {
	mounts := make(map[string]bool)
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		m.logger.Warn().Err(err).Msg("failed to read /proc/mounts")
		return mounts
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			mounts[fields[1]] = true
		}
	}
	return mounts
}

// setupChrootBindMounts creates read-only bind mounts for system binaries
// inside the tenant's chroot directory, enabling full SSH shell access.
func (m *SSHManager) setupChrootBindMounts(ctx context.Context, name, chrootDir string) error {
	mounted := m.loadMountState()

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

		if mounted[target] {
			continue
		}

		cmd := exec.CommandContext(ctx, "mount", "--bind", "-o", "ro", dir, target)
		m.logger.Debug().Strs("cmd", cmd.Args).Msg("bind mounting")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("mount --bind %s %s: %s: %w", dir, target, string(output), err)
		}
		mounted[target] = true
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
		{"ptmx", 5, 2},
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

	// Bind-mount host /dev/pts for PTY allocation. OpenSSH's privileged monitor
	// allocates PTYs from the host namespace, so the chroot must share it.
	ptsDir := filepath.Join(devDir, "pts")
	if err := os.MkdirAll(ptsDir, 0755); err != nil {
		return fmt.Errorf("mkdir dev/pts: %w", err)
	}
	if !mounted[ptsDir] {
		cmd := exec.CommandContext(ctx, "mount", "--bind", "/dev/pts", ptsDir)
		m.logger.Debug().Strs("cmd", cmd.Args).Msg("bind mounting /dev/pts")
		if output, err := cmd.CombinedOutput(); err != nil {
			m.logger.Warn().Str("output", string(output)).Err(err).Msg("/dev/pts bind mount failed")
		} else {
			mounted[ptsDir] = true
		}
	}

	// Mount /proc with hidepid=2 so tenants can only see their own processes.
	procDir := filepath.Join(chrootDir, "proc")
	if err := os.MkdirAll(procDir, 0755); err != nil {
		return fmt.Errorf("mkdir proc: %w", err)
	}
	if !mounted[procDir] {
		cmd := exec.CommandContext(ctx, "mount", "-t", "proc", "proc", procDir,
			"-o", "hidepid=2")
		m.logger.Debug().Strs("cmd", cmd.Args).Msg("mounting proc")
		if output, err := cmd.CombinedOutput(); err != nil {
			m.logger.Warn().Str("output", string(output)).Err(err).Msg("proc mount failed")
		} else {
			mounted[procDir] = true
		}
	}

	// Symlink ~/webroots -> /webroots so tenant sees webroots in home dir.
	homeWebroots := filepath.Join(chrootDir, "home", "webroots")
	if _, err := os.Lstat(homeWebroots); os.IsNotExist(err) {
		_ = os.Symlink("/webroots", homeWebroots)
	}

	// Create minimal /etc inside the chroot (for shell utilities).
	etcDir := filepath.Join(chrootDir, "etc")
	if err := os.MkdirAll(etcDir, 0755); err != nil {
		return fmt.Errorf("mkdir etc: %w", err)
	}

	// Bind-mount /etc subdirs that tools need (read-only).
	etcBindDirs := []string{"alternatives", "php"}
	for _, sub := range etcBindDirs {
		src := filepath.Join("/etc", sub)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		target := filepath.Join(etcDir, sub)
		if err := os.MkdirAll(target, 0755); err != nil {
			return fmt.Errorf("mkdir etc/%s: %w", sub, err)
		}
		if !mounted[target] {
			cmd := exec.CommandContext(ctx, "mount", "--bind", "-o", "ro", src, target)
			m.logger.Debug().Strs("cmd", cmd.Args).Msg("bind mounting")
			if output, err := cmd.CombinedOutput(); err != nil {
				m.logger.Warn().Str("output", string(output)).Err(err).Msg("bind mount failed")
			} else {
				mounted[target] = true
			}
		}
	}

	// Write /etc/passwd with the tenant's real UID/GID so that
	// hidepid=2 on /proc filters correctly and shell tools (whoami, ps) work.
	uid, gid := 0, 0
	if u, err := user.Lookup(name); err == nil {
		uid, _ = strconv.Atoi(u.Uid)
		gid, _ = strconv.Atoi(u.Gid)
	}
	passwdContent := fmt.Sprintf("root:x:0:0:root:/root:/bin/bash\n%s:x:%d:%d::%s:/bin/bash\n",
		name, uid, gid, "/home")
	_ = os.WriteFile(filepath.Join(etcDir, "passwd"), []byte(passwdContent), 0644)

	groupContent := fmt.Sprintf("root:x:0:\n%s:x:%d:\n", name, gid)
	_ = os.WriteFile(filepath.Join(etcDir, "group"), []byte(groupContent), 0644)

	// Set up direnv so env vars auto-load when cd-ing into a webroot directory.
	// The direnv binary is accessible via the /usr bind mount.
	direnvHook := "eval \"$(direnv hook bash)\"\n"
	_ = os.WriteFile(filepath.Join(etcDir, "bash.bashrc"), []byte(direnvHook), 0644)
	_ = os.WriteFile(filepath.Join(etcDir, "profile"), []byte(direnvHook), 0644)

	// Whitelist /webroots so .envrc files are auto-trusted without `direnv allow`.
	// direnv reads config from $XDG_CONFIG_HOME/direnv/direnv.toml which defaults
	// to ~/.config/direnv/direnv.toml. Inside the chroot, HOME=/home.
	direnvConfDir := filepath.Join(chrootDir, "home", ".config", "direnv")
	if err := os.MkdirAll(direnvConfDir, 0755); err != nil {
		return fmt.Errorf("mkdir home/.config/direnv: %w", err)
	}
	direnvToml := "[global]\nhide_env_diff = true\n\n[whitelist]\nprefix = [\"/webroots\"]\n"
	_ = os.WriteFile(filepath.Join(direnvConfDir, "direnv.toml"), []byte(direnvToml), 0644)
	// Chown the .config tree to the tenant so direnv can read it.
	_ = chownRecursive(filepath.Join(chrootDir, "home", ".config"), uid, gid)

	return nil
}

// chownRecursive changes ownership of a directory tree.
func chownRecursive(root string, uid, gid int) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, uid, gid)
	})
}
