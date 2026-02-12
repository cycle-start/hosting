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
			MaximumAttempts: 3,
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

	// Look up the valkey user.
	var vUser model.ValkeyUser
	err = workflow.ExecuteActivity(ctx, "GetValkeyUserByID", userID).Get(ctx, &vUser)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_users", userID)
		return err
	}

	// Look up the instance to get its name and port.
	var instance model.ValkeyInstance
	err = workflow.ExecuteActivity(ctx, "GetValkeyInstanceByID", vUser.ValkeyInstanceID).Get(ctx, &instance)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_users", userID)
		return err
	}

	// Look up nodes in the instance's shard.
	if instance.ShardID == nil {
		_ = setResourceFailed(ctx, "valkey_users", userID)
		return fmt.Errorf("valkey instance %s has no shard assigned", vUser.ValkeyInstanceID)
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *instance.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_users", userID)
		return err
	}

	// Create valkey user on each node in the shard.
	for _, node := range nodes {
		if node.GRPCAddress == "" {
			continue
		}
		err = workflow.ExecuteActivity(ctx, "CreateValkeyUserOnNode", activity.CreateValkeyUserOnNodeParams{
			NodeAddress: node.GRPCAddress,
			User: activity.CreateValkeyUserParams{
				InstanceName: instance.Name,
				Port:         instance.Port,
				Username:     vUser.Username,
				Password:     vUser.Password,
				Privileges:   vUser.Privileges,
				KeyPattern:   vUser.KeyPattern,
			},
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "valkey_users", userID)
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
			MaximumAttempts: 3,
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

	// Look up the valkey user.
	var vUser model.ValkeyUser
	err = workflow.ExecuteActivity(ctx, "GetValkeyUserByID", userID).Get(ctx, &vUser)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_users", userID)
		return err
	}

	// Look up the instance to get its name and port.
	var instance model.ValkeyInstance
	err = workflow.ExecuteActivity(ctx, "GetValkeyInstanceByID", vUser.ValkeyInstanceID).Get(ctx, &instance)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_users", userID)
		return err
	}

	// Update valkey user on node agent.
	err = workflow.ExecuteActivity(ctx, "UpdateValkeyUser", activity.UpdateValkeyUserParams{
		InstanceName: instance.Name,
		Port:         instance.Port,
		Username:     vUser.Username,
		Password:     vUser.Password,
		Privileges:   vUser.Privileges,
		KeyPattern:   vUser.KeyPattern,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_users", userID)
		return err
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
			MaximumAttempts: 3,
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

	// Look up the valkey user.
	var vUser model.ValkeyUser
	err = workflow.ExecuteActivity(ctx, "GetValkeyUserByID", userID).Get(ctx, &vUser)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_users", userID)
		return err
	}

	// Look up the instance to get its name and port.
	var instance model.ValkeyInstance
	err = workflow.ExecuteActivity(ctx, "GetValkeyInstanceByID", vUser.ValkeyInstanceID).Get(ctx, &instance)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_users", userID)
		return err
	}

	// Delete valkey user on node agent.
	err = workflow.ExecuteActivity(ctx, "DeleteValkeyUser", activity.DeleteValkeyUserParams{
		InstanceName: instance.Name,
		Port:         instance.Port,
		Username:     vUser.Username,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_users", userID)
		return err
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "valkey_users",
		ID:     userID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
