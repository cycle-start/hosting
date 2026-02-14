package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/edvin/hosting/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	temporalmocks "go.temporal.io/sdk/mocks"
)

func TestNewEmailAccountService(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewEmailAccountService(db, tc)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
	assert.Equal(t, tc, svc.tc)
}

// ---------- GetByID ----------

func TestEmailAccountService_GetByID_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewEmailAccountService(db, tc)
	ctx := context.Background()

	accountID := "test-email-1"
	fqdnID := "test-fqdn-1"
	now := time.Now().Truncate(time.Microsecond)

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = accountID
		*(dest[1].(*string)) = fqdnID
		*(dest[2].(*string)) = "user@example.com"
		*(dest[3].(*string)) = "Test User"
		*(dest[4].(*int64)) = 1073741824
		*(dest[5].(*string)) = model.StatusActive
		*(dest[6].(**string)) = nil // status_message
		*(dest[7].(*time.Time)) = now
		*(dest[8].(*time.Time)) = now
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, accountID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, accountID, result.ID)
	assert.Equal(t, fqdnID, result.FQDNID)
	assert.Equal(t, "user@example.com", result.Address)
	assert.Equal(t, "Test User", result.DisplayName)
	assert.Equal(t, int64(1073741824), result.QuotaBytes)
	assert.Equal(t, model.StatusActive, result.Status)
	assert.Equal(t, now, result.CreatedAt)
	assert.Equal(t, now, result.UpdatedAt)
	db.AssertExpectations(t)
}

func TestEmailAccountService_GetByID_NotFound(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewEmailAccountService(db, tc)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, "nonexistent-email")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get email account")
	db.AssertExpectations(t)
}

// ---------- ListByFQDN ----------

func TestEmailAccountService_ListByFQDN_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewEmailAccountService(db, tc)
	ctx := context.Background()

	fqdnID := "test-fqdn-1"
	id1 := "test-email-1"
	now := time.Now().Truncate(time.Microsecond)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = fqdnID
			*(dest[2].(*string)) = "alice@example.com"
			*(dest[3].(*string)) = "Alice"
			*(dest[4].(*int64)) = 536870912
			*(dest[5].(*string)) = model.StatusActive
			*(dest[6].(**string)) = nil // status_message
			*(dest[7].(*time.Time)) = now
			*(dest[8].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByFQDN(ctx, fqdnID, 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 1)
	assert.Equal(t, id1, result[0].ID)
	assert.Equal(t, "alice@example.com", result[0].Address)
	assert.Equal(t, "Alice", result[0].DisplayName)
	assert.Equal(t, int64(536870912), result[0].QuotaBytes)
	db.AssertExpectations(t)
}

func TestEmailAccountService_ListByFQDN_Empty(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewEmailAccountService(db, tc)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByFQDN(ctx, "test-fqdn-1", 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}
