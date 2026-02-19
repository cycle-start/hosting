package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CheckNodeHealthWorkflow runs on a cron schedule and detects nodes that
// haven't reported health within the expected interval (5 minutes).
func CheckNodeHealthWorkflow(ctx workflow.Context) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Find nodes that haven't reported health in 5 minutes.
	var unhealthyNodes []model.Node
	err := workflow.ExecuteActivity(ctx, "FindUnhealthyNodes", 5*time.Minute).Get(ctx, &unhealthyNodes)
	if err != nil {
		return fmt.Errorf("find unhealthy nodes: %w", err)
	}

	unhealthyIDs := make(map[string]bool, len(unhealthyNodes))
	for _, node := range unhealthyNodes {
		unhealthyIDs[node.ID] = true

		detail := fmt.Sprintf("Node %s (%s) has not reported health", node.Hostname, node.ID)
		if node.LastHealthAt != nil {
			detail = fmt.Sprintf("Node %s (%s) last reported health at %s", node.Hostname, node.ID, node.LastHealthAt.Format(time.RFC3339))
		} else {
			detail = fmt.Sprintf("Node %s (%s) has never reported health", node.Hostname, node.ID)
		}

		createIncident(ctx, activity.CreateIncidentParams{
			DedupeKey:    fmt.Sprintf("node_health_missing:%s", node.ID),
			Type:         "node_health_missing",
			Severity:     "critical",
			Title:        fmt.Sprintf("Node %s not reporting health", node.Hostname),
			Detail:       detail,
			ResourceType: strPtr("node"),
			ResourceID:   &node.ID,
			Source:       "node-health-monitor-cron",
		})
	}

	// Auto-resolve for nodes that are now healthy.
	var allActiveNodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListActiveNodes").Get(ctx, &allActiveNodes)
	if err != nil {
		workflow.GetLogger(ctx).Warn("failed to list active nodes for auto-resolve", "error", err)
		return nil
	}

	for _, node := range allActiveNodes {
		if !unhealthyIDs[node.ID] {
			autoResolveIncidents(ctx, activity.AutoResolveIncidentsParams{
				ResourceType: "node",
				ResourceID:   node.ID,
				TypePrefix:   "node_health_",
				Resolution:   "Node is reporting health again",
			})
		}
	}

	return nil
}
