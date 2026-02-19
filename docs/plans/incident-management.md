# AI-Driven Incident Management — Implementation Plan

## Overview

An internal incident management system where health checks detect problems, AI agents investigate and remediate them, and capability gaps are tracked as a feedback loop for platform development. No external dependencies (Jira, PagerDuty) — the platform is the source of truth, exposed entirely via MCP.

### Design Goals

1. **Machine-first** — structured data, not ticket descriptions. Every field queryable, every action an API call.
2. **Self-healing** — health crons create incidents, AI agents fix them, health crons auto-resolve them.
3. **Capability gap tracking** — when an agent can't fix something because the right MCP tool doesn't exist, that gap is recorded. The most-requested gaps tell you what to build next.
4. **Human escape hatch** — escalation to admin UI / webhook notifications when agents can't handle it.
5. **Deduplication** — the same problem doesn't create 100 incidents.

---

## 1. Data Model

### 1.1 Core DB Table: `incidents`

Migration file: `migrations/core/00041_incidents.sql`

```sql
-- +goose Up
CREATE TABLE incidents (
    id              TEXT PRIMARY KEY,
    dedupe_key      TEXT NOT NULL,
    type            TEXT NOT NULL,
    severity        TEXT NOT NULL DEFAULT 'warning',
    status          TEXT NOT NULL DEFAULT 'open',
    title           TEXT NOT NULL,
    detail          TEXT NOT NULL DEFAULT '',
    resource_type   TEXT,
    resource_id     TEXT,
    source          TEXT NOT NULL,
    assigned_to     TEXT,
    resolution      TEXT,
    detected_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at     TIMESTAMPTZ,
    escalated_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_incidents_dedupe_open
    ON incidents (dedupe_key) WHERE status NOT IN ('resolved', 'cancelled');
CREATE INDEX idx_incidents_status ON incidents(status);
CREATE INDEX idx_incidents_severity ON incidents(severity);
CREATE INDEX idx_incidents_resource ON incidents(resource_type, resource_id);
CREATE INDEX idx_incidents_created_at ON incidents(created_at);
```

**Field notes:**

| Field | Description |
|---|---|
| `id` | Generated via `platform.NewName("inc")` — e.g., `incab3f829d01` |
| `dedupe_key` | Uniqueness key for open incidents. E.g., `replication_broken:shard-db-1:node-replica-1`. The partial unique index ensures only one open incident per dedupe_key, but allows resolved duplicates. |
| `type` | Machine-readable incident type. E.g., `replication_broken`, `replication_lag`, `disk_pressure`, `cert_expiring`, `node_unreachable`, `cephfs_mount_failed`, `convergence_failed`. |
| `severity` | `critical`, `warning`, or `info`. Critical = immediate attention, warning = degraded but functional, info = advisory. |
| `status` | `open` → `investigating` → `remediating` → `resolved` or `escalated`. Also `cancelled` for false positives. |
| `title` | Human-readable one-liner. E.g., "Replication broken on shard-db-1 node replica-1". |
| `detail` | Structured description from the detector. Raw error messages, metric values, etc. |
| `resource_type` | The resource kind: `shard`, `node`, `tenant`, `database`, `certificate`, etc. |
| `resource_id` | The specific resource ID. |
| `source` | What created this incident: `replication-health-cron`, `cert-renewal-cron`, `convergence-workflow`, `manual`. |
| `assigned_to` | Agent or user handling this. `agent:maintenance` for AI agents, `user:admin` for humans. Null = unassigned. |
| `resolution` | What fixed it. Null until resolved. E.g., "Replication reconfigured via converge-shard workflow". |
| `detected_at` | When the problem was first observed (may predate incident creation if batched). |
| `resolved_at` | Timestamp when status became `resolved`. |
| `escalated_at` | Timestamp when status became `escalated`. |

### 1.2 Core DB Table: `incident_events`

Append-only timeline of everything that happens on an incident.

```sql
CREATE TABLE incident_events (
    id          TEXT PRIMARY KEY,
    incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    actor       TEXT NOT NULL,
    action      TEXT NOT NULL,
    detail      TEXT NOT NULL DEFAULT '',
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_incident_events_incident_id ON incident_events(incident_id);
```

