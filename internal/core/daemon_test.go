package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeDaemonPort_Range(t *testing.T) {
	testCases := []struct {
		tenant  string
		webroot string
		daemon  string
	}{
		{"t_abc123", "main", "daemon_xyz"},
		{"t_xyz789", "blog", "daemon_ws1"},
		{"a", "b", "c"},
		{"longtenantname", "longwebrootname", "longdaemonname"},
		{"", "", ""},
		{"t_test", "app", "daemon_q"},
	}

	for _, tc := range testCases {
		port := ComputeDaemonPort(tc.tenant, tc.webroot, tc.daemon)
		assert.GreaterOrEqual(t, port, 10000, "port for %s/%s/%s should be >= 10000", tc.tenant, tc.webroot, tc.daemon)
		assert.Less(t, port, 20000, "port for %s/%s/%s should be < 20000", tc.tenant, tc.webroot, tc.daemon)
	}
}

func TestComputeDaemonPort_Deterministic(t *testing.T) {
	port1 := ComputeDaemonPort("t_abc123", "main", "daemon_xyz")
	port2 := ComputeDaemonPort("t_abc123", "main", "daemon_xyz")
	assert.Equal(t, port1, port2)
}

func TestComputeDaemonPort_DifferentInputs(t *testing.T) {
	port1 := ComputeDaemonPort("t_abc123", "main", "daemon_a")
	port2 := ComputeDaemonPort("t_xyz789", "blog", "daemon_b")
	assert.NotEqual(t, port1, port2)
}

func TestComputeDaemonPort_DifferentDaemonsSameWebroot(t *testing.T) {
	port1 := ComputeDaemonPort("t_abc123", "main", "daemon_ws")
	port2 := ComputeDaemonPort("t_abc123", "main", "daemon_worker")
	assert.NotEqual(t, port1, port2)
}
