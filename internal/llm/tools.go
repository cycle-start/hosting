package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Registry manages tool definitions and executes tool calls via the core API.
type Registry struct {
	httpClient *http.Client
	coreAPIURL string
	apiKey     string
}

// NewRegistry creates a new tool registry that calls the core API.
func NewRegistry(coreAPIURL, apiKey string) *Registry {
	return &Registry{
		httpClient: &http.Client{},
		coreAPIURL: coreAPIURL,
		apiKey:     apiKey,
	}
}

// Tools returns the tool definitions for the LLM.
func (r *Registry) Tools() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "get_incident",
				Description: "Get incident details by ID.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"Incident ID"}},"required":["id"]}`),
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "list_incident_events",
				Description: "Get the timeline of events for an incident.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"incident_id":{"type":"string","description":"Incident ID"},"limit":{"type":"integer","description":"Max events to return (default 50)"}},"required":["incident_id"]}`),
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "add_incident_event",
				Description: "Record an investigation step or finding as an incident event.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"incident_id":{"type":"string","description":"Incident ID"},"action":{"type":"string","enum":["investigated","attempted_fix","fix_succeeded","fix_failed","escalated","capability_gap","commented"],"description":"Event action type"},"detail":{"type":"string","description":"Description of the investigation step or finding"}},"required":["incident_id","action","detail"]}`),
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "resolve_incident",
				Description: "Mark an incident as resolved. Only call this when the underlying issue is confirmed fixed.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"Incident ID"},"resolution":{"type":"string","description":"Description of what was done to resolve the incident"}},"required":["id","resolution"]}`),
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "escalate_incident",
				Description: "Escalate an incident to a human operator. Use when: root cause needs human judgment, fix is irreversible, or available tools are insufficient.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"Incident ID"},"reason":{"type":"string","description":"Reason for escalation, including what was investigated and why escalation is needed"}},"required":["id","reason"]}`),
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "get_shard",
				Description: "Get shard details including status, role, and configuration.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"Shard ID"}},"required":["id"]}`),
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "list_nodes_by_shard",
				Description: "List all nodes belonging to a shard.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"shard_id":{"type":"string","description":"Shard ID"}},"required":["shard_id"]}`),
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "get_node",
				Description: "Get node details including hostname, IP, status, and role.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"Node ID"}},"required":["id"]}`),
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "get_tenant",
				Description: "Get tenant details by ID.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"Tenant ID"}},"required":["id"]}`),
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "converge_shard",
				Description: "Trigger shard convergence to re-apply the desired state to all nodes. This is a safe, reversible operation.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"shard_id":{"type":"string","description":"Shard ID"}},"required":["shard_id"]}`),
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "report_capability_gap",
				Description: "Report a missing tool or capability that would be needed to investigate or fix this type of incident.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"tool_name":{"type":"string","description":"Name of the missing tool"},"description":{"type":"string","description":"What the tool should do"},"category":{"type":"string","enum":["investigation","remediation","notification"],"description":"Tool category"},"incident_id":{"type":"string","description":"Related incident ID"}},"required":["tool_name","description","category"]}`),
			},
		},
	}
}

type toolHandler func(ctx context.Context, args json.RawMessage) (string, error)

// Execute dispatches a tool call to the appropriate handler and returns the JSON result.
func (r *Registry) Execute(ctx context.Context, name string, argsJSON string) (string, error) {
	handlers := map[string]toolHandler{
		"get_incident":         r.getIncident,
		"list_incident_events": r.listIncidentEvents,
		"add_incident_event":   r.addIncidentEvent,
		"resolve_incident":     r.resolveIncident,
		"escalate_incident":    r.escalateIncident,
		"get_shard":            r.getShard,
		"list_nodes_by_shard":  r.listNodesByShard,
		"get_node":             r.getNode,
		"get_tenant":           r.getTenant,
		"converge_shard":       r.convergeShard,
		"report_capability_gap": r.reportCapabilityGap,
	}

	handler, ok := handlers[name]
	if !ok {
		return fmt.Sprintf(`{"error":"unknown tool: %s"}`, name), nil
	}

	return handler(ctx, json.RawMessage(argsJSON))
}

// IsTerminal returns true if the tool name indicates the investigation should stop.
func IsTerminal(name string) bool {
	return name == "resolve_incident" || name == "escalate_incident"
}

// --- Tool handlers ---

func (r *Registry) getIncident(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	return r.apiGet(ctx, "/api/v1/incidents/"+p.ID)
}

func (r *Registry) listIncidentEvents(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		IncidentID string `json:"incident_id"`
		Limit      int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	path := fmt.Sprintf("/api/v1/incidents/%s/events", p.IncidentID)
	if p.Limit > 0 {
		path += fmt.Sprintf("?limit=%d", p.Limit)
	}
	return r.apiGet(ctx, path)
}

func (r *Registry) addIncidentEvent(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		IncidentID string `json:"incident_id"`
		Action     string `json:"action"`
		Detail     string `json:"detail"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	body := map[string]any{
		"actor":  "agent:incident-investigator",
		"action": p.Action,
		"detail": p.Detail,
	}
	return r.apiPost(ctx, fmt.Sprintf("/api/v1/incidents/%s/events", p.IncidentID), body)
}

func (r *Registry) resolveIncident(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		ID         string `json:"id"`
		Resolution string `json:"resolution"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	body := map[string]string{"resolution": p.Resolution}
	return r.apiPost(ctx, fmt.Sprintf("/api/v1/incidents/%s/resolve", p.ID), body)
}

func (r *Registry) escalateIncident(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		ID     string `json:"id"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	body := map[string]string{"reason": p.Reason}
	return r.apiPost(ctx, fmt.Sprintf("/api/v1/incidents/%s/escalate", p.ID), body)
}

func (r *Registry) getShard(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	return r.apiGet(ctx, "/api/v1/shards/"+p.ID)
}

func (r *Registry) listNodesByShard(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		ShardID string `json:"shard_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	return r.apiGet(ctx, "/api/v1/nodes?shard_id="+p.ShardID)
}

func (r *Registry) getNode(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	return r.apiGet(ctx, "/api/v1/nodes/"+p.ID)
}

func (r *Registry) getTenant(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	return r.apiGet(ctx, "/api/v1/tenants/"+p.ID)
}

func (r *Registry) convergeShard(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		ShardID string `json:"shard_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	return r.apiPost(ctx, fmt.Sprintf("/api/v1/shards/%s/converge", p.ShardID), nil)
}

func (r *Registry) reportCapabilityGap(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		ToolName    string  `json:"tool_name"`
		Description string  `json:"description"`
		Category    string  `json:"category"`
		IncidentID  *string `json:"incident_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}
	body := map[string]any{
		"tool_name":   p.ToolName,
		"description": p.Description,
		"category":    p.Category,
	}
	if p.IncidentID != nil {
		body["incident_id"] = *p.IncidentID
	}
	return r.apiPost(ctx, "/api/v1/capability-gaps", body)
}

// --- HTTP helpers ---

func (r *Registry) apiGet(ctx context.Context, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.coreAPIURL+path, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	return r.doRequest(req)
}

func (r *Registry) apiPost(ctx context.Context, path string, body any) (string, error) {
	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return "", fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.coreAPIURL+path, bodyReader)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return r.doRequest(req)
}

func (r *Registry) doRequest(req *http.Request) (string, error) {
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("api request %s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Sprintf(`{"error":"API returned status %d","body":%s}`, resp.StatusCode, string(respBody)), nil
	}

	return string(respBody), nil
}
