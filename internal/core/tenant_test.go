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

func TestNewTenantService(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
	assert.Equal(t, tc, svc.tc)
}

// ---------- NextUID ----------

func TestTenantService_NextUID_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*int)) = 5001
		return nil
	}}

	db.On("QueryRow", ctx, "SELECT nextval('tenant_uid_seq')", []any(nil)).Return(row)

	uid, err := svc.NextUID(ctx)
	require.NoError(t, err)
	assert.Equal(t, 5001, uid)
	db.AssertExpectations(t)
}

func TestTenantService_NextUID_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("connection refused")
	}}

	db.On("QueryRow", ctx, "SELECT nextval('tenant_uid_seq')", []any(nil)).Return(row)

	uid, err := svc.NextUID(ctx)
	require.Error(t, err)
	assert.Equal(t, 0, uid)
	assert.Contains(t, err.Error(), "next tenant uid")
	db.AssertExpectations(t)
}

// ---------- Create ----------

func TestTenantService_Create_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenant := &model.Tenant{
		ID:          "test-tenant-1",
		RegionID:    "test-region-1",
		ClusterID:   "test-cluster-1",
		SFTPEnabled: true,
		Status:      model.StatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// NextUID query
	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*int)) = 5001
		return nil
	}}
	db.On("QueryRow", ctx, "SELECT nextval('tenant_uid_seq')", []any(nil)).Return(row)

	// INSERT exec
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	// Temporal workflow
	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Create(ctx, tenant)
	require.NoError(t, err)
	assert.Equal(t, 5001, tenant.UID)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestTenantService_Create_NextUIDError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenant := &model.Tenant{ID: "test-tenant-1"}

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("sequence error")
	}}
	db.On("QueryRow", ctx, "SELECT nextval('tenant_uid_seq')", []any(nil)).Return(row)

	err := svc.Create(ctx, tenant)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create tenant")
	db.AssertExpectations(t)
}

func TestTenantService_Create_InsertError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenant := &model.Tenant{
		ID:        "test-tenant-1",
		RegionID:  "test-region-1",
		ClusterID: "test-cluster-1",
		Status:    model.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*int)) = 5001
		return nil
	}}
	db.On("QueryRow", ctx, "SELECT nextval('tenant_uid_seq')", []any(nil)).Return(row)
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("unique violation"))

	err := svc.Create(ctx, tenant)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert tenant")
	db.AssertExpectations(t)
}

func TestTenantService_Create_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenant := &model.Tenant{
		ID:        "test-tenant-1",
		RegionID:  "test-region-1",
		ClusterID: "test-cluster-1",
		Status:    model.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*int)) = 5001
		return nil
	}}
	db.On("QueryRow", ctx, "SELECT nextval('tenant_uid_seq')", []any(nil)).Return(row)
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal unavailable"))

	err := svc.Create(ctx, tenant)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start CreateTenantWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- GetByID ----------

func TestTenantService_GetByID_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenantID := "test-tenant-1"
	regionID := "test-region-1"
	clusterID := "test-cluster-1"
	shardID := "test-shard-1"
	now := time.Now().Truncate(time.Microsecond)

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = tenantID
		*(dest[1].(*string)) = regionID
		*(dest[2].(*string)) = clusterID
		*(dest[3].(**string)) = &shardID
		*(dest[4].(*int)) = 5001
		*(dest[5].(*bool)) = true
		*(dest[6].(*string)) = model.StatusActive
		*(dest[7].(*time.Time)) = now
		*(dest[8].(*time.Time)) = now
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, tenantID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, tenantID, result.ID)
	assert.Equal(t, regionID, result.RegionID)
	assert.Equal(t, clusterID, result.ClusterID)
	assert.Equal(t, &shardID, result.ShardID)
	assert.Equal(t, 5001, result.UID)
	assert.True(t, result.SFTPEnabled)
	assert.Equal(t, model.StatusActive, result.Status)
	assert.Equal(t, now, result.CreatedAt)
	assert.Equal(t, now, result.UpdatedAt)
	db.AssertExpectations(t)
}

func TestTenantService_GetByID_NotFound(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenantID := "nonexistent-tenant"

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, tenantID)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get tenant")
	db.AssertExpectations(t)
}

// ---------- List ----------

func TestTenantService_List_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	id1, id2 := "test-tenant-1", "test-tenant-2"
	regionID := "test-region-1"
	clusterID := "test-cluster-1"
	shardID := "test-shard-1"
	now := time.Now().Truncate(time.Microsecond)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = regionID
			*(dest[2].(*string)) = clusterID
			*(dest[3].(**string)) = &shardID
			*(dest[4].(*int)) = 5001
			*(dest[5].(*bool)) = false
			*(dest[6].(*string)) = model.StatusActive
			*(dest[7].(*time.Time)) = now
			*(dest[8].(*time.Time)) = now
			return nil
		},
		func(dest ...any) error {
			*(dest[0].(*string)) = id2
			*(dest[1].(*string)) = regionID
			*(dest[2].(*string)) = clusterID
			*(dest[3].(**string)) = &shardID
			*(dest[4].(*int)) = 5002
			*(dest[5].(*bool)) = true
			*(dest[6].(*string)) = model.StatusPending
			*(dest[7].(*time.Time)) = now
			*(dest[8].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.List(ctx, request.ListParams{Limit: 50})
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 2)
	assert.Equal(t, id1, result[0].ID)
	assert.Equal(t, id2, result[1].ID)
	assert.Equal(t, 5001, result[0].UID)
	assert.Equal(t, 5002, result[1].UID)
	db.AssertExpectations(t)
}

