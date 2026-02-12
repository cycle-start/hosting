package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_EmptyCoreDBURL(t *testing.T) {
	// Config loads successfully even without CORE_DATABASE_URL set.
	os.Unsetenv("CORE_DATABASE_URL")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "", cfg.CoreDatabaseURL)
}

func TestLoad_WithCoreDBURL(t *testing.T) {
	t.Setenv("CORE_DATABASE_URL", "postgres://localhost:5432/core")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "postgres://localhost:5432/core", cfg.CoreDatabaseURL)
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("CORE_DATABASE_URL", "postgres://localhost/core")

	// Clear any env vars that might interfere with defaults.
	os.Unsetenv("TEMPORAL_ADDRESS")
	os.Unsetenv("HTTP_LISTEN_ADDR")
	os.Unsetenv("LOG_LEVEL")
	os.Unsetenv("MYSQL_DSN")
	os.Unsetenv("SERVICE_DATABASE_URL")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "localhost:7233", cfg.TemporalAddress)
	assert.Equal(t, ":8090", cfg.HTTPListenAddr)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "", cfg.MySQLDSN)
	assert.Equal(t, "", cfg.ServiceDatabaseURL)
}

func TestLoad_AllEnvVars(t *testing.T) {
	t.Setenv("CORE_DATABASE_URL", "postgres://core:5432/coredb")
	t.Setenv("SERVICE_DATABASE_URL", "postgres://svc:5432/svcdb")
	t.Setenv("TEMPORAL_ADDRESS", "temporal.example.com:7233")
	t.Setenv("HTTP_LISTEN_ADDR", ":7071")
	t.Setenv("MYSQL_DSN", "root:pass@tcp(localhost:3306)/")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "postgres://core:5432/coredb", cfg.CoreDatabaseURL)
	assert.Equal(t, "postgres://svc:5432/svcdb", cfg.ServiceDatabaseURL)
	assert.Equal(t, "temporal.example.com:7233", cfg.TemporalAddress)
	assert.Equal(t, ":7071", cfg.HTTPListenAddr)
	assert.Equal(t, "root:pass@tcp(localhost:3306)/", cfg.MySQLDSN)
	assert.Equal(t, "debug", cfg.LogLevel)
}
