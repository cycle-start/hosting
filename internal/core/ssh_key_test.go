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

func TestNewSSHKeyService(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewSSHKeyService(db, tc)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
	assert.Equal(t, tc, svc.tc)
}

// ---------- Create ----------

func TestSSHKeyService_Create_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewSSHKeyService(db, tc)
	ctx := context.Background()

	now := time.Now()
	key := &model.SSHKey{
		ID:          "test-key-1",
		TenantID:    "test-tenant-1",
		Name:        "my-key",
		PublicKey:   "ssh-ed25519 AAAAC3...",
		Fingerprint: "SHA256:abc123",
		Status:      model.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Create(ctx, key)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestSSHKeyService_Create_InsertError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewSSHKeyService(db, tc)
	ctx := context.Background()

	key := &model.SSHKey{ID: "test-key-1", TenantID: "test-tenant-1"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Create(ctx, key)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert SSH key")
	db.AssertExpectations(t)
}

func TestSSHKeyService_Create_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewSSHKeyService(db, tc)
	ctx := context.Background()

	key := &model.SSHKey{ID: "test-key-1", TenantID: "test-tenant-1"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Create(ctx, key)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signal AddSSHKeyWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- GetByID ----------

func TestSSHKeyService_GetByID_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewSSHKeyService(db, tc)
	ctx := context.Background()

	keyID := "test-key-1"
	tenantID := "test-tenant-1"
	now := time.Now().Truncate(time.Microsecond)

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = keyID
		*(dest[1].(*string)) = tenantID
		*(dest[2].(*string)) = "my-key"
		*(dest[3].(*string)) = "ssh-ed25519 AAAAC3..."
		*(dest[4].(*string)) = "SHA256:abc123"
		*(dest[5].(*string)) = model.StatusActive
		*(dest[6].(**string)) = nil // status_message
		*(dest[7].(*time.Time)) = now
		*(dest[8].(*time.Time)) = now
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, keyID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, keyID, result.ID)
	assert.Equal(t, tenantID, result.TenantID)
	assert.Equal(t, "my-key", result.Name)
	assert.Equal(t, "SHA256:abc123", result.Fingerprint)
	assert.Equal(t, model.StatusActive, result.Status)
	db.AssertExpectations(t)
}

func TestSSHKeyService_GetByID_NotFound(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewSSHKeyService(db, tc)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, "nonexistent-key")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get SSH key")
	db.AssertExpectations(t)
}

// ---------- ListByTenant ----------

func TestSSHKeyService_ListByTenant_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewSSHKeyService(db, tc)
	ctx := context.Background()

	tenantID := "test-tenant-1"
	id1 := "test-key-1"
	now := time.Now().Truncate(time.Microsecond)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = tenantID
			*(dest[2].(*string)) = "my-key"
			*(dest[3].(*string)) = "ssh-ed25519 AAAAC3..."
			*(dest[4].(*string)) = "SHA256:abc123"
			*(dest[5].(*string)) = model.StatusActive
			*(dest[6].(**string)) = nil // status_message
			*(dest[7].(*time.Time)) = now
			*(dest[8].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByTenant(ctx, tenantID, 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 1)
	assert.Equal(t, id1, result[0].ID)
	assert.Equal(t, tenantID, result[0].TenantID)
	db.AssertExpectations(t)
}

func TestSSHKeyService_ListByTenant_Empty(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewSSHKeyService(db, tc)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByTenant(ctx, "test-tenant-1", 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

func TestSSHKeyService_ListByTenant_QueryError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewSSHKeyService(db, tc)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("db error"))

	result, _, err := svc.ListByTenant(ctx, "test-tenant-1", 50, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list SSH keys")
	db.AssertExpectations(t)
}

func TestSSHKeyService_ListByTenant_WithCursor(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewSSHKeyService(db, tc)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByTenant(ctx, "test-tenant-1", 50, "cursor-id")
	require.NoError(t, err)
	assert.False(t, hasMore)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

// ---------- Delete ----------

func TestSSHKeyService_Delete_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewSSHKeyService(db, tc)
	ctx := context.Background()

	updateRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "my-key"
		*(dest[1].(*string)) = "test-tenant-1"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(updateRow).Once()

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Delete(ctx, "test-key-1")
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestSSHKeyService_Delete_ExecError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewSSHKeyService(db, tc)
	ctx := context.Background()

	errorRow := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("db error")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(errorRow)

	err := svc.Delete(ctx, "test-key-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "set SSH key")
	db.AssertExpectations(t)
}

func TestSSHKeyService_Delete_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewSSHKeyService(db, tc)
	ctx := context.Background()

	updateRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "my-key"
		*(dest[1].(*string)) = "test-tenant-1"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(updateRow).Once()

	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Delete(ctx, "test-key-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signal RemoveSSHKeyWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}
