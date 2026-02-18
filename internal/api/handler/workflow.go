package handler

import (
	"net/http"
	"strings"
	"time"

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
// Retries on "not found" errors since the workflow may be a child workflow
// that hasn't started yet (e.g., queued via the entity workflow signal).
func (h *Workflow) Await(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	if workflowID == "" {
		response.WriteError(w, http.StatusBadRequest, "missing workflow ID")
		return
	}

	var lastErr error
	for attempt := range 20 {
		run := h.tc.GetWorkflow(r.Context(), workflowID, "")
		if err := run.Get(r.Context(), nil); err != nil {
			if isWorkflowNotFound(err) && attempt < 19 {
				lastErr = err
				time.Sleep(time.Duration(min(1000, 100*(1<<attempt))) * time.Millisecond)
				continue
			}
			response.WriteServiceError(w, err)
			return
		}
		response.WriteJSON(w, http.StatusOK, map[string]string{"status": "completed"})
		return
	}

	response.WriteError(w, http.StatusInternalServerError, "workflow not found for ID: "+workflowID+": "+lastErr.Error())
}

func isWorkflowNotFound(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "not found") || strings.Contains(msg, "no rows")
}
