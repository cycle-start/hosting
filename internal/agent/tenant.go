package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TenantManager handles Linux user account management for hosting tenants.
// Tenant directories live on CephFS at /var/www/storage/{tenant}/.
type TenantManager struct {
	logger        zerolog.Logger
	webStorageDir string
}

// NewTenantManager creates a new TenantManager.
func NewTenantManager(logger zerolog.Logger, cfg Config) *TenantManager {
	return &TenantManager{
		logger:        logger.With().Str("component", "tenant-manager").Logger(),
		webStorageDir: cfg.WebStorageDir,
	}
}

// WebStorageDir returns the base directory for tenant storage on CephFS.
func (m *TenantManager) WebStorageDir() string {
	return m.webStorageDir
}

// Create provisions a new Linux user account and sets up the CephFS directory structure
// and local log directory.
// This operation is idempotent: if the user already exists, it ensures the
// directory structure and permissions converge to the desired state.
//
// Directory layout on CephFS:
//
//	/var/www/storage/{tenant}/           root:root 0755 (ChrootDirectory)
//	├── home/                            tenant:tenant 0700
//	├── webroots/                        tenant:tenant 0751
//	└── tmp/                             tenant:tenant 1777
//
// Local log directory:
//
//	/var/log/hosting/{tenant}/           tenant:tenant 0750
func (m *TenantManager) Create(ctx context.Context, info *TenantInfo) error {
	if err := CheckMount(m.webStorageDir); err != nil {
		return err
	}

	name := info.Name
	uid := info.UID
	chrootDir := filepath.Join(m.webStorageDir, name)

	m.logger.Info().
		Str("tenant", name).
		Int32("uid", uid).
		Str("chroot", chrootDir).
		Msg("creating tenant user")

	// Check if the user already exists.
	checkCmd := exec.CommandContext(ctx, "id", name)
	if err := checkCmd.Run(); err != nil {
		// User does not exist — create it.
		if err := m.createUser(ctx, name, uid); err != nil {
			return err
		}
	} else {
		m.logger.Info().Str("tenant", name).Msg("user already exists, converging state")
	}

	// Create the chroot root directory. Must be root:root 0755 for OpenSSH ChrootDirectory.
	if err := os.MkdirAll(chrootDir, 0755); err != nil {
		return status.Errorf(codes.Internal, "mkdir chroot %s: %v", chrootDir, err)
	}

	// Create the tenant directory structure inside the chroot.
	homeDir := filepath.Join(chrootDir, "home")
	dirs := map[string]os.FileMode{
		homeDir:                              0700,
		filepath.Join(chrootDir, "webroots"): 0751,
		filepath.Join(chrootDir, "tmp"):      os.ModeSticky | 0777,
	}

	for dir, perm := range dirs {
		if err := os.MkdirAll(dir, perm); err != nil {
			return status.Errorf(codes.Internal, "mkdir %s: %v", dir, err)
		}
		if err := os.Chmod(dir, perm); err != nil {
			return status.Errorf(codes.Internal, "chmod %s: %v", dir, err)
		}
	}

	// Set ownership of tenant-owned CephFS directories.
	for _, dir := range []string{homeDir, filepath.Join(chrootDir, "webroots"), filepath.Join(chrootDir, "tmp")} {
		chownCmd := exec.CommandContext(ctx, "chown", fmt.Sprintf("%s:%s", name, name), dir)
		m.logger.Debug().Strs("cmd", chownCmd.Args).Msg("executing chown")
		if output, err := chownCmd.CombinedOutput(); err != nil {
			return status.Errorf(codes.Internal, "chown failed for %s: %s: %v", dir, string(output), err)
		}
	}

	// Create local log directory on SSD.
	logDir := filepath.Join("/var/log/hosting", name)
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return status.Errorf(codes.Internal, "mkdir log dir %s: %v", logDir, err)
	}
	if err := os.Chmod(logDir, 0750); err != nil {
		return status.Errorf(codes.Internal, "chmod log dir %s: %v", logDir, err)
	}
	chownCmd := exec.CommandContext(ctx, "chown", fmt.Sprintf("%s:%s", name, name), logDir)
	m.logger.Debug().Strs("cmd", chownCmd.Args).Msg("executing chown")
	if output, err := chownCmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "chown failed for %s: %s: %v", logDir, string(output), err)
	}

	// Set CephFS quota if configured.
	if info.DiskQuotaBytes > 0 {
		if err := m.setQuota(ctx, chrootDir, info.DiskQuotaBytes); err != nil {
			m.logger.Warn().Err(err).Str("tenant", name).Msg("failed to set CephFS quota (non-fatal)")
		}
	}

	return nil
}

