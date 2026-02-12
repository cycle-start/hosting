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
	return NewValkeyInstance(nil)
}

// --- Migrate ---

func TestValkeyInstanceMigrate_Success(t *testing.T) {
	db := &handlerMockDB{}
	tc := &temporalmocks.Client{}
	svc := core.NewValkeyInstanceService(db, tc)
	h := NewValkeyInstance(svc)

	db.On("Exec", mock.Anything, mock.AnythingOfType("string"), mock.Anything).Return(pgconn.CommandTag{}, nil)

	wfRun := &temporalmocks.WorkflowRun{}
	wfRun.On("GetID").Return("mock-wf-id")
	wfRun.On("GetRunID").Return("mock-run-id")
	tc.On("ExecuteWorkflow", mock.Anything, mock.Anything, "MigrateValkeyInstanceWorkflow", mock.Anything).Return(wfRun, nil)

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