func TestTenantService_List_Empty(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.List(ctx, request.ListParams{Limit: 50})
	require.NoError(t, err)
	assert.False(t, hasMore)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

func TestTenantService_List_QueryError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("connection lost"))

	result, _, err := svc.List(ctx, request.ListParams{Limit: 50})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list tenants")
	db.AssertExpectations(t)
}

func TestTenantService_List_RowsErr(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	rows := newEmptyMockRows()
	rows.err = errors.New("iteration failed")
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, _, err := svc.List(ctx, request.ListParams{Limit: 50})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "iterate tenants")
	db.AssertExpectations(t)
}

// ---------- Update ----------

func TestTenantService_Update_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenant := &model.Tenant{
		ID:          "test-tenant-1",
		RegionID:    "test-region-1",
		ClusterID:   "test-cluster-1",
		SFTPEnabled: true,
		Status:      model.StatusActive,
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Update(ctx, tenant)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestTenantService_Update_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenant := &model.Tenant{ID: "test-tenant-1"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Update(ctx, tenant)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update tenant")
	db.AssertExpectations(t)
}

func TestTenantService_Update_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenant := &model.Tenant{ID: "test-tenant-1"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Update(ctx, tenant)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start UpdateTenantWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- Delete ----------

func TestTenantService_Delete_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenantID := "test-tenant-1"

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Delete(ctx, tenantID)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestTenantService_Delete_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenantID := "test-tenant-1"

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Delete(ctx, tenantID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "set tenant")
	assert.Contains(t, err.Error(), "status to deleting")
	db.AssertExpectations(t)
}

func TestTenantService_Delete_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenantID := "test-tenant-1"

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Delete(ctx, tenantID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start DeleteTenantWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- Suspend ----------

func TestTenantService_Suspend_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenantID := "test-tenant-1"

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Suspend(ctx, tenantID)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestTenantService_Suspend_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenantID := "test-tenant-1"

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Suspend(ctx, tenantID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status to suspended")
	db.AssertExpectations(t)
}

func TestTenantService_Suspend_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenantID := "test-tenant-1"

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Suspend(ctx, tenantID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start SuspendTenantWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- ListByShard ----------

func TestTenantService_ListByShard_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	shardID := "test-shard-1"
	id1, id2 := "test-tenant-1", "test-tenant-2"
	regionID := "test-region-1"
	clusterID := "test-cluster-1"
	now := time.Now().Truncate(time.Microsecond)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = regionID
			*(dest[2].(*string)) = clusterID
			*(dest[3].(**string)) = &shardID
			*(dest[4].(*int)) = 5001
			*(dest[5].(*bool)) = false
			*(dest[6].(*string)) = model.StatusActive
			*(dest[7].(*time.Time)) = now
			*(dest[8].(*time.Time)) = now
			return nil
		},
		func(dest ...any) error {
			*(dest[0].(*string)) = id2
			*(dest[1].(*string)) = regionID
			*(dest[2].(*string)) = clusterID
			*(dest[3].(**string)) = &shardID
			*(dest[4].(*int)) = 5002
			*(dest[5].(*bool)) = true
			*(dest[6].(*string)) = model.StatusPending
			*(dest[7].(*time.Time)) = now
			*(dest[8].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByShard(ctx, shardID, 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 2)
	assert.Equal(t, id1, result[0].ID)
	assert.Equal(t, id2, result[1].ID)
	assert.Equal(t, 5001, result[0].UID)
	assert.Equal(t, 5002, result[1].UID)
	assert.Equal(t, &shardID, result[0].ShardID)
	assert.Equal(t, &shardID, result[1].ShardID)
	db.AssertExpectations(t)
}

func TestTenantService_ListByShard_Empty(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	rows := newEmptyMockRows()
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByShard(ctx, "test-shard-1", 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	assert.Empty(t, result)
	db.AssertExpectations(t)
}

func TestTenantService_ListByShard_QueryError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("connection lost"))

	result, _, err := svc.ListByShard(ctx, "test-shard-1", 50, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list tenants for shard")
	db.AssertExpectations(t)
}

func TestTenantService_ListByShard_RowsErr(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	rows := newEmptyMockRows()
	rows.err = errors.New("iteration failed")
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, _, err := svc.ListByShard(ctx, "test-shard-1", 50, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "iterate tenants")
	db.AssertExpectations(t)
}

// ---------- Unsuspend ----------

func TestTenantService_Unsuspend_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenantID := "test-tenant-1"

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Unsuspend(ctx, tenantID)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestTenantService_Unsuspend_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenantID := "test-tenant-1"

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Unsuspend(ctx, tenantID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status to pending")
	db.AssertExpectations(t)
}

func TestTenantService_Unsuspend_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewTenantService(db, tc)
	ctx := context.Background()

	tenantID := "test-tenant-1"

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Unsuspend(ctx, tenantID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start UnsuspendTenantWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}
