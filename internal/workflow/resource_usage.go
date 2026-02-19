package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CollectResourceUsageWorkflow runs on a cron schedule, fans out to nodes to collect
// per-resource disk usage, and upserts the results into the core database.
func CollectResourceUsageWorkflow(ctx workflow.Context) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	logger := workflow.GetLogger(ctx)

	// Collect web shard usage.
	var webShards []model.Shard
	err := workflow.ExecuteActivity(ctx, "ListShardsByRole", model.ShardRoleWeb).Get(ctx, &webShards)
	if err != nil {
		return fmt.Errorf("list web shards: %w", err)
	}

	for _, shard := range webShards {
		if shard.Status != model.StatusActive {
			continue
		}

		var nodes []model.Node
		err := workflow.ExecuteActivity(ctx, "ListNodesByShard", shard.ID).Get(ctx, &nodes)
		if err != nil {
			logger.Warn("failed to list nodes for web shard", "shard", shard.ID, "error", err)
			continue
		}
		if len(nodes) == 0 {
			continue
		}

		// Pick the first node to collect from (all nodes share CephFS storage).
		node := nodes[0]
		nodeCtx := nodeActivityCtx(ctx, node.ID)

		var entries []activity.ResourceUsageEntry
		err = workflow.ExecuteActivity(nodeCtx, "GetResourceUsage", activity.GetResourceUsageParams{
			Role: "web",
		}).Get(ctx, &entries)
		if err != nil {
			logger.Warn("failed to collect web resource usage", "shard", shard.ID, "node", node.ID, "error", err)
			continue
		}

		for _, entry := range entries {
			_ = workflow.ExecuteActivity(ctx, "UpsertResourceUsage", activity.UpsertResourceUsageParams{
				ResourceType: entry.ResourceType,
				Name:         entry.Name,
				BytesUsed:    entry.BytesUsed,
			}).Get(ctx, nil)
		}
	}

	// Collect database shard usage.
	var dbShards []model.Shard
	err = workflow.ExecuteActivity(ctx, "ListShardsByRole", model.ShardRoleDatabase).Get(ctx, &dbShards)
	if err != nil {
		return fmt.Errorf("list database shards: %w", err)
	}

	for _, shard := range dbShards {
		if shard.Status != model.StatusActive && shard.Status != "degraded" {
			continue
		}

		primaryID, nodes, err := dbShardPrimary(ctx, shard.ID)
		if err != nil {
			logger.Warn("failed to get primary for db shard", "shard", shard.ID, "error", err)
			continue
		}

		// Find the primary node.
		var primaryNode *model.Node
		for i := range nodes {
			if nodes[i].ID == primaryID {
				primaryNode = &nodes[i]
				break
			}
		}
		if primaryNode == nil {
			continue
		}

		nodeCtx := nodeActivityCtx(ctx, primaryNode.ID)

		var entries []activity.ResourceUsageEntry
		err = workflow.ExecuteActivity(nodeCtx, "GetResourceUsage", activity.GetResourceUsageParams{
			Role: "database",
		}).Get(ctx, &entries)
		if err != nil {
			logger.Warn("failed to collect db resource usage", "shard", shard.ID, "node", primaryNode.ID, "error", err)
			continue
		}

		for _, entry := range entries {
			_ = workflow.ExecuteActivity(ctx, "UpsertResourceUsage", activity.UpsertResourceUsageParams{
				ResourceType: entry.ResourceType,
				Name:         entry.Name,
				BytesUsed:    entry.BytesUsed,
			}).Get(ctx, nil)
		}
	}

	return nil
}
