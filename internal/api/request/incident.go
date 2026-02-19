package request

import "encoding/json"

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

type UpdateIncident struct {
	Status     *string `json:"status" validate:"omitempty,oneof=open investigating remediating"`
	Severity   *string `json:"severity" validate:"omitempty,oneof=critical warning info"`
	AssignedTo *string `json:"assigned_to"`
}

type ResolveIncident struct {
	Resolution string `json:"resolution" validate:"required"`
}

type EscalateIncident struct {
	Reason string `json:"reason" validate:"required"`
}

type CancelIncident struct {
	Reason string `json:"reason"`
}

type AddIncidentEvent struct {
	Actor    string          `json:"actor" validate:"required"`
	Action   string          `json:"action" validate:"required,oneof=investigated attempted_fix fix_succeeded fix_failed escalated resolved cancelled capability_gap commented"`
	Detail   string          `json:"detail" validate:"required"`
	Metadata json.RawMessage `json:"metadata"`
}

type ReportCapabilityGap struct {
	ToolName    string  `json:"tool_name" validate:"required"`
	Description string  `json:"description" validate:"required"`
	Category    string  `json:"category" validate:"required,oneof=investigation remediation notification"`
	IncidentID  *string `json:"incident_id"`
}

type UpdateCapabilityGap struct {
	Status *string `json:"status" validate:"omitempty,oneof=open implemented wont_fix"`
}
