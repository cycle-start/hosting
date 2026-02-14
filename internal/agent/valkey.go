package agent

import (
	"bufio"
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

	"github.com/edvin/hosting/internal/agent/runtime"
)

// ValkeyManager handles Valkey instance and user operations via valkey-cli and systemd.
type ValkeyManager struct {
	logger    zerolog.Logger
	configDir string
	dataDir   string
	svcMgr    runtime.ServiceManager
}

// NewValkeyManager creates a new ValkeyManager.
func NewValkeyManager(logger zerolog.Logger, cfg Config, svcMgr runtime.ServiceManager) *ValkeyManager {
	return &ValkeyManager{
		logger:    logger.With().Str("component", "valkey-manager").Logger(),
		configDir: cfg.ValkeyConfigDir,
		dataDir:   cfg.ValkeyDataDir,
		svcMgr:    svcMgr,
	}
}

// configPath returns the path to a Valkey instance config file.
func (m *ValkeyManager) configPath(name string) string {
	return filepath.Join(m.configDir, fmt.Sprintf("%s.conf", name))
}

// readInstancePassword reads the requirepass from a Valkey instance config file.
func (m *ValkeyManager) readInstancePassword(name string) (string, error) {
	f, err := os.Open(m.configPath(name))
	if err != nil {
		return "", fmt.Errorf("open config for %s: %w", name, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "requirepass ") {
			return strings.TrimPrefix(line, "requirepass "), nil
		}
	}
	return "", fmt.Errorf("requirepass not found in config for %s", name)
}

// execValkeyCLI runs a valkey-cli command against the specified port with auth.
func (m *ValkeyManager) execValkeyCLI(ctx context.Context, port int, password string, valkeyArgs ...string) (string, error) {
	args := []string{"-p", fmt.Sprintf("%d", port), "-a", password, "--no-auth-warning"}
	args = append(args, valkeyArgs...)
	cmd := exec.CommandContext(ctx, "valkey-cli", args...)
	m.logger.Debug().Int("port", port).Strs("args", valkeyArgs).Msg("executing valkey-cli command")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", status.Errorf(codes.Internal, "valkey-cli failed: %s: %v", string(output), err)
	}
	return strings.TrimSpace(string(output)), nil
}

