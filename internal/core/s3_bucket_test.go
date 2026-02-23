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

func TestNewS3BucketService(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3BucketService(db, tc)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
	assert.Equal(t, tc, svc.tc)
}

// ---------- Create ----------

func TestS3BucketService_Create_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3BucketService(db, tc)
	ctx := context.Background()

	tenantID := "test-tenant-1"
	bucket := &model.S3Bucket{
		ID:         "test-bucket-1",
		TenantID:   tenantID,
		Public:     false,
		QuotaBytes: 1073741824,
		Status:     model.StatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Create(ctx, bucket)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestS3BucketService_Create_InsertError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3BucketService(db, tc)
	ctx := context.Background()

	bucket := &model.S3Bucket{ID: "test-bucket-1"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Create(ctx, bucket)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert s3 bucket")
	db.AssertExpectations(t)
}

func TestS3BucketService_Create_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3BucketService(db, tc)
	ctx := context.Background()

	bucket := &model.S3Bucket{ID: "test-bucket-1"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	tc.On("ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Create(ctx, bucket)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signal CreateS3BucketWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- GetByID ----------

func TestS3BucketService_GetByID_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3BucketService(db, tc)
	ctx := context.Background()

	bucketID := "test-bucket-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	shardName := "Test Shard"
	now := time.Now().Truncate(time.Microsecond)

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = bucketID
		*(dest[1].(*string)) = tenantID
		*(dest[2].(*string)) = "" // subscription_id
		*(dest[3].(**string)) = &shardID
		*(dest[4].(*bool)) = false
		*(dest[5].(*int64)) = 1073741824
		*(dest[6].(*string)) = model.StatusActive
		*(dest[7].(**string)) = nil // status_message
		*(dest[8].(*string)) = ""  // suspend_reason
		*(dest[9].(*time.Time)) = now
		*(dest[10].(*time.Time)) = now
		*(dest[11].(**string)) = &shardName
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, bucketID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, bucketID, result.ID)
	assert.Equal(t, tenantID, result.TenantID)
	assert.Equal(t, &shardID, result.ShardID)
	assert.Equal(t, false, result.Public)
	assert.Equal(t, int64(1073741824), result.QuotaBytes)
	assert.Equal(t, model.StatusActive, result.Status)
	assert.Equal(t, now, result.CreatedAt)
	assert.Equal(t, now, result.UpdatedAt)
	assert.Equal(t, &shardName, result.ShardName)
	db.AssertExpectations(t)
}

func TestS3BucketService_GetByID_NotFound(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3BucketService(db, tc)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, "nonexistent-bucket")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get s3 bucket")
	db.AssertExpectations(t)
}

// ---------- ListByTenant ----------

func TestS3BucketService_ListByTenant_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3BucketService(db, tc)
	ctx := context.Background()

	tenantID := "test-tenant-1"
	id1 := "test-bucket-1"
	shardID := "test-shard-1"
	shardName := "Test Shard"
	now := time.Now().Truncate(time.Microsecond)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = tenantID
			*(dest[2].(*string)) = "" // subscription_id
			*(dest[3].(**string)) = &shardID
			*(dest[4].(*bool)) = false
			*(dest[5].(*int64)) = 1073741824
			*(dest[6].(*string)) = model.StatusActive
			*(dest[7].(**string)) = nil // status_message
			*(dest[8].(*string)) = ""  // suspend_reason
			*(dest[9].(*time.Time)) = now
			*(dest[10].(*time.Time)) = now
			*(dest[11].(**string)) = &shardName
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByTenant(ctx, tenantID, request.ListParams{Limit: 50})
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 1)
	assert.Equal(t, id1, result[0].ID)
	db.AssertExpectations(t)
}

func TestS3BucketService_ListByTenant_QueryError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3BucketService(db, tc)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("connection lost"))

	result, _, err := svc.ListByTenant(ctx, "test-tenant-1", request.ListParams{Limit: 50})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list s3 buckets")
	db.AssertExpectations(t)
}

// ---------- Delete ----------

func TestS3BucketService_Delete_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3BucketService(db, tc)
	ctx := context.Background()

	bucketID := "test-bucket-1"

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	// resolveTenantIDFromS3Bucket
	tenantID := "test-tenant-1"
	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = tenantID
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow).Once()

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Delete(ctx, bucketID)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestS3BucketService_Delete_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3BucketService(db, tc)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Delete(ctx, "test-bucket-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status to deleting")
	db.AssertExpectations(t)
}
