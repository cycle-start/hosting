package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CheckConvergenceHealthWorkflow runs on a cron schedule and detects shards
// stuck in "converging" status for longer than 15 minutes.
func CheckConvergenceHealthWorkflow(ctx workflow.Context) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Find shards stuck converging for more than 15 minutes (time check in activity).
	var staleShards []activity.StaleConvergingShard
	err := workflow.ExecuteActivity(ctx, "FindStaleConvergingShards", activity.FindStaleConvergingShardsParams{
		MaxAge: 15 * time.Minute,
	}).Get(ctx, &staleShards)
	if err != nil {
		return fmt.Errorf("find stale converging shards: %w", err)
	}

	// Create incidents for stuck shards.
	staleIDs := make(map[string]bool, len(staleShards))
	for _, shard := range staleShards {
		staleIDs[shard.ID] = true
		createIncident(ctx, activity.CreateIncidentParams{
			DedupeKey:    fmt.Sprintf("convergence_stuck:%s", shard.ID),
			Type:         "convergence_stuck",
			Severity:     "warning",
			Title:        fmt.Sprintf("Shard %s stuck in converging state", shard.Name),
			Detail:       fmt.Sprintf("Shard %s (role: %s, cluster: %s) has been converging for more than 15 minutes", shard.Name, shard.Role, shard.ClusterID),
			ResourceType: strPtr("shard"),
			ResourceID:   &shard.ID,
			Source:       "convergence-monitor-cron",
		})
	}

	// Step 2: Auto-resolve convergence incidents for shards that are no longer converging.
	roles := []string{
		model.ShardRoleWeb, model.ShardRoleDatabase, model.ShardRoleDNS,
		model.ShardRoleEmail, model.ShardRoleValkey, model.ShardRoleStorage,
	}

	for _, role := range roles {
		var shards []model.Shard
		err := workflow.ExecuteActivity(ctx, "ListShardsByRole", role).Get(ctx, &shards)
		if err != nil {
			workflow.GetLogger(ctx).Warn("failed to list shards for role",
				"role", role, "error", err)
			continue
		}

		for _, shard := range shards {
			if shard.Status != model.StatusConverging && !staleIDs[shard.ID] {
				autoResolveIncidents(ctx, activity.AutoResolveIncidentsParams{
					ResourceType: "shard",
					ResourceID:   shard.ID,
					TypePrefix:   "convergence_",
					Resolution:   "Shard is no longer stuck in converging state",
				})
			}
		}
	}

	return nil
}
