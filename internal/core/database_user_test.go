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

func TestNewDatabaseUserService(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseUserService(db, tc)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
	assert.Equal(t, tc, svc.tc)
}

// ---------- Create ----------

func TestDatabaseUserService_Create_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseUserService(db, tc)
	ctx := context.Background()

	user := &model.DatabaseUser{
		ID:         "test-dbuser-1",
		DatabaseID: "test-database-1",
		Username:   "admin",
		Password:   "secret",
		Privileges: []string{"ALL"},
		Status:     model.StatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		tid := "test-tenant-1"
		*(dest[0].(**string)) = &tid
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Create(ctx, user)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestDatabaseUserService_Create_InsertError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseUserService(db, tc)
	ctx := context.Background()

	user := &model.DatabaseUser{ID: "test-dbuser-1", Username: "admin"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("unique violation"))

	err := svc.Create(ctx, user)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert database user")
	db.AssertExpectations(t)
}

func TestDatabaseUserService_Create_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseUserService(db, tc)
	ctx := context.Background()

	user := &model.DatabaseUser{ID: "test-dbuser-1", Username: "admin"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		tid := "test-tenant-1"
		*(dest[0].(**string)) = &tid
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow)

	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Create(ctx, user)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start CreateDatabaseUserWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- GetByID ----------

func TestDatabaseUserService_GetByID_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseUserService(db, tc)
	ctx := context.Background()

	userID := "test-dbuser-1"
	dbID := "test-database-1"
	now := time.Now().Truncate(time.Microsecond)

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = userID
		*(dest[1].(*string)) = dbID
		*(dest[2].(*string)) = "admin"
		*(dest[3].(*string)) = "hashed-pw"
		*(dest[4].(*[]string)) = []string{"ALL"}
		*(dest[5].(*string)) = model.StatusActive
		*(dest[6].(**string)) = nil // status_message
		*(dest[7].(*time.Time)) = now
		*(dest[8].(*time.Time)) = now
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, userID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, userID, result.ID)
	assert.Equal(t, dbID, result.DatabaseID)
	assert.Equal(t, "admin", result.Username)
	assert.Equal(t, []string{"ALL"}, result.Privileges)
	db.AssertExpectations(t)
}

func TestDatabaseUserService_GetByID_NotFound(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseUserService(db, tc)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, "nonexistent-dbuser")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get database user")
	db.AssertExpectations(t)
}

// ---------- ListByDatabase ----------

func TestDatabaseUserService_ListByDatabase_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseUserService(db, tc)
	ctx := context.Background()

	dbID := "test-database-1"
	id1, id2 := "test-dbuser-1", "test-dbuser-2"
	now := time.Now().Truncate(time.Microsecond)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = dbID
			*(dest[2].(*string)) = "admin"
			*(dest[3].(*string)) = "pw1"
			*(dest[4].(*[]string)) = []string{"ALL"}
			*(dest[5].(*string)) = model.StatusActive
			*(dest[6].(**string)) = nil // status_message
			*(dest[7].(*time.Time)) = now
			*(dest[8].(*time.Time)) = now
			return nil
		},
		func(dest ...any) error {
			*(dest[0].(*string)) = id2
			*(dest[1].(*string)) = dbID
			*(dest[2].(*string)) = "readonly"
			*(dest[3].(*string)) = "pw2"
			*(dest[4].(*[]string)) = []string{"SELECT"}
			*(dest[5].(*string)) = model.StatusActive
			*(dest[6].(**string)) = nil // status_message
			*(dest[7].(*time.Time)) = now
			*(dest[8].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByDatabase(ctx, dbID, 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 2)
	assert.Equal(t, "admin", result[0].Username)
	assert.Equal(t, "readonly", result[1].Username)
	db.AssertExpectations(t)
}

func TestDatabaseUserService_ListByDatabase_QueryError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseUserService(db, tc)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("db error"))

	result, _, err := svc.ListByDatabase(ctx, "test-database-1", 50, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list database users")
	db.AssertExpectations(t)
}

// ---------- Update ----------

func TestDatabaseUserService_Update_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseUserService(db, tc)
	ctx := context.Background()

	user := &model.DatabaseUser{
		ID:         "test-dbuser-1",
		Username:   "admin",
		Password:   "new-pass",
		Privileges: []string{"ALL"},
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		tid := "test-tenant-1"
		*(dest[0].(**string)) = &tid
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Update(ctx, user)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestDatabaseUserService_Update_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseUserService(db, tc)
	ctx := context.Background()

	user := &model.DatabaseUser{ID: "test-dbuser-1"}
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Update(ctx, user)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update database user")
	db.AssertExpectations(t)
}

func TestDatabaseUserService_Update_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseUserService(db, tc)
	ctx := context.Background()

	user := &model.DatabaseUser{ID: "test-dbuser-1"}
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		tid := "test-tenant-1"
		*(dest[0].(**string)) = &tid
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow)

	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Update(ctx, user)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start UpdateDatabaseUserWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- Delete ----------

func TestDatabaseUserService_Delete_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseUserService(db, tc)
	ctx := context.Background()

	userID := "test-dbuser-1"

	updateRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "admin"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(updateRow).Once()

	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		tid := "test-tenant-1"
		*(dest[0].(**string)) = &tid
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow).Once()

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Delete(ctx, userID)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestDatabaseUserService_Delete_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseUserService(db, tc)
	ctx := context.Background()

	errorRow := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("db error")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(errorRow)

	err := svc.Delete(ctx, "test-dbuser-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status to deleting")
	db.AssertExpectations(t)
}

func TestDatabaseUserService_Delete_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseUserService(db, tc)
	ctx := context.Background()

	updateRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "admin"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(updateRow).Once()

	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		tid := "test-tenant-1"
		*(dest[0].(**string)) = &tid
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow).Once()

	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Delete(ctx, "test-dbuser-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start DeleteDatabaseUserWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}
