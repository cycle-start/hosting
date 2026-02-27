package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
)

// CreateTempMySQLAccessArgs holds parameters for creating temporary MySQL access.
type CreateTempMySQLAccessArgs struct {
	DatabaseName string
	ShardID      string
	Username     string
	PasswordHash string
}

// CreateTempMySQLAccessWorkflow creates a temporary MySQL user on the primary DB node.
func CreateTempMySQLAccessWorkflow(ctx workflow.Context, args CreateTempMySQLAccessArgs) error {
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

	// Find primary node.
	primaryID, _, err := dbShardPrimary(ctx, args.ShardID)
	if err != nil {
		return fmt.Errorf("determine primary node: %w", err)
	}

	// Create temp user on primary node.
	primaryCtx := nodeActivityCtx(ctx, primaryID)
	err = workflow.ExecuteActivity(primaryCtx, "CreateTempMySQLUser", activity.CreateTempMySQLUserParams{
		DatabaseName: args.DatabaseName,
		Username:     args.Username,
		PasswordHash: args.PasswordHash,
	}).Get(ctx, nil)
	if err != nil {
		return fmt.Errorf("create temp user: %w", err)
	}

	return nil
}
