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
			MaximumAttempts: 3,
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
		_ = setResourceFailed(ctx, "databases", databaseID)
		return err
	}

	// Look up nodes in the database's shard.
	if database.ShardID == nil {
		_ = setResourceFailed(ctx, "databases", databaseID)
		return fmt.Errorf("database %s has no shard assigned", databaseID)
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *database.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "databases", databaseID)
		return err
	}

	// Create database on each node in the shard.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "CreateDatabase", database.Name).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "databases", databaseID)
			return err
		}
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
			MaximumAttempts: 3,
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
		_ = setResourceFailed(ctx, "databases", databaseID)
		return err
	}

	// Look up nodes in the database's shard.
	if database.ShardID == nil {
		_ = setResourceFailed(ctx, "databases", databaseID)
		return fmt.Errorf("database %s has no shard assigned", databaseID)
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *database.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "databases", databaseID)
		return err
	}

	// Delete database on each node in the shard.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "DeleteDatabase", database.Name).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "databases", databaseID)
			return err
		}
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "databases",
		ID:     databaseID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
