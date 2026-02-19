# Agent Control Plane

## Overview

The agent control plane is an autonomous LLM-powered system that investigates and remediates infrastructure incidents without human intervention. It runs a 70B parameter model (Qwen2.5-72B-Instruct) on a self-hosted NVIDIA DGX Spark via vLLM, exposed as an OpenAI-compatible API.

The agent operates through Temporal workflows: a cron-scheduled queue processor picks up unassigned incidents, groups them by type, and dispatches investigations using a leader-follower pattern that avoids redundant work and propagates resolution knowledge across similar incidents.

All tool calls and investigation steps are recorded as incident events, providing full observability into the agent's decision-making.

## Architecture

Three layers orchestrate the investigation:

```
ProcessIncidentQueueWorkflow (cron, every 1 min)
  |
  +-- Groups incidents by type (severity-ordered: critical > warning > info, then oldest first)
  |
  +-- Per group (up to AGENT_MAX_CONCURRENT in parallel):
        |
        +-- Leader: InvestigateIncidentWorkflow (no hints)
        |     |
        |     +-- AssembleIncidentContext (activity)
        |     +-- InvestigateIncident (activity, multi-turn LLM loop)
        |
        +-- If leader resolves: extract resolution hint
        |
        +-- Followers: InvestigateIncidentWorkflow (with hints, up to follower_concurrent in parallel)
```

### ProcessIncidentQueueWorkflow

Runs on a 1-minute cron schedule. Fetches up to 20 unassigned open incidents, groups them by type, and processes each group concurrently (bounded by `AGENT_MAX_CONCURRENT`). Reads agent configuration from `platform_config` before dispatching.

### InvestigateIncidentWorkflow

Per-incident child workflow. Two phases:

1. **Assemble context** -- fetches the incident and its event history (retryable, 30s timeout, 3 attempts).
2. **Run LLM loop** -- executes the `InvestigateIncident` activity (no retry since LLM responses are non-deterministic, 30-minute timeout, 2-minute heartbeat interval).

If the activity fails or reaches max turns without resolution, the workflow auto-escalates via `EscalateIncident`.

### InvestigateIncident Activity

The core investigation loop:

1. Builds message history: system prompt, incident context as JSON, optional resolution hints.
2. Sends to LLM with all tool definitions.
3. LLM responds with tool calls -- each is executed via HTTP against the core API.
4. Tool results are appended to the conversation and fed back to the LLM.
5. Every tool call is recorded as an `incident_event` with full metadata (tool name, arguments, result).
6. Loop terminates when the LLM calls `resolve_incident`, `escalate_incident`, or max turns is reached.

## Smart Scheduling (Leader-Follower Pattern)

The leader-follower pattern prevents the thundering herd problem where dozens of identical incidents (e.g., disk pressure across a cluster) each trigger independent, redundant investigations.

### How it works

1. **Grouping** -- incidents are grouped by `type`. Within each group, ordering is critical > warning > info, then oldest first.
2. **Leader election** -- the first incident in each group is the leader. It is claimed atomically (`UPDATE ... WHERE assigned_to IS NULL`) and investigated without hints.
3. **Hint extraction** -- if the leader resolves, a structured hint is built from its tool call history containing: incident type, title, resolution text, and up to 10 investigation steps.
4. **Follower dispatch** -- remaining incidents in the group are claimed and investigated in parallel (bounded by `follower_concurrent`), each receiving the leader's hint as additional user context.
5. **Hint adaptation** -- the LLM uses the hint as a starting point but adapts to each follower's specific context (different node, different shard, etc.).

If the leader does not resolve (escalated or max turns), followers proceed without hints.

### Per-type concurrency override

Global follower concurrency (`AGENT_FOLLOWER_CONCURRENT`) can be overridden per incident type via `platform_config`:

```sql
INSERT INTO platform_config (key, value) VALUES ('agent.concurrency.disk_pressure', '10');
INSERT INTO platform_config (key, value) VALUES ('agent.concurrency.replication_lag', '2');
```

This allows tuning: disk pressure incidents can fan out widely (the fix is usually the same `converge_shard` call), while replication lag should be conservative to avoid cascading actions.

## Configuration

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `AGENT_ENABLED` | `false` | Enable the LLM incident agent |
| `AGENT_MAX_CONCURRENT` | `3` | Max parallel incident group leaders |
| `AGENT_FOLLOWER_CONCURRENT` | `5` | Max parallel followers per group (global default) |
| `AGENT_API_KEY` | (required when enabled) | API key for the agent to call the core API |
| `LLM_BASE_URL` | (required when enabled) | vLLM endpoint base URL (e.g., `http://dgx:8000`) |
| `LLM_API_KEY` | `""` | API key for the LLM endpoint (optional, depends on vLLM config) |
| `LLM_MODEL` | `Qwen/Qwen2.5-72B-Instruct` | Model name passed to the chat completions API |
| `LLM_MAX_TURNS` | `10` | Max conversation turns per investigation |

When `AGENT_ENABLED` is `true`, `LLM_BASE_URL` and `AGENT_API_KEY` are required (validated at startup).

### Platform Config (Runtime)

These are stored in the `platform_config` table and read at the start of each queue processing cycle:

