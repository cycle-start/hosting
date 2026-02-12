package runtime

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agentv1 "github.com/edvin/hosting/proto/agent/v1"
)

func TestPHP_PoolConfigTemplate(t *testing.T) {
	// Test the PHP-FPM pool config template rendering directly.
	data := phpPoolData{
		TenantName: "tenant1",
		Version:    "8.2",
	}

	var buf bytes.Buffer
	err := phpPoolTmpl.Execute(&buf, data)
	require.NoError(t, err)

	config := buf.String()

	// Verify pool section header.
	assert.Contains(t, config, "[tenant1]")

	// Verify user/group.
	assert.Contains(t, config, "user = tenant1")
	assert.Contains(t, config, "group = tenant1")

	// Verify socket path.
	assert.Contains(t, config, "listen = /run/php/tenant1-php8.2.sock")

	// Verify socket permissions.
	assert.Contains(t, config, "listen.owner = www-data")
	assert.Contains(t, config, "listen.group = www-data")
	assert.Contains(t, config, "listen.mode = 0660")

	// Verify process manager settings.
	assert.Contains(t, config, "pm = dynamic")
	assert.Contains(t, config, "pm.max_children = 5")
	assert.Contains(t, config, "pm.start_servers = 2")
	assert.Contains(t, config, "pm.min_spare_servers = 1")
	assert.Contains(t, config, "pm.max_spare_servers = 3")
	assert.Contains(t, config, "pm.max_requests = 500")

	// Verify error log path.
	assert.Contains(t, config, "php_admin_value[error_log] = /home/tenant1/logs/php-error.log")
	assert.Contains(t, config, "php_admin_flag[log_errors] = on")
}

func TestPHP_PoolConfigTemplate_DifferentVersion(t *testing.T) {
	data := phpPoolData{
		TenantName: "user2",
		Version:    "8.3",
	}

	var buf bytes.Buffer
	err := phpPoolTmpl.Execute(&buf, data)
	require.NoError(t, err)

	config := buf.String()

	// Verify version-specific socket path.
	assert.Contains(t, config, "listen = /run/php/user2-php8.3.sock")
	assert.Contains(t, config, "[user2]")
	assert.Contains(t, config, "user = user2")
}

func TestPHP_PoolConfigPath(t *testing.T) {
	p := NewPHP(zerolog.Nop(), NewDirectManager(zerolog.Nop()))

	tests := []struct {
		name     string
		webroot  *agentv1.WebrootInfo
		expected string
	}{
		{
			name: "with version 8.2",
			webroot: &agentv1.WebrootInfo{
				TenantName:     "tenant1",
				RuntimeVersion: "8.2",
			},
			expected: "/etc/php/8.2/fpm/pool.d/tenant1.conf",
		},
		{
			name: "with version 8.3",
			webroot: &agentv1.WebrootInfo{
				TenantName:     "user2",
				RuntimeVersion: "8.3",
			},
			expected: "/etc/php/8.3/fpm/pool.d/user2.conf",
		},
		{
			name: "default version when empty",
			webroot: &agentv1.WebrootInfo{
				TenantName:     "user3",
				RuntimeVersion: "",
			},
			expected: "/etc/php/8.2/fpm/pool.d/user3.conf",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, p.poolConfigPath(tc.webroot))
		})
	}
}

func TestPHP_FpmServiceName(t *testing.T) {
	p := NewPHP(zerolog.Nop(), NewDirectManager(zerolog.Nop()))

	tests := []struct {
		name     string
		webroot  *agentv1.WebrootInfo
		expected string
	}{
		{
			name: "version 8.2",
			webroot: &agentv1.WebrootInfo{
				RuntimeVersion: "8.2",
			},
			expected: "php8.2-fpm",
		},
		{
			name: "version 8.3",
			webroot: &agentv1.WebrootInfo{
				RuntimeVersion: "8.3",
			},
			expected: "php8.3-fpm",
		},
		{
			name: "default version",
			webroot: &agentv1.WebrootInfo{
				RuntimeVersion: "",
			},
			expected: "php8.2-fpm",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, p.fpmServiceName(tc.webroot))
		})
	}
}

func TestPHP_ImplementsManagerInterface(t *testing.T) {
	p := NewPHP(zerolog.Nop(), NewDirectManager(zerolog.Nop()))
	var _ Manager = p
}
