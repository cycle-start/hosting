package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/edvin/hosting/internal/core"
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
		"shard_id": "test-shard-1",
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
	tc := &temporalmocks.Client{}
	svc := core.NewValkeyInstanceService(db, tc)
	h := &ValkeyInstance{svc: svc, userSvc: nil, tenantSvc: nil}

	// GetByID call from brand check (return instance with nil TenantID to skip brand check)
	now := time.Now()
	getRow := &handlerMockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = validID
		*(dest[1].(**string)) = nil // nil TenantID — skips brand check
		*(dest[2].(*string)) = "my-valkey"
		*(dest[3].(**string)) = nil
		*(dest[4].(*int)) = 0
		*(dest[5].(*int)) = 64
		*(dest[6].(*string)) = ""
		*(dest[7].(*string)) = "active"
		*(dest[8].(**string)) = nil
		*(dest[9].(*string)) = ""
		*(dest[10].(*time.Time)) = now
		*(dest[11].(*time.Time)) = now
		*(dest[12].(**string)) = nil
		return nil
	}}
	db.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(getRow).Once()

	updateRow := &handlerMockRow{scanFunc: func(dest ...any) error {
		*(dest[0].(*string)) = "my-valkey"
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
	r := newRequest(http.MethodPost, "/valkey-instances/"+validID+"/migrate", map[string]any{
		"target_shard_id": "test-shard-2",
	})
	r = withChiURLParam(r, "id", validID)

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
