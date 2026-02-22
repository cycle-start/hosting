package agent

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaemonConfigPath(t *testing.T) {
	mgr := NewDaemonManager(zerolog.Nop(), Config{WebStorageDir: "/var/www/storage"})

	info := &DaemonInfo{
		TenantName: "tabc1234567",
		Name:       "dxyz7891234",
	}

	path := mgr.configPath(info)
	assert.Equal(t, "/etc/supervisor/conf.d/daemon-tabc1234567-dxyz7891234.conf", path)
}

func TestDaemonProgramName(t *testing.T) {
	mgr := NewDaemonManager(zerolog.Nop(), Config{WebStorageDir: "/var/www/storage"})

	info := &DaemonInfo{
		TenantName: "tabc1234567",
		Name:       "dxyz7891234",
	}

	name := mgr.programName(info)
	assert.Equal(t, "daemon-tabc1234567-dxyz7891234", name)
}

func TestFormatDaemonEnvironment_Empty(t *testing.T) {
	result := formatDaemonEnvironment(map[string]string{})
	assert.Equal(t, "", result)
}

func TestFormatDaemonEnvironment_SingleVar(t *testing.T) {
	result := formatDaemonEnvironment(map[string]string{"PORT": "8080"})
	assert.Equal(t, `PORT="8080"`, result)
}

func TestFormatDaemonEnvironment_MultipleVars_Sorted(t *testing.T) {
	result := formatDaemonEnvironment(map[string]string{
		"PORT":    "8080",
		"APP_ENV": "production",
		"DEBUG":   "false",
	})
	assert.Equal(t, `APP_ENV="production",DEBUG="false",PORT="8080"`, result)
}

func TestDaemonConfigTemplate_BasicNoProxy(t *testing.T) {
	data := daemonConfigData{
		TenantName:   "tabc1234567",
		WebrootName:  "main",
		DaemonName:   "dxyz7891234",
		Command:      "php artisan queue:work",
		WorkingDir:   "/var/www/storage/tabc1234567/webroots/main",
		NumProcs:     1,
		StopSignal:   "TERM",
		StopWaitSecs: 30,
		MaxMemoryMB:  256,
		Environment:  `APP_ENV="production"`,
	}

	var b bytes.Buffer
	require.NoError(t, daemonConfigTmpl.Execute(&b, data))
	config := b.String()

	assert.Contains(t, config, "[program:daemon-tabc1234567-dxyz7891234]")
	assert.Contains(t, config, "command=php artisan queue:work")
	assert.Contains(t, config, "directory=/var/www/storage/tabc1234567/webroots/main")
	assert.Contains(t, config, "user=tabc1234567")
	assert.Contains(t, config, "numprocs=1")
	assert.Contains(t, config, "stopsignal=TERM")
	assert.Contains(t, config, "stopwaitsecs=30")
	assert.Contains(t, config, `environment=APP_ENV="production"`)
	assert.Contains(t, config, "autostart=true")
	assert.Contains(t, config, "autorestart=unexpected")
	assert.Contains(t, config, "stdout_logfile=/var/www/storage/tabc1234567/logs/daemon-dxyz7891234.log")
	assert.Contains(t, config, "stderr_logfile=/var/www/storage/tabc1234567/logs/daemon-dxyz7891234.error.log")
}

func TestDaemonConfigTemplate_WithPort(t *testing.T) {
	env := map[string]string{"APP_ENV": "production", "PORT": "14523"}

	data := daemonConfigData{
		TenantName:   "tabc1234567",
		WebrootName:  "main",
		DaemonName:   "dws123test00",
		Command:      "php artisan reverb:start --port=$PORT",
		WorkingDir:   "/var/www/storage/tabc1234567/webroots/main",
		NumProcs:     1,
		StopSignal:   "TERM",
		StopWaitSecs: 30,
		MaxMemoryMB:  256,
		Environment:  formatDaemonEnvironment(env),
	}

	var b bytes.Buffer
	require.NoError(t, daemonConfigTmpl.Execute(&b, data))
	config := b.String()

	assert.Contains(t, config, `PORT="14523"`)
	assert.Contains(t, config, `APP_ENV="production"`)
}