| Key | Description |
|---|---|
| `agent.system_prompt` | Override the default system prompt |
| `agent.concurrency.<type>` | Per-type follower concurrency (e.g., `agent.concurrency.disk_pressure = 10`) |

## Available Tools

The agent has 11 tools, all executed as HTTP calls to the core API using `AGENT_API_KEY`:

### Investigation

| Tool | Description |
|---|---|
| `get_incident` | Get incident details by ID |
| `list_incident_events` | Get the event timeline for an incident (default limit: 50) |
| `add_incident_event` | Record an investigation step or finding. Actions: `investigated`, `attempted_fix`, `fix_succeeded`, `fix_failed`, `escalated`, `capability_gap`, `commented` |
| `get_shard` | Get shard details (status, role, configuration) |
| `list_nodes_by_shard` | List all nodes belonging to a shard |
| `get_node` | Get node details (hostname, IP, status, role) |
| `get_tenant` | Get tenant details by ID |

### Remediation

| Tool | Description |
|---|---|
| `converge_shard` | Trigger shard convergence to re-apply desired state to all nodes. Safe and reversible |
| `resolve_incident` | Mark an incident as resolved (terminal -- ends the investigation loop) |
| `escalate_incident` | Escalate to a human operator (terminal -- ends the investigation loop) |

### Meta

| Tool | Description |
|---|---|
| `report_capability_gap` | Report a missing tool or capability. Categories: `investigation`, `remediation`, `notification` |

The `resolve_incident` and `escalate_incident` tools are terminal -- calling either one ends the investigation loop immediately.

## Investigation Loop Detail

```
System prompt + incident JSON → LLM
                                  ↓
                          Tool calls ← LLM response
                                  ↓
                    Execute via core API HTTP
                                  ↓
                    Record as incident_event
                                  ↓
                    Feed result back → LLM
                                  ↓
                          Repeat until:
                            - resolve_incident called
                            - escalate_incident called
                            - max turns reached (auto-escalate)
                            - LLM responds without tool calls (treated as max_turns)
```

The LLM HTTP client has a 5-minute timeout per request. The Temporal activity has a 30-minute `StartToCloseTimeout` and a 2-minute `HeartbeatTimeout`, with heartbeats recorded at each turn (`"turn N/M"`).

## Resolution Hints

When a leader incident resolves, a structured hint is built from its investigation history:

```
Incident type: replication_lag
Title: MySQL replication lag exceeded 30s on shard db-osl-1
Resolution: Converged shard to re-sync replication after node recovery

Investigation steps taken:
  1. get_incident({"id":"inc_abc123"})
  2. get_shard({"id":"db-osl-1"})
  3. list_nodes_by_shard({"shard_id":"db-osl-1"})
  4. get_node({"id":"node-xyz"})
  5. add_incident_event({"incident_id":"inc_abc123","action":"investigated","detail":"..."})
  6. converge_shard({"shard_id":"db-osl-1"})
  7. resolve_incident({"id":"inc_abc123","resolution":"..."})
```

The hint includes up to 10 steps (truncated with a count if more). Tool arguments are truncated at 120 characters. Followers receive this as a user message: *"A similar incident of the same type was recently investigated and resolved. Use this as a starting point."*

## Webhook Notifications

Webhooks fire on two triggers:

| Trigger | When |
|---|---|
| `critical` | New critical-severity incident created |
| `escalated` | Incident escalated (by agent or manually) |

Configuration is via `platform_config`:

```sql
-- Slack notifications for critical incidents
INSERT INTO platform_config (key, value) VALUES ('webhook.critical.url', 'https://hooks.slack.com/...');
INSERT INTO platform_config (key, value) VALUES ('webhook.critical.template', 'slack');

-- Generic JSON webhook for escalations
INSERT INTO platform_config (key, value) VALUES ('webhook.escalated.url', 'https://ops.example.com/webhook');
INSERT INTO platform_config (key, value) VALUES ('webhook.escalated.template', 'generic');
```

Templates:
- `generic` (default) -- standard JSON payload
- `slack` -- Slack Block Kit format

Webhook delivery retries up to 3 times with a 30-second timeout per attempt. Failures are logged but do not affect incident processing.

## Default System Prompt

The built-in system prompt (overridable via `agent.system_prompt` in `platform_config`) establishes:

- **Platform context** -- multi-region/cluster architecture, shard roles, MySQL replication pairs, CephFS web shards.
- **Responsibilities** -- investigate systematically, record every step, attempt safe remediation, escalate when tools are insufficient.
- **Decision framework** -- replication broken: check shard/node status then converge; shard degraded: list nodes then identify failures; unknown: gather context then escalate.
- **Constraints** -- no destructive actions, no resolving without confirmation, no escalating without at least one diagnostic action, no duplicate tool calls with identical arguments.

## Observability

Every action the agent takes is recorded as an `incident_event`:

- **Claim** -- `action: "investigated"`, detail: "Agent claimed incident for investigation"
- **Tool calls** -- `action: "investigated"`, detail: "Called tool: {name}", metadata: `{"tool": "...", "arguments": {...}, "result": {...}}`
- **Comments** -- `action: "commented"`, detail: LLM's text response (when it responds without tool calls)
- **Escalation** -- `action: "escalated"`, detail: reason

All events use actor `agent:incident-investigator`. The full investigation timeline is queryable via `GET /api/v1/incidents/{id}/events`.
