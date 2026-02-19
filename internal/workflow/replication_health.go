package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
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
				createIncident(ctx, activity.CreateIncidentParams{
					DedupeKey:    fmt.Sprintf("replication_check_failed:%s:%s", shard.ID, node.ID),
					Type:         "replication_check_failed",
					Severity:     "critical",
					Title:        fmt.Sprintf("Cannot check replication on %s node %s", shard.ID, node.ID),
					Detail:       fmt.Sprintf("%v", err),
					ResourceType: strPtr("shard"),
					ResourceID:   &shard.ID,
					Source:       "replication-health-cron",
				})
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
				createIncident(ctx, activity.CreateIncidentParams{
					DedupeKey:    fmt.Sprintf("replication_broken:%s:%s", shard.ID, node.ID),
					Type:         "replication_broken",
					Severity:     "critical",
					Title:        fmt.Sprintf("Replication broken on %s node %s", shard.ID, node.ID),
					Detail:       fmt.Sprintf("IO=%v SQL=%v Error=%s", status.IORunning, status.SQLRunning, status.LastError),
					ResourceType: strPtr("shard"),
					ResourceID:   &shard.ID,
					Source:       "replication-health-cron",
				})
				allHealthy = false
			} else if status.SecondsBehind != nil && *status.SecondsBehind > 300 {
				workflow.GetLogger(ctx).Warn("high replication lag",
					"shard", shard.ID, "node", node.ID,
					"seconds_behind", *status.SecondsBehind)
				setShardStatus(ctx, shard.ID, "degraded",
					strPtr(fmt.Sprintf("replication lag %ds on node %s", *status.SecondsBehind, node.ID)))
				createIncident(ctx, activity.CreateIncidentParams{
					DedupeKey:    fmt.Sprintf("replication_lag:%s:%s", shard.ID, node.ID),
					Type:         "replication_lag",
					Severity:     "warning",
					Title:        fmt.Sprintf("High replication lag on %s node %s: %ds", shard.ID, node.ID, *status.SecondsBehind),
					Detail:       fmt.Sprintf("Seconds behind: %d", *status.SecondsBehind),
					ResourceType: strPtr("shard"),
					ResourceID:   &shard.ID,
					Source:       "replication-health-cron",
				})
				allHealthy = false
			}
		}

		// If all replicas are healthy, auto-resolve replication incidents and restore shard status.
		if allHealthy {
			if shard.Status == "degraded" {
				setShardStatus(ctx, shard.ID, model.StatusActive, nil)
			}
			autoResolveIncidents(ctx, activity.AutoResolveIncidentsParams{
				ResourceType: "shard",
				ResourceID:   shard.ID,
				TypePrefix:   "replication_",
				Resolution:   "Health check confirmed all replicas healthy",
			})
		}
	}

	return nil
}

// createIncident fires a CreateIncident activity. Errors are logged but not propagated
// to avoid failing the health check workflow due to incident tracking issues.
// For newly created critical incidents, a webhook notification is also sent.
func createIncident(ctx workflow.Context, params activity.CreateIncidentParams) {
	var result activity.CreateIncidentResult
	err := workflow.ExecuteActivity(ctx, "CreateIncident", params).Get(ctx, &result)
	if err != nil {
		workflow.GetLogger(ctx).Warn("failed to create incident",
			"dedupe_key", params.DedupeKey, "error", err)
		return
	}

	// Fire webhook for newly created critical incidents.
	if result.Created && params.Severity == "critical" {
		inc := model.Incident{
			ID:           result.ID,
			Type:         params.Type,
			Severity:     params.Severity,
			Status:       model.IncidentOpen,
			Title:        params.Title,
			Detail:       params.Detail,
			Source:       params.Source,
			ResourceType: params.ResourceType,
			ResourceID:   params.ResourceID,
		}
		sendIncidentWebhook(ctx, inc, "critical")
	}
}

// autoResolveIncidents fires an AutoResolveIncidents activity. Errors are logged but not propagated.
func autoResolveIncidents(ctx workflow.Context, params activity.AutoResolveIncidentsParams) {
	var count int
	err := workflow.ExecuteActivity(ctx, "AutoResolveIncidents", params).Get(ctx, &count)
	if err != nil {
		workflow.GetLogger(ctx).Warn("failed to auto-resolve incidents",
			"resource", params.ResourceType+"/"+params.ResourceID, "error", err)
	} else if count > 0 {
		workflow.GetLogger(ctx).Info("auto-resolved incidents",
			"resource", params.ResourceType+"/"+params.ResourceID, "count", count)
	}
}
