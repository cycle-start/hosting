package agent

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func newTestWebrootManager(t *testing.T) *WebrootManager {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := Config{
		WebStorageDir: tmpDir + "/storage",
		HomeBaseDir:   tmpDir + "/home",
	}
	return NewWebrootManager(zerolog.Nop(), cfg)
}

func TestWebrootManager_StoragePath(t *testing.T) {
	mgr := newTestWebrootManager(t)

	path := mgr.storagePath("tenant1", "mysite")
	assert.Contains(t, path, "storage/tenant1/mysite")
}

func TestWebrootManager_SymlinkPath(t *testing.T) {
	mgr := newTestWebrootManager(t)

	path := mgr.symlinkPath("tenant1", "mysite")
	assert.Contains(t, path, "home/tenant1/webroots/mysite")
}

func TestWebrootManager_IsValidStoragePath_Valid(t *testing.T) {
	mgr := newTestWebrootManager(t)

	// Two levels deep should be valid.
	valid := mgr.isValidStoragePath(mgr.storagePath("tenant1", "mysite"))
	assert.True(t, valid)

	// More than two levels deep is also valid.
	valid = mgr.isValidStoragePath(mgr.storagePath("tenant1", "mysite") + "/subdir")
	assert.True(t, valid)
}

func TestWebrootManager_IsValidStoragePath_Invalid(t *testing.T) {
	mgr := newTestWebrootManager(t)

	tests := []struct {
		name string
		path string
	}{
		{"storage root only", mgr.webStorageDir},
		{"one level deep", mgr.webStorageDir + "/tenant1"},
		{"path traversal", mgr.webStorageDir + "/tenant1/../../../etc"},
		{"outside storage", "/etc/passwd"},
		{"relative path traversal", mgr.webStorageDir + "/tenant1/../../etc/passwd"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			valid := mgr.isValidStoragePath(tc.path)
			assert.False(t, valid, "path %q should be invalid", tc.path)
		})
	}
}

func TestWebrootManager_IsValidStoragePath_PathTraversal(t *testing.T) {
	mgr := newTestWebrootManager(t)

	// Attempt various path traversal attacks.
	attacks := []string{
		mgr.webStorageDir + "/../../../etc/passwd",
		mgr.webStorageDir + "/tenant/../../../etc",
		"/tmp/evil",
		"/",
		"",
	}

	for _, path := range attacks {
		t.Run(path, func(t *testing.T) {
			assert.False(t, mgr.isValidStoragePath(path), "path %q should be rejected", path)
		})
	}
}
