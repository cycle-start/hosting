package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
)

// ProcessIncidentQueueParams holds the configuration for the incident queue processor.
type ProcessIncidentQueueParams struct {
	MaxConcurrent      int `json:"max_concurrent"`       // max parallel group leaders
	FollowerConcurrent int `json:"follower_concurrent"`  // max parallel followers per group
}

// ProcessIncidentQueueWorkflow runs on a cron schedule, picks up unassigned incidents,
// groups them by type, and uses a leader-follower pattern to avoid thundering-herd
// and propagate resolution hints from the first resolved incident to similar ones.
func ProcessIncidentQueueWorkflow(ctx workflow.Context, params ProcessIncidentQueueParams) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Fetch agent configuration (system prompt + per-type concurrency).
	var agentCfg activity.AgentConfig
	err := workflow.ExecuteActivity(ctx, "GetAgentConfig").Get(ctx, &agentCfg)
	if err != nil {
		return fmt.Errorf("get agent config: %w", err)
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

	maxConcurrent := params.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	followerConcurrent := params.FollowerConcurrent
	if followerConcurrent <= 0 {
		followerConcurrent = 5
	}

	// Group incidents by type. Within each group, severity ordering is preserved
	// from the DB query (critical > warning > info, then oldest first).
	groups := groupByType(incidents)

	wg := workflow.NewWaitGroup(ctx)
	sem := workflow.NewSemaphore(ctx, int64(maxConcurrent))

	for _, group := range groups {
		group := group // capture

		// Use per-type concurrency if configured, otherwise the global default.
		fc := followerConcurrent
		if n, ok := agentCfg.TypeConcurrency[group.Type]; ok {
			fc = n
		}

		_ = sem.Acquire(ctx, 1)
		wg.Add(1)

		workflow.Go(ctx, func(gCtx workflow.Context) {
			defer wg.Done()
			defer sem.Release(1)

			processIncidentGroup(gCtx, group, agentCfg.SystemPrompt, fc)
		})
	}

	wg.Wait(ctx)
	return nil
}

// incidentGroup is a set of incidents sharing the same type.
type incidentGroup struct {
	Type      string
	Incidents []activity.UnassignedIncident
}

// groupByType groups incidents by their type, preserving order within each group.
func groupByType(incidents []activity.UnassignedIncident) []incidentGroup {
	order := []string{}
	byType := map[string][]activity.UnassignedIncident{}
	for _, inc := range incidents {
		if _, ok := byType[inc.Type]; !ok {
			order = append(order, inc.Type)
		}
		byType[inc.Type] = append(byType[inc.Type], inc)
	}
	groups := make([]incidentGroup, 0, len(order))
	for _, t := range order {
		groups = append(groups, incidentGroup{Type: t, Incidents: byType[t]})
	}
	return groups
}

// processIncidentGroup handles a group of similar incidents using leader-follower:
// 1. Investigate the leader (first incident) without hints
// 2. If resolved, extract resolution hint
// 3. Investigate remaining followers with the hint, up to followerConcurrent in parallel
func processIncidentGroup(ctx workflow.Context, group incidentGroup, systemPrompt string, followerConcurrent int) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	logger := workflow.GetLogger(ctx)

	leader := group.Incidents[0]
	followers := group.Incidents[1:]

	// Claim the leader.
	var leaderClaimed bool
	err := workflow.ExecuteActivity(ctx, "ClaimIncidentForAgent", leader.ID).Get(ctx, &leaderClaimed)
	if err != nil {
		logger.Warn("failed to claim leader incident", "id", leader.ID, "error", err)
		leaderClaimed = false
	}

	var hints string

	if leaderClaimed {
		// Investigate the leader without hints.
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: "investigate-" + leader.ID,
			TaskQueue:  "hosting-tasks",
		})

		var result activity.InvestigateIncidentResult
		err := workflow.ExecuteChildWorkflow(childCtx, InvestigateIncidentWorkflow, leader.ID, systemPrompt, "").Get(ctx, &result)
		if err != nil {
			logger.Error("leader investigation failed", "id", leader.ID, "error", err)
		} else if result.Outcome == "resolved" && result.ResolutionHint != "" {
			hints = result.ResolutionHint
			logger.Info("leader resolved, passing hints to followers",
				"type", group.Type, "followers", len(followers))
		}
	}

	if len(followers) == 0 {
		return
	}

	// Process followers in parallel with a per-group concurrency limit.
	wg := workflow.NewWaitGroup(ctx)
	followerSem := workflow.NewSemaphore(ctx, int64(followerConcurrent))

	for _, follower := range followers {
		follower := follower // capture

		// Claim the follower.
		var claimed bool
		err := workflow.ExecuteActivity(ctx, "ClaimIncidentForAgent", follower.ID).Get(ctx, &claimed)
		if err != nil {
			logger.Warn("failed to claim follower incident", "id", follower.ID, "error", err)
			continue
		}
		if !claimed {
			continue
		}

		_ = followerSem.Acquire(ctx, 1)
		wg.Add(1)

		workflow.Go(ctx, func(gCtx workflow.Context) {
			defer wg.Done()
			defer followerSem.Release(1)

			childCtx := workflow.WithChildOptions(gCtx, workflow.ChildWorkflowOptions{
				WorkflowID: "investigate-" + follower.ID,
				TaskQueue:  "hosting-tasks",
			})
			err := workflow.ExecuteChildWorkflow(childCtx, InvestigateIncidentWorkflow, follower.ID, systemPrompt, hints).Get(gCtx, nil)
			if err != nil {
				workflow.GetLogger(gCtx).Error("follower investigation failed", "id", follower.ID, "error", err)
			}
		})
	}

	wg.Wait(ctx)
}

// InvestigateIncidentWorkflow orchestrates the full investigation of a single incident.
// hints is an optional resolution hint from a previously resolved similar incident.
func InvestigateIncidentWorkflow(ctx workflow.Context, incidentID string, systemPrompt string, hints string) (activity.InvestigateIncidentResult, error) {
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
		return activity.InvestigateIncidentResult{}, err
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
		Hints:           hints,
	}

	var result activity.InvestigateIncidentResult
	err = workflow.ExecuteActivity(investigateCtx, "InvestigateIncident", params).Get(ctx, &result)
	if err != nil {
		logger.Error("investigation activity failed", "id", incidentID, "error", err)
		escalateOnFailure(ctx, incidentID, fmt.Sprintf("Investigation activity failed: %v", err))
		return activity.InvestigateIncidentResult{}, err
	}

	// If max turns reached without resolution, escalate.
	if result.Outcome == "max_turns" {
		escalateOnFailure(ctx, incidentID, "Agent reached maximum investigation turns without resolving or escalating")
	}

	logger.Info("investigation completed",
		"id", incidentID, "outcome", result.Outcome, "turns", result.Turns)
	return result, nil
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
