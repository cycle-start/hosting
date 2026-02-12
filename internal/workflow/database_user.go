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

	// Look up the database user.
	var dbUser model.DatabaseUser
	err = workflow.ExecuteActivity(ctx, "GetDatabaseUserByID", userID).Get(ctx, &dbUser)
	if err != nil {
		_ = setResourceFailed(ctx, "database_users", userID)
		return err
	}

	// Look up the database to get its name.
	var database model.Database
	err = workflow.ExecuteActivity(ctx, "GetDatabaseByID", dbUser.DatabaseID).Get(ctx, &database)
	if err != nil {
		_ = setResourceFailed(ctx, "database_users", userID)
		return err
	}

	// Look up nodes in the database's shard.
	if database.ShardID == nil {
		_ = setResourceFailed(ctx, "database_users", userID)
		return fmt.Errorf("database %s has no shard assigned", dbUser.DatabaseID)
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *database.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "database_users", userID)
		return err
	}

	// Create database user on each node in the shard.
	for _, node := range nodes {
		if node.GRPCAddress == "" {
			continue
		}
		err = workflow.ExecuteActivity(ctx, "CreateDatabaseUserOnNode", activity.CreateDatabaseUserOnNodeParams{
			NodeAddress: node.GRPCAddress,
			User: activity.CreateDatabaseUserParams{
				DatabaseName: database.Name,
				Username:     dbUser.Username,
				Password:     dbUser.Password,
				Privileges:   dbUser.Privileges,
			},
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "database_users", userID)
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

	// Look up the database user.
	var dbUser model.DatabaseUser
	err = workflow.ExecuteActivity(ctx, "GetDatabaseUserByID", userID).Get(ctx, &dbUser)
	if err != nil {
		_ = setResourceFailed(ctx, "database_users", userID)
		return err
	}

	// Look up the database to get its name.
	var database model.Database
	err = workflow.ExecuteActivity(ctx, "GetDatabaseByID", dbUser.DatabaseID).Get(ctx, &database)
	if err != nil {
		_ = setResourceFailed(ctx, "database_users", userID)
		return err
	}

	// Update database user on node agent.
	err = workflow.ExecuteActivity(ctx, "UpdateDatabaseUser", activity.UpdateDatabaseUserParams{
		DatabaseName: database.Name,
		Username:     dbUser.Username,
		Password:     dbUser.Password,
		Privileges:   dbUser.Privileges,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "database_users", userID)
		return err
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

	// Look up the database user.
	var dbUser model.DatabaseUser
	err = workflow.ExecuteActivity(ctx, "GetDatabaseUserByID", userID).Get(ctx, &dbUser)
	if err != nil {
		_ = setResourceFailed(ctx, "database_users", userID)
		return err
	}

	// Look up the database to get its name.
	var database model.Database
	err = workflow.ExecuteActivity(ctx, "GetDatabaseByID", dbUser.DatabaseID).Get(ctx, &database)
	if err != nil {
		_ = setResourceFailed(ctx, "database_users", userID)
		return err
	}

	// Delete database user on node agent.
	err = workflow.ExecuteActivity(ctx, "DeleteDatabaseUser", database.Name, dbUser.Username).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "database_users", userID)
		return err
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "database_users",
		ID:     userID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
