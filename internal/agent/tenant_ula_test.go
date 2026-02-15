package agent

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestNewTenantULAManager(t *testing.T) {
	mgr := NewTenantULAManager(zerolog.Nop())
	assert.NotNil(t, mgr)
}

func TestTenantULAInfo(t *testing.T) {
	info := &TenantULAInfo{
		TenantName:   "t_test123456",
		TenantUID:    5001,
		ClusterID:    "dev-1",
		NodeShardIdx: 1,
	}
	assert.Equal(t, "t_test123456", info.TenantName)
	assert.Equal(t, 5001, info.TenantUID)
	assert.Equal(t, "dev-1", info.ClusterID)
	assert.Equal(t, 1, info.NodeShardIdx)
}