func TestDaemonConfigTemplate_WithPortAndHost(t *testing.T) {
	env := map[string]string{
		"APP_ENV": "production",
		"HOST":    "fd00:1:2::2742",
		"PORT":    "14523",
	}

	data := daemonConfigData{
		TenantName:   "tabc1234567",
		WebrootName:  "main",
		DaemonName:   "dws123test00",
		Command:      "php artisan reverb:start --host=$HOST --port=$PORT",
		WorkingDir:   "/var/www/storage/tabc1234567/webroots/main",
		NumProcs:     1,
		StopSignal:   "TERM",
		StopWaitSecs: 30,
		MaxMemoryMB:  256,
		Environment:  formatDaemonEnvironment(env),
	}

	var b bytes.Buffer
	require.NoError(t, daemonConfigTmpl.Execute(&b, data))
	config := b.String()

	assert.Contains(t, config, `HOST="fd00:1:2::2742"`)
	assert.Contains(t, config, `PORT="14523"`)
	assert.Contains(t, config, `APP_ENV="production"`)
}

func TestDaemonConfigTemplate_MultiProc(t *testing.T) {
	data := daemonConfigData{
		TenantName:   "tabc1234567",
		WebrootName:  "main",
		DaemonName:   "dworker12345",
		Command:      "php artisan queue:work",
		WorkingDir:   "/var/www/storage/tabc1234567/webroots/main",
		NumProcs:     4,
		StopSignal:   "QUIT",
		StopWaitSecs: 60,
		MaxMemoryMB:  128,
	}

	var b bytes.Buffer
	require.NoError(t, daemonConfigTmpl.Execute(&b, data))
	config := b.String()

	assert.Contains(t, config, "numprocs=4")
	assert.Contains(t, config, "process_name=%(program_name)s_%(process_num)02d")
	assert.Contains(t, config, "stopsignal=QUIT")
	assert.Contains(t, config, "stopwaitsecs=60")
	// No environment line when env is empty.
	assert.NotContains(t, config, "environment=")
}

func TestDaemonConfigTemplate_NoEnvironment(t *testing.T) {
	data := daemonConfigData{
		TenantName:   "tabc1234567",
		WebrootName:  "main",
		DaemonName:   "dbg123456789",
		Command:      "node worker.js",
		WorkingDir:   "/var/www/storage/tabc1234567/webroots/main",
		NumProcs:     1,
		StopSignal:   "TERM",
		StopWaitSecs: 10,
		MaxMemoryMB:  256,
		Environment:  "",
	}

	var b bytes.Buffer
	require.NoError(t, daemonConfigTmpl.Execute(&b, data))
	config := b.String()

	assert.NotContains(t, config, "environment=")
}

func TestCleanOrphanedDaemonConfigs_RemovesOrphaned(t *testing.T) {
	confDir := t.TempDir()

	// Create test files: one expected, one orphaned, one non-daemon.
	require.NoError(t, os.WriteFile(filepath.Join(confDir, "daemon-tenant1-worker.conf"), []byte("ok"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(confDir, "daemon-tenant2-old.conf"), []byte("stale"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(confDir, "hosting.conf"), []byte("base config"), 0644))

	expected := map[string]bool{"daemon-tenant1-worker.conf": true}

	// Simulate the cleanup logic (since the real method hardcodes /etc/supervisor/conf.d).
	entries, err := os.ReadDir(confDir)
	require.NoError(t, err)

	var removed []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) < 7 || name[:7] != "daemon-" || filepath.Ext(name) != ".conf" {
			continue
		}
		if expected[name] {
			continue
		}
		require.NoError(t, os.Remove(filepath.Join(confDir, name)))
		removed = append(removed, name)
	}

	assert.Equal(t, []string{"daemon-tenant2-old.conf"}, removed)
	assert.True(t, fileExists(filepath.Join(confDir, "daemon-tenant1-worker.conf")))
	assert.True(t, fileExists(filepath.Join(confDir, "hosting.conf")))
	assert.False(t, fileExists(filepath.Join(confDir, "daemon-tenant2-old.conf")))
}

func TestReadEnvFile_Basic(t *testing.T) {
	dir := t.TempDir()
	content := "# comment\nAPP_ENV=\"production\"\nDEBUG=\"false\"\nSECRET=\"has\\\"quotes\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.hosting"), []byte(content), 0644))

	env := readEnvFile(dir, "")
	assert.Equal(t, "production", env["APP_ENV"])
	assert.Equal(t, "false", env["DEBUG"])
	assert.Equal(t, `has"quotes`, env["SECRET"])
}

func TestReadEnvFile_Missing(t *testing.T) {
	dir := t.TempDir()
	env := readEnvFile(dir, ".env.hosting")
	assert.Empty(t, env)
}

func TestReadEnvFile_CustomName(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.custom"), []byte("FOO=\"bar\"\n"), 0644))

	env := readEnvFile(dir, ".env.custom")
	assert.Equal(t, "bar", env["FOO"])
}

func TestReadEnvFile_UnquotedValues(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.hosting"), []byte("KEY=value\n"), 0644))

	env := readEnvFile(dir, "")
	assert.Equal(t, "value", env["KEY"])
}