// CreateInstance provisions a new Valkey instance with config and systemd unit.
// This method is idempotent: if the instance already exists, its config is
// converged and a running instance is updated via CONFIG SET.
func (m *ValkeyManager) CreateInstance(ctx context.Context, name string, port int, password string, maxMemoryMB int) error {
	if err := validateName(name); err != nil {
		return err
	}

	m.logger.Info().Str("instance", name).Int("port", port).Msg("creating valkey instance")

	dataPath := filepath.Join(m.dataDir, name)
	config := fmt.Sprintf(`port %d
bind 0.0.0.0
protected-mode yes
requirepass %s
maxmemory %dmb
maxmemory-policy allkeys-lru
dir %s
dbfilename dump.rdb
appendonly yes
appendfilename "appendonly.aof"
`, port, password, maxMemoryMB, dataPath)

	// Check if instance already exists (idempotency on retry).
	if existingPassword, err := m.readInstancePassword(name); err == nil {
		m.logger.Info().Str("instance", name).Msg("instance already exists, converging config")

		// Rewrite config file to desired state.
		if err := os.WriteFile(m.configPath(name), []byte(config), 0640); err != nil {
			return status.Errorf(codes.Internal, "rewrite config: %v", err)
		}

		// Try to update running instance via CLI.
		if _, pingErr := m.execValkeyCLI(ctx, port, existingPassword, "PING"); pingErr == nil {
			// Instance is running — update config live.
			if _, err := m.execValkeyCLI(ctx, port, existingPassword, "CONFIG", "SET", "maxmemory", fmt.Sprintf("%dmb", maxMemoryMB)); err != nil {
				m.logger.Warn().Err(err).Msg("CONFIG SET maxmemory failed")
			}
			if existingPassword != password {
				if _, err := m.execValkeyCLI(ctx, port, existingPassword, "CONFIG", "SET", "requirepass", password); err != nil {
					m.logger.Warn().Err(err).Msg("CONFIG SET requirepass failed")
				}
			}
			return nil
		}

		// Instance config exists but process not running — start it.
		m.logger.Info().Str("instance", name).Msg("instance not running, starting")
		cmd := exec.CommandContext(ctx, "valkey-server", m.configPath(name), "--daemonize", "yes")
		if output, err := cmd.CombinedOutput(); err != nil {
			return status.Errorf(codes.Internal, "valkey-server restart: %s: %v", string(output), err)
		}

		serviceName := fmt.Sprintf("valkey@%s.service", name)
		if err := m.svcMgr.Start(ctx, serviceName); err != nil {
			m.logger.Warn().Err(err).Str("service", serviceName).Msg("systemd enable failed")
		}
		return nil
	}

	// New instance — create from scratch.
	if err := os.MkdirAll(dataPath, 0750); err != nil {
		return status.Errorf(codes.Internal, "create data dir: %v", err)
	}

	if err := os.WriteFile(m.configPath(name), []byte(config), 0640); err != nil {
		return status.Errorf(codes.Internal, "write config: %v", err)
	}

	// Start valkey-server with the config file.
	cmd := exec.CommandContext(ctx, "valkey-server", m.configPath(name), "--daemonize", "yes")
	if output, err := cmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "valkey-server start: %s: %v", string(output), err)
	}

	// In systemd environments, also enable the unit for auto-start on boot.
	serviceName := fmt.Sprintf("valkey@%s.service", name)
	if err := m.svcMgr.Start(ctx, serviceName); err != nil {
		m.logger.Warn().Err(err).Str("service", serviceName).Msg("systemd enable failed (expected in Docker)")
	}

	return nil
}

// DeleteInstance stops and removes a Valkey instance.
func (m *ValkeyManager) DeleteInstance(ctx context.Context, name string, port int) error {
	if err := validateName(name); err != nil {
		return err
	}

	m.logger.Info().Str("instance", name).Int("port", port).Msg("deleting valkey instance")

	// Stop the valkey instance via CLI shutdown (graceful).
	instancePassword, err := m.readInstancePassword(name)
	if err == nil {
		if _, shutdownErr := m.execValkeyCLI(ctx, port, instancePassword, "SHUTDOWN", "NOSAVE"); shutdownErr != nil {
			m.logger.Warn().Err(shutdownErr).Msg("valkey SHUTDOWN failed, continuing cleanup")
		}
	} else {
		m.logger.Warn().Err(err).Msg("could not read instance password for shutdown, continuing cleanup")
	}

	// Also try service manager stop for systemd environments.
	serviceName := fmt.Sprintf("valkey@%s.service", name)
	if stopErr := m.svcMgr.Stop(ctx, serviceName); stopErr != nil {
		m.logger.Warn().Err(stopErr).Msg("service manager stop failed, continuing cleanup")
	}

	// Remove config file.
	if err := os.Remove(m.configPath(name)); err != nil && !os.IsNotExist(err) {
		return status.Errorf(codes.Internal, "remove config: %v", err)
	}

	// Remove data directory.
	dataPath := filepath.Join(m.dataDir, name)
	if err := os.RemoveAll(dataPath); err != nil {
		return status.Errorf(codes.Internal, "remove data dir: %v", err)
	}

	return nil
}

