# Incident Management

Automated incident detection, investigation, remediation, and escalation. Health crons detect problems, an LLM agent investigates and fixes them, and capability gaps track what tools the agent still needs. No external dependencies — the platform is the source of truth.

## How It Works

```
Health Crons (7 crons, 1-10 min intervals)
  |
  +-- Detect problem → CreateIncident (deduplicated)
  +-- Problem resolved → AutoResolveIncidents
  |
  v
ProcessIncidentQueueWorkflow (cron, every 1 min)
  |
  +-- Group unassigned incidents by type
  +-- Leader-follower pattern per group
  |     |
  |     +-- Leader: InvestigateIncidentWorkflow (no hints)
  |     +-- If resolved → extract resolution hint
  |     +-- Followers: InvestigateIncidentWorkflow (with hints)
  |
  v
InvestigateIncident (multi-turn LLM loop)
  |
  +-- LLM calls tools via core API
  +-- Every step recorded as incident_event
  +-- Admin can send live messages to guide the agent
  +-- Exits: resolve_incident | escalate_incident | max turns
  |
  v
EscalateStaleIncidentsWorkflow (cron, every 5 min)
  |
  +-- Critical unassigned >15 min → escalate
  +-- Warning unassigned >1 hour → escalate
  +-- Investigating/remediating >30 min → escalate
  +-- Sends webhook notification
```

## Data Model

### incidents

| Column | Type | Description |
|---|---|---|
| `id` | TEXT PK | `inc` prefix (e.g., `incab3f829d01`) |
| `dedupe_key` | TEXT | Uniqueness key for open incidents. Partial unique index: only one open incident per key |
| `type` | TEXT | Machine-readable type: `replication_broken`, `disk_pressure`, `cephfs_unmounted`, etc. |
| `severity` | TEXT | `critical`, `warning`, `info` |
| `status` | TEXT | `open` -> `investigating` -> `remediating` -> `resolved` or `escalated`. Also `cancelled` |
| `title` | TEXT | Human-readable one-liner |
| `detail` | TEXT | Raw error messages, metric values from the detector |
| `resource_type` | TEXT | `shard`, `node`, `certificate` |
| `resource_id` | TEXT | The specific resource ID |
| `source` | TEXT | What created it: `replication-health-cron`, `disk-pressure-cron`, `manual` |
| `assigned_to` | TEXT | `agent:incident-investigator` for the LLM agent, null = unassigned |
| `resolution` | TEXT | What fixed it (set on resolve) |
| `detected_at` | TIMESTAMPTZ | When the problem was first observed |
| `resolved_at` | TIMESTAMPTZ | When status became `resolved` |
| `escalated_at` | TIMESTAMPTZ | When status became `escalated` |

**Deduplication:** A partial unique index on `dedupe_key WHERE status NOT IN ('resolved', 'cancelled')` ensures only one open incident per key. Creating an incident with a matching key returns the existing one (200) instead of creating a new one (201).

### incident_events

Append-only timeline of everything that happens on an incident.

| Column | Type | Description |
|---|---|---|
| `id` | TEXT PK | UUID |
| `incident_id` | TEXT FK | References incidents(id) ON DELETE CASCADE |
| `actor` | TEXT | Who: `system:replication-health-cron`, `agent:incident-investigator`, `admin` |
| `action` | TEXT | What: `investigated`, `attempted_fix`, `fix_succeeded`, `fix_failed`, `escalated`, `resolved`, `cancelled`, `capability_gap`, `commented`, `admin_message` |
| `detail` | TEXT | Human-readable description |
| `metadata` | JSONB | Structured data (tool name, arguments, result for agent tool calls) |

### capability_gaps

Tracks tools that agents need but don't have. Separate table because gaps outlive individual incidents and aggregate across many.

| Column | Type | Description |
|---|---|---|
| `id` | TEXT PK | UUID |
| `tool_name` | TEXT UNIQUE | The tool needed (e.g., `restart_mysql_node`, `query_loki_logs`) |
| `description` | TEXT | What the agent was trying to do |
| `category` | TEXT | `investigation`, `remediation`, `notification` |
| `occurrences` | INT | How many times agents have hit this gap. Higher = build it first |
| `status` | TEXT | `open`, `implemented`, `wont_fix` |

