package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/edvin/hosting/internal/model"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewNodeService(t *testing.T) {
	db := &mockDB{}
	svc := NewNodeService(db)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
}

// ---------- Create ----------

func TestNodeService_Create_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewNodeService(db)
	ctx := context.Background()

	ip := "10.0.0.10"
	ip6 := "fd00::10"
	node := &model.Node{
		ID:         "test-node-1",
		ClusterID:  "test-cluster-1",
		Hostname:   "node-1.example.com",
		IPAddress:  &ip,
		IP6Address: &ip6,
		Roles:      []string{"web", "db"},
		Status:     model.StatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.Create(ctx, node)
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestNodeService_Create_DBError(t *testing.T) {
	db := &mockDB{}
	svc := NewNodeService(db)
	ctx := context.Background()

	node := &model.Node{ID: "test-node-1", Hostname: "node-1"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Create(ctx, node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create node")
	db.AssertExpectations(t)
}

// ---------- GetByID ----------

func TestNodeService_GetByID_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewNodeService(db)
	ctx := context.Background()

	nodeID := "test-node-1"
	clusterID := "test-cluster-1"
	shardID := "test-shard-1"
	now := time.Now().Truncate(time.Microsecond)

	ipAddr := "10.0.0.10"
	ip6Addr := "fd00::10"
	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = nodeID
		*(dest[1].(*string)) = clusterID
		*(dest[2].(**string)) = &shardID
		*(dest[3].(*string)) = "node-1.example.com"
		*(dest[4].(**string)) = &ipAddr
		*(dest[5].(**string)) = &ip6Addr
		*(dest[6].(*[]string)) = []string{"web", "db"}
		*(dest[7].(*string)) = model.StatusActive
		*(dest[8].(*time.Time)) = now
		*(dest[9].(*time.Time)) = now
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, nodeID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, nodeID, result.ID)
	assert.Equal(t, clusterID, result.ClusterID)
	assert.Equal(t, "node-1.example.com", result.Hostname)
	assert.Equal(t, "10.0.0.10", *result.IPAddress)
	assert.Equal(t, "fd00::10", *result.IP6Address)
	assert.Equal(t, []string{"web", "db"}, result.Roles)
	assert.Equal(t, model.StatusActive, result.Status)
	db.AssertExpectations(t)
}

func TestNodeService_GetByID_NotFound(t *testing.T) {
	db := &mockDB{}
	svc := NewNodeService(db)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, "nonexistent-node")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get node")
	db.AssertExpectations(t)
}

// ---------- ListByCluster ----------

func TestNodeService_ListByCluster_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewNodeService(db)
	ctx := context.Background()

	clusterID := "test-cluster-1"
	id1, id2 := "test-node-1", "test-node-2"
	shardID := "test-shard-1"
	now := time.Now().Truncate(time.Microsecond)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = clusterID
			*(dest[2].(**string)) = &shardID
			*(dest[3].(*string)) = "node-1"
			ip := "10.0.0.1"
			ip6 := "::1"
			*(dest[4].(**string)) = &ip
			*(dest[5].(**string)) = &ip6
			*(dest[6].(*[]string)) = []string{"web"}
			*(dest[7].(*string)) = model.StatusActive
			*(dest[8].(*time.Time)) = now
			*(dest[9].(*time.Time)) = now
			return nil
		},
		func(dest ...any) error {
			*(dest[0].(*string)) = id2
			*(dest[1].(*string)) = clusterID
			*(dest[2].(**string)) = &shardID
			*(dest[3].(*string)) = "node-2"
			ip := "10.0.0.2"
			ip6 := "::2"
			*(dest[4].(**string)) = &ip
			*(dest[5].(**string)) = &ip6
			*(dest[6].(*[]string)) = []string{"db"}
			*(dest[7].(*string)) = model.StatusActive
			*(dest[8].(*time.Time)) = now
			*(dest[9].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, err := svc.ListByCluster(ctx, clusterID)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "node-1", result[0].Hostname)
	assert.Equal(t, "node-2", result[1].Hostname)
	db.AssertExpectations(t)
}

