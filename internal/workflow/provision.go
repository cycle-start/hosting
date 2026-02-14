package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// TenantProvisionWorkflow is a long-running per-tenant orchestrator that
// processes provisioning tasks sequentially. Tasks are submitted via the
// "provision" signal and executed as child workflows one at a time.
//
// The workflow idles for up to 5 minutes between tasks. If no new task
// arrives within that window, the workflow completes gracefully. A new
// run is automatically started by SignalWithStartWorkflow when the next
// task is enqueued.
//
// After 1000 iterations the workflow uses ContinueAsNew to keep the
// event history bounded. Unread signals are carried over automatically
// by Temporal.
func TenantProvisionWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	signalCh := workflow.GetSignalChannel(ctx, model.ProvisionSignalName)

	iteration := 0
	const maxIterations = 1000

	for {
		// Drain any buffered signals first.
		for {
			var task model.ProvisionTask
			if !signalCh.ReceiveAsync(&task) {
				break
			}
			if err := executeProvisionTask(ctx, task); err != nil {
				logger.Error("provision task failed",
					"workflow", task.WorkflowName,
					"id", task.WorkflowID,
					"error", err)
			}
			iteration++
			if iteration >= maxIterations {
				return workflow.NewContinueAsNewError(ctx, TenantProvisionWorkflow)
			}
		}

		// No buffered signals — wait for a new signal or idle timeout.
		var task model.ProvisionTask
		gotSignal := false

		selector := workflow.NewSelector(ctx)
		selector.AddReceive(signalCh, func(c workflow.ReceiveChannel, _ bool) {
			c.Receive(ctx, &task)
			gotSignal = true
		})
		selector.AddFuture(workflow.NewTimer(ctx, 5*time.Minute), func(workflow.Future) {})
		selector.Select(ctx)

		if !gotSignal {
			return nil
		}

		if err := executeProvisionTask(ctx, task); err != nil {
			logger.Error("provision task failed",
				"workflow", task.WorkflowName,
				"id", task.WorkflowID,
				"error", err)
		}
		iteration++
		if iteration >= maxIterations {
			return workflow.NewContinueAsNewError(ctx, TenantProvisionWorkflow)
		}
	}
}

func executeProvisionTask(ctx workflow.Context, task model.ProvisionTask) error {
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID: task.WorkflowID,
		TaskQueue:  "hosting-tasks",
	})
	childErr := workflow.ExecuteChildWorkflow(childCtx, task.WorkflowName, task.Arg).Get(ctx, nil)

	if task.CallbackURL != "" {
		fireCallback(ctx, task, childErr)
	}

	return childErr
}

// fireCallback sends a callback notification after a provisioning task completes.
// It is best-effort: callback failures are logged but never block the orchestrator.
func fireCallback(ctx workflow.Context, task model.ProvisionTask, childErr error) {
	logger := workflow.GetLogger(ctx)

	payload := model.CallbackPayload{
		ResourceType: task.ResourceType,
		ResourceID:   task.ResourceID,
		Status:       model.StatusActive,
	}
	if childErr != nil {
		payload.Status = model.StatusFailed
		payload.StatusMessage = childErr.Error()
	}

	callbackCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    10,
			InitialInterval:    5 * time.Second,
			MaximumInterval:    5 * time.Minute,
			BackoffCoefficient: 2.0,
		},
	})

	err := workflow.ExecuteActivity(callbackCtx, "SendCallback", activity.SendCallbackParams{
		URL:     task.CallbackURL,
		Payload: payload,
	}).Get(ctx, nil)
	if err != nil {
		logger.Error("callback failed",
			"url", task.CallbackURL,
			"resource_type", task.ResourceType,
			"resource_id", task.ResourceID,
			"error", err)
	}
}
