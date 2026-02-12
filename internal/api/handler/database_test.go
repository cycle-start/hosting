package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/edvin/hosting/internal/core"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	temporalmocks "go.temporal.io/sdk/mocks"
)

func newDatabaseHandler() *Database {
	return NewDatabase(nil)
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
		"name":     "mydb",
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

func TestDatabaseCreate_MissingRequiredFields(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/databases", map[string]any{})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestDatabaseCreate_MissingName(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/databases", map[string]any{
		"shard_id": validID,
	})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestDatabaseCreate_MissingShardID(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants/"+validID+"/databases", map[string]any{
		"name": "mydb",
	})
	r = withChiURLParam(r, "tenantID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestDatabaseCreate_InvalidSlugName(t *testing.T) {
	tests := []struct {
		name string
		slug string
	}{
		{"uppercase", "MyDatabase"},
		{"spaces", "my database"},
		{"special chars", "my@db"},
		{"starts with digit", "1database"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newDatabaseHandler()
			rec := httptest.NewRecorder()
			r := newRequest(http.MethodPost, "/tenants/"+validID+"/databases", map[string]any{
				"name":     tt.slug,
				"shard_id": validID,
			})
			r = withChiURLParam(r, "tenantID", validID)

			h.Create(rec, r)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestDatabaseCreate_ValidBody(t *testing.T) {
	h := newDatabaseHandler()
	rec := httptest.NewRecorder()
	tid := "test-tenant-1"
	r := newRequest(http.MethodPost, "/tenants/"+tid+"/databases", map[string]any{
		"name":     "mydb",
		"shard_id": "test-shard-1",
	})
	r = withChiURLParam(r, "tenantID", tid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
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
	h := NewDatabase(svc)

	db.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("ExecuteWorkflow", mock.Anything, mock.Anything, "MigrateDatabaseWorkflow", mock.Anything).Return(wfRun, nil)

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