func TestNodeService_ListByCluster_Empty(t *testing.T) {
	db := &mockDB{}
	svc := NewNodeService(db)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, err := svc.ListByCluster(ctx, "test-cluster-1")
	require.NoError(t, err)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

func TestNodeService_ListByCluster_QueryError(t *testing.T) {
	db := &mockDB{}
	svc := NewNodeService(db)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("connection lost"))

	result, err := svc.ListByCluster(ctx, "test-cluster-1")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list nodes")
	db.AssertExpectations(t)
}

// ---------- ListByShard ----------

func TestNodeService_ListByShard_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewNodeService(db)
	ctx := context.Background()

	shardID := "test-shard-1"
	clusterID := "test-cluster-1"
	id1, id2 := "test-node-1", "test-node-2"
	now := time.Now().Truncate(time.Microsecond)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = clusterID
			*(dest[2].(**string)) = &shardID
			*(dest[3].(*string)) = "node-1"
			ip := "10.0.0.1"
			ip6 := "::1"
			*(dest[4].(**string)) = &ip
			*(dest[5].(**string)) = &ip6
			*(dest[6].(*[]string)) = []string{"web"}
			*(dest[7].(*string)) = model.StatusActive
			*(dest[8].(*time.Time)) = now
			*(dest[9].(*time.Time)) = now
			return nil
		},
		func(dest ...any) error {
			*(dest[0].(*string)) = id2
			*(dest[1].(*string)) = clusterID
			*(dest[2].(**string)) = &shardID
			*(dest[3].(*string)) = "node-2"
			ip := "10.0.0.2"
			ip6 := "::2"
			*(dest[4].(**string)) = &ip
			*(dest[5].(**string)) = &ip6
			*(dest[6].(*[]string)) = []string{"db"}
			*(dest[7].(*string)) = model.StatusActive
			*(dest[8].(*time.Time)) = now
			*(dest[9].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, err := svc.ListByShard(ctx, shardID)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "node-1", result[0].Hostname)
	assert.Equal(t, "node-2", result[1].Hostname)
	assert.Equal(t, &shardID, result[0].ShardID)
	assert.Equal(t, &shardID, result[1].ShardID)
	db.AssertExpectations(t)
}

func TestNodeService_ListByShard_Empty(t *testing.T) {
	db := &mockDB{}
	svc := NewNodeService(db)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, err := svc.ListByShard(ctx, "test-shard-1")
	require.NoError(t, err)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

func TestNodeService_ListByShard_QueryError(t *testing.T) {
	db := &mockDB{}
	svc := NewNodeService(db)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("connection lost"))

	result, err := svc.ListByShard(ctx, "test-shard-1")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list nodes for shard")
	db.AssertExpectations(t)
}

func TestNodeService_ListByShard_RowsErr(t *testing.T) {
	db := &mockDB{}
	svc := NewNodeService(db)
	ctx := context.Background()

	rows := newEmptyMockRows()
	rows.err = errors.New("iteration failed")
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, err := svc.ListByShard(ctx, "test-shard-1")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "iterate nodes")
	db.AssertExpectations(t)
}

// ---------- Update ----------

func TestNodeService_Update_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewNodeService(db)
	ctx := context.Background()

	updIP := "10.0.0.20"
	updIP6 := "fd00::20"
	node := &model.Node{
		ID:         "test-node-1",
		Hostname:   "node-updated",
		IPAddress:  &updIP,
		IP6Address: &updIP6,
		Roles:      []string{"web"},
		Status:     model.StatusActive,
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.Update(ctx, node)
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestNodeService_Update_DBError(t *testing.T) {
	db := &mockDB{}
	svc := NewNodeService(db)
	ctx := context.Background()

	node := &model.Node{ID: "test-node-1"}
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Update(ctx, node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update node")
	db.AssertExpectations(t)
}

// ---------- Delete ----------

func TestNodeService_Delete_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewNodeService(db)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.Delete(ctx, "test-node-1")
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestNodeService_Delete_DBError(t *testing.T) {
	db := &mockDB{}
	svc := NewNodeService(db)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("foreign key violation"))

	err := svc.Delete(ctx, "test-node-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete node")
	db.AssertExpectations(t)
}
