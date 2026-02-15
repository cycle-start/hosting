package workflow

import (
	"encoding/json"
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// setResourceFailed is a helper to set a resource status to failed with an error message.
// It returns any error but callers typically ignore it since the primary
// error is more important.
func setResourceFailed(ctx workflow.Context, table string, id string, err error) error {
	msg := err.Error()
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:         table,
		ID:            id,
		Status:        model.StatusFailed,
		StatusMessage: &msg,
	}).Get(ctx, nil)
}

// dbShardPrimary returns the primary node ID and the full node list for a DB shard.
// The primary is determined by the shard's config.primary_node_id field.
// If no primary is configured, the first node in the list is assumed primary.
func dbShardPrimary(ctx workflow.Context, shardID string) (primaryID string, nodes []model.Node, err error) {
	var shard model.Shard
	err = workflow.ExecuteActivity(ctx, "GetShardByID", shardID).Get(ctx, &shard)
	if err != nil {
		return "", nil, fmt.Errorf("get shard: %w", err)
	}

	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", shardID).Get(ctx, &nodes)
	if err != nil {
		return "", nil, fmt.Errorf("list nodes: %w", err)
	}

	var cfg model.DatabaseShardConfig
	if len(shard.Config) > 0 {
		_ = json.Unmarshal(shard.Config, &cfg)
	}

	if cfg.PrimaryNodeID != "" {
		return cfg.PrimaryNodeID, nodes, nil
	}

	// Fallback: first node is primary.
	if len(nodes) > 0 {
		return nodes[0].ID, nodes, nil
	}

	return "", nodes, fmt.Errorf("shard %s has no nodes", shardID)
}

// nodeActivityCtx returns a workflow context that routes activity execution
// to a specific node's Temporal task queue. Activities dispatched with this
// context will be picked up by the node-agent worker running on that node.
func nodeActivityCtx(ctx workflow.Context, nodeID string) workflow.Context {
	return workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:              "node-" + nodeID,
		StartToCloseTimeout:    2 * time.Minute,
		ScheduleToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    5,
			InitialInterval:    5 * time.Second,
			MaximumInterval:    30 * time.Second,
			BackoffCoefficient: 2.0,
		},
	})
}
