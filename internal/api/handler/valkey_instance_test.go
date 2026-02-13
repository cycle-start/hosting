package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/edvin/hosting/internal/core"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	temporalmocks "go.temporal.io/sdk/mocks"
)

func newValkeyInstanceHandler() *ValkeyInstance {
	return &ValkeyInstance{svc: nil, userSvc: nil}
}

// --- Create with nested ---

func TestValkeyInstanceCreate_WithNestedUsers_ValidationPasses(t *testing.T) {
	h := newValkeyInstanceHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequest(http.MethodPost, "/tenants/"+tid+"/valkey-instances", map[string]any{
		"name":     "my-cache",
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
		"name":     "my-cache",
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
		"name":     "my-cache",
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
	h := &ValkeyInstance{svc: svc, userSvc: nil}

	resolveRow := &handlerMockRow{scanFunc: func(dest ...any) error {
		tid := "test-tenant-1"
		*(dest[0].(**string)) = &tid
		return nil
	}}
	db.On("QueryRow", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(resolveRow)
	db.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

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