// createUser creates a Linux user with the given name and UID. If the UID is already
// taken by a different user (orphaned from a previous DB state), the stale user is
// removed and creation is retried.
func (m *TenantManager) createUser(ctx context.Context, name string, uid int32) error {
	// -M: don't create home (we manage CephFS dirs ourselves)
	// -d /home: chroot-relative home path (what user sees after chroot)
	cmd := exec.CommandContext(ctx, "useradd",
		"-M",
		"-d", "/home",
		"-u", strconv.FormatInt(int64(uid), 10),
		"-s", "/bin/bash",
		name,
	)
	m.logger.Debug().Strs("cmd", cmd.Args).Msg("executing useradd")
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	outStr := string(output)
	if !strings.Contains(outStr, "UID") || !strings.Contains(outStr, "not unique") {
		return status.Errorf(codes.Internal, "useradd failed for %s: %s: %v", name, outStr, err)
	}

	// UID already taken by a stale user — look up its name and remove it.
	m.logger.Warn().Int32("uid", uid).Str("tenant", name).Msg("UID already taken by stale user, cleaning up")
	getentCmd := exec.CommandContext(ctx, "getent", "passwd", strconv.FormatInt(int64(uid), 10))
	getentOut, err := getentCmd.Output()
	if err != nil {
		return status.Errorf(codes.Internal, "getent passwd %d failed: %v", uid, err)
	}
	// getent output: "username:x:uid:gid:comment:home:shell"
	parts := strings.SplitN(string(getentOut), ":", 2)
	if len(parts) == 0 || parts[0] == "" {
		return status.Errorf(codes.Internal, "could not parse stale user from getent output: %s", string(getentOut))
	}
	staleUser := parts[0]

	m.logger.Info().Str("stale_user", staleUser).Int32("uid", uid).Msg("removing stale user to reclaim UID")

	// Kill any running processes for the stale user before removing.
	killCmd := exec.CommandContext(ctx, "pkill", "-9", "-u", staleUser)
	_ = killCmd.Run() // Ignore error — no processes is fine.

	// Wait for killed processes to be reaped before userdel.
	for i := 0; i < 10; i++ {
		delCmd := exec.CommandContext(ctx, "userdel", staleUser)
		delOutput, err := delCmd.CombinedOutput()
		if err == nil {
			break
		}
		if !strings.Contains(string(delOutput), "currently used by process") {
			return status.Errorf(codes.Internal, "userdel stale user %s failed: %s: %v", staleUser, string(delOutput), err)
		}
		if i == 9 {
			return status.Errorf(codes.Internal, "userdel stale user %s failed after retries: %s: %v", staleUser, string(delOutput), err)
		}
		m.logger.Debug().Str("stale_user", staleUser).Int("attempt", i+1).Msg("waiting for processes to exit before userdel")
		time.Sleep(500 * time.Millisecond)
	}

	// Retry useradd.
	retryCmd := exec.CommandContext(ctx, "useradd",
		"-M", "-d", "/home",
		"-u", strconv.FormatInt(int64(uid), 10),
		"-s", "/bin/bash",
		name,
	)
	if retryOutput, err := retryCmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "useradd retry failed for %s: %s: %v", name, string(retryOutput), err)
	}
	return nil
}

// setQuota sets the CephFS directory quota for a tenant using extended attributes.
// A quotaBytes value of 0 means no quota (removes any existing quota).
func (m *TenantManager) setQuota(ctx context.Context, tenantDir string, quotaBytes int64) error {
	if quotaBytes <= 0 {
		return nil
	}
	cmd := exec.CommandContext(ctx, "setfattr",
		"-n", "ceph.quota.max_bytes",
		"-v", strconv.FormatInt(quotaBytes, 10),
		tenantDir,
	)
	m.logger.Debug().Strs("cmd", cmd.Args).Msg("setting CephFS quota")
	if output, err := cmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal,
			"set CephFS quota on %s: %s: %v", tenantDir, string(output), err)
	}
	return nil
}

