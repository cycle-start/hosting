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
			MaximumAttempts: 3,
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
		_ = setResourceFailed(ctx, "valkey_instances", instanceID)
		return err
	}

	// Look up nodes in the instance's shard.
	if instance.ShardID == nil {
		_ = setResourceFailed(ctx, "valkey_instances", instanceID)
		return fmt.Errorf("valkey instance %s has no shard assigned", instanceID)
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *instance.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_instances", instanceID)
		return err
	}

	// Create instance on each node in the shard.
	for _, node := range nodes {
		if node.GRPCAddress == "" {
			continue
		}
		err = workflow.ExecuteActivity(ctx, "CreateValkeyInstanceOnNode", activity.CreateValkeyInstanceOnNodeParams{
			NodeAddress: node.GRPCAddress,
			Instance: activity.CreateValkeyInstanceParams{
				Name:        instance.Name,
				Port:        instance.Port,
				Password:    instance.Password,
				MaxMemoryMB: instance.MaxMemoryMB,
			},
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "valkey_instances", instanceID)
			return err
		}
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "valkey_instances",
		ID:     instanceID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteValkeyInstanceWorkflow deletes a Valkey instance from the node agent.
func DeleteValkeyInstanceWorkflow(ctx workflow.Context, instanceID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
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
		_ = setResourceFailed(ctx, "valkey_instances", instanceID)
		return err
	}

	// Delete instance on node agent.
	err = workflow.ExecuteActivity(ctx, "DeleteValkeyInstance", activity.DeleteValkeyInstanceParams{
		Name: instance.Name,
		Port: instance.Port,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_instances", instanceID)
		return err
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "valkey_instances",
		ID:     instanceID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