| Field | Description |
|---|---|
| `id` | Generated via `platform.NewID()`. |
| `actor` | Who did this. `system:replication-health-cron`, `agent:maintenance`, `user:admin@example.com`. |
| `action` | What happened: `created`, `investigated`, `attempted_fix`, `fix_succeeded`, `fix_failed`, `escalated`, `resolved`, `cancelled`, `capability_gap`, `commented`. |
| `detail` | Human-readable description of the event. |
| `metadata` | Structured data. For `investigated`: `{"checked": ["replica_status", "node_health"], "findings": "IO thread stopped, last error: connection refused"}`. For `attempted_fix`: `{"tool": "converge_shard", "params": {"shard_id": "..."}, "result": "success"}`. For `capability_gap`: `{"needed_tool": "restart_mysql", "reason": "no MCP tool to restart MySQL on a specific node"}`. |

### 1.3 Core DB Table: `capability_gaps`

Tracks MCP tools that agents need but don't have. Separate table because gaps outlive individual incidents and aggregate across many.

```sql
CREATE TABLE capability_gaps (
    id           TEXT PRIMARY KEY,
    tool_name    TEXT NOT NULL,
    description  TEXT NOT NULL,
    category     TEXT NOT NULL DEFAULT 'remediation',
    occurrences  INT NOT NULL DEFAULT 1,
    status       TEXT NOT NULL DEFAULT 'open',
    implemented_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tool_name)
);

CREATE TABLE incident_capability_gaps (
    incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    gap_id      TEXT NOT NULL REFERENCES capability_gaps(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (incident_id, gap_id)
);

-- +goose Down
DROP TABLE incident_capability_gaps;
DROP TABLE capability_gaps;
DROP TABLE incident_events;
DROP TABLE incidents;
```

| Field | Description |
|---|---|
| `tool_name` | The MCP tool that's needed. E.g., `restart_mysql_node`, `query_loki_logs`, `run_mysqlcheck`. Unique — repeated requests increment `occurrences`. |
| `description` | What the agent was trying to do and why it couldn't. |
| `category` | `investigation` (need to read/query something), `remediation` (need to fix something), `notification` (need to alert someone). |
| `occurrences` | How many times agents have hit this gap. Higher = build it first. |
| `status` | `open`, `implemented`, `wont_fix`. |

---

## 2. Model

```go
// internal/model/incident.go

package model

import "time"

// Incident severities.
const (
    SeverityCritical = "critical"
    SeverityWarning  = "warning"
    SeverityInfo     = "info"
)

// Incident statuses.
const (
    IncidentOpen          = "open"
    IncidentInvestigating = "investigating"
    IncidentRemediating   = "remediating"
    IncidentResolved      = "resolved"
    IncidentEscalated     = "escalated"
    IncidentCancelled     = "cancelled"
)

type Incident struct {
    ID           string     `json:"id" db:"id"`
    DedupeKey    string     `json:"dedupe_key" db:"dedupe_key"`
    Type         string     `json:"type" db:"type"`
    Severity     string     `json:"severity" db:"severity"`
    Status       string     `json:"status" db:"status"`
    Title        string     `json:"title" db:"title"`
    Detail       string     `json:"detail" db:"detail"`
    ResourceType *string    `json:"resource_type,omitempty" db:"resource_type"`
    ResourceID   *string    `json:"resource_id,omitempty" db:"resource_id"`
    Source       string     `json:"source" db:"source"`
    AssignedTo   *string    `json:"assigned_to,omitempty" db:"assigned_to"`
    Resolution   *string    `json:"resolution,omitempty" db:"resolution"`
    DetectedAt   time.Time  `json:"detected_at" db:"detected_at"`
    ResolvedAt   *time.Time `json:"resolved_at,omitempty" db:"resolved_at"`
    EscalatedAt  *time.Time `json:"escalated_at,omitempty" db:"escalated_at"`
    CreatedAt    time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
}

type IncidentEvent struct {
    ID         string          `json:"id" db:"id"`
    IncidentID string          `json:"incident_id" db:"incident_id"`
    Actor      string          `json:"actor" db:"actor"`
    Action     string          `json:"action" db:"action"`
    Detail     string          `json:"detail" db:"detail"`
    Metadata   json.RawMessage `json:"metadata" db:"metadata"`
    CreatedAt  time.Time       `json:"created_at" db:"created_at"`
}

type CapabilityGap struct {
    ID            string     `json:"id" db:"id"`
    ToolName      string     `json:"tool_name" db:"tool_name"`
    Description   string     `json:"description" db:"description"`
    Category      string     `json:"category" db:"category"`
    Occurrences   int        `json:"occurrences" db:"occurrences"`
    Status        string     `json:"status" db:"status"`
    ImplementedAt *time.Time `json:"implemented_at,omitempty" db:"implemented_at"`
    CreatedAt     time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}
```

---

## 3. API Endpoints

All endpoints under `/api/v1/incidents`. OpenAPI tag: `Incidents`.