// DumpData triggers a Valkey BGSAVE and copies the RDB file to the dump path.
func (m *ValkeyManager) DumpData(ctx context.Context, name string, port int, password, dumpPath string) error {
	if err := validateName(name); err != nil {
		return err
	}

	m.logger.Info().Str("instance", name).Int("port", port).Str("path", dumpPath).Msg("dumping valkey data")

	// Create parent directory.
	if err := os.MkdirAll(filepath.Dir(dumpPath), 0750); err != nil {
		return status.Errorf(codes.Internal, "create dump directory: %v", err)
	}

	// Record LASTSAVE before BGSAVE.
	beforeSave, err := m.execValkeyCLI(ctx, port, password, "LASTSAVE")
	if err != nil {
		return fmt.Errorf("LASTSAVE before: %w", err)
	}
	beforeTS, err := strconv.ParseInt(beforeSave, 10, 64)
	if err != nil {
		return fmt.Errorf("parse LASTSAVE timestamp %q: %w", beforeSave, err)
	}

	// Trigger BGSAVE.
	if _, err := m.execValkeyCLI(ctx, port, password, "BGSAVE"); err != nil {
		return fmt.Errorf("BGSAVE: %w", err)
	}

	// Poll LASTSAVE until the timestamp changes, meaning BGSAVE completed.
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}

		afterSave, err := m.execValkeyCLI(ctx, port, password, "LASTSAVE")
		if err != nil {
			return fmt.Errorf("LASTSAVE poll: %w", err)
		}
		afterTS, err := strconv.ParseInt(afterSave, 10, 64)
		if err != nil {
			return fmt.Errorf("parse LASTSAVE timestamp %q: %w", afterSave, err)
		}
		if afterTS > beforeTS {
			break
		}
	}

	// Copy the RDB file to the dump path.
	dataPath := filepath.Join(m.dataDir, name)
	rdbPath := filepath.Join(dataPath, "dump.rdb")
	cmd := exec.CommandContext(ctx, "cp", rdbPath, dumpPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "copy RDB: %s: %v", string(output), err)
	}

	return nil
}

// ImportData stops the Valkey instance, replaces the RDB file, and restarts it.
func (m *ValkeyManager) ImportData(ctx context.Context, name string, port int, dumpPath string) error {
	if err := validateName(name); err != nil {
		return err
	}

	m.logger.Info().Str("instance", name).Int("port", port).Str("path", dumpPath).Msg("importing valkey data")

	// Stop the instance.
	serviceName := fmt.Sprintf("valkey@%s.service", name)
	instancePassword, err := m.readInstancePassword(name)
	if err == nil {
		if _, shutdownErr := m.execValkeyCLI(ctx, port, instancePassword, "SHUTDOWN", "NOSAVE"); shutdownErr != nil {
			m.logger.Warn().Err(shutdownErr).Msg("valkey SHUTDOWN failed, trying service manager")
		}
	}
	if stopErr := m.svcMgr.Stop(ctx, serviceName); stopErr != nil {
		m.logger.Warn().Err(stopErr).Msg("service manager stop failed during import")
	}

	// Wait briefly for process to exit.
	time.Sleep(1 * time.Second)

	// Replace the RDB file.
	dataPath := filepath.Join(m.dataDir, name)
	rdbPath := filepath.Join(dataPath, "dump.rdb")
	cmd := exec.CommandContext(ctx, "cp", dumpPath, rdbPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "copy RDB: %s: %v", string(output), err)
	}

	// Remove the AOF so Valkey loads from the RDB on startup.
	aofPath := filepath.Join(dataPath, "appendonly.aof")
	if err := os.Remove(aofPath); err != nil && !os.IsNotExist(err) {
		m.logger.Warn().Err(err).Str("path", aofPath).Msg("remove AOF failed")
	}
	// Also remove any appendonlydir if present.
	aofDir := filepath.Join(dataPath, "appendonlydir")
	if err := os.RemoveAll(aofDir); err != nil {
		m.logger.Warn().Err(err).Str("path", aofDir).Msg("remove AOF dir failed")
	}

	// Restart the instance.
	startCmd := exec.CommandContext(ctx, "valkey-server", m.configPath(name), "--daemonize", "yes")
	if output, err := startCmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "valkey-server restart: %s: %v", string(output), err)
	}

	if err := m.svcMgr.Start(ctx, serviceName); err != nil {
		m.logger.Warn().Err(err).Str("service", serviceName).Msg("systemd start failed after import")
	}

	return nil
}

