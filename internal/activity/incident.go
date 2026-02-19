package activity

import (
	"context"
	"fmt"
	"time"

	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
)

// CreateIncidentParams holds parameters for creating an incident via Temporal activity.
type CreateIncidentParams struct {
	DedupeKey    string  `json:"dedupe_key"`
	Type         string  `json:"type"`
	Severity     string  `json:"severity"`
	Title        string  `json:"title"`
	Detail       string  `json:"detail"`
	ResourceType *string `json:"resource_type"`
	ResourceID   *string `json:"resource_id"`
	Source       string  `json:"source"`
}

// CreateIncidentResult holds the result of creating an incident.
type CreateIncidentResult struct {
	ID      string `json:"id"`
	Created bool   `json:"created"` // true if new, false if deduplicated
}

// CreateIncident creates an incident or returns an existing open one matching the dedupe_key.
func (a *CoreDB) CreateIncident(ctx context.Context, params CreateIncidentParams) (*CreateIncidentResult, error) {
	// Try to find existing open incident with same dedupe_key.
	var existingID string
	err := a.db.QueryRow(ctx,
		`SELECT id FROM incidents WHERE dedupe_key = $1 AND status NOT IN ('resolved', 'cancelled')`,
		params.DedupeKey,
	).Scan(&existingID)
	if err == nil {
		return &CreateIncidentResult{ID: existingID, Created: false}, nil
	}

	id := platform.NewName("inc")
	now := time.Now()

	_, err = a.db.Exec(ctx,
		`INSERT INTO incidents (id, dedupe_key, type, severity, status, title, detail,
		                        resource_type, resource_id, source, detected_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		id, params.DedupeKey, params.Type, params.Severity, model.IncidentOpen,
		params.Title, params.Detail, params.ResourceType, params.ResourceID,
		params.Source, now, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create incident: %w", err)
	}

	// Add "created" event.
	_, _ = a.db.Exec(ctx,
		`INSERT INTO incident_events (id, incident_id, actor, action, detail, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		platform.NewID(), id, "system:"+params.Source, "created", params.Title, now,
	)

	return &CreateIncidentResult{ID: id, Created: true}, nil
}

// AutoResolveIncidentsParams holds parameters for auto-resolving incidents.
type AutoResolveIncidentsParams struct {
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	TypePrefix   string `json:"type_prefix"`
	Resolution   string `json:"resolution"`
}

// AutoResolveIncidents resolves all open incidents matching a resource and type prefix.
func (a *CoreDB) AutoResolveIncidents(ctx context.Context, params AutoResolveIncidentsParams) (int, error) {
	now := time.Now()
	rows, err := a.db.Query(ctx,
		`UPDATE incidents SET status = $1, resolution = $2, resolved_at = $3, updated_at = $3
		 WHERE resource_type = $4 AND resource_id = $5 AND type LIKE $6
		   AND status NOT IN ('resolved', 'cancelled')
		 RETURNING id`,
		model.IncidentResolved, params.Resolution, now,
		params.ResourceType, params.ResourceID, params.TypePrefix+"%",
	)
	if err != nil {
		return 0, fmt.Errorf("auto-resolve incidents: %w", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var incID string
		if err := rows.Scan(&incID); err != nil {
			return count, fmt.Errorf("scan resolved id: %w", err)
		}
		_, _ = a.db.Exec(ctx,
			`INSERT INTO incident_events (id, incident_id, actor, action, detail, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			platform.NewID(), incID, "system:auto-resolve", "resolved", params.Resolution, now,
		)
		count++
	}
	return count, nil
}

// EscalateIncidentParams holds parameters for escalating an incident.
type EscalateIncidentParams struct {
	IncidentID string `json:"incident_id"`
	Reason     string `json:"reason"`
	Actor      string `json:"actor"`
}

// EscalateIncident marks an incident as escalated, records an event, and sets escalated_at.
func (a *CoreDB) EscalateIncident(ctx context.Context, params EscalateIncidentParams) error {
	now := time.Now()

	_, err := a.db.Exec(ctx,
		`UPDATE incidents SET status = $1, escalated_at = $2, updated_at = $2 WHERE id = $3`,
		model.IncidentEscalated, now, params.IncidentID,
	)
	if err != nil {
		return fmt.Errorf("escalate incident: %w", err)
	}

	actor := params.Actor
	if actor == "" {
		actor = "system:escalation-cron"
	}

	_, _ = a.db.Exec(ctx,
		`INSERT INTO incident_events (id, incident_id, actor, action, detail, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		platform.NewID(), params.IncidentID, actor, "escalated", params.Reason, now,
	)

	return nil
}

// StaleIncident represents an incident that should be auto-escalated.
type StaleIncident struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Reason   string `json:"reason"`
}

// FindStaleIncidents finds incidents that should be auto-escalated per the escalation policy:
// - Critical open + unassigned > 15 min
// - Warning open + unassigned > 1 hour
// - Investigating or remediating > 30 min
func (a *CoreDB) FindStaleIncidents(ctx context.Context) ([]StaleIncident, error) {
	now := time.Now()

	rows, err := a.db.Query(ctx,
		`SELECT id, status, severity, title FROM incidents
		 WHERE (
		   (status = 'open' AND assigned_to IS NULL AND severity = 'critical' AND updated_at < $1)
		   OR (status = 'open' AND assigned_to IS NULL AND severity = 'warning' AND updated_at < $2)
		   OR (status IN ('investigating', 'remediating') AND updated_at < $3)
		 )`,
		now.Add(-15*time.Minute),
		now.Add(-1*time.Hour),
		now.Add(-30*time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("find stale incidents: %w", err)
	}
	defer rows.Close()

	var stale []StaleIncident
	for rows.Next() {
		var s StaleIncident
		if err := rows.Scan(&s.ID, &s.Status, &s.Severity, &s.Title); err != nil {
			return nil, fmt.Errorf("scan stale incident: %w", err)
		}
		switch {
		case s.Status == model.IncidentOpen && s.Severity == model.SeverityCritical:
			s.Reason = "Critical incident unassigned for more than 15 minutes"
		case s.Status == model.IncidentOpen && s.Severity == model.SeverityWarning:
			s.Reason = "Warning incident unassigned for more than 1 hour"
		default:
			s.Reason = fmt.Sprintf("Incident stuck in '%s' status for more than 30 minutes", s.Status)
		}
		stale = append(stale, s)
	}
	return stale, nil
}

// FindStaleConvergingShardsParams holds the threshold for stale convergence detection.
type FindStaleConvergingShardsParams struct {
	MaxAge time.Duration `json:"max_age"`
}

// StaleConvergingShard represents a shard stuck in converging status.
type StaleConvergingShard struct {
	ID        string `json:"id"`
	ClusterID string `json:"cluster_id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
}

// FindStaleConvergingShards returns shards that have been in "converging" status
// longer than the given threshold.
func (a *CoreDB) FindStaleConvergingShards(ctx context.Context, params FindStaleConvergingShardsParams) ([]StaleConvergingShard, error) {
	cutoff := time.Now().Add(-params.MaxAge)
	rows, err := a.db.Query(ctx,
		`SELECT id, cluster_id, name, role FROM shards
		 WHERE status = 'converging' AND updated_at < $1`, cutoff,
	)
	if err != nil {
		return nil, fmt.Errorf("find stale converging shards: %w", err)
	}
	defer rows.Close()

	var shards []StaleConvergingShard
	for rows.Next() {
		var s StaleConvergingShard
		if err := rows.Scan(&s.ID, &s.ClusterID, &s.Name, &s.Role); err != nil {
			return nil, fmt.Errorf("scan stale converging shard: %w", err)
		}
		shards = append(shards, s)
	}
	return shards, rows.Err()
}

// FindUnhealthyNodes returns active nodes that haven't reported health within the given threshold.
func (a *CoreDB) FindUnhealthyNodes(ctx context.Context, maxAge time.Duration) ([]model.Node, error) {
	cutoff := time.Now().Add(-maxAge)
	rows, err := a.db.Query(ctx,
		`SELECT id, cluster_id, hostname, ip_address::text, ip6_address::text, roles, status, last_health_at, created_at, updated_at
		 FROM nodes
		 WHERE status = $1 AND (last_health_at IS NULL OR last_health_at < $2)`,
		model.StatusActive, cutoff,
	)
	if err != nil {
		return nil, fmt.Errorf("find unhealthy nodes: %w", err)
	}
	defer rows.Close()

	var nodes []model.Node
	for rows.Next() {
		var n model.Node
		if err := rows.Scan(&n.ID, &n.ClusterID, &n.Hostname, &n.IPAddress, &n.IP6Address, &n.Roles, &n.Status, &n.LastHealthAt, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan unhealthy node: %w", err)
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}
