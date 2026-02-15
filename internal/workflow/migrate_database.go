package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// MigrateDatabaseParams holds parameters for MigrateDatabaseWorkflow.
type MigrateDatabaseParams struct {
	DatabaseID    string `json:"database_id"`
	TargetShardID string `json:"target_shard_id"`
}

// MigrateDatabaseWorkflow moves a database from one database shard to another
// within the same cluster. It dumps data on the source node, imports on the
// target node, migrates all database users, and updates the shard assignment.
func MigrateDatabaseWorkflow(ctx workflow.Context, params MigrateDatabaseParams) error {
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

	databaseID := params.DatabaseID

	// Set database status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "databases",
		ID:     databaseID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Get the database.
	var database model.Database
	err = workflow.ExecuteActivity(ctx, "GetDatabaseByID", databaseID).Get(ctx, &database)
	if err != nil {
		_ = setResourceFailed(ctx, "databases", databaseID, err)
		return err
	}

	if database.ShardID == nil {
		noShardErr := fmt.Errorf("database %s has no current shard assignment", databaseID)
		_ = setResourceFailed(ctx, "databases", databaseID, noShardErr)
		return noShardErr
	}
	sourceShardID := *database.ShardID

	// Get source shard nodes.
	var sourceNodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", sourceShardID).Get(ctx, &sourceNodes)
	if err != nil {
		_ = setResourceFailed(ctx, "databases", databaseID, err)
		return err
	}
	if len(sourceNodes) == 0 {
		noNodesErr := fmt.Errorf("source shard %s has no nodes", sourceShardID)
		_ = setResourceFailed(ctx, "databases", databaseID, noNodesErr)
		return noNodesErr
	}

	// Get target shard nodes.
	var targetNodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", params.TargetShardID).Get(ctx, &targetNodes)
	if err != nil {
		_ = setResourceFailed(ctx, "databases", databaseID, err)
		return err
	}
	if len(targetNodes) == 0 {
		noNodesErr := fmt.Errorf("target shard %s has no nodes", params.TargetShardID)
		_ = setResourceFailed(ctx, "databases", databaseID, noNodesErr)
		return noNodesErr
	}

	sourceNode := sourceNodes[0]
	targetNode := targetNodes[0]

	dumpPath := fmt.Sprintf("/var/backups/hosting/migrate/%s.sql.gz", database.Name)

	// Create the database on the target node.
	targetCtx := nodeActivityCtx(ctx, targetNode.ID)
	err = workflow.ExecuteActivity(targetCtx, "CreateDatabase", database.Name).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "databases", databaseID, err)
		return fmt.Errorf("create database on target node %s: %w", targetNode.ID, err)
	}

	// Dump the database on the source node.
	sourceCtx := nodeActivityCtx(ctx, sourceNode.ID)
	err = workflow.ExecuteActivity(sourceCtx, "DumpMySQLDatabase", activity.DumpMySQLDatabaseParams{
		DatabaseName: database.Name,
		DumpPath:     dumpPath,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "databases", databaseID, err)
		return fmt.Errorf("dump database on source node %s: %w", sourceNode.ID, err)
	}

	// Import the dump on the target node.
	err = workflow.ExecuteActivity(targetCtx, "ImportMySQLDatabase", activity.ImportMySQLDatabaseParams{
		DatabaseName: database.Name,
		DumpPath:     dumpPath,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "databases", databaseID, err)
		return fmt.Errorf("import database on target node %s: %w", targetNode.ID, err)
	}

	// Migrate database users to the target node.
	var users []model.DatabaseUser
	err = workflow.ExecuteActivity(ctx, "ListDatabaseUsersByDatabaseID", databaseID).Get(ctx, &users)
	if err != nil {
		_ = setResourceFailed(ctx, "databases", databaseID, err)
		return fmt.Errorf("list database users: %w", err)
	}

	for _, user := range users {
		err = workflow.ExecuteActivity(targetCtx, "CreateDatabaseUser", activity.CreateDatabaseUserParams{
			DatabaseName: database.Name,
			Username:     user.Username,
			Password:     user.Password,
			Privileges:   user.Privileges,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "databases", databaseID, err)
			return fmt.Errorf("create user %s on target node %s: %w", user.Username, targetNode.ID, err)
		}
	}

	// Update the database shard assignment in the core DB.
	err = workflow.ExecuteActivity(ctx, "UpdateDatabaseShardID", databaseID, params.TargetShardID).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "databases", databaseID, err)
		return err
	}

	// Cleanup: drop database on source node (best effort).
	_ = workflow.ExecuteActivity(sourceCtx, "DeleteDatabase", database.Name).Get(ctx, nil)

	// Cleanup: remove dump file on source node (best effort).
	_ = workflow.ExecuteActivity(sourceCtx, "CleanupMigrateFile", dumpPath).Get(ctx, nil)

	// Cleanup: remove dump file on target node (best effort).
	_ = workflow.ExecuteActivity(targetCtx, "CleanupMigrateFile", dumpPath).Get(ctx, nil)

	// Set database status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "databases",
		ID:     databaseID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}
