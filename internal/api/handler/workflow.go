package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/api/response"
	"github.com/go-chi/chi/v5"
	temporalclient "go.temporal.io/sdk/client"
)

type Workflow struct {
	tc temporalclient.Client
}

func NewWorkflow(tc temporalclient.Client) *Workflow {
	return &Workflow{tc: tc}
}

// Await blocks until the specified Temporal workflow completes.
// Returns 200 on success, 500 if the workflow failed.
func (h *Workflow) Await(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	if workflowID == "" {
		response.WriteError(w, http.StatusBadRequest, "missing workflow ID")
		return
	}

	run := h.tc.GetWorkflow(r.Context(), workflowID, "")
	if err := run.Get(r.Context(), nil); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}