### incident_capability_gaps

Many-to-many link between incidents and capability gaps.

## API Endpoints

### Incidents

| Method | Path | Description |
|---|---|---|
| GET | `/incidents` | List. Filter by `status`, `severity`, `type`, `resource_type`, `resource_id`, `source` |
| POST | `/incidents` | Create (or return existing if dedupe_key matches). 201 or 200 |
| GET | `/incidents/{id}` | Get incident details |
| PATCH | `/incidents/{id}` | Update status, severity, assigned_to |
| POST | `/incidents/{id}/resolve` | Resolve with a resolution message |
| POST | `/incidents/{id}/escalate` | Escalate with a reason |
| POST | `/incidents/{id}/cancel` | Cancel (false positive) |
| GET | `/incidents/{id}/events` | List timeline events (chronological) |
| POST | `/incidents/{id}/events` | Add an event |
| GET | `/incidents/{id}/gaps` | List linked capability gaps |

### Capability Gaps

| Method | Path | Description |
|---|---|---|
| GET | `/capability-gaps` | List gaps, sorted by occurrences desc. Filter by `status`, `category` |
| POST | `/capability-gaps` | Report gap (or increment occurrences if tool_name exists) |
| PATCH | `/capability-gaps/{id}` | Update status (mark as `implemented` or `wont_fix`) |
| GET | `/capability-gaps/{id}/incidents` | List incidents linked to this gap |

### MCP

The `operations` MCP group at `/mcp/operations` exposes all incident and capability gap endpoints as AI-callable tools, auto-generated from the OpenAPI spec.

## Health Crons

Seven health check workflows run on Temporal cron schedules. Each detects problems by querying infrastructure state, creates incidents via the deduplicated create API, and auto-resolves incidents when the underlying problem is fixed.

### Replication Health (`replication-health-cron`, every 1 min)

Checks MySQL replication status on all database shard replicas.

| Incident Type | Severity | Condition |
|---|---|---|
| `replication_check_failed` | critical | Cannot reach replica to check status |
| `replication_broken` | critical | IO_Running=false or SQL_Running=false |
| `replication_lag` | warning | Seconds_Behind_Master > 300 (5 min) |

Dedupe key: `{type}:{shard_id}:{node_id}`. Auto-resolves all `replication_*` incidents for a shard when all replicas are healthy. Sets shard status to `degraded` on failure, `active` on recovery.

### Node Health (`node-health-cron`, every 2 min)

Detects nodes not reporting health within 5 minutes.

| Incident Type | Severity | Condition |
|---|---|---|
| `node_health_missing` | critical | `last_health_at` is null or > 5 min old |

Dedupe key: `node_health_missing:{node_id}`. Auto-resolves when node resumes reporting.

### Convergence Health (`convergence-health-cron`, every 5 min)

Detects shards stuck in `converging` status.

| Incident Type | Severity | Condition |
|---|---|---|
| `convergence_stuck` | warning | Shard in `converging` status for > 15 min |

Dedupe key: `convergence_stuck:{shard_id}`. Auto-resolves when shard exits `converging`.

### Disk Pressure (`disk-pressure-cron`, every 5 min)

Monitors disk usage on all nodes.

| Incident Type | Severity | Condition |
|---|---|---|
| `disk_pressure` | warning | Disk usage >= 90% |
| `disk_pressure` | critical | Disk usage >= 95% |

Dedupe key: `disk_pressure:{node_id}`. Auto-resolves when usage drops below 90%.

### Cert Expiry (`cert-expiry-health-cron`, daily)

Checks for certificates expiring within 14 days.

| Incident Type | Severity | Condition |
|---|---|---|
| `cert_expiring` | warning | Certificate expires within 14 days |

### CephFS Health (`cephfs-health-cron`, every 10 min)

Verifies CephFS mounts on all web shard nodes.

| Incident Type | Severity | Condition |
|---|---|---|
| `cephfs_unmounted` | critical | CephFS not mounted on web node |

