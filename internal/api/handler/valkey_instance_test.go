package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/edvin/hosting/internal/core"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	temporalmocks "go.temporal.io/sdk/mocks"
)

func newValkeyInstanceHandler() *ValkeyInstance {
	return &ValkeyInstance{svc: nil, userSvc: nil, tenantSvc: nil}
}

// --- Create with nested ---

func TestValkeyInstanceCreate_WithNestedUsers_ValidationPasses(t *testing.T) {
	h := newValkeyInstanceHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequest(http.MethodPost, "/tenants/"+tid+"/valkey-instances", map[string]any{
		"subscription_id": "sub-1",
		"shard_id":        "test-shard-1",
		"users": []map[string]any{
			{
				"username":    "cacheuser",
				"password":    "securepassword123",
				"privileges":  []string{"allcommands"},
				"key_pattern": "~app:*",
			},
			{
				"username":   "readonly",
				"password":   "anotherpassword1",
				"privileges": []string{"get"},
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

func TestValkeyInstanceCreate_WithInvalidNestedUser_ValidationFails(t *testing.T) {
	h := newValkeyInstanceHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequest(http.MethodPost, "/tenants/"+tid+"/valkey-instances", map[string]any{
		"shard_id": "test-shard-1",
		"users": []map[string]any{
			{
				"username": "cacheuser",
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

func TestValkeyInstanceCreate_WithNestedUserShortPassword_ValidationFails(t *testing.T) {
	h := newValkeyInstanceHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequest(http.MethodPost, "/tenants/"+tid+"/valkey-instances", map[string]any{
		"shard_id": "test-shard-1",
		"users": []map[string]any{
			{
				"username":   "cacheuser",
				"password":   "short", // too short, min=8
				"privileges": []string{"allcommands"},
			},
		},
	})
	r = withChiURLParam(r, "tenantID", tid)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

// --- Migrate ---

func TestValkeyInstanceMigrate_Success(t *testing.T) {
	db := &handlerMockDB{}
	tenantDB := &handlerMockDB{}
	tc := &temporalmocks.Client{}
	svc := core.NewValkeyInstanceService(db, tc)
	tenantSvc := core.NewTenantService(tenantDB, tc)
	h := &ValkeyInstance{svc: svc, userSvc: nil, tenantSvc: tenantSvc}

	tenantID := "test-tenant-1"

	// GetByID call for the valkey instance
	now := time.Now()
	getRow := &handlerMockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = validID     // ID
		*(dest[1].(*string)) = tenantID    // TenantID
		*(dest[2].(*string)) = ""          // SubscriptionID
		*(dest[3].(**string)) = nil        // ShardID
		*(dest[4].(*int)) = 0              // Port
		*(dest[5].(*int)) = 64             // MaxMemoryMB
		*(dest[6].(*string)) = ""          // Password
		*(dest[7].(*string)) = "active"    // Status
		*(dest[8].(**string)) = nil        // StatusMessage
		*(dest[9].(*string)) = ""          // SuspendReason
		*(dest[10].(*time.Time)) = now     // CreatedAt
		*(dest[11].(*time.Time)) = now     // UpdatedAt
		*(dest[12].(**string)) = nil       // ShardName
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

	// Migrate: resolveTenantIDFromValkeyInstance QueryRow.
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
	r := newRequest(http.MethodPost, "/valkey-instances/"+validID+"/migrate", map[string]any{
		"target_shard_id": "test-shard-2",
	})
	r = withChiURLParam(r, "id", validID)
	r = withPlatformAdmin(r)

	h.Migrate(rec, r)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	db.AssertExpectations(t)
	tc.AssertExpectations(t)
}

func TestValkeyInstanceMigrate_BadID(t *testing.T) {
	h := newValkeyInstanceHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/valkey-instances//migrate", map[string]any{
		"target_shard_id": "test-shard-2",
	})
	r = withChiURLParam(r, "id", "")

	h.Migrate(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}
