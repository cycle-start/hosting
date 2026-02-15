package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/model"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	temporalmocks "go.temporal.io/sdk/mocks"
)

func TestNewDatabaseService(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
	assert.Equal(t, tc, svc.tc)
}

// ---------- Create ----------

func TestDatabaseService_Create_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)
	ctx := context.Background()

	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	database := &model.Database{
		ID:        "test-database-1",
		TenantID:  &tenantID,
		Name:      "mydb",
		ShardID:   &shardID,
		Status:    model.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Create(ctx, database)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestDatabaseService_Create_InsertError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)
	ctx := context.Background()

	database := &model.Database{ID: "test-database-1", Name: "mydb"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Create(ctx, database)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert database")
	db.AssertExpectations(t)
}

func TestDatabaseService_Create_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)
	ctx := context.Background()

	database := &model.Database{ID: "test-database-1", Name: "mydb"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	tc.On("ExecuteWorkflow", ctx, mock.Anything, "CreateDatabaseWorkflow", mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Create(ctx, database)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start CreateDatabaseWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- GetByID ----------

func TestDatabaseService_GetByID_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)
	ctx := context.Background()

	databaseID := "test-database-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	nodeID := "test-node-1"
	shardName := "Test Shard"
	now := time.Now().Truncate(time.Microsecond)

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = databaseID
		*(dest[1].(**string)) = &tenantID
		*(dest[2].(*string)) = "mydb"
		*(dest[3].(**string)) = &shardID
		*(dest[4].(**string)) = &nodeID
		*(dest[5].(*string)) = model.StatusActive
		*(dest[6].(**string)) = nil // status_message
		*(dest[7].(*string)) = ""  // suspend_reason
		*(dest[8].(*time.Time)) = now
		*(dest[9].(*time.Time)) = now
		*(dest[10].(**string)) = &shardName
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, databaseID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, databaseID, result.ID)
	assert.Equal(t, "mydb", result.Name)
	assert.Equal(t, &tenantID, result.TenantID)
	assert.Equal(t, &shardID, result.ShardID)
	assert.Equal(t, &nodeID, result.NodeID)
	assert.Equal(t, &shardName, result.ShardName)
	db.AssertExpectations(t)
}

func TestDatabaseService_GetByID_NotFound(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, "nonexistent-database")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get database")
	db.AssertExpectations(t)
}

// ---------- ListByTenant ----------

func TestDatabaseService_ListByTenant_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)
	ctx := context.Background()

	tenantID := "test-tenant-1"
	id1 := "test-database-1"
	shardID := "test-shard-1"
	nodeID := "test-node-1"
	shardName := "Test Shard"
	now := time.Now().Truncate(time.Microsecond)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(**string)) = &tenantID
			*(dest[2].(*string)) = "mydb"
			*(dest[3].(**string)) = &shardID
			*(dest[4].(**string)) = &nodeID
			*(dest[5].(*string)) = model.StatusActive
			*(dest[6].(**string)) = nil // status_message
			*(dest[7].(*string)) = ""  // suspend_reason
			*(dest[8].(*time.Time)) = now
			*(dest[9].(*time.Time)) = now
			*(dest[10].(**string)) = &shardName
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByTenant(ctx, tenantID, request.ListParams{Limit: 50})
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 1)
	assert.Equal(t, "mydb", result[0].Name)
	db.AssertExpectations(t)
}

func TestDatabaseService_ListByTenant_QueryError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("connection lost"))

	result, _, err := svc.ListByTenant(ctx, "test-tenant-1", request.ListParams{Limit: 50})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list databases")
	db.AssertExpectations(t)
}

// ---------- ListByShard ----------

func TestDatabaseService_ListByShard_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)
	ctx := context.Background()

	shardID := "test-shard-1"
	id1, id2 := "test-database-1", "test-database-2"
	tenantID := "test-tenant-1"
	nodeID := "test-node-1"
	shardName := "Test Shard"
	now := time.Now().Truncate(time.Microsecond)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(**string)) = &tenantID
			*(dest[2].(*string)) = "db_alpha"
			*(dest[3].(**string)) = &shardID
			*(dest[4].(**string)) = &nodeID
			*(dest[5].(*string)) = model.StatusActive
			*(dest[6].(**string)) = nil // status_message
			*(dest[7].(*string)) = ""  // suspend_reason
			*(dest[8].(*time.Time)) = now
			*(dest[9].(*time.Time)) = now
			*(dest[10].(**string)) = &shardName
			return nil
		},
		func(dest ...any) error {
			*(dest[0].(*string)) = id2
			*(dest[1].(**string)) = &tenantID
			*(dest[2].(*string)) = "db_beta"
			*(dest[3].(**string)) = &shardID
			*(dest[4].(**string)) = &nodeID
			*(dest[5].(*string)) = model.StatusPending
			*(dest[6].(**string)) = nil // status_message
			*(dest[7].(*string)) = ""  // suspend_reason
			*(dest[8].(*time.Time)) = now
			*(dest[9].(*time.Time)) = now
			*(dest[10].(**string)) = &shardName
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByShard(ctx, shardID, 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 2)
	assert.Equal(t, "db_alpha", result[0].Name)
	assert.Equal(t, "db_beta", result[1].Name)
	assert.Equal(t, &shardID, result[0].ShardID)
	assert.Equal(t, &shardID, result[1].ShardID)
	db.AssertExpectations(t)
}

func TestDatabaseService_ListByShard_Empty(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByShard(ctx, "test-shard-1", 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

func TestDatabaseService_ListByShard_QueryError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("connection lost"))

	result, _, err := svc.ListByShard(ctx, "test-shard-1", 50, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list databases for shard")
	db.AssertExpectations(t)
}

func TestDatabaseService_ListByShard_RowsErr(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)
	ctx := context.Background()

	rows := newEmptyMockRows()
	rows.err = errors.New("iteration failed")
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, _, err := svc.ListByShard(ctx, "test-shard-1", 50, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "iterate databases")
	db.AssertExpectations(t)
}

// ---------- Delete ----------

func TestDatabaseService_Delete_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)
	ctx := context.Background()

	databaseID := "test-database-1"

	updateRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "mydb"
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

	err := svc.Delete(ctx, databaseID)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestDatabaseService_Delete_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)
	ctx := context.Background()

	errorRow := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("db error")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(errorRow)

	err := svc.Delete(ctx, "test-database-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status to deleting")
	db.AssertExpectations(t)
}

func TestDatabaseService_Delete_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)
	ctx := context.Background()

	updateRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "mydb"
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

	err := svc.Delete(ctx, "test-database-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start DeleteDatabaseWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- ReassignTenant ----------

func TestDatabaseService_ReassignTenant_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)
	ctx := context.Background()

	databaseID := "test-database-1"
	tenantID := "test-tenant-1"

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.ReassignTenant(ctx, databaseID, &tenantID)
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestDatabaseService_ReassignTenant_NilTenant(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.ReassignTenant(ctx, "test-database-1", nil)
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestDatabaseService_ReassignTenant_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewDatabaseService(db, tc)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.ReassignTenant(ctx, "test-database-1", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reassign database")
	db.AssertExpectations(t)
}
