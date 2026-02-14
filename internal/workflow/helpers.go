package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// setResourceFailed is a helper to set a resource status to failed with an error message.
// It returns any error but callers typically ignore it since the primary
// error is more important.
func setResourceFailed(ctx workflow.Context, table string, id string, err error) error {
	msg := err.Error()
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:         table,
		ID:            id,
		Status:        model.StatusFailed,
		StatusMessage: &msg,
	}).Get(ctx, nil)
}

// nodeActivityCtx returns a workflow context that routes activity execution
// to a specific node's Temporal task queue. Activities dispatched with this
// context will be picked up by the node-agent worker running on that node.
func nodeActivityCtx(ctx workflow.Context, nodeID string) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:              "node-" + nodeID,
		StartToCloseTimeout:    2 * time.Minute,
		ScheduleToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    5,
			InitialInterval:    5 * time.Second,
			MaximumInterval:    30 * time.Second,
			BackoffCoefficient: 2.0,
		},
	})
}
