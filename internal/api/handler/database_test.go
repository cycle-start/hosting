package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/edvin/hosting/internal/core"
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
		*(dest[0].(*string)) = validID
		*(dest[1].(*string)) = tenantID
		*(dest[2].(*string)) = "" // SubscriptionID
		*(dest[3].(*string)) = "mydb"
		*(dest[4].(**string)) = nil
		*(dest[5].(**string)) = nil
		*(dest[6].(*string)) = "active"
		*(dest[7].(**string)) = nil
		*(dest[8].(*string)) = ""
		*(dest[9].(*time.Time)) = now
		*(dest[10].(*time.Time)) = now
		*(dest[11].(**string)) = nil
		return nil
	}}
	db.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(getRow).Once()

	// Brand check: tenant GetByID
	// Scan order: ID, Name, BrandID, CustomerID, RegionID, ClusterID, ShardID, UID,
	//   SFTPEnabled, SSHEnabled, DiskQuotaBytes, Status, StatusMessage, SuspendReason,
	//   CreatedAt, UpdatedAt, RegionName, ClusterName, ShardName
	tenantRow := &handlerMockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = tenantID         // ID
		*(dest[1].(*string)) = "t_testtenant01" // Name
		*(dest[2].(*string)) = "test-brand"     // BrandID
		*(dest[3].(*string)) = ""               // CustomerID
		*(dest[4].(*string)) = "dev"            // RegionID
		*(dest[5].(*string)) = "dev"            // ClusterID
		*(dest[6].(**string)) = nil             // ShardID
		*(dest[7].(*int)) = 1000                // UID
		*(dest[8].(*bool)) = false              // SFTPEnabled
		*(dest[9].(*bool)) = false              // SSHEnabled
		*(dest[10].(*int64)) = int64(0)         // DiskQuotaBytes
		*(dest[11].(*string)) = "active"        // Status
		*(dest[12].(**string)) = nil            // StatusMessage
		*(dest[13].(*string)) = ""              // SuspendReason
		*(dest[14].(*time.Time)) = now          // CreatedAt
		*(dest[15].(*time.Time)) = now          // UpdatedAt
		*(dest[16].(*string)) = "dev"           // RegionName
		*(dest[17].(*string)) = "dev"           // ClusterName
		*(dest[18].(**string)) = nil            // ShardName
		return nil
	}}
	tenantDB.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(tenantRow).Once()

	updateRow := &handlerMockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "mydb"
		return nil
	}}
	db.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(updateRow).Once()

	resolveRow := &handlerMockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = tenantID
		return nil
	}}
	db.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow).Once()

	tenantNameRow := &handlerMockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "t_testtenant01"
		return nil
	}}
	db.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(tenantNameRow).Once()

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
