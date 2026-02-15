package agent

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaemonConfigPath(t *testing.T) {
	mgr := NewDaemonManager(zerolog.Nop(), Config{WebStorageDir: "/var/www/storage"})

	info := &DaemonInfo{
		TenantName: "t_abc123",
		Name:       "daemon_xyz789",
	}

	path := mgr.configPath(info)
	assert.Equal(t, "/etc/supervisor/conf.d/daemon-t_abc123-daemon_xyz789.conf", path)
}

func TestDaemonProgramName(t *testing.T) {
	mgr := NewDaemonManager(zerolog.Nop(), Config{WebStorageDir: "/var/www/storage"})

	info := &DaemonInfo{
		TenantName: "t_abc123",
		Name:       "daemon_xyz789",
	}

	name := mgr.programName(info)
	assert.Equal(t, "daemon-t_abc123-daemon_xyz789", name)
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
		TenantName:   "t_abc123",
		WebrootName:  "main",
		DaemonName:   "daemon_xyz789",
		Command:      "php artisan queue:work",
		WorkingDir:   "/var/www/storage/t_abc123/webroots/main",
		NumProcs:     1,
		StopSignal:   "TERM",
		StopWaitSecs: 30,
		MaxMemoryMB:  256,
		Environment:  `APP_ENV="production"`,
	}

	var b bytes.Buffer
	require.NoError(t, daemonConfigTmpl.Execute(&b, data))
	config := b.String()

	assert.Contains(t, config, "[program:daemon-t_abc123-daemon_xyz789]")
	assert.Contains(t, config, "command=php artisan queue:work")
	assert.Contains(t, config, "directory=/var/www/storage/t_abc123/webroots/main")
	assert.Contains(t, config, "user=t_abc123")
	assert.Contains(t, config, "numprocs=1")
	assert.Contains(t, config, "stopsignal=TERM")
	assert.Contains(t, config, "stopwaitsecs=30")
	assert.Contains(t, config, `environment=APP_ENV="production"`)
	assert.Contains(t, config, "autostart=true")
	assert.Contains(t, config, "autorestart=unexpected")
	assert.Contains(t, config, "stdout_logfile=/var/www/storage/t_abc123/logs/daemon-daemon_xyz789.log")
	assert.Contains(t, config, "stderr_logfile=/var/www/storage/t_abc123/logs/daemon-daemon_xyz789.error.log")
}

func TestDaemonConfigTemplate_WithPort(t *testing.T) {
	env := map[string]string{"APP_ENV": "production", "PORT": "14523"}

	data := daemonConfigData{
		TenantName:   "t_abc123",
		WebrootName:  "main",
		DaemonName:   "daemon_ws123",
		Command:      "php artisan reverb:start --port=$PORT",
		WorkingDir:   "/var/www/storage/t_abc123/webroots/main",
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
		TenantName:   "t_abc123",
		WebrootName:  "main",
		DaemonName:   "daemon_ws123",
		Command:      "php artisan reverb:start --host=$HOST --port=$PORT",
		WorkingDir:   "/var/www/storage/t_abc123/webroots/main",
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
		TenantName:   "t_abc123",
		WebrootName:  "main",
		DaemonName:   "daemon_worker",
		Command:      "php artisan queue:work",
		WorkingDir:   "/var/www/storage/t_abc123/webroots/main",
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
		TenantName:   "t_abc123",
		WebrootName:  "main",
		DaemonName:   "daemon_bg",
		Command:      "node worker.js",
		WorkingDir:   "/var/www/storage/t_abc123/webroots/main",
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
