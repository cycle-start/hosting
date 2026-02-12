package runtime

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agentv1 "github.com/edvin/hosting/proto/agent/v1"
)

func TestRuby_ServiceTemplate(t *testing.T) {
	data := rubyServiceData{
		TenantName:  "tenant1",
		WebrootName: "railsapp",
		WorkingDir:  "/var/www/storage/tenant1/railsapp",
	}

	var buf bytes.Buffer
	err := rubyServiceTmpl.Execute(&buf, data)
	require.NoError(t, err)

	config := buf.String()

	// Verify [Unit] section.
	assert.Contains(t, config, "[Unit]")
	assert.Contains(t, config, "Description=Puma app for tenant1/railsapp")
	assert.Contains(t, config, "After=network.target")

	// Verify [Service] section.
	assert.Contains(t, config, "[Service]")
	assert.Contains(t, config, "Type=simple")
	assert.Contains(t, config, "User=tenant1")
	assert.Contains(t, config, "Group=tenant1")
	assert.Contains(t, config, "WorkingDirectory=/var/www/storage/tenant1/railsapp")
	assert.Contains(t, config, "ExecStart=/usr/local/bin/bundle exec puma")
	assert.Contains(t, config, "-b unix:///run/puma/tenant1-railsapp.sock")
	assert.Contains(t, config, "-e production")
	assert.Contains(t, config, "--workers 2")
	assert.Contains(t, config, "--threads 1:5")
	assert.Contains(t, config, "ExecReload=/bin/kill -s USR1 $MAINPID")
	assert.Contains(t, config, "Restart=on-failure")
	assert.Contains(t, config, "RestartSec=5")

	// Verify log paths.
	assert.Contains(t, config, "StandardOutput=append:/home/tenant1/logs/puma-railsapp.log")
	assert.Contains(t, config, "StandardError=append:/home/tenant1/logs/puma-railsapp.error.log")

	// Verify [Install] section.
	assert.Contains(t, config, "[Install]")
	assert.Contains(t, config, "WantedBy=multi-user.target")
}

func TestRuby_ServiceName(t *testing.T) {
	r := NewRuby(zerolog.Nop(), NewDirectManager(zerolog.Nop()))

	webroot := &agentv1.WebrootInfo{
		TenantName: "tenant1",
		Name:       "railsapp",
	}

	assert.Equal(t, "puma-tenant1-railsapp", r.serviceName(webroot))
}

func TestRuby_UnitFilePath(t *testing.T) {
	r := NewRuby(zerolog.Nop(), NewDirectManager(zerolog.Nop()))

	webroot := &agentv1.WebrootInfo{
		TenantName: "tenant1",
		Name:       "railsapp",
	}

	assert.Equal(t, "/etc/systemd/system/puma-tenant1-railsapp.service", r.unitFilePath(webroot))
}

func TestRuby_ImplementsManagerInterface(t *testing.T) {
	r := NewRuby(zerolog.Nop(), NewDirectManager(zerolog.Nop()))
	var _ Manager = r
}