Dedupe key: `cephfs_unmounted:{node_id}`. Auto-resolves when mount check passes.

### Incident Escalation (`incident-escalation-cron`, every 5 min)

Auto-escalates stale incidents based on severity and age.

| Condition | Action |
|---|---|
| Critical, open, unassigned > 15 min | Escalate |
| Warning, open, unassigned > 1 hour | Escalate |
| Investigating or remediating > 30 min | Escalate |

Sets `status=escalated`, `escalated_at=now()`, adds event with actor `system:escalation-cron`, and fires the `escalated` webhook.

## LLM Investigation Agent

Autonomous incident responder powered by a self-hosted LLM (vLLM + Qwen 2.5 72B) running on an NVIDIA DGX Spark. The full lifecycle is Temporal-orchestrated.

### Architecture

```
ProcessIncidentQueueWorkflow (cron, every 1 min)
  |
  +-- GetAgentConfig — reads system prompt + per-type concurrency from platform_config
  +-- ListUnassignedOpenIncidents — up to 20, ordered: critical > warning > info, then oldest
  +-- groupByType — group incidents by type
  |
  +-- Per group (up to AGENT_MAX_CONCURRENT in parallel):
        |
        +-- ClaimIncidentForAgent — atomic UPDATE ... WHERE assigned_to IS NULL
        +-- Leader: InvestigateIncidentWorkflow (no hints)
        |     +-- AssembleIncidentContext (retryable, 30s, 3 attempts)
        |     +-- InvestigateIncident (no retry, 30min timeout, 2min heartbeat)
        |
        +-- If leader resolves → extract resolution hint from tool call history
        |
        +-- Followers (up to follower_concurrent in parallel):
              +-- InvestigateIncidentWorkflow (with resolution hint)
              +-- On failure → EscalateIncident
```

### Investigation Loop

The `InvestigateIncident` activity runs the full multi-turn LLM conversation inside a single Temporal activity (non-deterministic LLM responses prevent per-turn activities due to replay issues).

```
messages = [system_prompt, incident_context_json, optional_hints]

for turn 1..max_turns:
    heartbeat("turn N/M")
    check for admin_message events → inject as user messages
    response = LLM.Chat(messages, tools)
    for each tool_call in response:
        result = execute via HTTP to core API
        record as incident_event with metadata
        if resolve_incident → return resolved + resolution_hint
        if escalate_incident → return escalated
    if no tool_calls → return max_turns

max turns reached → escalate
```

### Live Admin Chat

Admins can send messages to the agent while it's investigating. Between each LLM turn, the activity checks for `admin_message` events in the incident timeline. New messages are injected into the conversation as `"Message from admin operator: ..."`.

The admin UI auto-polls every 3 seconds when the agent is active, showing a chat bar at the top of the incident detail page. Timeline events are visually distinguished — admin messages in blue, agent events in orange.

### Leader-Follower Pattern

Prevents redundant investigation when many incidents share the same type (e.g., disk pressure across a cluster).

1. **Grouping** — incidents grouped by `type`, ordered by severity then age
2. **Leader** — first incident investigated without hints
3. **Hint extraction** — on resolution, a structured hint is built from the leader's tool call history (up to 10 steps, truncated args)
4. **Followers** — remaining incidents receive the hint as context, enabling faster resolution

If the leader fails to resolve, followers proceed without hints.

### Per-Type Concurrency

Global follower concurrency (`AGENT_FOLLOWER_CONCURRENT`, default 5) can be overridden per incident type via `platform_config`:

```sql
INSERT INTO platform_config (key, value) VALUES ('agent.concurrency.disk_pressure', '10');
INSERT INTO platform_config (key, value) VALUES ('agent.concurrency.replication_lag', '2');
```

Disk pressure can fan out widely (fix is usually the same). Replication lag should be conservative.

### Agent Tools

11 tools, all executed as HTTP calls to the core API using `AGENT_API_KEY`:

