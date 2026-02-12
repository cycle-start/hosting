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

func TestNewZoneRecordService(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneRecordService(db, tc)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
	assert.Equal(t, tc, svc.tc)
}

// ---------- Create ----------

func TestZoneRecordService_Create_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneRecordService(db, tc)
	ctx := context.Background()

	priority := 10
	record := &model.ZoneRecord{
		ID:        "test-record-1",
		ZoneID:    "test-zone-1",
		Type:      "A",
		Name:      "@",
		Content:   "1.2.3.4",
		TTL:       3600,
		Priority:  &priority,
		ManagedBy: model.ManagedByUser,
		Status:    model.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("ExecuteWorkflow", ctx, mock.Anything, "CreateZoneRecordWorkflow", mock.Anything).Return(wfRun, nil)

	err := svc.Create(ctx, record)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestZoneRecordService_Create_InsertError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneRecordService(db, tc)
	ctx := context.Background()

	record := &model.ZoneRecord{ID: "test-record-1", Type: "A"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Create(ctx, record)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert zone record")
	db.AssertExpectations(t)
}

func TestZoneRecordService_Create_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneRecordService(db, tc)
	ctx := context.Background()

	record := &model.ZoneRecord{ID: "test-record-1", Type: "A"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	tc.On("ExecuteWorkflow", ctx, mock.Anything, "CreateZoneRecordWorkflow", mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Create(ctx, record)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start CreateZoneRecordWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- GetByID ----------

func TestZoneRecordService_GetByID_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneRecordService(db, tc)
	ctx := context.Background()

	recordID := "test-record-1"
	zoneID := "test-zone-1"
	now := time.Now().Truncate(time.Microsecond)
	priority := 10

	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = recordID
		*(dest[1].(*string)) = zoneID
		*(dest[2].(*string)) = "A"
		*(dest[3].(*string)) = "@"
		*(dest[4].(*string)) = "1.2.3.4"
		*(dest[5].(*int)) = 3600
		*(dest[6].(**int)) = &priority
		*(dest[7].(*string)) = model.ManagedByUser
		*(dest[8].(**string)) = nil
		*(dest[9].(*string)) = model.StatusActive
		*(dest[10].(*time.Time)) = now
		*(dest[11].(*time.Time)) = now
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, recordID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, recordID, result.ID)
	assert.Equal(t, "A", result.Type)
	assert.Equal(t, "@", result.Name)
	assert.Equal(t, "1.2.3.4", result.Content)
	assert.Equal(t, 3600, result.TTL)
	assert.Equal(t, &priority, result.Priority)
	db.AssertExpectations(t)
}

func TestZoneRecordService_GetByID_NotFound(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneRecordService(db, tc)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, "nonexistent-record")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get zone record")
	db.AssertExpectations(t)
}

// ---------- ListByZone ----------

func TestZoneRecordService_ListByZone_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneRecordService(db, tc)
	ctx := context.Background()

	zoneID := "test-zone-1"
	id1 := "test-record-1"
	now := time.Now().Truncate(time.Microsecond)
	priority := 10

	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = zoneID
			*(dest[2].(*string)) = "A"
			*(dest[3].(*string)) = "@"
			*(dest[4].(*string)) = "1.2.3.4"
			*(dest[5].(*int)) = 3600
			*(dest[6].(**int)) = &priority
			*(dest[7].(*string)) = model.ManagedByUser
			*(dest[8].(**string)) = nil
			*(dest[9].(*string)) = model.StatusActive
			*(dest[10].(*time.Time)) = now
			*(dest[11].(*time.Time)) = now
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, err := svc.ListByZone(ctx, zoneID)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "A", result[0].Type)
	assert.Equal(t, "@", result[0].Name)
	db.AssertExpectations(t)
}

func TestZoneRecordService_ListByZone_QueryError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneRecordService(db, tc)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("db error"))

	result, err := svc.ListByZone(ctx, "test-zone-1")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list zone records")
	db.AssertExpectations(t)
}

// ---------- Update ----------

func TestZoneRecordService_Update_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneRecordService(db, tc)
	ctx := context.Background()

	record := &model.ZoneRecord{
		ID:      "test-record-1",
		Type:    "CNAME",
		Name:    "www",
		Content: "example.com",
		TTL:     7200,
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("ExecuteWorkflow", ctx, mock.Anything, "UpdateZoneRecordWorkflow", mock.Anything).Return(wfRun, nil)

	err := svc.Update(ctx, record)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestZoneRecordService_Update_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneRecordService(db, tc)
	ctx := context.Background()

	record := &model.ZoneRecord{ID: "test-record-1"}
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Update(ctx, record)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update zone record")
	db.AssertExpectations(t)
}

func TestZoneRecordService_Update_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneRecordService(db, tc)
	ctx := context.Background()

	record := &model.ZoneRecord{ID: "test-record-1"}
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	tc.On("ExecuteWorkflow", ctx, mock.Anything, "UpdateZoneRecordWorkflow", mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Update(ctx, record)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start UpdateZoneRecordWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- Delete ----------

func TestZoneRecordService_Delete_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneRecordService(db, tc)
	ctx := context.Background()

	recordID := "test-record-1"

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("ExecuteWorkflow", ctx, mock.Anything, "DeleteZoneRecordWorkflow", mock.Anything).Return(wfRun, nil)

	err := svc.Delete(ctx, recordID)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestZoneRecordService_Delete_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneRecordService(db, tc)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Delete(ctx, "test-record-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status to deleting")
	db.AssertExpectations(t)
}

func TestZoneRecordService_Delete_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneRecordService(db, tc)
	ctx := context.Background()

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	tc.On("ExecuteWorkflow", ctx, mock.Anything, "DeleteZoneRecordWorkflow", mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Delete(ctx, "test-record-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start DeleteZoneRecordWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}
