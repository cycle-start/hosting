package agent

import (
	"os"
	"syscall"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// cephFSMagic is the filesystem magic number for CephFS.
const cephFSMagic = 0x00C36400

// CheckMount verifies that the given path is a mounted CephFS filesystem.
// Returns nil if the path is a valid CephFS mount, or a gRPC Unavailable error otherwise.
// This is called before any mutating webroot/tenant operation to guard against
// operating on an unmounted or wrong filesystem. It is also used as a startup
// readiness gate for web node agents.
// Set CEPHFS_ENABLED=false to bypass (for deployments without CephFS).
func CheckMount(path string) error {
	if os.Getenv("CEPHFS_ENABLED") == "false" {
		return nil
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return status.Errorf(codes.Unavailable,
			"CephFS not mounted at %s: %v", path, err)
	}
	if stat.Type != cephFSMagic {
		return status.Errorf(codes.Unavailable,
			"unexpected filesystem at %s: type=0x%X (expected CephFS 0x%X)",
			path, stat.Type, cephFSMagic)
	}
	return nil
}
