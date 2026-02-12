package core

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewPlatformConfigService(t *testing.T) {
	db := &mockDB{}
	svc := NewPlatformConfigService(db)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
}

// ---------- Get ----------

func TestPlatformConfigService_Get_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewPlatformConfigService(db)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "site_name"
		*(dest[1].(*string)) = "My Hosting Platform"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.Get(ctx, "site_name")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "site_name", result.Key)
	assert.Equal(t, "My Hosting Platform", result.Value)
	db.AssertExpectations(t)
}

func TestPlatformConfigService_Get_NotFound(t *testing.T) {
	db := &mockDB{}
	svc := NewPlatformConfigService(db)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.Get(ctx, "nonexistent")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get platform config")
	db.AssertExpectations(t)
}

// ---------- GetAll ----------

func TestPlatformConfigService_GetAll_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewPlatformConfigService(db)
	ctx := context.Background()

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = "site_name"
			*(dest[1].(*string)) = "My Platform"
			return nil
		},
		func(dest ...any) error {
			*(dest[0].(*string)) = "support_email"
			*(dest[1].(*string)) = "support@example.com"
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), []any(nil)).Return(rows, nil)

	result, err := svc.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, "site_name", result[0].Key)
	assert.Equal(t, "My Platform", result[0].Value)
	assert.Equal(t, "support_email", result[1].Key)
	assert.Equal(t, "support@example.com", result[1].Value)
	db.AssertExpectations(t)
}

func TestPlatformConfigService_GetAll_Empty(t *testing.T) {
	db := &mockDB{}
	svc := NewPlatformConfigService(db)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), []any(nil)).Return(rows, nil)

	result, err := svc.GetAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

func TestPlatformConfigService_GetAll_QueryError(t *testing.T) {
	db := &mockDB{}
	svc := NewPlatformConfigService(db)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), []any(nil)).Return(nil, errors.New("connection lost"))

	result, err := svc.GetAll(ctx)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list platform config")
	db.AssertExpectations(t)
}

func TestPlatformConfigService_GetAll_RowsErr(t *testing.T) {
	db := &mockDB{}
	svc := NewPlatformConfigService(db)
	ctx := context.Background()

	rows := newEmptyMockRows()
	rows.err = errors.New("iteration failed")
	db.On("Query", ctx, mock.AnythingOfType("string"), []any(nil)).Return(rows, nil)

	result, err := svc.GetAll(ctx)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "iterate platform config")
	db.AssertExpectations(t)
}

// ---------- Set ----------

func TestPlatformConfigService_Set_Success(t *testing.T) {
	db := &mockDB{}
	svc := NewPlatformConfigService(db)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.Set(ctx, "site_name", "Updated Platform")
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestPlatformConfigService_Set_DBError(t *testing.T) {
	db := &mockDB{}
	svc := NewPlatformConfigService(db)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Set(ctx, "site_name", "Updated Platform")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "set platform config")
	db.AssertExpectations(t)
}
