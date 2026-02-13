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
	temporalmocks "go.temporal.io/sdk/mocks"
)

func TestNewFQDNService(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewFQDNService(db, tc)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
	assert.Equal(t, tc, svc.tc)
}

// ---------- Create ----------

func TestFQDNService_Create_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewFQDNService(db, tc)
	ctx := context.Background()

	fqdn := &model.FQDN{
		ID:         "test-fqdn-1",
		FQDN:       "example.com",
		WebrootID:  "test-webroot-1",
		SSLEnabled: true,
		Status:     model.StatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "test-tenant-1"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow)
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Create(ctx, fqdn)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestFQDNService_Create_InsertError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewFQDNService(db, tc)
	ctx := context.Background()

	fqdn := &model.FQDN{ID: "test-fqdn-1", FQDN: "example.com"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("unique violation"))

	err := svc.Create(ctx, fqdn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert fqdn")
	db.AssertExpectations(t)
}

func TestFQDNService_Create_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewFQDNService(db, tc)
	ctx := context.Background()

	fqdn := &model.FQDN{ID: "test-fqdn-1", FQDN: "example.com"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "test-tenant-1"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow)
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Create(ctx, fqdn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start BindFQDNWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- GetByID ----------

func TestFQDNService_GetByID_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewFQDNService(db, tc)
	ctx := context.Background()

	fqdnID := "test-fqdn-1"
	webrootID := "test-webroot-1"
	now := time.Now().Truncate(time.Microsecond)

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = fqdnID
		*(dest[1].(*string)) = "example.com"
		*(dest[2].(*string)) = webrootID
		*(dest[3].(*bool)) = true
		*(dest[4].(*string)) = model.StatusActive
		*(dest[5].(*time.Time)) = now
		*(dest[6].(*time.Time)) = now
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, fqdnID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, fqdnID, result.ID)
	assert.Equal(t, "example.com", result.FQDN)
	assert.Equal(t, webrootID, result.WebrootID)
	assert.True(t, result.SSLEnabled)
	assert.Equal(t, model.StatusActive, result.Status)
	db.AssertExpectations(t)
}

func TestFQDNService_GetByID_NotFound(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewFQDNService(db, tc)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, "nonexistent-fqdn")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get fqdn")
	db.AssertExpectations(t)
}

// ---------- ListByWebroot ----------

func TestFQDNService_ListByWebroot_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewFQDNService(db, tc)
	ctx := context.Background()

	webrootID := "test-webroot-1"
	id1, id2 := "test-fqdn-1", "test-fqdn-2"
	now := time.Now().Truncate(time.Microsecond)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = "alpha.example.com"
			*(dest[2].(*string)) = webrootID
			*(dest[3].(*bool)) = true
			*(dest[4].(*string)) = model.StatusActive
			*(dest[5].(*time.Time)) = now
			*(dest[6].(*time.Time)) = now
			return nil
		},
		func(dest ...any) error {
			*(dest[0].(*string)) = id2
			*(dest[1].(*string)) = "beta.example.com"
			*(dest[2].(*string)) = webrootID
			*(dest[3].(*bool)) = false
			*(dest[4].(*string)) = model.StatusPending
			*(dest[5].(*time.Time)) = now
			*(dest[6].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByWebroot(ctx, webrootID, 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 2)
	assert.Equal(t, "alpha.example.com", result[0].FQDN)
	assert.Equal(t, "beta.example.com", result[1].FQDN)
	db.AssertExpectations(t)
}

func TestFQDNService_ListByWebroot_Empty(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewFQDNService(db, tc)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByWebroot(ctx, "test-webroot-1", 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

func TestFQDNService_ListByWebroot_QueryError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewFQDNService(db, tc)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("connection lost"))

	result, _, err := svc.ListByWebroot(ctx, "test-webroot-1", 50, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list fqdns")
	db.AssertExpectations(t)
}

// ---------- Delete ----------

func TestFQDNService_Delete_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewFQDNService(db, tc)
	ctx := context.Background()

	fqdnID := "test-fqdn-1"

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "test-tenant-1"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow)
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Delete(ctx, fqdnID)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestFQDNService_Delete_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewFQDNService(db, tc)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Delete(ctx, "test-fqdn-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status to deleting")
	db.AssertExpectations(t)
}

func TestFQDNService_Delete_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewFQDNService(db, tc)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "test-tenant-1"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow)
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Delete(ctx, "test-fqdn-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start UnbindFQDNWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}