| Tool | Type | Description |
|---|---|---|
| `get_incident` | read | Get incident details by ID |
| `list_incident_events` | read | Get timeline events |
| `add_incident_event` | write | Record investigation step (actor: `agent:incident-investigator`) |
| `resolve_incident` | terminal | Mark incident resolved — ends investigation |
| `escalate_incident` | terminal | Escalate to human — ends investigation |
| `get_shard` | read | Shard status, role, config |
| `list_nodes_by_shard` | read | Nodes in a shard |
| `get_node` | read | Node hostname, IP, status |
| `get_tenant` | read | Tenant details |
| `converge_shard` | write | Trigger shard convergence (safe, reversible) |
| `report_capability_gap` | write | Report missing tool |

### System Prompt

The default system prompt (in `internal/activity/agent.go`) covers platform architecture, responsibilities, decision framework, admin message handling, and constraints. Override at runtime via `platform_config` key `agent.system_prompt`.

Key constraints in the default prompt:
- No destructive actions (delete resources, revoke keys)
- Do not resolve unless the underlying issue is confirmed fixed
- Do not escalate without at least one diagnostic action
- Do not call the same tool twice with identical arguments

### Configuration

#### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `AGENT_ENABLED` | `false` | Enable the LLM incident agent |
| `AGENT_MAX_CONCURRENT` | `3` | Max parallel incident group leaders |
| `AGENT_FOLLOWER_CONCURRENT` | `5` | Max parallel followers per group |
| `AGENT_API_KEY` | (required when enabled) | API key for the agent to call the core API |
| `LLM_BASE_URL` | (required when enabled) | vLLM endpoint URL (e.g., `http://dgx:8000`) |
| `LLM_API_KEY` | `""` | API key for LLM endpoint (depends on vLLM config) |
| `LLM_MODEL` | `Qwen/Qwen2.5-72B-Instruct` | Model name |
| `LLM_MAX_TURNS` | `10` | Max conversation turns per investigation |

When `AGENT_ENABLED=true`, `LLM_BASE_URL` and `AGENT_API_KEY` are required (validated at startup).

#### Platform Config (Runtime)

| Key | Description |
|---|---|
| `agent.system_prompt` | Override the default system prompt |
| `agent.concurrency.<type>` | Per-type follower concurrency |
| `webhook.critical.url` | Webhook URL for critical incidents |
| `webhook.critical.template` | `generic` (default) or `slack` |
| `webhook.escalated.url` | Webhook URL for escalated incidents |
| `webhook.escalated.template` | `generic` (default) or `slack` |

## Webhook Notifications

Fire on two triggers:

| Trigger | When |
|---|---|
| `critical` | New critical-severity incident created |
| `escalated` | Incident escalated (by agent, escalation cron, or manually) |

Templates:
- **`generic`** — JSON payload with `event` type and full incident object
- **`slack`** — Slack Block Kit with severity emoji, status fields, code-blocked detail

Webhook delivery: 30-second timeout per request, retries up to 3 times for 5xx errors. 4xx errors are non-retryable. Failures are logged but don't affect incident processing.

## Capability Gap Flywheel

The capability gap table is the feedback loop that drives platform development:

1. Agent encounters a situation it can't handle (e.g., no tool to restart MySQL)
2. Agent calls `report_capability_gap` with what it needed
3. Gap is created or `occurrences` incremented if tool_name already exists
4. Gap is linked to the triggering incident
5. Admin views gaps sorted by occurrences — most-requested gaps = what to build next
6. Tool gets built, gap marked `implemented`
7. Future similar incidents get auto-fixed

## Dashboard

The admin dashboard shows incident health at a glance:

- **Open Incidents** — count of open, investigating, remediating, escalated
- **Critical** — critical severity, not resolved/cancelled
- **Escalated** — status = escalated
- **MTTR (30d)** — mean time to resolve (avg of resolved_at - detected_at for last 30 days)
- **Open Gaps** — capability gaps with status = open
- **Incidents by Status** — breakdown card
- **Agent Activity** — investigating, resolved, escalated counts with resolution rate

## Admin UI

### Incidents Page (`/incidents`)

