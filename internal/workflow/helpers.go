package workflow

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// fanOutNodes executes fn for each node in parallel using Temporal goroutines
// and collects errors. Returns nil if all succeeded, or the collected error
// strings if any failed.
func fanOutNodes(ctx workflow.Context, nodes []model.Node, fn func(workflow.Context, model.Node) error) []string {
	if len(nodes) <= 1 {
		// No benefit from fan-out with 0 or 1 node — run inline.
		for _, node := range nodes {
			if err := fn(ctx, node); err != nil {
				return []string{err.Error()}
			}
		}
		return nil
	}

	mu := workflow.NewMutex(ctx)
	wg := workflow.NewWaitGroup(ctx)
	var errs []string
	for _, node := range nodes {
		node := node // capture loop variable
		wg.Add(1)
		workflow.Go(ctx, func(gCtx workflow.Context) {
			defer wg.Done()
			if err := fn(gCtx, node); err != nil {
				_ = mu.Lock(gCtx)
				errs = append(errs, err.Error())
				mu.Unlock()
			}
		})
	}
	wg.Wait(ctx)
	return errs
}

// joinErrors joins error strings, truncating at 4000 chars.
func joinErrors(errs []string) string {
	msg := strings.Join(errs, "; ")
	if len(msg) > 4000 {
		msg = msg[:4000]
	}
	return msg
}

// setResourceFailed sets a resource status to failed with an error message and
// creates an incident so the failure is visible and tracked. Errors from
// incident creation are logged but do not affect the return value.
func setResourceFailed(ctx workflow.Context, table string, id string, err error) error {
	msg := err.Error()
	statusErr := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:         table,
		ID:            id,
		Status:        model.StatusFailed,
		StatusMessage: &msg,
	}).Get(ctx, nil)

	// Create an incident — deduped by resource so repeated failures don't spam.
	resType := strings.TrimSuffix(table, "s") // "tenants" -> "tenant"
	createIncident(ctx, activity.CreateIncidentParams{
		DedupeKey:    fmt.Sprintf("provisioning_failed:%s:%s", table, id),
		Type:         "provisioning_failed",
		Severity:     "warning",
		Title:        fmt.Sprintf("%s provisioning failed", resType),
		Detail:       msg,
		ResourceType: &resType,
		ResourceID:   &id,
		Source:       "workflow",
	})

	return statusErr
}

// createIncident fires a CreateIncident activity. Errors are logged but not propagated
// to avoid failing the calling workflow due to incident tracking issues.
// For newly created critical incidents, a webhook notification is also sent.
func createIncident(ctx workflow.Context, params activity.CreateIncidentParams) {
	var result activity.CreateIncidentResult
	err := workflow.ExecuteActivity(ctx, "CreateIncident", params).Get(ctx, &result)
	if err != nil {
		workflow.GetLogger(ctx).Warn("failed to create incident",
			"dedupe_key", params.DedupeKey, "error", err)
		return
	}

	// Fire webhook for newly created critical/warning incidents.
	if result.Created && (params.Severity == "critical" || params.Severity == "warning") {
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
		sendIncidentWebhook(ctx, inc, params.Severity)
	}
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

// ChildWorkflowSpec describes a child workflow to be spawned in parallel.
type ChildWorkflowSpec struct {
	WorkflowName string
	WorkflowID   string
	Arg          any
}

// fanOutChildWorkflows spawns all children in parallel and collects errors.
// Returns nil if all succeeded, or the collected error strings if any failed.
func fanOutChildWorkflows(ctx workflow.Context, children []ChildWorkflowSpec) []string {
	if len(children) == 0 {
		return nil
	}

	mu := workflow.NewMutex(ctx)
	wg := workflow.NewWaitGroup(ctx)
	var errs []string

	for _, child := range children {
		child := child // capture
		wg.Add(1)
		workflow.Go(ctx, func(gCtx workflow.Context) {
			defer wg.Done()
			childCtx := workflow.WithChildOptions(gCtx, workflow.ChildWorkflowOptions{
				WorkflowID: child.WorkflowID,
				TaskQueue:  "hosting-tasks",
			})
			if err := workflow.ExecuteChildWorkflow(childCtx, child.WorkflowName, child.Arg).Get(gCtx, nil); err != nil {
				_ = mu.Lock(gCtx)
				errs = append(errs, fmt.Sprintf("%s(%s): %v", child.WorkflowName, child.WorkflowID, err))
				mu.Unlock()
			}
		})
	}

	wg.Wait(ctx)
	return errs
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
			MaximumAttempts:    0, // unlimited — bounded by ScheduleToCloseTimeout
			InitialInterval:    5 * time.Second,
			MaximumInterval:    1 * time.Minute,
			BackoffCoefficient: 2.0,
		},
	})
}
