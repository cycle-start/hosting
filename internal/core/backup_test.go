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

func TestNewBackupService(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewBackupService(db, tc)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
	assert.Equal(t, tc, svc.tc)
}

// ---------- Create ----------

func TestBackupService_Create_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewBackupService(db, tc)
	ctx := context.Background()

	now := time.Now()
	backup := &model.Backup{
		ID:         "test-backup-1",
		TenantID:   "test-tenant-1",
		Type:       model.BackupTypeWeb,
		SourceID:   "test-webroot-1",
		SourceName: "mysite",
		Status:     model.StatusPending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Create(ctx, backup)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestBackupService_Create_InsertError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewBackupService(db, tc)
	ctx := context.Background()

	backup := &model.Backup{ID: "test-backup-1", TenantID: "test-tenant-1"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Create(ctx, backup)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert backup")
	db.AssertExpectations(t)
}

func TestBackupService_Create_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewBackupService(db, tc)
	ctx := context.Background()

	backup := &model.Backup{ID: "test-backup-1", TenantID: "test-tenant-1"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Create(ctx, backup)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start CreateBackupWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- GetByID ----------

func TestBackupService_GetByID_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewBackupService(db, tc)
	ctx := context.Background()

	backupID := "test-backup-1"
	now := time.Now().Truncate(time.Microsecond)

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = backupID
		*(dest[1].(*string)) = "test-tenant-1"
		*(dest[2].(*string)) = model.BackupTypeWeb
		*(dest[3].(*string)) = "test-webroot-1"
		*(dest[4].(*string)) = "mysite"
		*(dest[5].(*string)) = "/var/backups/hosting/tenant1/test-backup-1.tar.gz"
		*(dest[6].(*int64)) = 1024
		*(dest[7].(*string)) = model.StatusActive
		*(dest[8].(**string)) = nil // status_message
		*(dest[9].(**time.Time)) = &now
		*(dest[10].(**time.Time)) = &now
		*(dest[11].(*time.Time)) = now
		*(dest[12].(*time.Time)) = now
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, backupID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, backupID, result.ID)
	assert.Equal(t, "test-tenant-1", result.TenantID)
	assert.Equal(t, model.BackupTypeWeb, result.Type)
	assert.Equal(t, "test-webroot-1", result.SourceID)
	assert.Equal(t, "mysite", result.SourceName)
	assert.Equal(t, int64(1024), result.SizeBytes)
	assert.Equal(t, model.StatusActive, result.Status)
	db.AssertExpectations(t)
}

func TestBackupService_GetByID_NotFound(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewBackupService(db, tc)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, "nonexistent")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get backup")
	db.AssertExpectations(t)
}

// ---------- ListByTenant ----------

func TestBackupService_ListByTenant_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewBackupService(db, tc)
	ctx := context.Background()

	now := time.Now().Truncate(time.Microsecond)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = "test-backup-1"
			*(dest[1].(*string)) = "test-tenant-1"
			*(dest[2].(*string)) = model.BackupTypeWeb
			*(dest[3].(*string)) = "test-webroot-1"
			*(dest[4].(*string)) = "mysite"
			*(dest[5].(*string)) = "/var/backups/hosting/tenant1/test-backup-1.tar.gz"
			*(dest[6].(*int64)) = 1024
			*(dest[7].(*string)) = model.StatusActive
			*(dest[8].(**string)) = nil // status_message
			*(dest[9].(**time.Time)) = &now
			*(dest[10].(**time.Time)) = &now
			*(dest[11].(*time.Time)) = now
			*(dest[12].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByTenant(ctx, "test-tenant-1", 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 1)
	assert.Equal(t, "mysite", result[0].SourceName)
	db.AssertExpectations(t)
}

