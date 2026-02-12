package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/edvin/hosting/internal/model"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewShardService(t *testing.T) {
	db := &mockDB{}
	svc := NewShardService(db, nil)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
}

// ---------- Create ----------

func TestShardService_Create_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewShardService(db, nil)
	ctx := context.Background()

	shard := &model.Shard{
		ID:        "test-shard-1",
		ClusterID: "test-cluster-1",
		Name:      "web-shard-01",
		Role:      model.ShardRoleWeb,
		LBBackend: "backend-1",
		Config:    json.RawMessage(`{"replicas":3}`),
		Status:    model.StatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.Create(ctx, shard)
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestShardService_Create_NilConfig(t *testing.T) {
	db := &mockDB{}
	svc := NewShardService(db, nil)
	ctx := context.Background()

	shard := &model.Shard{
		ID:        "test-shard-1",
		ClusterID: "test-cluster-1",
		Name:      "web-shard-01",
		Role:      model.ShardRoleWeb,
		Config:    nil,
		Status:    model.StatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.Create(ctx, shard)
	require.NoError(t, err)
	assert.Equal(t, json.RawMessage(`{}`), shard.Config)
	db.AssertExpectations(t)
}

func TestShardService_Create_DBError(t *testing.T) {
	db := &mockDB{}
	svc := NewShardService(db, nil)
	ctx := context.Background()

	shard := &model.Shard{ID: "test-shard-1", Name: "web-shard-01"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Create(ctx, shard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create shard")
	db.AssertExpectations(t)
}

// ---------- GetByID ----------

func TestShardService_GetByID_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewShardService(db, nil)
	ctx := context.Background()

	shardID := "test-shard-1"
	clusterID := "test-cluster-1"
	now := time.Now().Truncate(time.Microsecond)
	cfg := json.RawMessage(`{"replicas":3}`)

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = shardID
		*(dest[1].(*string)) = clusterID
		*(dest[2].(*string)) = "web-shard-01"
		*(dest[3].(*string)) = model.ShardRoleWeb
		*(dest[4].(*string)) = "backend-1"
		*(dest[5].(*json.RawMessage)) = cfg
		*(dest[6].(*string)) = model.StatusActive
		*(dest[7].(*time.Time)) = now
		*(dest[8].(*time.Time)) = now
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, shardID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, shardID, result.ID)
	assert.Equal(t, clusterID, result.ClusterID)
	assert.Equal(t, "web-shard-01", result.Name)
	assert.Equal(t, model.ShardRoleWeb, result.Role)
	assert.Equal(t, "backend-1", result.LBBackend)
	assert.Equal(t, model.StatusActive, result.Status)
	db.AssertExpectations(t)
}

func TestShardService_GetByID_NotFound(t *testing.T) {
	db := &mockDB{}
	svc := NewShardService(db, nil)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, "nonexistent-shard")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get shard")
	db.AssertExpectations(t)
}

// ---------- ListByCluster ----------

func TestShardService_ListByCluster_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewShardService(db, nil)
	ctx := context.Background()

	clusterID := "test-cluster-1"
	id1 := "test-shard-1"
	id2 := "test-shard-2"
	now := time.Now().Truncate(time.Microsecond)
	cfg := json.RawMessage(`{}`)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = clusterID
			*(dest[2].(*string)) = "db-shard-01"
			*(dest[3].(*string)) = model.ShardRoleDatabase
			*(dest[4].(*string)) = "backend-db"
			*(dest[5].(*json.RawMessage)) = cfg
			*(dest[6].(*string)) = model.StatusActive
			*(dest[7].(*time.Time)) = now
			*(dest[8].(*time.Time)) = now
			return nil
		},
		func(dest ...any) error {
			*(dest[0].(*string)) = id2
			*(dest[1].(*string)) = clusterID
			*(dest[2].(*string)) = "web-shard-01"
			*(dest[3].(*string)) = model.ShardRoleWeb
			*(dest[4].(*string)) = "backend-web"
			*(dest[5].(*json.RawMessage)) = cfg
			*(dest[6].(*string)) = model.StatusActive
			*(dest[7].(*time.Time)) = now
			*(dest[8].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByCluster(ctx, clusterID, 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 2)
	assert.Equal(t, "db-shard-01", result[0].Name)
	assert.Equal(t, "web-shard-01", result[1].Name)
	db.AssertExpectations(t)
}

func TestShardService_ListByCluster_Empty(t *testing.T) {
	db := &mockDB{}
	svc := NewShardService(db, nil)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByCluster(ctx, "test-cluster-1", 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

func TestShardService_ListByCluster_QueryError(t *testing.T) {
	db := &mockDB{}
	svc := NewShardService(db, nil)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("connection lost"))

	result, _, err := svc.ListByCluster(ctx, "test-cluster-1", 50, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list shards")
	db.AssertExpectations(t)
}

// ---------- Update ----------

func TestShardService_Update_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewShardService(db, nil)
	ctx := context.Background()

	shard := &model.Shard{
		ID:        "test-shard-1",
		LBBackend: "new-backend",
		Config:    json.RawMessage(`{"replicas":5}`),
		Status:    model.StatusActive,
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.Update(ctx, shard)
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestShardService_Update_DBError(t *testing.T) {
	db := &mockDB{}
	svc := NewShardService(db, nil)
	ctx := context.Background()

	shard := &model.Shard{ID: "test-shard-1"}
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Update(ctx, shard)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update shard")
	db.AssertExpectations(t)
}

// ---------- Delete ----------

func TestShardService_Delete_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewShardService(db, nil)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.Delete(ctx, "test-shard-1")
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestShardService_Delete_DBError(t *testing.T) {
	db := &mockDB{}
	svc := NewShardService(db, nil)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("foreign key violation"))

	err := svc.Delete(ctx, "test-shard-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete shard")
	db.AssertExpectations(t)
}