Table with severity, title, status, source, assigned_to, detected_at. Filters: status dropdown, severity dropdown, type text, source text. Click navigates to detail page.

### Incident Detail Page (`/incidents/{id}`)

- **Header:** title, type, source, status badge, action buttons (Resolve, Escalate, Cancel)
- **Agent chat bar:** appears when agent is investigating — input field to send messages to the agent, pulsing Bot icon, auto-polling timeline
- **Timeline tab:** chronological events with visual distinction (blue = admin messages, orange = agent events), collapsible metadata JSON, auto-scroll on new events
- **Capability Gaps tab:** linked gaps with tool_name, status, category, occurrences
- **Details tab:** full incident fields (ID, dedupe_key, severity, type, resource, timestamps, resolution)
- **Add Note:** available when not being investigated by agent — select action type + detail text
- **Dialogs:** Resolve (requires resolution text), Escalate (requires reason), Cancel (optional reason)

### Capability Gaps Page (`/capability-gaps`)

Table: tool_name, description, category, occurrences (sorted desc), status. Actions: Mark as Implemented, Won't Fix.

### Platform Config Page (`/platform/config`)

Three tabs:
- **Webhooks:** structured form for critical and escalated webhook URLs + template selectors
- **Agent:** system prompt textarea, per-type concurrency table with add/remove entries
- **Raw JSON:** full config editor

## Observability

Every agent action is recorded as an `incident_event`:

| Actor | Action | Detail | Metadata |
|---|---|---|---|
| `agent:incident-investigator` | `investigated` | "Agent claimed incident for investigation" | — |
| `agent:incident-investigator` | `investigated` | "Called tool: get_shard" | `{"tool": "get_shard", "arguments": {...}, "result": {...}}` |
| `agent:incident-investigator` | `commented` | LLM text response | — |
| `agent:incident-investigator` | `escalated` | "Agent reached maximum investigation turns" | — |
| `admin` | `admin_message` | "Try converging the shard first" | — |
| `system:escalation-cron` | `escalated` | "Critical incident unassigned for >15 min" | — |
| `system:replication-health-cron` | `resolved` | "Health check confirmed all replicas healthy" | — |

Full timeline: `GET /api/v1/incidents/{id}/events`. Temporal UI shows every workflow run with history.

## Files

| File | Purpose |
|---|---|
| `migrations/core/00041_incidents.sql` | Schema: incidents, incident_events, capability_gaps, incident_capability_gaps |
| `internal/model/incident.go` | Model structs and status/severity constants |
| `internal/core/incident.go` | Incident service (CRUD, dedup, resolve, auto-resolve, escalate) |
| `internal/core/capability_gap.go` | Capability gap service (report/upsert, list, update) |
| `internal/core/dashboard.go` | Dashboard stats (incident counts, MTTR) |
| `internal/api/handler/incident.go` | REST handlers for incidents and events |
| `internal/api/handler/capability_gap.go` | REST handlers for capability gaps |
| `internal/api/request/incident.go` | Request validation structs |
| `internal/activity/incident.go` | Activities: CreateIncident, AutoResolve, Escalate, FindStale, webhook |
| `internal/activity/webhook.go` | Webhook HTTP delivery (generic + Slack templates) |
| `internal/activity/agent.go` | Agent activities: claim, assemble, investigate (LLM loop), admin message fetch |
| `internal/llm/client.go` | OpenAI-compatible HTTP client |
| `internal/llm/tools.go` | Tool registry with 11 tools, HTTP-based execution |
| `internal/workflow/incident_agent.go` | ProcessIncidentQueueWorkflow, InvestigateIncidentWorkflow |
| `internal/workflow/replication_health.go` | Replication health cron |
| `internal/workflow/node_health_monitor.go` | Node health cron |
| `internal/workflow/convergence_monitor.go` | Convergence health cron |
| `internal/workflow/incident_escalation.go` | Escalation cron |
| `internal/workflow/cephfs_health_monitor.go` | CephFS health cron |
| `internal/config/config.go` | Agent/LLM environment variable config |
| `mcp.yaml` | MCP `operations` group (Incidents, Incident Events, Capability Gaps tags) |
