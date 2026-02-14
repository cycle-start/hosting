package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateDatabaseUserWorkflow provisions a new database user on the node agent.
func CreateDatabaseUserWorkflow(ctx workflow.Context, userID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "database_users",
		ID:     userID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the database user, database, and nodes.
	var dctx activity.DatabaseUserContext
	err = workflow.ExecuteActivity(ctx, "GetDatabaseUserContext", userID).Get(ctx, &dctx)
	if err != nil {
		_ = setResourceFailed(ctx, "database_users", userID, err)
		return err
	}

	if dctx.Database.ShardID == nil {
		noShardErr := fmt.Errorf("database %s has no shard assigned", dctx.User.DatabaseID)
		_ = setResourceFailed(ctx, "database_users", userID, noShardErr)
		return noShardErr
	}

	// Create database user on each node in the shard.
	for _, node := range dctx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "CreateDatabaseUser", activity.CreateDatabaseUserParams{
			DatabaseName: dctx.Database.Name,
			Username:     dctx.User.Username,
			Password:     dctx.User.Password,
			Privileges:   dctx.User.Privileges,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "database_users", userID, err)
			return err
		}
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "database_users",
		ID:     userID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// UpdateDatabaseUserWorkflow updates a database user on the node agent.
func UpdateDatabaseUserWorkflow(ctx workflow.Context, userID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "database_users",
		ID:     userID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the database user, database, and nodes.
	var dctx activity.DatabaseUserContext
	err = workflow.ExecuteActivity(ctx, "GetDatabaseUserContext", userID).Get(ctx, &dctx)
	if err != nil {
		_ = setResourceFailed(ctx, "database_users", userID, err)
		return err
	}

	if dctx.Database.ShardID == nil {
		noShardErr := fmt.Errorf("database %s has no shard assigned", dctx.User.DatabaseID)
		_ = setResourceFailed(ctx, "database_users", userID, noShardErr)
		return noShardErr
	}

	// Update database user on each node in the shard.
	for _, node := range dctx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "UpdateDatabaseUser", activity.UpdateDatabaseUserParams{
			DatabaseName: dctx.Database.Name,
			Username:     dctx.User.Username,
			Password:     dctx.User.Password,
			Privileges:   dctx.User.Privileges,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "database_users", userID, err)
			return err
		}
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "database_users",
		ID:     userID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteDatabaseUserWorkflow deletes a database user from the node agent.
func DeleteDatabaseUserWorkflow(ctx workflow.Context, userID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to deleting.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "database_users",
		ID:     userID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the database user, database, and nodes.
	var dctx activity.DatabaseUserContext
	err = workflow.ExecuteActivity(ctx, "GetDatabaseUserContext", userID).Get(ctx, &dctx)
	if err != nil {
		_ = setResourceFailed(ctx, "database_users", userID, err)
		return err
	}

	if dctx.Database.ShardID == nil {
		noShardErr := fmt.Errorf("database %s has no shard assigned", dctx.User.DatabaseID)
		_ = setResourceFailed(ctx, "database_users", userID, noShardErr)
		return noShardErr
	}

	// Delete database user on each node in the shard.
	for _, node := range dctx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "DeleteDatabaseUser", dctx.Database.Name, dctx.User.Username).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "database_users", userID, err)
			return err
		}
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "database_users",
		ID:     userID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
