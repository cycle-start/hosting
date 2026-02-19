package core

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewDashboardService(t *testing.T) {
	db := &mockDB{}
	svc := NewDashboardService(db)
	require.NotNil(t, svc)
}

func TestDashboardService_Stats_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewDashboardService(db)
	ctx := context.Background()

	// Mock the counts query (15 fields)
	countRow := &mockRow{
		scanFunc: func(dest ...any) error {
			*(dest[0].(*int)) = 3   // regions
			*(dest[1].(*int)) = 5   // clusters
			*(dest[2].(*int)) = 15  // shards
			*(dest[3].(*int)) = 30  // nodes
			*(dest[4].(*int)) = 100 // tenants
			*(dest[5].(*int)) = 90  // tenants_active
			*(dest[6].(*int)) = 10  // tenants_suspended
			*(dest[7].(*int)) = 50  // databases
			*(dest[8].(*int)) = 20  // zones
			*(dest[9].(*int)) = 10  // valkey_instances
			*(dest[10].(*int)) = 30 // fqdns
			*(dest[11].(*int)) = 5  // incidents_open
			*(dest[12].(*int)) = 2  // incidents_critical
			*(dest[13].(*int)) = 1  // incidents_escalated
			*(dest[14].(*int)) = 3  // capability_gaps_open
			return nil
		},
	}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(countRow).Once()

	// Mock tenants per shard query
	tpsRows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = "shard-1"
			*(dest[1].(*string)) = "web-1"
			*(dest[2].(*string)) = "web"
			*(dest[3].(*int)) = 50
			return nil
		},
	)
	// Mock nodes per cluster query
	npcRows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = "cluster-1"
			*(dest[1].(*string)) = "osl-1"
			*(dest[2].(*int)) = 10
			return nil
		},
	)
	// Mock tenants by status query
	tbsRows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = "active"
			*(dest[1].(*int)) = 90
			return nil
		},
	)
	// Mock incidents by status query
	ibsRows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = "open"
			*(dest[1].(*int)) = 5
			return nil
		},
	)

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(tpsRows, nil).Once()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(npcRows, nil).Once()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(tbsRows, nil).Once()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(ibsRows, nil).Once()

	// Mock MTTR query
	mttrVal := 45.5
	mttrRow := &mockRow{
		scanFunc: func(dest ...any) error {
			*(dest[0].(**float64)) = &mttrVal
			return nil
		},
	}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(mttrRow).Once()

	stats, err := svc.Stats(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.Regions)
	assert.Equal(t, 5, stats.Clusters)
	assert.Equal(t, 15, stats.Shards)
	assert.Equal(t, 30, stats.Nodes)
	assert.Equal(t, 100, stats.Tenants)
	assert.Equal(t, 90, stats.TenantsActive)
	assert.Equal(t, 10, stats.TenantsSuspended)
	assert.Equal(t, 50, stats.Databases)
	assert.Equal(t, 20, stats.Zones)
	assert.Equal(t, 10, stats.ValkeyInstances)
	assert.Equal(t, 30, stats.FQDNs)
	assert.Equal(t, 5, stats.IncidentsOpen)
	assert.Equal(t, 2, stats.IncidentsCritical)
	assert.Equal(t, 1, stats.IncidentsEscalated)
	assert.Equal(t, 3, stats.CapabilityGapsOpen)

	require.Len(t, stats.TenantsPerShard, 1)
	assert.Equal(t, "shard-1", stats.TenantsPerShard[0].ShardID)
	assert.Equal(t, "web-1", stats.TenantsPerShard[0].ShardName)
	assert.Equal(t, "web", stats.TenantsPerShard[0].Role)
	assert.Equal(t, 50, stats.TenantsPerShard[0].Count)

	require.Len(t, stats.NodesPerCluster, 1)
	assert.Equal(t, "cluster-1", stats.NodesPerCluster[0].ClusterID)
	assert.Equal(t, "osl-1", stats.NodesPerCluster[0].ClusterName)
	assert.Equal(t, 10, stats.NodesPerCluster[0].Count)

	require.Len(t, stats.TenantsByStatus, 1)
	assert.Equal(t, "active", stats.TenantsByStatus[0].Status)
	assert.Equal(t, 90, stats.TenantsByStatus[0].Count)

	require.Len(t, stats.IncidentsByStatus, 1)
	assert.Equal(t, "open", stats.IncidentsByStatus[0].Status)
	assert.Equal(t, 5, stats.IncidentsByStatus[0].Count)

	require.NotNil(t, stats.MTTRMinutes)
	assert.InDelta(t, 45.5, *stats.MTTRMinutes, 0.01)

	db.AssertExpectations(t)
}

func TestDashboardService_Stats_CountsQueryError(t *testing.T) {
	db := &mockDB{}
	svc := NewDashboardService(db)
	ctx := context.Background()

	countRow := &mockRow{
		scanFunc: func(dest ...any) error {
			return errors.New("connection lost")
		},
	}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(countRow)

	stats, err := svc.Stats(ctx)
	require.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "dashboard counts")
	db.AssertExpectations(t)
}

func TestDashboardService_Stats_TenantsPerShardQueryError(t *testing.T) {
	db := &mockDB{}
	svc := NewDashboardService(db)
	ctx := context.Background()

	countRow := &mockRow{
		scanFunc: func(dest ...any) error {
			for i := 0; i < 15; i++ {
				*(dest[i].(*int)) = 0
			}
			return nil
		},
	}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(countRow)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("query failed")).Once()

	stats, err := svc.Stats(ctx)
	require.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "dashboard tenants per shard")
	db.AssertExpectations(t)
}
