package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CheckCephFSHealthWorkflow runs on a cron schedule and checks CephFS mount
// status on all web shard nodes. Creates incidents for unmounted CephFS.
func CheckCephFSHealthWorkflow(ctx workflow.Context) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// List all web shards — only web nodes need CephFS.
	var shards []model.Shard
	err := workflow.ExecuteActivity(ctx, "ListShardsByRole", model.ShardRoleWeb).Get(ctx, &shards)
	if err != nil {
		return fmt.Errorf("list web shards: %w", err)
	}

	for _, shard := range shards {
		if shard.Status != model.StatusActive && shard.Status != model.StatusConverging {
			continue
		}

		var nodes []model.Node
		err := workflow.ExecuteActivity(ctx, "ListNodesByShard", shard.ID).Get(ctx, &nodes)
		if err != nil {
			workflow.GetLogger(ctx).Warn("failed to list nodes for shard",
				"shard", shard.ID, "error", err)
			continue
		}

		for _, node := range nodes {
			nodeCtx := nodeActivityCtx(ctx, node.ID)
			nodeCtx = workflow.WithActivityOptions(nodeCtx, workflow.ActivityOptions{
				StartToCloseTimeout: 15 * time.Second,
				RetryPolicy: &temporal.RetryPolicy{
					MaximumAttempts: 1,
				},
			})

			var status activity.CephFSStatus
			err := workflow.ExecuteActivity(nodeCtx, "CheckCephFSMount").Get(ctx, &status)
			if err != nil {
				workflow.GetLogger(ctx).Warn("failed to check CephFS on node",
					"node", node.ID, "shard", shard.ID, "error", err)
				continue
			}

			if !status.Mounted {
				createIncident(ctx, activity.CreateIncidentParams{
					DedupeKey:    fmt.Sprintf("cephfs_unmounted:%s", node.ID),
					Type:         "cephfs_unmounted",
					Severity:     "critical",
					Title:        fmt.Sprintf("CephFS not mounted on %s", node.Hostname),
					Detail:       fmt.Sprintf("CephFS mount check failed on node %s (%s): %s", node.Hostname, node.ID, status.Error),
					ResourceType: strPtr("node"),
					ResourceID:   &node.ID,
					Source:       "cephfs-health-monitor-cron",
				})
			} else {
				autoResolveIncidents(ctx, activity.AutoResolveIncidentsParams{
					ResourceType: "node",
					ResourceID:   node.ID,
					TypePrefix:   "cephfs_",
					Resolution:   "CephFS mount check passed",
				})
			}
		}
	}

	return nil
}