// CreateUser creates a Valkey ACL user via valkey-cli.
func (m *ValkeyManager) CreateUser(ctx context.Context, instanceName string, port int, username, password string, privileges []string, keyPattern string) error {
	if err := validateName(username); err != nil {
		return err
	}

	m.logger.Info().Str("instance", instanceName).Str("username", username).Msg("creating valkey user")

	instancePassword, err := m.readInstancePassword(instanceName)
	if err != nil {
		return status.Errorf(codes.Internal, "read instance password: %v", err)
	}

	// Build ACL SETUSER command: ACL SETUSER username on >password ~keyPattern +@priv1 +@priv2
	aclArgs := []string{"ACL", "SETUSER", username, "on", ">" + password}
	if keyPattern != "" {
		aclArgs = append(aclArgs, keyPattern)
	} else {
		aclArgs = append(aclArgs, "~*")
	}
	aclArgs = append(aclArgs, privileges...)

	if _, err := m.execValkeyCLI(ctx, port, instancePassword, aclArgs...); err != nil {
		return err
	}

	// Persist ACL changes.
	if _, err := m.execValkeyCLI(ctx, port, instancePassword, "ACL", "SAVE"); err != nil {
		m.logger.Warn().Err(err).Msg("ACL SAVE failed")
	}

	return nil
}

// UpdateUser updates a Valkey ACL user by deleting and recreating.
func (m *ValkeyManager) UpdateUser(ctx context.Context, instanceName string, port int, username, password string, privileges []string, keyPattern string) error {
	if err := validateName(username); err != nil {
		return err
	}

	m.logger.Info().Str("instance", instanceName).Str("username", username).Msg("updating valkey user")

	instancePassword, err := m.readInstancePassword(instanceName)
	if err != nil {
		return status.Errorf(codes.Internal, "read instance password: %v", err)
	}

	// Delete existing user first.
	if _, err := m.execValkeyCLI(ctx, port, instancePassword, "ACL", "DELUSER", username); err != nil {
		m.logger.Warn().Err(err).Str("username", username).Msg("ACL DELUSER failed, continuing with create")
	}

	// Recreate user with new settings.
	aclArgs := []string{"ACL", "SETUSER", username, "on", ">" + password}
	if keyPattern != "" {
		aclArgs = append(aclArgs, keyPattern)
	} else {
		aclArgs = append(aclArgs, "~*")
	}
	aclArgs = append(aclArgs, privileges...)

	if _, err := m.execValkeyCLI(ctx, port, instancePassword, aclArgs...); err != nil {
		return err
	}

	// Persist ACL changes.
	if _, err := m.execValkeyCLI(ctx, port, instancePassword, "ACL", "SAVE"); err != nil {
		m.logger.Warn().Err(err).Msg("ACL SAVE failed")
	}

	return nil
}

// DeleteUser deletes a Valkey ACL user via valkey-cli.
func (m *ValkeyManager) DeleteUser(ctx context.Context, instanceName string, port int, username string) error {
	if err := validateName(username); err != nil {
		return err
	}

	m.logger.Info().Str("instance", instanceName).Str("username", username).Msg("deleting valkey user")

	instancePassword, err := m.readInstancePassword(instanceName)
	if err != nil {
		return status.Errorf(codes.Internal, "read instance password: %v", err)
	}

	if _, err := m.execValkeyCLI(ctx, port, instancePassword, "ACL", "DELUSER", username); err != nil {
		return err
	}

	// Persist ACL changes.
	if _, err := m.execValkeyCLI(ctx, port, instancePassword, "ACL", "SAVE"); err != nil {
		m.logger.Warn().Err(err).Msg("ACL SAVE failed")
	}

	return nil
}
