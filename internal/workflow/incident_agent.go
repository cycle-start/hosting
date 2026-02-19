package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
)

// ProcessIncidentQueueWorkflow runs on a cron schedule, picks up unassigned incidents,
// and fans out InvestigateIncidentWorkflow child workflows.
func ProcessIncidentQueueWorkflow(ctx workflow.Context, maxConcurrent int) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Fetch the system prompt once per tick.
	var systemPrompt string
	err := workflow.ExecuteActivity(ctx, "GetAgentSystemPrompt").Get(ctx, &systemPrompt)
	if err != nil {
		return fmt.Errorf("get system prompt: %w", err)
	}

	// List unassigned incidents.
	var incidents []activity.UnassignedIncident
	err = workflow.ExecuteActivity(ctx, "ListUnassignedOpenIncidents").Get(ctx, &incidents)
	if err != nil {
		return fmt.Errorf("list unassigned incidents: %w", err)
	}

	if len(incidents) == 0 {
		return nil
	}

	// Claim and investigate each incident, capped at maxConcurrent.
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}

	wg := workflow.NewWaitGroup(ctx)
	sem := workflow.NewSemaphore(ctx, int64(maxConcurrent))

	for _, inc := range incidents {
		inc := inc // capture

		// Claim the incident atomically.
		var claimed bool
		err := workflow.ExecuteActivity(ctx, "ClaimIncidentForAgent", inc.ID).Get(ctx, &claimed)
		if err != nil {
			workflow.GetLogger(ctx).Warn("failed to claim incident", "id", inc.ID, "error", err)
			continue
		}
		if !claimed {
			continue
		}

		// Acquire semaphore slot.
		_ = sem.Acquire(ctx, 1)
		wg.Add(1)

		workflow.Go(ctx, func(gCtx workflow.Context) {
			defer wg.Done()
			defer sem.Release(1)

			childCtx := workflow.WithChildOptions(gCtx, workflow.ChildWorkflowOptions{
				WorkflowID: "investigate-" + inc.ID,
				TaskQueue:  "hosting-tasks",
			})
			err := workflow.ExecuteChildWorkflow(childCtx, InvestigateIncidentWorkflow, inc.ID, systemPrompt).Get(gCtx, nil)
			if err != nil {
				workflow.GetLogger(gCtx).Error("investigation failed", "id", inc.ID, "error", err)
			}
		})
	}

	wg.Wait(ctx)
	return nil
}

// InvestigateIncidentWorkflow orchestrates the full investigation of a single incident.
func InvestigateIncidentWorkflow(ctx workflow.Context, incidentID string, systemPrompt string) error {
	logger := workflow.GetLogger(ctx)

	// Assemble context — retryable, short timeout.
	assembleCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})

	var incCtx activity.IncidentContext
	err := workflow.ExecuteActivity(assembleCtx, "AssembleIncidentContext", incidentID).Get(ctx, &incCtx)
	if err != nil {
		logger.Error("failed to assemble incident context", "id", incidentID, "error", err)
		escalateOnFailure(ctx, incidentID, fmt.Sprintf("Failed to assemble context: %v", err))
		return err
	}

	// Run the LLM investigation loop — no retry (non-deterministic), long timeout.
	investigateCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	})

	params := activity.InvestigateIncidentParams{
		SystemPrompt:    systemPrompt,
		IncidentContext: &incCtx,
	}

	var result activity.InvestigateIncidentResult
	err = workflow.ExecuteActivity(investigateCtx, "InvestigateIncident", params).Get(ctx, &result)
	if err != nil {
		logger.Error("investigation activity failed", "id", incidentID, "error", err)
		escalateOnFailure(ctx, incidentID, fmt.Sprintf("Investigation activity failed: %v", err))
		return err
	}

	// If max turns reached without resolution, escalate.
	if result.Outcome == "max_turns" {
		escalateOnFailure(ctx, incidentID, "Agent reached maximum investigation turns without resolving or escalating")
	}

	logger.Info("investigation completed",
		"id", incidentID, "outcome", result.Outcome, "turns", result.Turns)
	return nil
}

// escalateOnFailure escalates the incident when the investigation cannot continue.
func escalateOnFailure(ctx workflow.Context, incidentID, reason string) {
	escalateCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})

	err := workflow.ExecuteActivity(escalateCtx, "EscalateIncident", activity.EscalateIncidentParams{
		IncidentID: incidentID,
		Reason:     reason,
		Actor:      "agent:incident-investigator",
	}).Get(ctx, nil)
	if err != nil {
		workflow.GetLogger(ctx).Error("failed to escalate incident",
			"id", incidentID, "error", err)
	}
}
