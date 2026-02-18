package workflow

import (
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/model"
)

const maxOpsBeforeContinueAsNew = 100

// TenantProvisionWorkflow is a per-tenant entity workflow that serializes all
// workflow executions for a given tenant. It receives ProvisionTask signals and
// executes each as a child workflow, one at a time. The workflow completes
// when idle for 30 seconds and uses ContinueAsNew after processing a batch of
// operations to prevent unbounded history growth.
func TenantProvisionWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	ch := workflow.GetSignalChannel(ctx, model.ProvisionSignalName)
	opsCount := 0

	for {
		if opsCount >= maxOpsBeforeContinueAsNew {
			logger.Info("continuing as new after max operations", "ops", opsCount)
			return workflow.NewContinueAsNewError(ctx, TenantProvisionWorkflow)
		}

		var task model.ProvisionTask
		ok, _ := ch.ReceiveWithTimeout(ctx, 30*time.Second, &task)
		if !ok {
			logger.Info("idle timeout, completing", "ops", opsCount)
			return nil
		}

		logger.Info("executing task", "workflow", task.WorkflowName, "id", task.WorkflowID)

		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: task.WorkflowID,
			TaskQueue:  "hosting-tasks",
		})
		if err := workflow.ExecuteChildWorkflow(childCtx, task.WorkflowName, task.Arg).Get(ctx, nil); err != nil {
			logger.Error("child workflow failed", "workflow", task.WorkflowName, "id", task.WorkflowID, "error", err)
			// Don't return error — continue processing next signal.
			// The child workflow is responsible for setting its own failed status.
		}

		opsCount++
	}
}