func TestBackupService_ListByTenant_Empty(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewBackupService(db, tc)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByTenant(ctx, "test-tenant-1", 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

func TestBackupService_ListByTenant_QueryError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewBackupService(db, tc)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("connection lost"))

	result, _, err := svc.ListByTenant(ctx, "test-tenant-1", 50, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list backups")
	db.AssertExpectations(t)
}

func TestBackupService_ListByTenant_RowsErr(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewBackupService(db, tc)
	ctx := context.Background()

	rows := newEmptyMockRows()
	rows.err = errors.New("iteration failed")
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, _, err := svc.ListByTenant(ctx, "test-tenant-1", 50, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "iterate backups")
	db.AssertExpectations(t)
}

// ---------- Delete ----------

func TestBackupService_Delete_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewBackupService(db, tc)
	ctx := context.Background()

	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "test-tenant-1"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow)
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Delete(ctx, "test-backup-1")
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestBackupService_Delete_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewBackupService(db, tc)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Delete(ctx, "test-backup-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status to deleting")
	db.AssertExpectations(t)
}

func TestBackupService_Delete_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewBackupService(db, tc)
	ctx := context.Background()

	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "test-tenant-1"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow)
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Delete(ctx, "test-backup-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start DeleteBackupWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- Restore ----------

func TestBackupService_Restore_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewBackupService(db, tc)
	ctx := context.Background()

	now := time.Now().Truncate(time.Microsecond)

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "test-backup-1"
		*(dest[1].(*string)) = "test-tenant-1"
		*(dest[2].(*string)) = model.BackupTypeWeb
		*(dest[3].(*string)) = "test-webroot-1"
		*(dest[4].(*string)) = "mysite"
		*(dest[5].(*string)) = "/var/backups/hosting/tenant1/test-backup-1.tar.gz"
		*(dest[6].(*int64)) = 1024
		*(dest[7].(*string)) = model.StatusActive
		*(dest[8].(**string)) = nil // status_message
		*(dest[9].(**time.Time)) = &now
		*(dest[10].(**time.Time)) = &now
		*(dest[11].(*time.Time)) = now
		*(dest[12].(*time.Time)) = now
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Restore(ctx, "test-backup-1")
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestBackupService_Restore_NotActive(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewBackupService(db, tc)
	ctx := context.Background()

	now := time.Now().Truncate(time.Microsecond)

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "test-backup-1"
		*(dest[1].(*string)) = "test-tenant-1"
		*(dest[2].(*string)) = model.BackupTypeWeb
		*(dest[3].(*string)) = "test-webroot-1"
		*(dest[4].(*string)) = "mysite"
		*(dest[5].(*string)) = ""
		*(dest[6].(*int64)) = 0
		*(dest[7].(*string)) = model.StatusPending
		*(dest[8].(**string)) = nil // status_message
		*(dest[9].(**time.Time)) = nil
		*(dest[10].(**time.Time)) = nil
		*(dest[11].(*time.Time)) = now
		*(dest[12].(*time.Time)) = now
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	err := svc.Restore(ctx, "test-backup-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not active")
	db.AssertExpectations(t)
}

func TestBackupService_Restore_NotFound(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewBackupService(db, tc)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	err := svc.Restore(ctx, "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get backup for restore")
	db.AssertExpectations(t)
}

func TestBackupService_Restore_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewBackupService(db, tc)
	ctx := context.Background()

	now := time.Now().Truncate(time.Microsecond)

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "test-backup-1"
		*(dest[1].(*string)) = "test-tenant-1"
		*(dest[2].(*string)) = model.BackupTypeWeb
		*(dest[3].(*string)) = "test-webroot-1"
		*(dest[4].(*string)) = "mysite"
		*(dest[5].(*string)) = "/var/backups/hosting/tenant1/test-backup-1.tar.gz"
		*(dest[6].(*int64)) = 1024
		*(dest[7].(*string)) = model.StatusActive
		*(dest[8].(**string)) = nil // status_message
		*(dest[9].(**time.Time)) = &now
		*(dest[10].(**time.Time)) = &now
		*(dest[11].(*time.Time)) = now
		*(dest[12].(*time.Time)) = now
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Restore(ctx, "test-backup-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start RestoreBackupWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}
