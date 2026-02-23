package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// MigrateValkeyInstanceParams holds parameters for MigrateValkeyInstanceWorkflow.
type MigrateValkeyInstanceParams struct {
	InstanceID    string `json:"instance_id"`
	TargetShardID string `json:"target_shard_id"`
}

// MigrateValkeyInstanceWorkflow moves a Valkey instance from one shard to another
// within the same cluster. It dumps the RDB on the source node, creates a new
// instance on the target, imports the data, migrates all users, and updates
// the shard assignment.
func MigrateValkeyInstanceWorkflow(ctx workflow.Context, params MigrateValkeyInstanceParams) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	instanceID := params.InstanceID

	// Set instance status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "valkey_instances",
		ID:     instanceID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Get the instance.
	var instance model.ValkeyInstance
	err = workflow.ExecuteActivity(ctx, "GetValkeyInstanceByID", instanceID).Get(ctx, &instance)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_instances", instanceID, err)
		return err
	}

	if instance.ShardID == nil {
		noShardErr := fmt.Errorf("valkey instance %s has no current shard assignment", instanceID)
		_ = setResourceFailed(ctx, "valkey_instances", instanceID, noShardErr)
		return noShardErr
	}
	sourceShardID := *instance.ShardID

	// Get source shard nodes.
	var sourceNodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", sourceShardID).Get(ctx, &sourceNodes)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_instances", instanceID, err)
		return err
	}
	if len(sourceNodes) == 0 {
		noNodesErr := fmt.Errorf("source shard %s has no nodes", sourceShardID)
		_ = setResourceFailed(ctx, "valkey_instances", instanceID, noNodesErr)
		return noNodesErr
	}

	// Get target shard nodes.
	var targetNodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", params.TargetShardID).Get(ctx, &targetNodes)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_instances", instanceID, err)
		return err
	}
	if len(targetNodes) == 0 {
		noNodesErr := fmt.Errorf("target shard %s has no nodes", params.TargetShardID)
		_ = setResourceFailed(ctx, "valkey_instances", instanceID, noNodesErr)
		return noNodesErr
	}

	sourceNode := sourceNodes[0]
	targetNode := targetNodes[0]

	dumpPath := fmt.Sprintf("/var/backups/hosting/migrate/%s.rdb", instance.ID)

	// Create the instance on the target node.
	targetCtx := nodeActivityCtx(ctx, targetNode.ID)
	err = workflow.ExecuteActivity(targetCtx, "CreateValkeyInstance", activity.CreateValkeyInstanceParams{
		Name:        instance.ID,
		Port:        instance.Port,
		Password:    instance.Password,
		MaxMemoryMB: instance.MaxMemoryMB,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_instances", instanceID, err)
		return fmt.Errorf("create valkey instance on target node %s: %w", targetNode.ID, err)
	}

	// Dump data on the source node.
	sourceCtx := nodeActivityCtx(ctx, sourceNode.ID)
	err = workflow.ExecuteActivity(sourceCtx, "DumpValkeyData", activity.DumpValkeyDataParams{
		Name:     instance.ID,
		Port:     instance.Port,
		Password: instance.Password,
		DumpPath: dumpPath,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_instances", instanceID, err)
		return fmt.Errorf("dump valkey data on source node %s: %w", sourceNode.ID, err)
	}

	// Import data on the target node.
	err = workflow.ExecuteActivity(targetCtx, "ImportValkeyData", activity.ImportValkeyDataParams{
		Name:     instance.ID,
		Port:     instance.Port,
		DumpPath: dumpPath,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_instances", instanceID, err)
		return fmt.Errorf("import valkey data on target node %s: %w", targetNode.ID, err)
	}

	// Migrate Valkey users to the target node.
	var users []model.ValkeyUser
	err = workflow.ExecuteActivity(ctx, "ListValkeyUsersByInstanceID", instanceID).Get(ctx, &users)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_instances", instanceID, err)
		return fmt.Errorf("list valkey users: %w", err)
	}

	for _, user := range users {
		err = workflow.ExecuteActivity(targetCtx, "CreateValkeyUser", activity.CreateValkeyUserParams{
			InstanceName: instance.ID,
			Port:         instance.Port,
			Username:     user.Username,
			Password:     user.Password,
			Privileges:   user.Privileges,
			KeyPattern:   user.KeyPattern,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "valkey_instances", instanceID, err)
			return fmt.Errorf("create valkey user %s on target node %s: %w", user.Username, targetNode.ID, err)
		}
	}

	// Update the instance shard assignment in the core DB.
	err = workflow.ExecuteActivity(ctx, "UpdateValkeyInstanceShardID", instanceID, params.TargetShardID).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "valkey_instances", instanceID, err)
		return err
	}

	// Cleanup: delete instance on source node (best effort).
	_ = workflow.ExecuteActivity(sourceCtx, "DeleteValkeyInstance", activity.DeleteValkeyInstanceParams{
		Name: instance.ID,
		Port: instance.Port,
	}).Get(ctx, nil)

	// Cleanup: remove dump file on source node (best effort).
	_ = workflow.ExecuteActivity(sourceCtx, "CleanupMigrateFile", dumpPath).Get(ctx, nil)

	// Cleanup: remove dump file on target node (best effort).
	_ = workflow.ExecuteActivity(targetCtx, "CleanupMigrateFile", dumpPath).Get(ctx, nil)

	// Set instance status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "valkey_instances",
		ID:     instanceID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}
