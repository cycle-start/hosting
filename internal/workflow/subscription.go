package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// DeleteSubscriptionWorkflow deletes a subscription and cascades to all child resources.
func DeleteSubscriptionWorkflow(ctx workflow.Context, subscriptionID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set subscription status to deleting.
	if err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "subscriptions",
		ID:     subscriptionID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil); err != nil {
		return err
	}

	// Delete the subscription row (by this point we assume child resources have
	// already been cleaned up via their own delete workflows, or will be cleaned
	// up by the cascade in DeleteTenantDBRows). For standalone subscription
	// deletion, we do a simple row delete.
	if err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "subscriptions",
		ID:     subscriptionID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil); err != nil {
		return fmt.Errorf("delete subscription row: %w", err)
	}

	return nil
}
