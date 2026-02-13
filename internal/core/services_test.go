package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	temporalmocks "go.temporal.io/sdk/mocks"
)

func TestNewServices(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}

	svcs := NewServices(db, tc, "http://localhost")

	require.NotNil(t, svcs)
	assert.NotNil(t, svcs.PlatformConfig)
	assert.NotNil(t, svcs.Region)
	assert.NotNil(t, svcs.Cluster)
	assert.NotNil(t, svcs.Node)
	assert.NotNil(t, svcs.Tenant)
	assert.NotNil(t, svcs.Webroot)
	assert.NotNil(t, svcs.FQDN)
	assert.NotNil(t, svcs.Certificate)
	assert.NotNil(t, svcs.Zone)
	assert.NotNil(t, svcs.ZoneRecord)
	assert.NotNil(t, svcs.Database)
	assert.NotNil(t, svcs.DatabaseUser)
	assert.NotNil(t, svcs.OIDC)
}
