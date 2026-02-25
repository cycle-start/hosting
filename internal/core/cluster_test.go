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

func TestNewClusterService(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
}

// ---------- Create ----------

func TestClusterService_Create_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	cluster := &model.Cluster{
		ID:        "test-cluster-1",
		RegionID:  "test-region-1",
		Name:      "prod-1",
		Config:    json.RawMessage(`{}`),
		Status:    model.StatusPending,
		Spec:      json.RawMessage(`{}`),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.Create(ctx, cluster)
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestClusterService_Create_DBError(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	cluster := &model.Cluster{ID: "test-cluster-1", Name: "prod-1"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Create(ctx, cluster)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create cluster")
	db.AssertExpectations(t)
}

// ---------- GetByID ----------

func TestClusterService_GetByID_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	clusterID := "test-cluster-1"
	regionID := "test-region-1"
	now := time.Now().Truncate(time.Microsecond)
	cfg := json.RawMessage(`{"max_nodes":10}`)
	spec := json.RawMessage(`{}`)

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = clusterID
		*(dest[1].(*string)) = regionID
		*(dest[2].(*string)) = "prod-1"
		*(dest[3].(*json.RawMessage)) = cfg
		*(dest[4].(*string)) = model.StatusPending
		*(dest[5].(*json.RawMessage)) = spec
		*(dest[6].(*time.Time)) = now
		*(dest[7].(*time.Time)) = now
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, clusterID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, clusterID, result.ID)
	assert.Equal(t, regionID, result.RegionID)
	assert.Equal(t, "prod-1", result.Name)
	assert.Equal(t, model.StatusPending, result.Status)
	db.AssertExpectations(t)
}

func TestClusterService_GetByID_NotFound(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, "nonexistent-cluster")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get cluster")
	db.AssertExpectations(t)
}

// ---------- ListByRegion ----------

func TestClusterService_ListByRegion_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	regionID := "test-region-1"
	id1 := "test-cluster-1"
	now := time.Now().Truncate(time.Microsecond)
	cfg := json.RawMessage(`{}`)
	spec := json.RawMessage(`{}`)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = regionID
			*(dest[2].(*string)) = "prod-1"
			*(dest[3].(*json.RawMessage)) = cfg
			*(dest[4].(*string)) = model.StatusPending
			*(dest[5].(*json.RawMessage)) = spec
			*(dest[6].(*time.Time)) = now
			*(dest[7].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByRegion(ctx, regionID, 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 1)
	assert.Equal(t, "prod-1", result[0].Name)
	db.AssertExpectations(t)
}

func TestClusterService_ListByRegion_Empty(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByRegion(ctx, "test-region-1", 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

func TestClusterService_ListByRegion_QueryError(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("connection lost"))

	result, _, err := svc.ListByRegion(ctx, "test-region-1", 50, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list clusters")
	db.AssertExpectations(t)
}

// ---------- Update ----------

func TestClusterService_Update_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	cluster := &model.Cluster{
		ID:     "test-cluster-1",
		Name:   "updated-cluster",
		Config: json.RawMessage(`{}`),
		Status: model.StatusActive,
		Spec:   json.RawMessage(`{}`),
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.Update(ctx, cluster)
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestClusterService_Update_DBError(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	cluster := &model.Cluster{ID: "test-cluster-1"}
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Update(ctx, cluster)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update cluster")
	db.AssertExpectations(t)
}

// ---------- Delete ----------

func TestClusterService_Delete_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.Delete(ctx, "test-cluster-1")
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestClusterService_Delete_DBError(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("foreign key violation"))

	err := svc.Delete(ctx, "test-cluster-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete cluster")
	db.AssertExpectations(t)
}

// ---------- ListRuntimes ----------

func TestClusterService_ListRuntimes_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	clusterID := "test-cluster-1"

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = clusterID
			*(dest[1].(*string)) = model.RuntimePHP
			*(dest[2].(*string)) = "8.2"
			*(dest[3].(*bool)) = true
			return nil
		},
		func(dest ...any) error {
			*(dest[0].(*string)) = clusterID
			*(dest[1].(*string)) = model.RuntimeNode
			*(dest[2].(*string)) = "20"
			*(dest[3].(*bool)) = true
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListRuntimes(ctx, clusterID, 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 2)
	assert.Equal(t, model.RuntimePHP, result[0].Runtime)
	assert.Equal(t, "8.2", result[0].Version)
	assert.Equal(t, model.RuntimeNode, result[1].Runtime)
	assert.Equal(t, "20", result[1].Version)
	db.AssertExpectations(t)
}

func TestClusterService_ListRuntimes_Empty(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListRuntimes(ctx, "test-cluster-1", 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

func TestClusterService_ListRuntimes_QueryError(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("db error"))

	result, _, err := svc.ListRuntimes(ctx, "test-cluster-1", 50, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list cluster runtimes")
	db.AssertExpectations(t)
}

// ---------- AddRuntime ----------

func TestClusterService_AddRuntime_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	rt := &model.ClusterRuntime{
		ClusterID: "test-cluster-1",
		Runtime:   model.RuntimePHP,
		Version:   "8.3",
		Available: true,
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.AddRuntime(ctx, rt)
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestClusterService_AddRuntime_DBError(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	rt := &model.ClusterRuntime{ClusterID: "test-cluster-1", Runtime: "php", Version: "8.3"}
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.AddRuntime(ctx, rt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "add cluster runtime")
	db.AssertExpectations(t)
}

// ---------- RemoveRuntime ----------

func TestClusterService_RemoveRuntime_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.RemoveRuntime(ctx, "test-cluster-1", "php", "8.2")
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestClusterService_RemoveRuntime_DBError(t *testing.T) {
	db := &mockDB{}
	svc := NewClusterService(db)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.RemoveRuntime(ctx, "test-cluster-1", "php", "8.2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remove cluster runtime")
	db.AssertExpectations(t)
}
