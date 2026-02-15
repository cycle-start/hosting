package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateValkeyUserWorkflow provisions a new Valkey user on the node agent.
func CreateValkeyUserWorkflow(ctx workflow.Context, userID string) error {
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

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "valkey_users",
		ID:     userID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the valkey user, instance, and nodes.
	var vctx activity.ValkeyUserContext
	err = workflow.ExecuteActivity(ctx, "GetValkeyUserContext", userID).Get(ctx, &vctx)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_users", userID, err)
		return err
	}

	if vctx.Instance.ShardID == nil {
		noShardErr := fmt.Errorf("valkey instance %s has no shard assigned", vctx.User.ValkeyInstanceID)
		_ = setResourceFailed(ctx, "valkey_users", userID, noShardErr)
		return noShardErr
	}

	// Create valkey user on each node in the shard.
	for _, node := range vctx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "CreateValkeyUser", activity.CreateValkeyUserParams{
			InstanceName: vctx.Instance.Name,
			Port:         vctx.Instance.Port,
			Username:     vctx.User.Username,
			Password:     vctx.User.Password,
			Privileges:   vctx.User.Privileges,
			KeyPattern:   vctx.User.KeyPattern,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "valkey_users", userID, err)
			return err
		}
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "valkey_users",
		ID:     userID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// UpdateValkeyUserWorkflow updates a Valkey user on the node agent.
func UpdateValkeyUserWorkflow(ctx workflow.Context, userID string) error {
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

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "valkey_users",
		ID:     userID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the valkey user, instance, and nodes.
	var vctx activity.ValkeyUserContext
	err = workflow.ExecuteActivity(ctx, "GetValkeyUserContext", userID).Get(ctx, &vctx)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_users", userID, err)
		return err
	}

	if vctx.Instance.ShardID == nil {
		noShardErr := fmt.Errorf("valkey instance %s has no shard assigned", vctx.User.ValkeyInstanceID)
		_ = setResourceFailed(ctx, "valkey_users", userID, noShardErr)
		return noShardErr
	}

	// Update valkey user on each node in the shard.
	for _, node := range vctx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "UpdateValkeyUser", activity.UpdateValkeyUserParams{
			InstanceName: vctx.Instance.Name,
			Port:         vctx.Instance.Port,
			Username:     vctx.User.Username,
			Password:     vctx.User.Password,
			Privileges:   vctx.User.Privileges,
			KeyPattern:   vctx.User.KeyPattern,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "valkey_users", userID, err)
			return err
		}
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "valkey_users",
		ID:     userID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteValkeyUserWorkflow deletes a Valkey user from the node agent.
func DeleteValkeyUserWorkflow(ctx workflow.Context, userID string) error {
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

	// Set status to deleting.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "valkey_users",
		ID:     userID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the valkey user, instance, and nodes.
	var vctx activity.ValkeyUserContext
	err = workflow.ExecuteActivity(ctx, "GetValkeyUserContext", userID).Get(ctx, &vctx)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_users", userID, err)
		return err
	}

	if vctx.Instance.ShardID == nil {
		noShardErr := fmt.Errorf("valkey instance %s has no shard assigned", vctx.User.ValkeyInstanceID)
		_ = setResourceFailed(ctx, "valkey_users", userID, noShardErr)
		return noShardErr
	}

	// Delete valkey user on each node in the shard.
	for _, node := range vctx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "DeleteValkeyUser", activity.DeleteValkeyUserParams{
			InstanceName: vctx.Instance.Name,
			Port:         vctx.Instance.Port,
			Username:     vctx.User.Username,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "valkey_users", userID, err)
			return err
		}
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "valkey_users",
		ID:     userID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
