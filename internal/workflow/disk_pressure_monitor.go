package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CheckDiskPressureWorkflow runs on a cron schedule and checks disk usage
// on all active nodes. Creates incidents for nodes with high disk usage.
func CheckDiskPressureWorkflow(ctx workflow.Context) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// List all active nodes.
	var nodes []model.Node
	err := workflow.ExecuteActivity(ctx, "ListActiveNodes").Get(ctx, &nodes)
	if err != nil {
		return fmt.Errorf("list active nodes: %w", err)
	}

	for _, node := range nodes {
		// Call GetDiskUsage on each node's task queue.
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		// Longer timeout + allow failure (node may be unreachable).
		nodeCtx = workflow.WithActivityOptions(nodeCtx, workflow.ActivityOptions{
			StartToCloseTimeout: 15 * time.Second,
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 1,
			},
		})

		var disks []activity.DiskUsage
		err := workflow.ExecuteActivity(nodeCtx, "GetDiskUsage").Get(ctx, &disks)
		if err != nil {
			workflow.GetLogger(ctx).Warn("failed to get disk usage",
				"node", node.ID, "hostname", node.Hostname, "error", err)
			continue
		}

		hasPressure := false
		for _, disk := range disks {
			if disk.UsedPct >= 90 {
				severity := "warning"
				if disk.UsedPct >= 95 {
					severity = "critical"
				}
				createIncident(ctx, activity.CreateIncidentParams{
					DedupeKey:    fmt.Sprintf("disk_pressure:%s:%s", node.ID, disk.Path),
					Type:         "disk_pressure",
					Severity:     severity,
					Title:        fmt.Sprintf("High disk usage on %s (%s: %.1f%%)", node.Hostname, disk.Path, disk.UsedPct),
					Detail:       fmt.Sprintf("Disk %s on node %s (%s) is %.1f%% full (%d/%d bytes used)", disk.Path, node.Hostname, node.ID, disk.UsedPct, disk.UsedBytes, disk.TotalBytes),
					ResourceType: strPtr("node"),
					ResourceID:   &node.ID,
					Source:       "disk-pressure-monitor-cron",
				})
				hasPressure = true
			}
		}

		if !hasPressure {
			// All disks are healthy — auto-resolve any disk_pressure incidents for this node.
			autoResolveIncidents(ctx, activity.AutoResolveIncidentsParams{
				ResourceType: "node",
				ResourceID:   node.ID,
				TypePrefix:   "disk_pressure",
				Resolution:   "Disk usage returned to normal",
			})
		}
	}

	return nil
}
