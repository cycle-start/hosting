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
		"shard_id": "test-shard-1",
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
		"shard_id": "test-shard-1",
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

// --- ReassignTenant ---

func TestDatabaseReassignTenant_EmptyID(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/databases//reassign", map[string]any{
		"tenant_id": validID,
	})
	r = withChiURLParam(r, "id", "")

	h.ReassignTenant(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestDatabaseReassignTenant_InvalidJSON(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/databases/"+validID+"/reassign", "{bad json")
	r = withChiURLParam(r, "id", validID)

	h.ReassignTenant(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestDatabaseReassignTenant_EmptyBody(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/databases/"+validID+"/reassign", "")
	r = withChiURLParam(r, "id", validID)

	h.ReassignTenant(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Migrate ---

func TestDatabaseMigrate_Success(t *testing.T) {
	db := &handlerMockDB{}
	tc := &temporalmocks.Client{}
	svc := core.NewDatabaseService(db, tc)
	h := &Database{svc: svc, userSvc: nil, tenantSvc: nil}

	// GetByID call from brand check (return database with nil TenantID to skip brand check)
	now := time.Now()
	getRow := &handlerMockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = validID
		*(dest[1].(**string)) = nil // nil TenantID — skips brand check
		*(dest[2].(*string)) = "mydb"
		*(dest[3].(**string)) = nil
		*(dest[4].(**string)) = nil
		*(dest[5].(*string)) = "active"
		*(dest[6].(**string)) = nil
		*(dest[7].(*time.Time)) = now
		*(dest[8].(*time.Time)) = now
		*(dest[9].(**string)) = nil
		return nil
	}}
	db.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(getRow).Once()

	updateRow := &handlerMockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "mydb"
		return nil
	}}
	db.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(updateRow).Once()

	resolveRow := &handlerMockRow{scanFunc: func(dest ...any) error {
		tid := "test-tenant-1"
		*(dest[0].(**string)) = &tid
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
