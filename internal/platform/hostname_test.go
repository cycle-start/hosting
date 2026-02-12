package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServiceHostname(t *testing.T) {
	result := ServiceHostname("no-1.example.com", "acme", "web")
	assert.Equal(t, "web.acme.no-1.example.com", result)
}

func TestServiceHostname_SSHService(t *testing.T) {
	result := ServiceHostname("no-1.example.com", "acme", "ssh")
	assert.Equal(t, "ssh.acme.no-1.example.com", result)
}

func TestServiceHostname_MySQLService(t *testing.T) {
	result := ServiceHostname("no-1.example.com", "acme", "mysql")
	assert.Equal(t, "mysql.acme.no-1.example.com", result)
}
