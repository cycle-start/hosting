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
	temporalmocks "go.temporal.io/sdk/mocks"
)

func TestNewWebrootService(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewWebrootService(db, tc)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
	assert.Equal(t, tc, svc.tc)
}

// ---------- Create ----------

func TestWebrootService_Create_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewWebrootService(db, tc)
	ctx := context.Background()

	webroot := &model.Webroot{
		ID:             "test-webroot-1",
		TenantID:       "test-tenant-1",
		Name:           "my-site",
		Runtime:        "php",
		RuntimeVersion: "8.2",
		RuntimeConfig:  json.RawMessage(`{}`),
		PublicFolder:   "/public",
		Status:         model.StatusPending,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Create(ctx, webroot)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestWebrootService_Create_InsertError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewWebrootService(db, tc)
	ctx := context.Background()

	webroot := &model.Webroot{ID: "test-webroot-1", Name: "my-site"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("unique violation"))

	err := svc.Create(ctx, webroot)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert webroot")
	db.AssertExpectations(t)
}

func TestWebrootService_Create_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewWebrootService(db, tc)
	ctx := context.Background()

	webroot := &model.Webroot{ID: "test-webroot-1", Name: "my-site"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	tc.On("ExecuteWorkflow", ctx, mock.Anything, "CreateWebrootWorkflow", mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Create(ctx, webroot)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start CreateWebrootWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- GetByID ----------

func TestWebrootService_GetByID_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewWebrootService(db, tc)
	ctx := context.Background()

	webrootID := "test-webroot-1"
	tenantID := "test-tenant-1"
	now := time.Now().Truncate(time.Microsecond)
	cfg := json.RawMessage(`{"pool_size":5}`)

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = webrootID
		*(dest[1].(*string)) = tenantID
		*(dest[2].(*string)) = "my-site"
		*(dest[3].(*string)) = "php"
		*(dest[4].(*string)) = "8.2"
		*(dest[5].(*json.RawMessage)) = cfg
		*(dest[6].(*string)) = "/public"
		*(dest[7].(*string)) = model.StatusActive
		*(dest[8].(*time.Time)) = now
		*(dest[9].(*time.Time)) = now
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, webrootID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, webrootID, result.ID)
	assert.Equal(t, "my-site", result.Name)
	assert.Equal(t, "php", result.Runtime)
	assert.Equal(t, "8.2", result.RuntimeVersion)
	assert.Equal(t, "/public", result.PublicFolder)
	db.AssertExpectations(t)
}

func TestWebrootService_GetByID_NotFound(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewWebrootService(db, tc)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, "nonexistent-webroot")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get webroot")
	db.AssertExpectations(t)
}

// ---------- ListByTenant ----------

func TestWebrootService_ListByTenant_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewWebrootService(db, tc)
	ctx := context.Background()

	tenantID := "test-tenant-1"
	id1 := "test-webroot-1"
	now := time.Now().Truncate(time.Microsecond)
	cfg := json.RawMessage(`{}`)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = tenantID
			*(dest[2].(*string)) = "site-a"
			*(dest[3].(*string)) = "php"
			*(dest[4].(*string)) = "8.2"
			*(dest[5].(*json.RawMessage)) = cfg
			*(dest[6].(*string)) = "/public"
			*(dest[7].(*string)) = model.StatusActive
			*(dest[8].(*time.Time)) = now
			*(dest[9].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByTenant(ctx, tenantID, 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 1)
	assert.Equal(t, "site-a", result[0].Name)
	db.AssertExpectations(t)
}

func TestWebrootService_ListByTenant_QueryError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewWebrootService(db, tc)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("connection lost"))

	result, _, err := svc.ListByTenant(ctx, "test-tenant-1", 50, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list webroots")
	db.AssertExpectations(t)
}

// ---------- Update ----------

func TestWebrootService_Update_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewWebrootService(db, tc)
	ctx := context.Background()

	webroot := &model.Webroot{
		ID:     "test-webroot-1",
		Name:   "updated-site",
		Status: model.StatusActive,
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("ExecuteWorkflow", ctx, mock.Anything, "UpdateWebrootWorkflow", mock.Anything).Return(wfRun, nil)

	err := svc.Update(ctx, webroot)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestWebrootService_Update_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewWebrootService(db, tc)
	ctx := context.Background()

	webroot := &model.Webroot{ID: "test-webroot-1"}
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Update(ctx, webroot)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update webroot")
	db.AssertExpectations(t)
}

func TestWebrootService_Update_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewWebrootService(db, tc)
	ctx := context.Background()

	webroot := &model.Webroot{ID: "test-webroot-1"}
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	tc.On("ExecuteWorkflow", ctx, mock.Anything, "UpdateWebrootWorkflow", mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Update(ctx, webroot)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start UpdateWebrootWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- Delete ----------

func TestWebrootService_Delete_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewWebrootService(db, tc)
	ctx := context.Background()

	webrootID := "test-webroot-1"

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

	err := svc.Delete(ctx, webrootID)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestWebrootService_Delete_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewWebrootService(db, tc)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Delete(ctx, "test-webroot-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status to deleting")
	db.AssertExpectations(t)
}

func TestWebrootService_Delete_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewWebrootService(db, tc)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "test-tenant-1"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow)
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Delete(ctx, "test-webroot-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start DeleteWebrootWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}
