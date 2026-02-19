package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go.temporal.io/sdk/activity"

	"github.com/edvin/hosting/internal/llm"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
)

// AgentActivities contains activities for the LLM incident investigation agent.
type AgentActivities struct {
	db       DB
	llm      *llm.Client
	tools    *llm.Registry
	maxTurns int
}

// NewAgentActivities creates a new AgentActivities struct.
func NewAgentActivities(db DB, llmClient *llm.Client, tools *llm.Registry, maxTurns int) *AgentActivities {
	return &AgentActivities{
		db:       db,
		llm:      llmClient,
		tools:    tools,
		maxTurns: maxTurns,
	}
}

// GetAgentConfig returns agent configuration from platform_config with defaults.
// Keys checked: agent.system_prompt, agent.concurrency.<type> (e.g. agent.concurrency.disk_pressure).
type AgentConfig struct {
	SystemPrompt       string         `json:"system_prompt"`
	TypeConcurrency    map[string]int `json:"type_concurrency"`
}

func (a *AgentActivities) GetAgentConfig(ctx context.Context) (*AgentConfig, error) {
	cfg := &AgentConfig{
		SystemPrompt:    DefaultSystemPrompt,
		TypeConcurrency: map[string]int{},
	}

	rows, err := a.db.Query(ctx,
		`SELECT key, value FROM platform_config WHERE key LIKE 'agent.%'`)
	if err != nil {
		return cfg, nil // non-fatal, use defaults
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		switch {
		case key == "agent.system_prompt" && value != "":
			cfg.SystemPrompt = value
		case len(key) > len("agent.concurrency."):
			typeName := key[len("agent.concurrency."):]
			if n, err := strconv.Atoi(value); err == nil && n > 0 {
				cfg.TypeConcurrency[typeName] = n
			}
		}
	}

	return cfg, nil
}

// GetAgentSystemPrompt returns the system prompt from platform_config, or the default.
func (a *AgentActivities) GetAgentSystemPrompt(ctx context.Context) (string, error) {
	var value string
	err := a.db.QueryRow(ctx,
		`SELECT value FROM platform_config WHERE key = 'agent.system_prompt'`,
	).Scan(&value)
	if err == nil && value != "" {
		return value, nil
	}
	return DefaultSystemPrompt, nil
}

// UnassignedIncident is a lightweight incident summary for the queue processor.
type UnassignedIncident struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
}

// ListUnassignedOpenIncidents returns open incidents not yet assigned to any agent.
func (a *AgentActivities) ListUnassignedOpenIncidents(ctx context.Context) ([]UnassignedIncident, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, type, severity, title FROM incidents
		 WHERE status = 'open' AND assigned_to IS NULL
		 ORDER BY
		   CASE severity WHEN 'critical' THEN 0 WHEN 'warning' THEN 1 ELSE 2 END,
		   detected_at ASC
		 LIMIT 20`,
	)
	if err != nil {
		return nil, fmt.Errorf("list unassigned incidents: %w", err)
	}
	defer rows.Close()

	var incidents []UnassignedIncident
	for rows.Next() {
		var inc UnassignedIncident
		if err := rows.Scan(&inc.ID, &inc.Type, &inc.Severity, &inc.Title); err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		incidents = append(incidents, inc)
	}
	return incidents, nil
}

// ClaimIncidentForAgent atomically claims an incident for the agent.
// Returns true if the claim succeeded, false if already claimed.
func (a *AgentActivities) ClaimIncidentForAgent(ctx context.Context, incidentID string) (bool, error) {
	now := time.Now()
	tag, err := a.db.Exec(ctx,
		`UPDATE incidents SET assigned_to = 'agent:incident-investigator', status = $1, updated_at = $2
		 WHERE id = $3 AND assigned_to IS NULL AND status = 'open'`,
		model.IncidentInvestigating, now, incidentID,
	)
	if err != nil {
		return false, fmt.Errorf("claim incident: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}

	// Record the claim event.
	_, _ = a.db.Exec(ctx,
		`INSERT INTO incident_events (id, incident_id, actor, action, detail, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		platform.NewID(), incidentID, "agent:incident-investigator", "investigated",
		"Agent claimed incident for investigation", now,
	)
	return true, nil
}

