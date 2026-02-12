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

	agentv1 "github.com/edvin/hosting/proto/agent/v1"
)

// TenantManager handles Linux user account management for hosting tenants.
type TenantManager struct {
	logger      zerolog.Logger
	homeBaseDir string
}

// NewTenantManager creates a new TenantManager.
func NewTenantManager(logger zerolog.Logger, cfg Config) *TenantManager {
	return &TenantManager{
		logger:      logger.With().Str("component", "tenant-manager").Logger(),
		homeBaseDir: cfg.HomeBaseDir,
	}
}

// HomeBaseDir returns the base directory for tenant home directories.
func (m *TenantManager) HomeBaseDir() string {
	return m.homeBaseDir
}

// Create provisions a new Linux user account and sets up the directory structure.
// This operation is idempotent: if the user already exists, it ensures the
// directory structure and permissions converge to the desired state.
func (m *TenantManager) Create(ctx context.Context, info *agentv1.TenantInfo) error {
	name := info.GetName()
	uid := info.GetUid()
	homeDir := filepath.Join(m.homeBaseDir, name)

	m.logger.Info().
		Str("tenant", name).
		Int32("uid", uid).
		Str("home", homeDir).
		Msg("creating tenant user")

	// Check if the user already exists.
	checkCmd := exec.CommandContext(ctx, "id", name)
	if err := checkCmd.Run(); err != nil {
		// User does not exist — create it.
		cmd := exec.CommandContext(ctx, "useradd",
			"-m",
			"-d", homeDir,
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

	// Create the standard directory structure.
	dirs := []string{
		filepath.Join(homeDir, "webroots"),
		filepath.Join(homeDir, "logs"),
		filepath.Join(homeDir, "tmp"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return status.Errorf(codes.Internal, "mkdir %s: %v", dir, err)
		}
	}

	// Set ownership of the home directory tree to the tenant user.
	chownCmd := exec.CommandContext(ctx, "chown", "-R", fmt.Sprintf("%s:%s", name, name), homeDir)
	m.logger.Debug().Strs("cmd", chownCmd.Args).Msg("executing chown")
	if output, err := chownCmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "chown failed for %s: %s: %v", homeDir, string(output), err)
	}

	// Set home directory permissions.
	chmodCmd := exec.CommandContext(ctx, "chmod", "750", homeDir)
	m.logger.Debug().Strs("cmd", chmodCmd.Args).Msg("executing chmod")
	if output, err := chmodCmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "chmod failed for %s: %s: %v", homeDir, string(output), err)
	}

	// Configure SFTP access if enabled.
	if info.GetSftpEnabled() {
		if err := m.configureSFTP(ctx, name, homeDir); err != nil {
			return err
		}
	}

	return nil
}

// Update modifies an existing tenant user configuration.
func (m *TenantManager) Update(ctx context.Context, info *agentv1.TenantInfo) error {
	name := info.GetName()

	m.logger.Info().
		Str("tenant", name).
		Bool("sftp_enabled", info.GetSftpEnabled()).
		Msg("updating tenant user")

	// Toggle shell based on SFTP configuration.
	shell := "/bin/bash"
	if info.GetSftpEnabled() {
		shell = "/usr/sbin/nologin"
	}

	cmd := exec.CommandContext(ctx, "usermod", "-s", shell, name)
	m.logger.Debug().Strs("cmd", cmd.Args).Msg("executing usermod")
	if output, err := cmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "usermod shell failed for %s: %s: %v", name, string(output), err)
	}

	// Ensure .ssh directory exists for SFTP users.
	if info.GetSftpEnabled() {
		homeDir := filepath.Join(m.homeBaseDir, name)
		if err := m.configureSFTP(ctx, name, homeDir); err != nil {
			return err
		}
	}

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

	// Kill any running processes for the user. Ignore errors since
	// there may not be any running processes.
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

// Delete removes a tenant user account and its home directory.
func (m *TenantManager) Delete(ctx context.Context, name string) error {
	m.logger.Info().Str("tenant", name).Msg("deleting tenant user")

	// Kill all processes owned by the user. Ignore errors since
	// the user may have no running processes.
	killCmd := exec.CommandContext(ctx, "pkill", "-u", name)
	m.logger.Debug().Strs("cmd", killCmd.Args).Msg("executing pkill")
	_ = killCmd.Run()

	// Remove the user and their home directory.
	cmd := exec.CommandContext(ctx, "userdel", "-r", name)
	m.logger.Debug().Strs("cmd", cmd.Args).Msg("executing userdel -r")
	if output, err := cmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "userdel failed for %s: %s: %v", name, string(output), err)
	}

	return nil
}

// configureSFTP sets up the .ssh directory for SFTP access.
func (m *TenantManager) configureSFTP(ctx context.Context, name, homeDir string) error {
	sshDir := filepath.Join(homeDir, ".ssh")

	m.logger.Debug().Str("tenant", name).Str("ssh_dir", sshDir).Msg("configuring SFTP access")

	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return status.Errorf(codes.Internal, "mkdir .ssh for %s: %v", name, err)
	}

	// Create authorized_keys if it does not exist.
	authKeysPath := filepath.Join(sshDir, "authorized_keys")
	if _, err := os.Stat(authKeysPath); os.IsNotExist(err) {
		if err := os.WriteFile(authKeysPath, []byte(""), 0600); err != nil {
			return status.Errorf(codes.Internal, "create authorized_keys for %s: %v", name, err)
		}
	}

	// Set ownership of the .ssh directory.
	chownCmd := exec.CommandContext(ctx, "chown", "-R", fmt.Sprintf("%s:%s", name, name), sshDir)
	m.logger.Debug().Strs("cmd", chownCmd.Args).Msg("executing chown on .ssh")
	if output, err := chownCmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "chown .ssh failed for %s: %s: %v", name, string(output), err)
	}

	return nil
}
