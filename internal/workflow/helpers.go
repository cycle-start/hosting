package workflow

import (
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// setResourceFailed is a helper to set a resource status to failed.
// It returns any error but callers typically ignore it since the primary
// error is more important.
func setResourceFailed(ctx workflow.Context, table string, id string) error {
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  table,
		ID:     id,
		Status: model.StatusFailed,
	}).Get(ctx, nil)
}
