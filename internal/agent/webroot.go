package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	agentv1 "github.com/edvin/hosting/proto/agent/v1"
)

// WebrootManager handles webroot directory creation, symlinking, and cleanup.
type WebrootManager struct {
	logger        zerolog.Logger
	webStorageDir string
	homeBaseDir   string
}

// NewWebrootManager creates a new WebrootManager.
func NewWebrootManager(logger zerolog.Logger, cfg Config) *WebrootManager {
	return &WebrootManager{
		logger:        logger.With().Str("component", "webroot-manager").Logger(),
		webStorageDir: cfg.WebStorageDir,
		homeBaseDir:   cfg.HomeBaseDir,
	}
}

// storagePath returns the on-disk storage path for a webroot.
func (m *WebrootManager) storagePath(tenantName, webrootName string) string {
	return filepath.Join(m.webStorageDir, tenantName, webrootName)
}

// symlinkPath returns the symlink path in the tenant's home directory.
func (m *WebrootManager) symlinkPath(tenantName, webrootName string) string {
	return filepath.Join(m.homeBaseDir, tenantName, "webroots", webrootName)
}

// Create provisions a new webroot: creates storage directory, public folder,
// and a symlink from the tenant home to the storage location.
func (m *WebrootManager) Create(ctx context.Context, info *agentv1.WebrootInfo) error {
	tenantName := info.GetTenantName()
	webrootName := info.GetName()
	publicFolder := info.GetPublicFolder()

	storageDir := m.storagePath(tenantName, webrootName)
	symlinkDir := m.symlinkPath(tenantName, webrootName)

	m.logger.Info().
		Str("tenant", tenantName).
		Str("webroot", webrootName).
		Str("storage", storageDir).
		Str("symlink", symlinkDir).
		Msg("creating webroot")

	// Create the storage directory.
	if err := os.MkdirAll(storageDir, 0750); err != nil {
		return status.Errorf(codes.Internal, "mkdir storage %s: %v", storageDir, err)
	}

	// Create the public folder inside storage if specified.
	if publicFolder != "" {
		publicDir := filepath.Join(storageDir, publicFolder)
		if err := os.MkdirAll(publicDir, 0750); err != nil {
			return status.Errorf(codes.Internal, "mkdir public folder %s: %v", publicDir, err)
		}
	}

	// Ensure the webroots directory exists in the tenant home.
	webrootsDir := filepath.Join(m.homeBaseDir, tenantName, "webroots")
	if err := os.MkdirAll(webrootsDir, 0750); err != nil {
		return status.Errorf(codes.Internal, "mkdir webroots dir %s: %v", webrootsDir, err)
	}

	// Create the symlink from the tenant home to storage.
	// Remove any existing symlink first.
	_ = os.Remove(symlinkDir)
	if err := os.Symlink(storageDir, symlinkDir); err != nil {
		return status.Errorf(codes.Internal, "symlink %s -> %s: %v", symlinkDir, storageDir, err)
	}

	// Set ownership of the storage directory to the tenant user.
	chownCmd := exec.CommandContext(ctx, "chown", "-R", fmt.Sprintf("%s:%s", tenantName, tenantName), storageDir)
	m.logger.Debug().Strs("cmd", chownCmd.Args).Msg("executing chown on webroot storage")
	if output, err := chownCmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "chown storage %s: %s: %v", storageDir, string(output), err)
	}

	// Set ownership of the symlink itself.
	lchownCmd := exec.CommandContext(ctx, "chown", "-h", fmt.Sprintf("%s:%s", tenantName, tenantName), symlinkDir)
	m.logger.Debug().Strs("cmd", lchownCmd.Args).Msg("executing chown on webroot symlink")
	if output, err := lchownCmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "chown symlink %s: %s: %v", symlinkDir, string(output), err)
	}

	return nil
}

// Update ensures the webroot directories are in the expected state.
func (m *WebrootManager) Update(ctx context.Context, info *agentv1.WebrootInfo) error {
	tenantName := info.GetTenantName()
	webrootName := info.GetName()
	publicFolder := info.GetPublicFolder()

	storageDir := m.storagePath(tenantName, webrootName)

	m.logger.Info().
		Str("tenant", tenantName).
		Str("webroot", webrootName).
		Str("public_folder", publicFolder).
		Msg("updating webroot")

	// Ensure the storage directory exists.
	if err := os.MkdirAll(storageDir, 0750); err != nil {
		return status.Errorf(codes.Internal, "mkdir storage %s: %v", storageDir, err)
	}

	// Ensure the public folder exists if specified.
	if publicFolder != "" {
		publicDir := filepath.Join(storageDir, publicFolder)
		if err := os.MkdirAll(publicDir, 0750); err != nil {
			return status.Errorf(codes.Internal, "mkdir public folder %s: %v", publicDir, err)
		}
	}

	// Ensure the symlink is correct.
	symlinkDir := m.symlinkPath(tenantName, webrootName)
	target, err := os.Readlink(symlinkDir)
	if err != nil || target != storageDir {
		_ = os.Remove(symlinkDir)
		if err := os.Symlink(storageDir, symlinkDir); err != nil {
			return status.Errorf(codes.Internal, "symlink %s -> %s: %v", symlinkDir, storageDir, err)
		}
	}

	return nil
}

// Delete removes a webroot's symlink and storage directory.
func (m *WebrootManager) Delete(ctx context.Context, tenantName, webrootName string) error {
	storageDir := m.storagePath(tenantName, webrootName)
	symlinkDir := m.symlinkPath(tenantName, webrootName)

	m.logger.Info().
		Str("tenant", tenantName).
		Str("webroot", webrootName).
		Str("storage", storageDir).
		Str("symlink", symlinkDir).
		Msg("deleting webroot")

	// Remove the symlink first.
	if err := os.Remove(symlinkDir); err != nil && !os.IsNotExist(err) {
		return status.Errorf(codes.Internal, "remove symlink %s: %v", symlinkDir, err)
	}

	// Validate the storage directory path before removing to prevent
	// accidental deletion of unrelated directories.
	if !m.isValidStoragePath(storageDir) {
		return status.Errorf(codes.Internal, "refusing to remove invalid storage path: %s", storageDir)
	}

	if err := os.RemoveAll(storageDir); err != nil {
		return status.Errorf(codes.Internal, "remove storage %s: %v", storageDir, err)
	}

	return nil
}

// isValidStoragePath checks that the path is a subdirectory of the web storage
// directory and contains no path traversal components.
func (m *WebrootManager) isValidStoragePath(path string) bool {
	// Resolve to an absolute path.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// The path must be strictly under webStorageDir.
	absStorage, err := filepath.Abs(m.webStorageDir)
	if err != nil {
		return false
	}

	// Ensure we are at least two levels deep (tenant/webroot).
	rel, err := filepath.Rel(absStorage, absPath)
	if err != nil {
		return false
	}

	// Reject path traversal.
	if strings.Contains(rel, "..") {
		return false
	}

	// Must be at least two path components deep: {tenant}/{webroot}.
	parts := strings.Split(rel, string(filepath.Separator))
	return len(parts) >= 2
}
