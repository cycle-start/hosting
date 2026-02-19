# LLM Incident Investigation Agent

Autonomous incident investigation and remediation powered by a self-hosted LLM (Qwen 2.5 72B or similar) via vLLM with OpenAI-compatible API.

## Architecture

```
ProcessIncidentQueueWorkflow (cron, every 1 min)
  -> ListUnassignedOpenIncidents (activity)
  -> For each: ClaimIncidentForAgent (activity)
  -> Fan out: InvestigateIncidentWorkflow (child workflow per incident)
      -> AssembleIncidentContext (activity, retryable)
      -> InvestigateIncident (activity, single-attempt, runs full LLM loop)
          -> LLM chat call -> parse tool_calls -> execute via HTTP to core API -> repeat
          -> Exits on: resolve_incident / escalate_incident / max turns
      -> On failure: EscalateIncident (activity, retryable)
```

### Key Design Decisions

1. **Single activity for agent loop** -- The full multi-turn conversation runs inside one `InvestigateIncident` activity. LLM responses are non-deterministic, so separate Temporal activities per turn would cause replay issues. Each turn is logged as an `incident_event`.

2. **Tool execution via HTTP to core API** -- Tools call the REST API with an internal API key, reusing all existing validation and business logic. Same interface as external MCP agents.

3. **Cron trigger** -- Polls every minute for unassigned open incidents. Simpler than signal-based triggering. Atomic `ClaimIncidentForAgent` prevents duplicate investigations.

4. **Feature flag** -- `AGENT_ENABLED=false` by default. LLM config only validated when enabled.

5. **Concurrency limit** -- `AGENT_MAX_CONCURRENT` (default: 3) caps parallel investigations.

## Configuration

| Env Var | Default | Description |
|---------|---------|-------------|
| `AGENT_ENABLED` | `false` | Enable the incident agent |
| `AGENT_MAX_CONCURRENT` | `3` | Max parallel investigations |
| `AGENT_API_KEY` | (required when enabled) | API key for core API calls |
| `LLM_BASE_URL` | (required when enabled) | OpenAI-compatible API URL |
| `LLM_API_KEY` | `""` | API key for LLM endpoint |
| `LLM_MODEL` | `Qwen/Qwen2.5-72B-Instruct` | Model name |
| `LLM_MAX_TURNS` | `10` | Max conversation turns per investigation |

## Tools

The agent has access to 11 tools, executed via HTTP calls to the core API:

| Tool | Type | Description |
|------|------|-------------|
| `get_incident` | read | Get incident details by ID |
| `list_incident_events` | read | Get timeline for an incident |
| `add_incident_event` | write | Record investigation step (actor: `agent:incident-investigator`) |
| `resolve_incident` | write | Mark incident resolved |
| `escalate_incident` | write | Escalate with reason |
| `get_shard` | read | Shard status/role/config |
| `list_nodes_by_shard` | read | Nodes in a shard |
| `get_node` | read | Node hostname/IP/status |
| `get_tenant` | read | Tenant details |
| `converge_shard` | write | Trigger shard convergence |
| `report_capability_gap` | write | Report missing tool |

## System Prompt

The default system prompt is hardcoded in `internal/activity/agent.go` and can be overridden via `platform_config` key `agent.system_prompt`.

Update it at runtime:

```
PUT /api/v1/platform/config
{"key": "agent.system_prompt", "value": "..."}
```

## Observability

Every tool call and investigation step is recorded as an `incident_event` with:
- Actor: `agent:incident-investigator`
- Metadata: tool name, arguments, and result as JSON

The full investigation timeline is visible via `GET /api/v1/incidents/{id}/events`.

## Capability Gaps

When the agent encounters a situation it can't handle, it reports a `capability_gap` via the `report_capability_gap` tool. These are tracked in the `capability_gaps` table and visible via `GET /api/v1/capability-gaps`, sorted by occurrence count.

## Files

| File | Purpose |
|------|---------|
| `internal/llm/client.go` | OpenAI-compatible HTTP client |
| `internal/llm/tools.go` | Tool registry with HTTP-based execution |
| `internal/activity/agent.go` | Agent activities (system prompt, claim, investigate) |
| `internal/activity/incident.go` | `EscalateIncident` activity |
| `internal/workflow/incident_agent.go` | Queue processor + investigation workflows |
| `internal/config/config.go` | LLM/agent configuration |