### 3.1 Incidents

| Method | Path | Description |
|---|---|---|
| `GET` | `/incidents` | List incidents. Filter by `status`, `severity`, `type`, `resource_type`, `resource_id`, `source`. Paginated. |
| `POST` | `/incidents` | Create an incident (or return existing if dedupe_key matches an open one). |
| `GET` | `/incidents/{id}` | Get incident with full details. |
| `PATCH` | `/incidents/{id}` | Update status, assigned_to, severity. |
| `POST` | `/incidents/{id}/resolve` | Resolve with a resolution message. |
| `POST` | `/incidents/{id}/escalate` | Escalate with a reason. |
| `POST` | `/incidents/{id}/cancel` | Cancel (false positive). |

### 3.2 Incident Events

| Method | Path | Description |
|---|---|---|
| `GET` | `/incidents/{id}/events` | List timeline events for an incident. Paginated. |
| `POST` | `/incidents/{id}/events` | Add an event (investigation finding, fix attempt, comment). |

### 3.3 Capability Gaps

| Method | Path | Description |
|---|---|---|
| `GET` | `/capability-gaps` | List gaps, sorted by `occurrences` desc. Filter by `status`, `category`. |
| `POST` | `/capability-gaps` | Report a new gap (or increment occurrences if tool_name exists). |
| `PATCH` | `/capability-gaps/{id}` | Update status (e.g., mark as `implemented`). |

### 3.4 Create Incident Request

```go
type CreateIncident struct {
    DedupeKey    string  `json:"dedupe_key" validate:"required"`
    Type         string  `json:"type" validate:"required"`
    Severity     string  `json:"severity" validate:"required,oneof=critical warning info"`
    Title        string  `json:"title" validate:"required,max=256"`
    Detail       string  `json:"detail"`
    ResourceType *string `json:"resource_type"`
    ResourceID   *string `json:"resource_id"`
    Source       string  `json:"source" validate:"required"`
}
```

**Deduplication behavior:** If an open incident exists with the same `dedupe_key`, return the existing incident (200) instead of creating a new one (201). This lets health crons call "create incident" every minute without worrying about duplicates.

### 3.5 Add Event Request

```go
type AddIncidentEvent struct {
    Actor    string          `json:"actor" validate:"required"`
    Action   string          `json:"action" validate:"required,oneof=investigated attempted_fix fix_succeeded fix_failed escalated resolved cancelled capability_gap commented"`
    Detail   string          `json:"detail" validate:"required"`
    Metadata json.RawMessage `json:"metadata"`
}
```

### 3.6 Report Capability Gap Request

```go
type ReportCapabilityGap struct {
    ToolName    string  `json:"tool_name" validate:"required"`
    Description string  `json:"description" validate:"required"`
    Category    string  `json:"category" validate:"required,oneof=investigation remediation notification"`
    IncidentID  *string `json:"incident_id"` // Links gap to the incident that triggered it.
}
```

---

## 4. MCP Configuration

New MCP group in `mcp.yaml`:

```yaml
  operations:
    description: "Incident management, capability gaps, and operational tools for AI maintenance agents"
    tags:
      - "Incidents"
      - "Incident Events"
      - "Capability Gaps"
```

This gives AI agents a dedicated MCP endpoint at `/mcp/operations` with all incident tools auto-generated from the OpenAPI spec.

---

## 5. Health Check Integration

Existing health crons evolve from "set shard status" to "create structured incidents". The replication health cron becomes the reference implementation.

### 5.1 Updated Replication Health Cron

Current behavior: sets shard status to `degraded` with a message.

New behavior: creates an incident **and** sets shard status to `degraded`.

```go
func CheckReplicationHealthWorkflow(ctx workflow.Context) error {
    // ... existing shard/node iteration ...

    for _, node := range nodes {
        if node.ID == primaryID { continue }

        var status agent.ReplicationStatus
        err = workflow.ExecuteActivity(nodeCtx, "GetReplicationStatus").Get(ctx, &status)
        if err != nil {
            // Create incident for unreachable node.
            createIncident(ctx, activity.CreateIncidentParams{
                DedupeKey:    fmt.Sprintf("replication_check_failed:%s:%s", shard.ID, node.ID),
                Type:         "replication_check_failed",
                Severity:     "critical",
                Title:        fmt.Sprintf("Cannot check replication on %s node %s", shard.ID, node.ID),
                Detail:       err.Error(),
                ResourceType: strPtr("node"),
                ResourceID:   &node.ID,
                Source:       "replication-health-cron",
            })
            setShardStatus(ctx, shard.ID, "degraded", ...)
            continue
        }

        if !status.IORunning || !status.SQLRunning {
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
            setShardStatus(ctx, shard.ID, "degraded", ...)
        } else if status.SecondsBehind != nil && *status.SecondsBehind > 300 {
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
            setShardStatus(ctx, shard.ID, "degraded", ...)
        }
    }

    // Auto-resolve: if all replicas healthy, resolve open replication incidents for this shard.
    if allHealthy {
        autoResolveIncidents(ctx, shard.ID, "replication_")
    }
}
```

