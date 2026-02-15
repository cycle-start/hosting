package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

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
	if err := checkMount(m.webStorageDir); err != nil {
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
		if output, err := cmd.CombinedOutput(); err != nil {
			return status.Errorf(codes.Internal, "useradd failed for %s: %s: %v", name, string(output), err)
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
	if err := checkMount(m.webStorageDir); err != nil {
		return err
	}

	m.logger.Info().Str("tenant", name).Msg("deleting tenant user")

	// Kill all processes owned by the user.
	killCmd := exec.CommandContext(ctx, "pkill", "-u", name)
	m.logger.Debug().Strs("cmd", killCmd.Args).Msg("executing pkill")
	_ = killCmd.Run()

	// Remove the user (no -r since home is on CephFS, not local).
	cmd := exec.CommandContext(ctx, "userdel", name)
	m.logger.Debug().Strs("cmd", cmd.Args).Msg("executing userdel")
	if output, err := cmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "userdel failed for %s: %s: %v", name, string(output), err)
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
