package runtime

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNode_ServiceTemplate(t *testing.T) {
	data := nodeServiceData{
		TenantName:  "tenant1",
		WebrootName: "myapp",
		WorkingDir:  "/var/www/storage/tenant1/webroots/myapp",
		EntryPoint:  "index.js",
		Port:        3456,
	}

	var buf bytes.Buffer
	err := nodeServiceTmpl.Execute(&buf, data)
	require.NoError(t, err)

	config := buf.String()

	// Verify [Unit] section.
	assert.Contains(t, config, "[Unit]")
	assert.Contains(t, config, "Description=Node.js app for tenant1/myapp")
	assert.Contains(t, config, "After=network.target")

	// Verify [Service] section.
	assert.Contains(t, config, "[Service]")
	assert.Contains(t, config, "Type=simple")
	assert.Contains(t, config, "User=tenant1")
	assert.Contains(t, config, "Group=tenant1")
	assert.Contains(t, config, "WorkingDirectory=/var/www/storage/tenant1/webroots/myapp")
	assert.Contains(t, config, "ExecStart=/usr/bin/node index.js")
	assert.Contains(t, config, "Restart=on-failure")
	assert.Contains(t, config, "RestartSec=5")
	assert.Contains(t, config, "Environment=NODE_ENV=production")
	assert.Contains(t, config, "Environment=PORT=3456")

	// Verify log paths on CephFS.
	assert.Contains(t, config, "StandardOutput=append:/var/www/storage/tenant1/logs/node-myapp.log")
	assert.Contains(t, config, "StandardError=append:/var/www/storage/tenant1/logs/node-myapp.error.log")

	// Verify [Install] section.
	assert.Contains(t, config, "[Install]")
	assert.Contains(t, config, "WantedBy=multi-user.target")
}

func TestNode_ServiceName(t *testing.T) {
	n := NewNode(zerolog.Nop(), NewDirectManager(zerolog.Nop()))

	webroot := &WebrootInfo{
		TenantName: "tenant1",
		Name:       "myapp",
	}

	assert.Equal(t, "node-tenant1-myapp", n.serviceName(webroot))
}

func TestNode_UnitFilePath(t *testing.T) {
	n := NewNode(zerolog.Nop(), NewDirectManager(zerolog.Nop()))

	webroot := &WebrootInfo{
		TenantName: "tenant1",
		Name:       "myapp",
	}

	assert.Equal(t, "/etc/systemd/system/node-tenant1-myapp.service", n.unitFilePath(webroot))
}

func TestComputePort_Range(t *testing.T) {
	// Test that computePort always returns a port in range [3000, 9999].
	testCases := []struct {
		tenant  string
		webroot string
	}{
		{"tenant1", "site1"},
		{"tenant2", "site2"},
		{"a", "b"},
		{"longtenantname", "longwebrootname"},
		{"", ""},
		{"test", "app123"},
		{"user_with_underscore", "site-with-dash"},
	}

	for _, tc := range testCases {
		port := computePort(tc.tenant, tc.webroot)
		assert.GreaterOrEqual(t, port, uint32(3000), "port for %s/%s should be >= 3000", tc.tenant, tc.webroot)
		assert.Less(t, port, uint32(10000), "port for %s/%s should be < 10000", tc.tenant, tc.webroot)
	}
}

func TestComputePort_Deterministic(t *testing.T) {
	port1 := computePort("tenant1", "site1")
	port2 := computePort("tenant1", "site1")
	assert.Equal(t, port1, port2)
}

func TestComputePort_DifferentInputs(t *testing.T) {
	port1 := computePort("tenant1", "site1")
	port2 := computePort("tenant2", "site2")
	// Different inputs should generally produce different ports.
	assert.NotEqual(t, port1, port2)
}

func TestNode_ImplementsManagerInterface(t *testing.T) {
	n := NewNode(zerolog.Nop(), NewDirectManager(zerolog.Nop()))
	var _ Manager = n
}