// IncidentContext is the structured context assembled for the LLM.
type IncidentContext struct {
	Incident model.Incident      `json:"incident"`
	Events   []model.IncidentEvent `json:"events"`
}

// AssembleIncidentContext fetches the incident and its events for the LLM.
func (a *AgentActivities) AssembleIncidentContext(ctx context.Context, incidentID string) (*IncidentContext, error) {
	var inc model.Incident
	err := a.db.QueryRow(ctx,
		`SELECT id, dedupe_key, type, severity, status, title, detail,
		        resource_type, resource_id, source, assigned_to, resolution,
		        detected_at, resolved_at, escalated_at, created_at, updated_at
		 FROM incidents WHERE id = $1`, incidentID,
	).Scan(&inc.ID, &inc.DedupeKey, &inc.Type, &inc.Severity,
		&inc.Status, &inc.Title, &inc.Detail, &inc.ResourceType,
		&inc.ResourceID, &inc.Source, &inc.AssignedTo, &inc.Resolution,
		&inc.DetectedAt, &inc.ResolvedAt, &inc.EscalatedAt,
		&inc.CreatedAt, &inc.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get incident: %w", err)
	}

	rows, err := a.db.Query(ctx,
		`SELECT id, incident_id, actor, action, detail, metadata, created_at
		 FROM incident_events WHERE incident_id = $1 ORDER BY created_at ASC LIMIT 50`,
		incidentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var events []model.IncidentEvent
	for rows.Next() {
		var evt model.IncidentEvent
		if err := rows.Scan(&evt.ID, &evt.IncidentID, &evt.Actor, &evt.Action,
			&evt.Detail, &evt.Metadata, &evt.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, evt)
	}

	return &IncidentContext{Incident: inc, Events: events}, nil
}

// InvestigateIncidentParams holds the input for the investigation loop.
type InvestigateIncidentParams struct {
	SystemPrompt    string           `json:"system_prompt"`
	IncidentContext *IncidentContext  `json:"incident_context"`
	Hints           string           `json:"hints,omitempty"`
}

// InvestigateIncidentResult holds the outcome of the investigation.
type InvestigateIncidentResult struct {
	Outcome        string `json:"outcome"` // "resolved", "escalated", "max_turns"
	Turns          int    `json:"turns"`
	Summary        string `json:"summary"`
	ResolutionHint string `json:"resolution_hint,omitempty"`
}

// InvestigateIncident runs the multi-turn LLM investigation loop.
// Between each turn, it checks for admin_message events posted by human operators
// and injects them into the conversation, enabling live chat with the agent.
func (a *AgentActivities) InvestigateIncident(ctx context.Context, params InvestigateIncidentParams) (*InvestigateIncidentResult, error) {
	incidentJSON, err := json.Marshal(params.IncidentContext)
	if err != nil {
		return nil, fmt.Errorf("marshal incident context: %w", err)
	}

	messages := []llm.Message{
		{Role: "system", Content: params.SystemPrompt},
		{Role: "user", Content: fmt.Sprintf("Investigate the following incident:\n\n%s", string(incidentJSON))},
	}

	// Inject hints from a previously resolved similar incident.
	if params.Hints != "" {
		messages = append(messages, llm.Message{
			Role: "user",
			Content: fmt.Sprintf("A similar incident of the same type was recently investigated and resolved. "+
				"Use this as a starting point — the same approach may apply:\n\n%s", params.Hints),
		})
	}

	tools := a.tools.Tools()
	incidentID := params.IncidentContext.Incident.ID

	// Track tool calls for building resolution hints.
	var toolHistory []toolCallRecord
	var resolveArgs string

	// Track which admin messages we've already injected (by event ID).
	seenAdminMessages := map[string]bool{}

	for turn := 1; turn <= a.maxTurns; turn++ {
		activity.RecordHeartbeat(ctx, fmt.Sprintf("turn %d/%d", turn, a.maxTurns))

		// Check for new admin messages between turns.
		adminMsgs := a.fetchAdminMessages(ctx, incidentID, seenAdminMessages)
		for _, msg := range adminMsgs {
			messages = append(messages, llm.Message{
				Role:    "user",
				Content: fmt.Sprintf("Message from admin operator:\n\n%s", msg.Detail),
			})
			// Record that we acknowledged the message.
			a.recordEvent(ctx, incidentID, "commented",
				fmt.Sprintf("Received admin message: %s", truncate(msg.Detail, 100)))
		}

		resp, err := a.llm.Chat(ctx, llm.ChatRequest{
			Messages: messages,
			Tools:    tools,
		})
		if err != nil {
			return nil, fmt.Errorf("llm chat turn %d: %w", turn, err)
		}

		if len(resp.Choices) == 0 {
			return nil, fmt.Errorf("llm returned no choices on turn %d", turn)
		}

		assistantMsg := resp.Choices[0].Message
		messages = append(messages, assistantMsg)

		// If no tool calls, the model wants to stop.
		if len(assistantMsg.ToolCalls) == 0 {
			// Record the final message as an event.
			if assistantMsg.Content != "" {
				a.recordEvent(ctx, incidentID, "commented", assistantMsg.Content)
			}
			return &InvestigateIncidentResult{
				Outcome: "max_turns",
				Turns:   turn,
				Summary: assistantMsg.Content,
			}, nil
		}

		// Execute each tool call.
		var outcome string
		for _, tc := range assistantMsg.ToolCalls {
			result, execErr := a.tools.Execute(ctx, tc.Function.Name, tc.Function.Arguments)
			if execErr != nil {
				result = fmt.Sprintf(`{"error":"%s"}`, execErr.Error())
			}

			messages = append(messages, llm.Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})

			// Track for resolution hint building.
			toolHistory = append(toolHistory, toolCallRecord{
				Name: tc.Function.Name, Args: tc.Function.Arguments, Result: result,
			})

			// Record tool call as an event.
			metadata, _ := json.Marshal(map[string]any{
				"tool":      tc.Function.Name,
				"arguments": json.RawMessage(tc.Function.Arguments),
				"result":    json.RawMessage(result),
			})
			a.recordEventWithMetadata(ctx, incidentID, "investigated",
				fmt.Sprintf("Called tool: %s", tc.Function.Name), metadata)

			if llm.IsTerminal(tc.Function.Name) {
				if tc.Function.Name == "resolve_incident" {
					outcome = "resolved"
					resolveArgs = tc.Function.Arguments
				} else {
					outcome = "escalated"
				}
			}
		}

		if outcome != "" {
			result := &InvestigateIncidentResult{
				Outcome: outcome,
				Turns:   turn,
				Summary: fmt.Sprintf("Investigation completed after %d turns: %s", turn, outcome),
			}
			if outcome == "resolved" {
				result.ResolutionHint = buildResolutionHint(
					params.IncidentContext.Incident.Type,
					params.IncidentContext.Incident.Title,
					resolveArgs, toolHistory,
				)
			}
			return result, nil
		}
	}

	// Max turns reached — escalate.
	a.recordEvent(ctx, incidentID, "escalated", "Agent reached maximum investigation turns without resolution")

	return &InvestigateIncidentResult{
		Outcome: "max_turns",
		Turns:   a.maxTurns,
		Summary: "Agent reached maximum investigation turns",
	}, nil
}

