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

func TestNewRegionService(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
}

// ---------- Create ----------

func TestRegionService_Create_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	region := &model.Region{
		ID:        "test-region-1",
		Name:      "eu-west-1",
		Config:    json.RawMessage(`{"provider":"aws"}`),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.Create(ctx, region)
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestRegionService_Create_DBError(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	region := &model.Region{ID: "test-region-1", Name: "eu-west-1"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("unique violation"))

	err := svc.Create(ctx, region)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create region")
	db.AssertExpectations(t)
}

// ---------- GetByID ----------

func TestRegionService_GetByID_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	regionID := "test-region-1"
	now := time.Now().Truncate(time.Microsecond)
	cfg := json.RawMessage(`{"provider":"aws"}`)

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = regionID
		*(dest[1].(*string)) = "eu-west-1"
		*(dest[2].(*json.RawMessage)) = cfg
		*(dest[3].(*time.Time)) = now
		*(dest[4].(*time.Time)) = now
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, regionID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, regionID, result.ID)
	assert.Equal(t, "eu-west-1", result.Name)
	assert.JSONEq(t, `{"provider":"aws"}`, string(result.Config))
	assert.Equal(t, now, result.CreatedAt)
	db.AssertExpectations(t)
}

func TestRegionService_GetByID_NotFound(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, "nonexistent-region")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get region")
	db.AssertExpectations(t)
}

// ---------- List ----------

func TestRegionService_List_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	id1, id2 := "test-region-1", "test-region-2"
	now := time.Now().Truncate(time.Microsecond)
	cfg := json.RawMessage(`{}`)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = "eu-west-1"
			*(dest[2].(*json.RawMessage)) = cfg
			*(dest[3].(*time.Time)) = now
			*(dest[4].(*time.Time)) = now
			return nil
		},
		func(dest ...any) error {
			*(dest[0].(*string)) = id2
			*(dest[1].(*string)) = "us-east-1"
			*(dest[2].(*json.RawMessage)) = cfg
			*(dest[3].(*time.Time)) = now
			*(dest[4].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.List(ctx, 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 2)
	assert.Equal(t, "eu-west-1", result[0].Name)
	assert.Equal(t, "us-east-1", result[1].Name)
	db.AssertExpectations(t)
}

func TestRegionService_List_Empty(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.List(ctx, 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

func TestRegionService_List_QueryError(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("connection lost"))

	result, _, err := svc.List(ctx, 50, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list regions")
	db.AssertExpectations(t)
}

// ---------- Update ----------

func TestRegionService_Update_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	region := &model.Region{
		ID:     "test-region-1",
		Name:   "eu-west-2",
		Config: json.RawMessage(`{"updated":true}`),
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.Update(ctx, region)
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestRegionService_Update_DBError(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	region := &model.Region{ID: "test-region-1"}
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Update(ctx, region)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update region")
	db.AssertExpectations(t)
}

// ---------- Delete ----------

func TestRegionService_Delete_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.Delete(ctx, "test-region-1")
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestRegionService_Delete_DBError(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("foreign key violation"))

	err := svc.Delete(ctx, "test-region-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete region")
	db.AssertExpectations(t)
}

// ---------- ListRuntimes ----------

func TestRegionService_ListRuntimes_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	regionID := "test-region-1"

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = regionID
			*(dest[1].(*string)) = model.RuntimePHP
			*(dest[2].(*string)) = "8.2"
			*(dest[3].(*bool)) = true
			return nil
		},
		func(dest ...any) error {
			*(dest[0].(*string)) = regionID
			*(dest[1].(*string)) = model.RuntimeNode
			*(dest[2].(*string)) = "20"
			*(dest[3].(*bool)) = true
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListRuntimes(ctx, regionID, 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 2)
	assert.Equal(t, model.RuntimePHP, result[0].Runtime)
	assert.Equal(t, "8.2", result[0].Version)
	assert.Equal(t, model.RuntimeNode, result[1].Runtime)
	assert.Equal(t, "20", result[1].Version)
	db.AssertExpectations(t)
}

func TestRegionService_ListRuntimes_Empty(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListRuntimes(ctx, "test-region-1", 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

func TestRegionService_ListRuntimes_QueryError(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("db error"))

	result, _, err := svc.ListRuntimes(ctx, "test-region-1", 50, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list region runtimes")
	db.AssertExpectations(t)
}

// ---------- AddRuntime ----------

func TestRegionService_AddRuntime_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	rt := &model.RegionRuntime{
		RegionID:  "test-region-1",
		Runtime:   model.RuntimePHP,
		Version:   "8.3",
		Available: true,
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.AddRuntime(ctx, rt)
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestRegionService_AddRuntime_DBError(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	rt := &model.RegionRuntime{RegionID: "test-region-1", Runtime: "php", Version: "8.3"}
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.AddRuntime(ctx, rt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "add region runtime")
	db.AssertExpectations(t)
}

// ---------- RemoveRuntime ----------

func TestRegionService_RemoveRuntime_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.RemoveRuntime(ctx, "test-region-1", "php", "8.2")
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestRegionService_RemoveRuntime_DBError(t *testing.T) {
	db := &mockDB{}
	svc := NewRegionService(db)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.RemoveRuntime(ctx, "test-region-1", "php", "8.2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remove region runtime")
	db.AssertExpectations(t)
}
