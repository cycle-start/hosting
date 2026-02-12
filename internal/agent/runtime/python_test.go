package runtime

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agentv1 "github.com/edvin/hosting/proto/agent/v1"
)

func TestPython_ServiceTemplate(t *testing.T) {
	data := pythonServiceData{
		TenantName:  "tenant1",
		WebrootName: "djangoapp",
		WorkingDir:  "/var/www/storage/tenant1/djangoapp",
		WSGIModule:  "app:application",
	}

	var buf bytes.Buffer
	err := pythonServiceTmpl.Execute(&buf, data)
	require.NoError(t, err)

	config := buf.String()

	// Verify [Unit] section.
	assert.Contains(t, config, "[Unit]")
	assert.Contains(t, config, "Description=Gunicorn app for tenant1/djangoapp")
	assert.Contains(t, config, "After=network.target")

	// Verify [Service] section.
	assert.Contains(t, config, "[Service]")
	assert.Contains(t, config, "Type=notify")
	assert.Contains(t, config, "User=tenant1")
	assert.Contains(t, config, "Group=tenant1")
	assert.Contains(t, config, "WorkingDirectory=/var/www/storage/tenant1/djangoapp")
	assert.Contains(t, config, "--bind unix:/run/gunicorn/tenant1-djangoapp.sock")
	assert.Contains(t, config, "--workers 3")
	assert.Contains(t, config, "--timeout 120")
	assert.Contains(t, config, "app:application")
	assert.Contains(t, config, "ExecReload=/bin/kill -s HUP $MAINPID")
	assert.Contains(t, config, "Restart=on-failure")
	assert.Contains(t, config, "RestartSec=5")

	// Verify log paths.
	assert.Contains(t, config, "StandardOutput=append:/home/tenant1/logs/gunicorn-djangoapp.log")
	assert.Contains(t, config, "StandardError=append:/home/tenant1/logs/gunicorn-djangoapp.error.log")

	// Verify [Install] section.
	assert.Contains(t, config, "[Install]")
	assert.Contains(t, config, "WantedBy=multi-user.target")
}

func TestPython_ServiceName(t *testing.T) {
	p := NewPython(zerolog.Nop(), NewDirectManager(zerolog.Nop()))

	webroot := &agentv1.WebrootInfo{
		TenantName: "tenant1",
		Name:       "djangoapp",
	}

	assert.Equal(t, "gunicorn-tenant1-djangoapp", p.serviceName(webroot))
}

func TestPython_UnitFilePath(t *testing.T) {
	p := NewPython(zerolog.Nop(), NewDirectManager(zerolog.Nop()))

	webroot := &agentv1.WebrootInfo{
		TenantName: "tenant1",
		Name:       "djangoapp",
	}

	assert.Equal(t, "/etc/systemd/system/gunicorn-tenant1-djangoapp.service", p.unitFilePath(webroot))
}

func TestPython_ImplementsManagerInterface(t *testing.T) {
	p := NewPython(zerolog.Nop(), NewDirectManager(zerolog.Nop()))
	var _ Manager = p
}