// adminMessage is a lightweight struct for admin messages fetched between turns.
type adminMessage struct {
	ID     string
	Detail string
}

// fetchAdminMessages queries for admin_message events not yet seen and marks them as seen.
func (a *AgentActivities) fetchAdminMessages(ctx context.Context, incidentID string, seen map[string]bool) []adminMessage {
	rows, err := a.db.Query(ctx,
		`SELECT id, detail FROM incident_events
		 WHERE incident_id = $1 AND action = 'admin_message'
		 ORDER BY created_at ASC`,
		incidentID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var msgs []adminMessage
	for rows.Next() {
		var id, detail string
		if err := rows.Scan(&id, &detail); err != nil {
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		msgs = append(msgs, adminMessage{ID: id, Detail: detail})
	}
	return msgs
}

// buildResolutionHint creates a structured summary of how an incident was resolved,
// suitable for passing as context to similar incidents.
func buildResolutionHint(incidentType, title, resolveArgs string, history []toolCallRecord) string {
	var resolution string
	if resolveArgs != "" {
		var args struct {
			Resolution string `json:"resolution"`
		}
		if json.Unmarshal([]byte(resolveArgs), &args) == nil {
			resolution = args.Resolution
		}
	}

	hint := fmt.Sprintf("Incident type: %s\nTitle: %s\n", incidentType, title)
	if resolution != "" {
		hint += fmt.Sprintf("Resolution: %s\n", resolution)
	}

	hint += "\nInvestigation steps taken:\n"
	for i, tc := range history {
		if i >= 10 {
			hint += fmt.Sprintf("... and %d more steps\n", len(history)-10)
			break
		}
		hint += fmt.Sprintf("  %d. %s(%s)\n", i+1, tc.Name, truncate(tc.Args, 120))
	}

	return hint
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

type toolCallRecord struct {
	Name   string `json:"tool"`
	Args   string `json:"arguments"`
	Result string `json:"result"`
}

func (a *AgentActivities) recordEvent(ctx context.Context, incidentID, action, detail string) {
	a.recordEventWithMetadata(ctx, incidentID, action, detail, nil)
}

func (a *AgentActivities) recordEventWithMetadata(ctx context.Context, incidentID, action, detail string, metadata []byte) {
	if metadata == nil {
		metadata = []byte("{}")
	}
	_, _ = a.db.Exec(ctx,
		`INSERT INTO incident_events (id, incident_id, actor, action, detail, metadata, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		platform.NewID(), incidentID, "agent:incident-investigator", action, detail, metadata, time.Now(),
	)
}

// DefaultSystemPrompt is the fallback system prompt when none is configured.
const DefaultSystemPrompt = `You are an autonomous infrastructure incident responder for a hosting platform.

## Platform Architecture
- Multi-region, multi-cluster hosting platform
- Shard roles: web, database, dns, email, valkey, s3
- Database shards: MySQL primary/replica pairs
- Web shards: 2-3 nodes with CephFS shared storage

## Your Responsibilities
1. Investigate incidents systematically — gather information before acting
2. Record every investigation step using add_incident_event
3. Attempt safe, reversible remediation (e.g. converge_shard)
4. Escalate when: root cause needs human judgment, fix is irreversible, or tools are insufficient
5. Report missing tools using report_capability_gap

## Decision Framework
- Replication broken → check shard/node status → converge if nodes healthy
- Shard degraded → list nodes → identify failures → consider convergence
- Unknown type → gather all context → escalate with full summary

## Admin Messages
Human operators may send you messages during your investigation. These appear as
"Message from admin operator:" in the conversation. When you receive one:
- Acknowledge the message and adjust your investigation accordingly
- Follow any instructions or guidance provided
- Continue reporting your findings and actions as normal

## Constraints
- No destructive actions (delete resources, revoke keys)
- Do not resolve unless the underlying issue is confirmed fixed
- Do not escalate without at least one diagnostic action
- Do not call the same tool twice with identical arguments`
