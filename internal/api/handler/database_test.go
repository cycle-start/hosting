package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/edvin/hosting/internal/core"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	temporalmocks "go.temporal.io/sdk/mocks"
)

func newDatabaseHandler() *Database {
	return &Database{svc: nil, userSvc: nil, tenantSvc: nil}
}

// --- ListByTenant ---

func TestDatabaseListByTenant_EmptyID(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/tenants//databases", nil)
	r = withChiURLParam(r, "tenantID", "")

	h.ListByTenant(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Create ---

func TestDatabaseCreate_EmptyTenantID(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants//databases", map[string]any{
		"shard_id": validID,
	})
	r = withChiURLParam(r, "tenantID", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestDatabaseCreate_InvalidJSON(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants/"+validID+"/databases", "{bad json")
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestDatabaseCreate_EmptyBody(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants/"+validID+"/databases", "")
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDatabaseCreate_MissingShardID(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/databases", map[string]any{})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestDatabaseCreate_ValidBody(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequest(http.MethodPost, "/tenants/"+tid+"/databases", map[string]any{
		"subscription_id": "sub-1",
		"shard_id":        "test-shard-1",
	})
	r = withChiURLParam(r, "tenantID", tid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- Nested resource validation ---

func TestDatabaseCreate_WithNestedUsers_ValidationPasses(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequest(http.MethodPost, "/tenants/"+tid+"/databases", map[string]any{
		"subscription_id": "sub-1",
		"shard_id":        "test-shard-1",
		"users": []map[string]any{
			{
				"username":   "db_placeholder_admin",
				"password":   "securepassword123",
				"privileges": []string{"ALL"},
			},
			{
				"username":   "db_placeholder_readonly",
				"password":   "anotherpassword1",
				"privileges": []string{"SELECT"},
			},
		},
	})
	r = withChiURLParam(r, "tenantID", tid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestDatabaseCreate_WithInvalidNestedUser_ValidationFails(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequest(http.MethodPost, "/tenants/"+tid+"/databases", map[string]any{
		"shard_id": "test-shard-1",
		"users": []map[string]any{
			{
				"username":   "admin",
				"password":   "short", // too short, min=8
				"privileges": []string{"ALL"},
			},
		},
	})
	r = withChiURLParam(r, "tenantID", tid)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestDatabaseCreate_WithNestedUserMissingPrivileges_ValidationFails(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequest(http.MethodPost, "/tenants/"+tid+"/databases", map[string]any{
		"shard_id": "test-shard-1",
		"users": []map[string]any{
			{
				"username": "admin",
				"password": "securepassword123",
				// missing privileges
			},
		},
	})
	r = withChiURLParam(r, "tenantID", tid)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

// --- Get ---

func TestDatabaseGet_EmptyID(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/databases/", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Delete ---

func TestDatabaseDelete_EmptyID(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/databases/", nil)
	r = withChiURLParam(r, "id", "")

	h.Delete(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Migrate ---

func TestDatabaseMigrate_Success(t *testing.T) {
	db := &handlerMockDB{}
	tenantDB := &handlerMockDB{}
	tc := &temporalmocks.Client{}
	svc := core.NewDatabaseService(db, tc)
	tenantSvc := core.NewTenantService(tenantDB, tc)
	h := &Database{svc: svc, userSvc: nil, tenantSvc: tenantSvc}

	tenantID := "test-tenant-1"

	// GetByID call for the database
	now := time.Now()
	getRow := &handlerMockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = validID    // ID
		*(dest[1].(*string)) = tenantID   // TenantID
		*(dest[2].(*string)) = ""         // SubscriptionID
		*(dest[3].(**string)) = nil       // ShardID
		*(dest[4].(**string)) = nil       // NodeID
		*(dest[5].(*string)) = "active"   // Status
		*(dest[6].(**string)) = nil       // StatusMessage
		*(dest[7].(*string)) = ""         // SuspendReason
		*(dest[8].(*time.Time)) = now     // CreatedAt
		*(dest[9].(*time.Time)) = now     // UpdatedAt
		*(dest[10].(**string)) = nil      // ShardName
		return nil
	}}
	db.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(getRow).Once()

	// Brand check: tenant GetByID
	// Scan order: ID, BrandID, CustomerID, RegionID, ClusterID, ShardID, UID,
	//   SFTPEnabled, SSHEnabled, DiskQuotaBytes, Status, StatusMessage, SuspendReason,
	//   CreatedAt, UpdatedAt, RegionName, ClusterName, ShardName
	tenantRow := &handlerMockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = tenantID     // ID
		*(dest[1].(*string)) = "test-brand" // BrandID
		*(dest[2].(*string)) = ""           // CustomerID
		*(dest[3].(*string)) = "dev"        // RegionID
		*(dest[4].(*string)) = "dev"        // ClusterID
		*(dest[5].(**string)) = nil         // ShardID
		*(dest[6].(*int)) = 1000            // UID
		*(dest[7].(*bool)) = false          // SFTPEnabled
		*(dest[8].(*bool)) = false          // SSHEnabled
		*(dest[9].(*int64)) = int64(0)      // DiskQuotaBytes
		*(dest[10].(*string)) = "active"    // Status
		*(dest[11].(**string)) = nil        // StatusMessage
		*(dest[12].(*string)) = ""          // SuspendReason
		*(dest[13].(*time.Time)) = now      // CreatedAt
		*(dest[14].(*time.Time)) = now      // UpdatedAt
		*(dest[15].(*string)) = "dev"       // RegionName
		*(dest[16].(*string)) = "dev"       // ClusterName
		*(dest[17].(**string)) = nil        // ShardName
		return nil
	}}
	tenantDB.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(tenantRow).Once()

	// Migrate: Exec to update status to provisioning.
	db.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.NewCommandTag("UPDATE 1"), nil).Once()

	// Migrate: resolveTenantIDFromDatabase QueryRow.
	resolveRow := &handlerMockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = tenantID
		return nil
	}}
	db.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow).Once()

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("SignalWithStartWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(wfRun, nil)

	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/databases/"+validID+"/migrate", map[string]any{
		"target_shard_id": "test-shard-2",
	})
	r = withChiURLParam(r, "id", validID)
	r = withPlatformAdmin(r)

	h.Migrate(rec, r)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestDatabaseMigrate_BadID(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/databases//migrate", map[string]any{
		"target_shard_id": "test-shard-2",
	})
	r = withChiURLParam(r, "id", "")

	h.Migrate(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestDatabaseMigrate_MissingTargetShard(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/databases/"+validID+"/migrate", map[string]any{})
	r = withChiURLParam(r, "id", validID)

	h.Migrate(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

// --- Error response format ---

func TestDatabaseCreate_ErrorResponseFormat(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants/"+validID+"/databases", "{bad")
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	_, hasError := body["error"]
	assert.True(t, hasError)
}
