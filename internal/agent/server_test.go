package agent

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

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
		CertDir:        "/etc/ssl/certs",
	}
	srv := NewServer(zerolog.Nop(), cfg)

	assert.NotNil(t, srv.tenant)
	assert.NotNil(t, srv.webroot)
	assert.NotNil(t, srv.nginx)
	assert.NotNil(t, srv.database)
}
