package model

import (
	"encoding/json"
	"time"
)

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

// Capability gap statuses.
const (
	GapOpen        = "open"
	GapImplemented = "implemented"
	GapWontFix     = "wont_fix"
)

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
