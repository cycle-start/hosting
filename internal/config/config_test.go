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
	os.Unsetenv("POWERDNS_DATABASE_URL")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "localhost:7233", cfg.TemporalAddress)
	assert.Equal(t, ":8090", cfg.HTTPListenAddr)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "", cfg.MySQLDSN)
	assert.Equal(t, "", cfg.PowerDNSDatabaseURL)
}

func TestLoad_AllEnvVars(t *testing.T) {
	t.Setenv("CORE_DATABASE_URL", "postgres://core:5432/coredb")
	t.Setenv("POWERDNS_DATABASE_URL", "postgres://svc:5432/svcdb")
	t.Setenv("TEMPORAL_ADDRESS", "temporal.example.com:7233")
	t.Setenv("HTTP_LISTEN_ADDR", ":7071")
	t.Setenv("MYSQL_DSN", "root:pass@tcp(localhost:3306)/")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "postgres://core:5432/coredb", cfg.CoreDatabaseURL)
	assert.Equal(t, "postgres://svc:5432/svcdb", cfg.PowerDNSDatabaseURL)
	assert.Equal(t, "temporal.example.com:7233", cfg.TemporalAddress)
	assert.Equal(t, ":7071", cfg.HTTPListenAddr)
	assert.Equal(t, "root:pass@tcp(localhost:3306)/", cfg.MySQLDSN)
	assert.Equal(t, "debug", cfg.LogLevel)
}

func TestValidate_CoreAPI_MissingFields(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate("core-api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CORE_DATABASE_URL")
	assert.Contains(t, err.Error(), "TEMPORAL_ADDRESS")
	assert.Contains(t, err.Error(), "HTTP_LISTEN_ADDR")
}

func TestValidate_Worker_MissingFields(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate("worker")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CORE_DATABASE_URL")
	assert.Contains(t, err.Error(), "POWERDNS_DATABASE_URL")
	assert.Contains(t, err.Error(), "TEMPORAL_ADDRESS")
}

func TestValidate_NodeAgent_MissingFields(t *testing.T) {
	cfg := &Config{}
	err := cfg.Validate("node-agent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NODE_ID")
	assert.Contains(t, err.Error(), "TEMPORAL_ADDRESS")
}

func TestValidate_TLS_MismatchedCertKey(t *testing.T) {
	cfg := &Config{
		CoreDatabaseURL: "postgres://localhost/db",
		TemporalAddress: "localhost:7233",
		HTTPListenAddr:  ":8090",
		TemporalTLSCert: "/path/to/cert.pem",
	}
	err := cfg.Validate("core-api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TEMPORAL_TLS_CERT and TEMPORAL_TLS_KEY must both be set")
}

func TestValidate_AllPresent(t *testing.T) {
	cfg := &Config{
		CoreDatabaseURL:    "postgres://localhost/db",
		PowerDNSDatabaseURL: "postgres://localhost/svc",
		TemporalAddress:    "localhost:7233",
		HTTPListenAddr:     ":8090",
		NodeID:             "node-1",
		TemporalTLSCert:    "/path/to/cert.pem",
		TemporalTLSKey:     "/path/to/key.pem",
	}

	assert.NoError(t, cfg.Validate("core-api"))
	assert.NoError(t, cfg.Validate("worker"))
	assert.NoError(t, cfg.Validate("node-agent"))
}
