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

func TestNewS3AccessKeyService(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3AccessKeyService(db, tc)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
	assert.Equal(t, tc, svc.tc)
}

// ---------- Create ----------

func TestS3AccessKeyService_Create_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3AccessKeyService(db, tc)
	ctx := context.Background()

	key := &model.S3AccessKey{
		ID:          "test-key-1",
		S3BucketID:  "test-bucket-1",
		Permissions: "read-write",
		Status:      model.StatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

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

	err := svc.Create(ctx, key)
	require.NoError(t, err)

	// Assert that AccessKeyID and SecretAccessKey were generated
	assert.NotEmpty(t, key.AccessKeyID)
	assert.NotEmpty(t, key.SecretAccessKey)
	assert.Len(t, key.AccessKeyID, 20)
	assert.Len(t, key.SecretAccessKey, 40)

	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestS3AccessKeyService_Create_InsertError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3AccessKeyService(db, tc)
	ctx := context.Background()

	key := &model.S3AccessKey{
		ID:          "test-key-1",
		S3BucketID:  "test-bucket-1",
		Permissions: "read-write",
		Status:      model.StatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Create(ctx, key)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert s3 access key")
	db.AssertExpectations(t)
}

// ---------- GetByID ----------

func TestS3AccessKeyService_GetByID_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3AccessKeyService(db, tc)
	ctx := context.Background()

	keyID := "test-key-1"
	now := time.Now().Truncate(time.Microsecond)

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = keyID
		*(dest[1].(*string)) = "test-bucket-1"
		*(dest[2].(*string)) = "AKIAIOSFODNN7EXAMPLE"
		*(dest[3].(*string)) = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
		*(dest[4].(*string)) = "read-write"
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
	assert.Equal(t, "test-bucket-1", result.S3BucketID)
	assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", result.AccessKeyID)
	assert.Equal(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", result.SecretAccessKey)
	assert.Equal(t, "read-write", result.Permissions)
	assert.Equal(t, model.StatusActive, result.Status)
	assert.Equal(t, now, result.CreatedAt)
	assert.Equal(t, now, result.UpdatedAt)
	db.AssertExpectations(t)
}

func TestS3AccessKeyService_GetByID_NotFound(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3AccessKeyService(db, tc)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, "nonexistent-key")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get s3 access key")
	db.AssertExpectations(t)
}

// ---------- ListByBucket ----------

func TestS3AccessKeyService_ListByBucket_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3AccessKeyService(db, tc)
	ctx := context.Background()

	bucketID := "test-bucket-1"
	id1 := "test-key-1"
	now := time.Now().Truncate(time.Microsecond)

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = bucketID
			*(dest[2].(*string)) = "AKIAIOSFODNN7EXAMPLE"
			*(dest[3].(*string)) = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
			*(dest[4].(*string)) = "read-write"
			*(dest[5].(*string)) = model.StatusActive
			*(dest[6].(**string)) = nil // status_message
			*(dest[7].(*time.Time)) = now
			*(dest[8].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.ListByBucket(ctx, bucketID, 50, "")
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 1)
	assert.Equal(t, id1, result[0].ID)
	assert.Equal(t, bucketID, result[0].S3BucketID)
	assert.Equal(t, "read-write", result[0].Permissions)
	db.AssertExpectations(t)
}

func TestS3AccessKeyService_ListByBucket_QueryError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3AccessKeyService(db, tc)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("connection lost"))

	result, _, err := svc.ListByBucket(ctx, "test-bucket-1", 50, "")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list s3 access keys")
	db.AssertExpectations(t)
}

// ---------- Delete ----------

func TestS3AccessKeyService_Delete_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3AccessKeyService(db, tc)
	ctx := context.Background()

	keyID := "test-key-1"

	updateRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "AKIAIOSFODNN7EXAMPLE"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(updateRow).Once()

	// resolveTenantIDFromS3AccessKey
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

	err := svc.Delete(ctx, keyID)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestS3AccessKeyService_Delete_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewS3AccessKeyService(db, tc)
	ctx := context.Background()

	errorRow := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("db error")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(errorRow)

	err := svc.Delete(ctx, "test-key-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status to deleting")
	db.AssertExpectations(t)
}
