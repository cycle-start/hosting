package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// EscalateStaleIncidentsWorkflow runs on a cron schedule and auto-escalates incidents
// that have been unhandled for too long:
// - Critical open + unassigned > 15 min
// - Warning open + unassigned > 1 hour
// - Investigating or remediating > 30 min
func EscalateStaleIncidentsWorkflow(ctx workflow.Context) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var stale []activity.StaleIncident
	err := workflow.ExecuteActivity(ctx, "FindStaleIncidents").Get(ctx, &stale)
	if err != nil {
		return err
	}

	for _, inc := range stale {
		err := workflow.ExecuteActivity(ctx, "EscalateIncident", activity.EscalateIncidentParams{
			IncidentID: inc.ID,
			Reason:     inc.Reason,
			Actor:      "system:escalation-cron",
		}).Get(ctx, nil)
		if err != nil {
			workflow.GetLogger(ctx).Warn("failed to escalate stale incident",
				"id", inc.ID, "error", err)
			continue
		}

		// Fire webhook for escalated incidents.
		sendIncidentWebhook(ctx, model.Incident{
			ID:       inc.ID,
			Severity: inc.Severity,
			Status:   model.IncidentEscalated,
			Title:    inc.Title,
		}, "escalated")
	}

	return nil
}