### 5.2 Auto-Resolution

New activity: `AutoResolveIncidents` — queries for open incidents matching a resource and type prefix, resolves them with a system message.

```go
type AutoResolveParams struct {
    ResourceType string
    ResourceID   string
    TypePrefix   string // e.g., "replication_" matches replication_broken, replication_lag
    Resolution   string // e.g., "Health check confirmed all replicas healthy"
}
```

This prevents stale incidents — when the health check passes, the incident auto-resolves without waiting for an agent.

### 5.3 Future Health Checks

The same pattern applies to new health crons:

| Cron | Type | Detects |
|---|---|---|
| `replication-health-cron` | `replication_broken`, `replication_lag` | MySQL replication issues |
| `cert-expiry-cron` | `cert_expiring` | Certificates expiring within 14 days |
| `disk-pressure-cron` | `disk_pressure` | Node disk usage > 85% |
| `node-health-cron` | `node_unreachable` | Nodes not reporting health |
| `cephfs-health-cron` | `cephfs_degraded` | CephFS mount or cluster issues |
| `convergence-monitor` | `convergence_failed` | Shards stuck in converging state |

---

## 6. AI Agent Workflow

### 6.1 The Loop

An AI agent connected via MCP follows this loop:

```
1. list_incidents(status=open, status=escalated)     → find work
2. update_incident(status=investigating, assigned_to=agent:maintenance)  → claim it
3. add_incident_event(action=investigated, detail=...)  → record findings
4. Attempt remediation using available MCP tools
   a. If tool exists → call it → add_incident_event(action=attempted_fix)
   b. If tool missing → report_capability_gap() → add_incident_event(action=capability_gap)
5. If fix succeeded → resolve_incident(resolution=...)
6. If fix failed or no tool → escalate_incident(reason=...)
```

### 6.2 Handling Capability Gaps

This is the key innovation. When an agent identifies a fix but can't execute it:

**Scenario:** Replication broken. Agent knows it needs to reconfigure replication on the replica node. But there's no `reconfigure_replication` MCP tool.

```
Agent calls: POST /capability-gaps
{
    "tool_name": "reconfigure_replication",
    "description": "Need to run STOP REPLICA / CHANGE REPLICATION SOURCE / START REPLICA on a specific database node to restore replication. Currently only available as part of the full converge-shard workflow which is too heavy for targeted fixes.",
    "category": "remediation",
    "incident_id": "inc..."
}
```

The agent then adds a timeline event recording the gap and escalates:

```
Agent calls: POST /incidents/{id}/events
{
    "actor": "agent:maintenance",
    "action": "capability_gap",
    "detail": "Identified fix (reconfigure replication) but no MCP tool available. Reported as capability gap. Escalating.",
    "metadata": {"gap_tool": "reconfigure_replication"}
}

Agent calls: POST /incidents/{id}/escalate
```

**What this achieves:**

1. The incident gets human attention with full context (what's wrong, what the fix is, why the agent couldn't do it).
2. The capability gap is tracked. If 50 incidents hit the same gap, `occurrences=50` makes it obvious what to build next.
3. Once the tool is built and marked `implemented`, similar future incidents get auto-fixed.

### 6.3 Handling Faulty MCP Endpoints

When an MCP tool exists but returns errors or unexpected results:

```
Agent calls: POST /incidents/{id}/events
{
    "actor": "agent:maintenance",
    "action": "attempted_fix",
    "detail": "Called converge_shard for shard-db-1 but got HTTP 500: internal server error",
    "metadata": {
        "tool": "converge_shard",
        "params": {"shard_id": "shard-db-1"},
        "error": "HTTP 500: internal server error",
        "suggestion": "The converge_shard endpoint may have a bug when called on database shards"
    }
}
```

The agent should:
1. Log the failure as a timeline event with full request/response details.
2. Try alternative approaches if available (e.g., a more targeted tool).
3. If no alternatives, report a capability gap with category `remediation` and a description noting the existing tool is broken.
4. Escalate.

This creates a feedback signal for broken tools too — not just missing ones.

