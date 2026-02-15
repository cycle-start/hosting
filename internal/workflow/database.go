package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateDatabaseWorkflow provisions a new database on the node agent.
func CreateDatabaseWorkflow(ctx workflow.Context, databaseID string) error {
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
		Table:  "databases",
		ID:     databaseID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the database.
	var database model.Database
	err = workflow.ExecuteActivity(ctx, "GetDatabaseByID", databaseID).Get(ctx, &database)
	if err != nil {
		_ = setResourceFailed(ctx, "databases", databaseID, err)
		return err
	}

	// Look up the primary node in the database's shard.
	if database.ShardID == nil {
		noShardErr := fmt.Errorf("database %s has no shard assigned", databaseID)
		_ = setResourceFailed(ctx, "databases", databaseID, noShardErr)
		return noShardErr
	}

	// Determine the primary node.
	primaryID, _, err := dbShardPrimary(ctx, *database.ShardID)
	if err != nil {
		_ = setResourceFailed(ctx, "databases", databaseID, err)
		return err
	}

	// Create database on the PRIMARY only (replicas get data via replication).
	primaryCtx := nodeActivityCtx(ctx, primaryID)
	err = workflow.ExecuteActivity(primaryCtx, "CreateDatabase", database.Name).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "databases", databaseID, err)
		return err
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "databases",
		ID:     databaseID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteDatabaseWorkflow deletes a database from the node agent.
func DeleteDatabaseWorkflow(ctx workflow.Context, databaseID string) error {
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
		Table:  "databases",
		ID:     databaseID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the database.
	var database model.Database
	err = workflow.ExecuteActivity(ctx, "GetDatabaseByID", databaseID).Get(ctx, &database)
	if err != nil {
		_ = setResourceFailed(ctx, "databases", databaseID, err)
		return err
	}

	// Look up the primary node in the database's shard.
	if database.ShardID == nil {
		noShardErr := fmt.Errorf("database %s has no shard assigned", databaseID)
		_ = setResourceFailed(ctx, "databases", databaseID, noShardErr)
		return noShardErr
	}

	// Determine the primary node.
	primaryID, _, err := dbShardPrimary(ctx, *database.ShardID)
	if err != nil {
		_ = setResourceFailed(ctx, "databases", databaseID, err)
		return err
	}

	// Delete database on the PRIMARY only (change replicates to replicas).
	primaryCtx := nodeActivityCtx(ctx, primaryID)
	err = workflow.ExecuteActivity(primaryCtx, "DeleteDatabase", database.Name).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "databases", databaseID, err)
		return err
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "databases",
		ID:     databaseID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
