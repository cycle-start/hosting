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

func TestNewZoneService(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneService(db, tc)

	require.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
	assert.Equal(t, tc, svc.tc)
}

// ---------- Create ----------

func TestZoneService_Create_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneService(db, tc)
	ctx := context.Background()

	tenantID := "test-tenant-1"
	zone := &model.Zone{
		ID:        "test-zone-1",
		BrandID:   "test-brand",
		TenantID:  tenantID,
		Name:      "example.com",
		RegionID:  "test-region-1",
		Status:    model.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	tenantNameRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "t_testtenant01"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(tenantNameRow)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Create(ctx, zone)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestZoneService_Create_InsertError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneService(db, tc)
	ctx := context.Background()

	zone := &model.Zone{ID: "test-zone-1", BrandID: "test-brand", Name: "example.com"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("duplicate"))

	err := svc.Create(ctx, zone)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert zone")
	db.AssertExpectations(t)
}

func TestZoneService_Create_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneService(db, tc)
	ctx := context.Background()

	zone := &model.Zone{ID: "test-zone-1", BrandID: "test-brand", Name: "example.com"}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)
	tc.On("ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Create(ctx, zone)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signal CreateZoneWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

// ---------- GetByID ----------

func TestZoneService_GetByID_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneService(db, tc)
	ctx := context.Background()

	zoneID := "test-zone-1"
	tenantID := "test-tenant-1"
	regionID := "test-region-1"
	regionName := "Test Region"
	now := time.Now().Truncate(time.Microsecond)

	tenantName := "Test Tenant"
	row := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = zoneID
		*(dest[1].(*string)) = "test-brand"
		*(dest[2].(*string)) = tenantID
		*(dest[3].(*string)) = "" // subscription_id
		*(dest[4].(*string)) = "example.com"
		*(dest[5].(*string)) = regionID
		*(dest[6].(*string)) = model.StatusActive
		*(dest[7].(**string)) = nil // status_message
		*(dest[8].(*string)) = ""  // suspend_reason
		*(dest[9].(*time.Time)) = now
		*(dest[10].(*time.Time)) = now
		*(dest[11].(*string)) = regionName
		*(dest[12].(**string)) = &tenantName
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, zoneID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, zoneID, result.ID)
	assert.Equal(t, "example.com", result.Name)
	assert.Equal(t, tenantID, result.TenantID)
	assert.Equal(t, regionName, result.RegionName)
	db.AssertExpectations(t)
}

func TestZoneService_GetByID_NotFound(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneService(db, tc)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("no rows in result set")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	result, err := svc.GetByID(ctx, "nonexistent-zone")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get zone")
	db.AssertExpectations(t)
}

// ---------- List ----------

func TestZoneService_List_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneService(db, tc)
	ctx := context.Background()

	id1 := "test-zone-1"
	tenantID := "test-tenant-1"
	regionID := "test-region-1"
	regionName := "Test Region"
	now := time.Now().Truncate(time.Microsecond)

	tenantName := "Test Tenant"
	rows := newMockRows(
		func(dest ...any) error {
			*(dest[0].(*string)) = id1
			*(dest[1].(*string)) = "test-brand"
			*(dest[2].(*string)) = tenantID
			*(dest[3].(*string)) = "" // subscription_id
			*(dest[4].(*string)) = "example.com"
			*(dest[5].(*string)) = regionID
			*(dest[6].(*string)) = model.StatusActive
			*(dest[7].(**string)) = nil // status_message
			*(dest[8].(*string)) = ""  // suspend_reason
			*(dest[9].(*time.Time)) = now
			*(dest[10].(*time.Time)) = now
			*(dest[11].(*string)) = regionName
			*(dest[12].(**string)) = &tenantName
			return nil
		},
	)
	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(rows, nil)

	result, hasMore, err := svc.List(ctx, request.ListParams{Limit: 50})
	require.NoError(t, err)
	assert.False(t, hasMore)
	require.Len(t, result, 1)
	assert.Equal(t, "example.com", result[0].Name)
	db.AssertExpectations(t)
}

func TestZoneService_List_QueryError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneService(db, tc)
	ctx := context.Background()

	db.On("Query", ctx, mock.AnythingOfType("string"), mock.Anything).Return(nil, errors.New("connection lost"))

	result, _, err := svc.List(ctx, request.ListParams{Limit: 50})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "list zones")
	db.AssertExpectations(t)
}

// ---------- Update ----------

func TestZoneService_Update_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneService(db, tc)
	ctx := context.Background()

	zone := &model.Zone{ID: "test-zone-1", BrandID: "test-brand", Name: "updated.com", Status: model.StatusActive}

	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	err := svc.Update(ctx, zone)
	require.NoError(t, err)
	db.AssertExpectations(t)
}

func TestZoneService_Update_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneService(db, tc)
	ctx := context.Background()

	zone := &model.Zone{ID: "test-zone-1", BrandID: "test-brand"}
	db.On("Exec", ctx, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, errors.New("db error"))

	err := svc.Update(ctx, zone)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update zone")
	db.AssertExpectations(t)
}

// ---------- Delete ----------

func TestZoneService_Delete_Success(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneService(db, tc)
	ctx := context.Background()

	zoneID := "test-zone-1"

	updateRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "example.com"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(updateRow).Once()

	// resolveTenantIDFromZone
	tenantID := "test-tenant-1"
	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = tenantID
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow).Once()

	// signalProvision tenant name lookup
	tenantNameRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "t_testtenant01"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(tenantNameRow).Once()

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	err := svc.Delete(ctx, zoneID)
	require.NoError(t, err)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestZoneService_Delete_DBError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneService(db, tc)
	ctx := context.Background()

	row := &mockRow{scanFunc: func(dest ...any) error {
		return errors.New("db error")
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(row)

	err := svc.Delete(ctx, "test-zone-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status to deleting")
	db.AssertExpectations(t)
}

func TestZoneService_Delete_WorkflowError(t *testing.T) {
	db := &mockDB{}
	tc := &temporalmocks.Client{}
	svc := NewZoneService(db, tc)
	ctx := context.Background()

	updateRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "example.com"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(updateRow).Once()

	// resolveTenantIDFromZone
	tenantID := "test-tenant-1"
	resolveRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = tenantID
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow).Once()

	// signalProvision tenant name lookup
	tenantNameRow := &mockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "t_testtenant01"
		return nil
	}}
	db.On("QueryRow", ctx, mock.AnythingOfType("string"), mock.Anything).Return(tenantNameRow).Once()

	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("temporal down"))

	err := svc.Delete(ctx, "test-zone-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "signal DeleteZoneWorkflow")
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