### 6.4 Escalation Policy

Escalation triggers (implemented by an `incident-escalation-cron` running every 5 minutes):

| Condition | Action |
|---|---|
| Critical incident open > 15 min with no `assigned_to` | Auto-escalate |
| Warning incident open > 1 hour with no `assigned_to` | Auto-escalate |
| Incident in `investigating` or `remediating` > 30 min | Auto-escalate |
| Agent explicitly escalates | Immediate |

Escalation sets `status=escalated`, `escalated_at=now()`, and fires a webhook (Slack, email, PagerDuty — configurable per severity).

---

## 7. Admin UI

### 7.1 Incidents Page (`/incidents`)

- Table: title, type, severity (color badge), status, resource, source, age, assigned_to
- Filters: status (tabs: Open / Investigating / Escalated / Resolved), severity, type
- Click → incident detail page

### 7.2 Incident Detail Page (`/incidents/:id`)

- Header: title, severity badge, status badge, resource link
- Actions: Assign, Escalate, Resolve, Cancel
- Timeline: chronological list of events with actor, action, detail, metadata (collapsible JSON)
- Linked capability gaps (if any)

### 7.3 Capability Gaps Page (`/capability-gaps`)

- Table: tool_name, description, category, occurrences (sorted desc), status
- Occurrences as a bar or number — instant visual for "what to build next"
- Actions: Mark as Implemented, Won't Fix

---

## 8. Webhook Notifications

Configurable in platform config (existing `platform_config` table):

```json
{
    "incident_webhooks": {
        "critical": {
            "url": "https://hooks.slack.com/...",
            "template": "slack"
        },
        "escalated": {
            "url": "https://hooks.slack.com/...",
            "template": "slack"
        }
    }
}
```

Webhook fires on:
- New critical incident created
- Any incident escalated
- Configurable per severity

Templates: `slack` (Slack Block Kit), `generic` (simple JSON POST), `pagerduty`. Keep it simple — start with `generic` and `slack`.

---

## 9. Implementation Phases

### Phase 1 — Core (build first)

- [ ] Migration: `incidents`, `incident_events`, `capability_gaps`, `incident_capability_gaps`
- [ ] Models: `Incident`, `IncidentEvent`, `CapabilityGap`
- [ ] Core service: `IncidentService` with Create (dedup), GetByID, List, Update, Resolve, Escalate, Cancel
- [ ] Core service: `IncidentEventService` with Create, ListByIncident
- [ ] Core service: `CapabilityGapService` with Report (upsert), List, Update
- [ ] Activity: `CreateIncident`, `AutoResolveIncidents`, `CreateIncidentEvent`
- [ ] API handlers + routes for all endpoints above
- [ ] Update replication health cron to create incidents
- [ ] MCP group: `operations` with Incidents + Capability Gaps tags

**12 tasks. After this, agents can create/read/update incidents via MCP.**

### Phase 2 — Agent Intelligence

- [ ] Escalation cron workflow (5-min schedule, auto-escalates by policy)
- [ ] Admin UI: incidents list page with filters
- [ ] Admin UI: incident detail page with timeline
- [ ] Admin UI: capability gaps page
- [ ] Add disk-pressure and node-health crons that create incidents
- [ ] Add convergence-monitor cron (detect shards stuck in converging)

**6 tasks. After this, the full loop works: detect → incident → agent → resolve/escalate → human sees it.**

### Phase 3 — Notifications and Polish

- [ ] Webhook notification system (generic + Slack)
- [ ] Platform config UI for webhook URLs
- [ ] Incident statistics on dashboard (open/escalated counts, MTTR)
- [ ] Cert-expiry and CephFS health crons
- [ ] Capability gap → incident linking in UI (click gap → see all related incidents)

**5 tasks. Full production readiness.**

---

## 10. Summary

| Component | Purpose |
|---|---|
| `incidents` table | Structured, deduplicated issue tracking |
| `incident_events` table | Append-only audit trail of investigation + remediation |
| `capability_gaps` table | Tracks what MCP tools agents need but don't have |
| Health crons | Detect problems, create incidents, auto-resolve when fixed |
| Escalation cron | Auto-escalates unhandled incidents by time + severity |
| MCP `operations` group | AI agent interface for the entire incident lifecycle |
| Admin UI pages | Human visibility into incidents, timeline, and capability gaps |
| Webhooks | Push notifications for critical/escalated incidents |

The capability gap table is the flywheel: agents hit gaps → gaps get tracked → most-requested gaps get built → agents become more capable → fewer escalations. Over time the platform becomes increasingly autonomous.
