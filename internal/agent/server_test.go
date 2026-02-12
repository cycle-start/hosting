package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agentv1 "github.com/edvin/hosting/proto/agent/v1"
)

func TestHealthCheck(t *testing.T) {
	srv := NewServer(zerolog.Nop(), Config{})

	resp, err := srv.HealthCheck(context.Background(), &agentv1.HealthCheckRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Healthy)
	assert.Equal(t, "node agent is running", resp.Message)
}

func TestHealthCheck_NilRequest(t *testing.T) {
	srv := NewServer(zerolog.Nop(), Config{})

	resp, err := srv.HealthCheck(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Healthy)
}

func TestNewServer_InitializesRuntimes(t *testing.T) {
	srv := NewServer(zerolog.Nop(), Config{})

	// Verify all expected runtimes are registered.
	expectedRuntimes := []string{"php", "node", "python", "ruby", "static"}
	for _, rt := range expectedRuntimes {
		_, ok := srv.runtimes[rt]
		assert.True(t, ok, "runtime %q should be registered", rt)
	}

	// Verify no extra runtimes are registered.
	assert.Len(t, srv.runtimes, len(expectedRuntimes))
}

func TestNewServer_InitializesManagers(t *testing.T) {
	cfg := Config{
		MySQLDSN:       "root:pass@tcp(localhost:3306)/db",
		NginxConfigDir: "/tmp/nginx",
		WebStorageDir:  "/var/www/storage",
		HomeBaseDir:    "/home",
		CertDir:        "/etc/ssl/certs",
	}
	srv := NewServer(zerolog.Nop(), cfg)

	assert.NotNil(t, srv.tenant)
	assert.NotNil(t, srv.webroot)
	assert.NotNil(t, srv.nginx)
	assert.NotNil(t, srv.database)
}

func TestConfigureRuntime_UnsupportedRuntime(t *testing.T) {
	srv := NewServer(zerolog.Nop(), Config{})

	req := &agentv1.ConfigureRuntimeRequest{
		Webroot: &agentv1.WebrootInfo{
			TenantName: "tenant1",
			Name:       "mysite",
			Runtime:    "unsupported_runtime",
		},
	}

	_, err := srv.ConfigureRuntime(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported runtime")
}

func TestCreateWebroot_UnsupportedRuntime(t *testing.T) {
	// CreateWebroot calls webroot.Create first, which runs chown via exec.
	// Since chown requires root, this test would fail at the OS level.
	// We only test that creating with an unsupported runtime is rejected
	// when webroot.Create happens to succeed (e.g., in an integration test).
	// For unit tests, we test ConfigureRuntime_UnsupportedRuntime instead.
	t.Skip("requires root privileges for chown in webroot.Create")
}

func TestUpdateWebroot_UnsupportedRuntime(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		WebStorageDir:  tmpDir,
		HomeBaseDir:    tmpDir,
		NginxConfigDir: tmpDir,
	}
	srv := NewServer(zerolog.Nop(), cfg)

	// Pre-create the webroots directory so Update can create the symlink.
	webrootsDir := filepath.Join(tmpDir, "tenant1", "webroots")
	require.NoError(t, os.MkdirAll(webrootsDir, 0750))

	req := &agentv1.UpdateWebrootRequest{
		Webroot: &agentv1.WebrootInfo{
			TenantName: "tenant1",
			Name:       "mysite",
			Runtime:    "perl", // Not a supported runtime.
		},
	}

	_, err := srv.UpdateWebroot(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported runtime")
}