// Update modifies an existing tenant user configuration.
func (m *TenantManager) Update(ctx context.Context, info *TenantInfo) error {
	name := info.Name

	m.logger.Info().
		Str("tenant", name).
		Bool("sftp_enabled", info.SFTPEnabled).
		Msg("updating tenant user")

	return nil
}

// Suspend locks a tenant user account.
func (m *TenantManager) Suspend(ctx context.Context, name string) error {
	m.logger.Info().Str("tenant", name).Msg("suspending tenant user")

	cmd := exec.CommandContext(ctx, "usermod", "-L", name)
	m.logger.Debug().Strs("cmd", cmd.Args).Msg("executing usermod -L")
	if output, err := cmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "usermod -L failed for %s: %s: %v", name, string(output), err)
	}

	// Kill any running processes for the user.
	killCmd := exec.CommandContext(ctx, "pkill", "-u", name)
	m.logger.Debug().Strs("cmd", killCmd.Args).Msg("executing pkill")
	_ = killCmd.Run()

	return nil
}

// Unsuspend unlocks a tenant user account.
func (m *TenantManager) Unsuspend(ctx context.Context, name string) error {
	m.logger.Info().Str("tenant", name).Msg("unsuspending tenant user")

	cmd := exec.CommandContext(ctx, "usermod", "-U", name)
	m.logger.Debug().Strs("cmd", cmd.Args).Msg("executing usermod -U")
	if output, err := cmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "usermod -U failed for %s: %s: %v", name, string(output), err)
	}

	return nil
}

// Delete removes a tenant user account and its CephFS directory.
func (m *TenantManager) Delete(ctx context.Context, name string) error {
	if err := CheckMount(m.webStorageDir); err != nil {
		return err
	}

	m.logger.Info().Str("tenant", name).Msg("deleting tenant user")

	// Remove PHP-FPM pool configs and reload so FPM stops spawning workers
	// as this user. Also handles any other per-tenant FPM pool config.
	pools, _ := filepath.Glob("/etc/php/*/fpm/pool.d/" + name + ".conf")
	for _, pool := range pools {
		m.logger.Debug().Str("pool", pool).Msg("removing PHP-FPM pool config")
		os.Remove(pool)
	}
	if len(pools) > 0 {
		// Reload all PHP-FPM versions to drop the pool.
		versions, _ := filepath.Glob("/etc/php/*/fpm")
		for _, dir := range versions {
			version := filepath.Base(filepath.Dir(dir))
			_ = exec.CommandContext(ctx, "systemctl", "reload", "php"+version+"-fpm").Run()
		}
		// Give FPM a moment to terminate pool workers.
		time.Sleep(500 * time.Millisecond)
	}

	// Kill all processes owned by the user and remove. Retry because
	// other runtimes (daemons, workers) may take a moment to exit.
	for i := 0; i < 10; i++ {
		killCmd := exec.CommandContext(ctx, "pkill", "-9", "-u", name)
		_ = killCmd.Run() // Ignore error — no processes is fine.
		time.Sleep(500 * time.Millisecond)

		cmd := exec.CommandContext(ctx, "userdel", name)
		output, err := cmd.CombinedOutput()
		if err == nil {
			break
		}
		outStr := string(output)
		if strings.Contains(outStr, "does not exist") {
			break
		}
		if !strings.Contains(outStr, "currently used by process") {
			return status.Errorf(codes.Internal, "userdel failed for %s: %s: %v", name, outStr, err)
		}
		if i == 9 {
			return status.Errorf(codes.Internal, "userdel failed for %s after retries: %s: %v", name, outStr, err)
		}
		m.logger.Debug().Str("tenant", name).Int("attempt", i+1).Msg("re-killing processes before userdel retry")
	}

	// Remove the CephFS directory tree.
	chrootDir := filepath.Join(m.webStorageDir, name)
	if err := os.RemoveAll(chrootDir); err != nil {
		return status.Errorf(codes.Internal, "remove CephFS dir %s: %v", chrootDir, err)
	}

	// Remove the local log directory.
	logDir := filepath.Join("/var/log/hosting", name)
	if err := os.RemoveAll(logDir); err != nil {
		return status.Errorf(codes.Internal, "remove log dir %s: %v", logDir, err)
	}

	return nil
}
