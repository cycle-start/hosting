package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// SyncDatabaseAccessWorkflow rebuilds MySQL user host patterns for all users
// of a database based on the current access rules. When access rules exist,
// each user is recreated with host patterns matching the allowed CIDRs.
// When no rules exist, users get host '%' (any host).
func SyncDatabaseAccessWorkflow(ctx workflow.Context, databaseID string) error {
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

	// Set all pending/deleting rules for this database to provisioning.
	err := workflow.ExecuteActivity(ctx, "SetDatabaseAccessRulesProvisioning", databaseID).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Fetch the database.
	var database model.Database
	err = workflow.ExecuteActivity(ctx, "GetDatabaseByID", databaseID).Get(ctx, &database)
	if err != nil {
		_ = setResourceFailed(ctx, "database_access_rules", databaseID, err)
		return err
	}

	if database.ShardID == nil {
		noShardErr := fmt.Errorf("database %s has no shard assigned", databaseID)
		_ = setResourceFailed(ctx, "database_access_rules", databaseID, noShardErr)
		return noShardErr
	}

	// Determine the primary node.
	primaryID, _, err := dbShardPrimary(ctx, *database.ShardID)
	if err != nil {
		_ = setResourceFailed(ctx, "database_access_rules", databaseID, err)
		return err
	}

	// Get all active access rules for this database.
	var rules []model.DatabaseAccessRule
	err = workflow.ExecuteActivity(ctx, "GetActiveDatabaseAccessRules", databaseID).Get(ctx, &rules)
	if err != nil {
		_ = setResourceFailed(ctx, "database_access_rules", databaseID, err)
		return err
	}

	// Get all active users for this database.
	var users []model.DatabaseUser
	err = workflow.ExecuteActivity(ctx, "GetActiveDatabaseUsers", databaseID).Get(ctx, &users)
	if err != nil {
		_ = setResourceFailed(ctx, "database_access_rules", databaseID, err)
		return err
	}

	// Rebuild MySQL users with updated host patterns on the primary node.
	primaryCtx := nodeActivityCtx(ctx, primaryID)
	err = workflow.ExecuteActivity(primaryCtx, "SyncDatabaseUserHosts", activity.SyncDatabaseUserHostsParams{
		DatabaseName: database.Name,
		Users:        users,
		AccessRules:  rules,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "database_access_rules", databaseID, err)
		return err
	}

	// Finalize rules: active ones stay active, deleting ones get hard-deleted.
	return workflow.ExecuteActivity(ctx, "FinalizeDatabaseAccessRules", databaseID).Get(ctx, nil)
}
