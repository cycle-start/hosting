package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/agent"
	"github.com/edvin/hosting/internal/model"
)

// CheckReplicationHealthWorkflow runs on a cron schedule and checks all DB shard replicas.
func CheckReplicationHealthWorkflow(ctx workflow.Context) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// List all database shards.
	var shards []model.Shard
	err := workflow.ExecuteActivity(ctx, "ListShardsByRole", model.ShardRoleDatabase).Get(ctx, &shards)
	if err != nil {
		return fmt.Errorf("list database shards: %w", err)
	}

	for _, shard := range shards {
		if shard.Status != model.StatusActive && shard.Status != "degraded" {
			continue
		}

		primaryID, nodes, err := dbShardPrimary(ctx, shard.ID)
		if err != nil {
			workflow.GetLogger(ctx).Warn("failed to get primary for shard",
				"shard", shard.ID, "error", err)
			continue
		}

		allHealthy := true
		for _, node := range nodes {
			if node.ID == primaryID {
				continue // Skip primary.
			}

			nodeCtx := nodeActivityCtx(ctx, node.ID)
			var status agent.ReplicationStatus
			err = workflow.ExecuteActivity(nodeCtx, "GetReplicationStatus").Get(ctx, &status)
			if err != nil {
				workflow.GetLogger(ctx).Error("replication check failed",
					"shard", shard.ID, "node", node.ID, "error", err)
				setShardStatus(ctx, shard.ID, "degraded",
					strPtr(fmt.Sprintf("replication check failed on node %s: %v", node.ID, err)))
				allHealthy = false
				continue
			}

			if !status.IORunning || !status.SQLRunning {
				workflow.GetLogger(ctx).Error("replication broken",
					"shard", shard.ID, "node", node.ID,
					"io_running", status.IORunning,
					"sql_running", status.SQLRunning,
					"last_error", status.LastError)
				setShardStatus(ctx, shard.ID, "degraded",
					strPtr(fmt.Sprintf("replication broken on node %s: %s", node.ID, status.LastError)))
				allHealthy = false
			} else if status.SecondsBehind != nil && *status.SecondsBehind > 300 {
				workflow.GetLogger(ctx).Warn("high replication lag",
					"shard", shard.ID, "node", node.ID,
					"seconds_behind", *status.SecondsBehind)
				setShardStatus(ctx, shard.ID, "degraded",
					strPtr(fmt.Sprintf("replication lag %ds on node %s", *status.SecondsBehind, node.ID)))
				allHealthy = false
			}
		}

		// If all replicas are healthy and shard was degraded, restore to active.
		if allHealthy && shard.Status == "degraded" {
			setShardStatus(ctx, shard.ID, model.StatusActive, nil)
		}
	}

	return nil
}
