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

	"github.com/edvin/hosting/internal/agent/runtime"
)

// WebrootManager handles webroot directory creation and cleanup on CephFS.
type WebrootManager struct {
	logger        zerolog.Logger
	webStorageDir string
}

// NewWebrootManager creates a new WebrootManager.
func NewWebrootManager(logger zerolog.Logger, cfg Config) *WebrootManager {
	return &WebrootManager{
		logger:        logger.With().Str("component", "webroot-manager").Logger(),
		webStorageDir: cfg.WebStorageDir,
	}
}

// storagePath returns the on-disk storage path for a webroot on CephFS.
func (m *WebrootManager) storagePath(tenantName, webrootName string) string {
	return filepath.Join(m.webStorageDir, tenantName, "webroots", webrootName)
}

// Create provisions a new webroot directory on CephFS.
func (m *WebrootManager) Create(ctx context.Context, info *runtime.WebrootInfo) error {
	tenantName := info.TenantName
	webrootName := info.Name
	publicFolder := info.PublicFolder

	storageDir := m.storagePath(tenantName, webrootName)

	m.logger.Info().
		Str("tenant", tenantName).
		Str("webroot", webrootName).
		Str("storage", storageDir).
		Msg("creating webroot")

	// Create the storage directory.
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return status.Errorf(codes.Internal, "mkdir storage %s: %v", storageDir, err)
	}

	// Create the public folder inside storage if specified.
	if publicFolder != "" {
		publicDir := filepath.Join(storageDir, publicFolder)
		if err := os.MkdirAll(publicDir, 0755); err != nil {
			return status.Errorf(codes.Internal, "mkdir public folder %s: %v", publicDir, err)
		}
	}

	// Set ownership of the storage directory to the tenant user.
	chownCmd := exec.CommandContext(ctx, "chown", "-R", fmt.Sprintf("%s:%s", tenantName, tenantName), storageDir)
	m.logger.Debug().Strs("cmd", chownCmd.Args).Msg("executing chown on webroot storage")
	if output, err := chownCmd.CombinedOutput(); err != nil {
		return status.Errorf(codes.Internal, "chown storage %s: %s: %v", storageDir, string(output), err)
	}

	return nil
}

// Update ensures the webroot directories are in the expected state.
func (m *WebrootManager) Update(ctx context.Context, info *runtime.WebrootInfo) error {
	tenantName := info.TenantName
	webrootName := info.Name
	publicFolder := info.PublicFolder

	storageDir := m.storagePath(tenantName, webrootName)

	m.logger.Info().
		Str("tenant", tenantName).
		Str("webroot", webrootName).
		Str("public_folder", publicFolder).
		Msg("updating webroot")

	// Ensure the storage directory exists.
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return status.Errorf(codes.Internal, "mkdir storage %s: %v", storageDir, err)
	}

	// Ensure the public folder exists if specified.
	if publicFolder != "" {
		publicDir := filepath.Join(storageDir, publicFolder)
		if err := os.MkdirAll(publicDir, 0755); err != nil {
			return status.Errorf(codes.Internal, "mkdir public folder %s: %v", publicDir, err)
		}
	}

	return nil
}

// Delete removes a webroot's storage directory.
func (m *WebrootManager) Delete(ctx context.Context, tenantName, webrootName string) error {
	storageDir := m.storagePath(tenantName, webrootName)

	m.logger.Info().
		Str("tenant", tenantName).
		Str("webroot", webrootName).
		Str("storage", storageDir).
		Msg("deleting webroot")

	// Validate the storage directory path before removing.
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
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	absStorage, err := filepath.Abs(m.webStorageDir)
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(absStorage, absPath)
	if err != nil {
		return false
	}

	if strings.Contains(rel, "..") {
		return false
	}

	// Must be at least three path components deep: {tenant}/webroots/{webroot}.
	parts := strings.Split(rel, string(filepath.Separator))
	return len(parts) >= 3
}
