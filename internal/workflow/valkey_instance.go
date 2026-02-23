package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateValkeyInstanceWorkflow provisions a new Valkey instance on the node agent.
func CreateValkeyInstanceWorkflow(ctx workflow.Context, instanceID string) error {
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
		Table:  "valkey_instances",
		ID:     instanceID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the instance.
	var instance model.ValkeyInstance
	err = workflow.ExecuteActivity(ctx, "GetValkeyInstanceByID", instanceID).Get(ctx, &instance)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_instances", instanceID, err)
		return err
	}

	// Look up nodes in the instance's shard.
	if instance.ShardID == nil {
		noShardErr := fmt.Errorf("valkey instance %s has no shard assigned", instanceID)
		_ = setResourceFailed(ctx, "valkey_instances", instanceID, noShardErr)
		return noShardErr
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *instance.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_instances", instanceID, err)
		return err
	}

	// Create instance on each node in the shard.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "CreateValkeyInstance", activity.CreateValkeyInstanceParams{
			Name:        instance.ID,
			Port:        instance.Port,
			Password:    instance.Password,
			MaxMemoryMB: instance.MaxMemoryMB,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "valkey_instances", instanceID, err)
			return err
		}
	}

	// Set status to active.
	err = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "valkey_instances",
		ID:     instanceID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Spawn pending child workflows in parallel.
	var children []ChildWorkflowSpec

	var users []model.ValkeyUser
	_ = workflow.ExecuteActivity(ctx, "ListValkeyUsersByInstanceID", instanceID).Get(ctx, &users)
	for _, u := range users {
		if u.Status == model.StatusPending {
			children = append(children, ChildWorkflowSpec{WorkflowName: "CreateValkeyUserWorkflow", WorkflowID: fmt.Sprintf("create-valkey-user-%s", u.ID), Arg: u.ID})
		}
	}

	if errs := fanOutChildWorkflows(ctx, children); len(errs) > 0 {
		workflow.GetLogger(ctx).Warn("child workflow failures", "errors", joinErrors(errs))
	}
	return nil
}

// DeleteValkeyInstanceWorkflow deletes a Valkey instance from the node agent.
func DeleteValkeyInstanceWorkflow(ctx workflow.Context, instanceID string) error {
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
		Table:  "valkey_instances",
		ID:     instanceID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the instance.
	var instance model.ValkeyInstance
	err = workflow.ExecuteActivity(ctx, "GetValkeyInstanceByID", instanceID).Get(ctx, &instance)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_instances", instanceID, err)
		return err
	}

	// Look up nodes in the instance's shard.
	if instance.ShardID == nil {
		noShardErr := fmt.Errorf("valkey instance %s has no shard assigned", instanceID)
		_ = setResourceFailed(ctx, "valkey_instances", instanceID, noShardErr)
		return noShardErr
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *instance.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_instances", instanceID, err)
		return err
	}

	// Delete instance on each node in the shard.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "DeleteValkeyInstance", activity.DeleteValkeyInstanceParams{
			Name: instance.ID,
			Port: instance.Port,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "valkey_instances", instanceID, err)
			return err
		}
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "valkey_instances",
		ID:     instanceID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
