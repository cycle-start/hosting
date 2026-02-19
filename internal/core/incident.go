package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
)

type IncidentService struct {
	db DB
}

func NewIncidentService(db DB) *IncidentService {
	return &IncidentService{db: db}
}

// Create creates an incident or returns the existing open one if dedupe_key matches.
// Returns the incident and true if it was newly created, false if it already existed.
func (s *IncidentService) Create(ctx context.Context, inc *model.Incident) (bool, error) {
	// Try to find existing open incident with same dedupe_key.
	var existing model.Incident
	err := s.db.QueryRow(ctx,
		`SELECT id, dedupe_key, type, severity, status, title, detail,
		        resource_type, resource_id, source, assigned_to, resolution,
		        detected_at, resolved_at, escalated_at, created_at, updated_at
		 FROM incidents WHERE dedupe_key = $1 AND status NOT IN ('resolved', 'cancelled')`,
		inc.DedupeKey,
	).Scan(&existing.ID, &existing.DedupeKey, &existing.Type, &existing.Severity,
		&existing.Status, &existing.Title, &existing.Detail, &existing.ResourceType,
		&existing.ResourceID, &existing.Source, &existing.AssignedTo, &existing.Resolution,
		&existing.DetectedAt, &existing.ResolvedAt, &existing.EscalatedAt,
		&existing.CreatedAt, &existing.UpdatedAt)
	if err == nil {
		*inc = existing
		return false, nil
	}

	inc.ID = platform.NewName("inc")
	now := time.Now()
	inc.Status = model.IncidentOpen
	if inc.DetectedAt.IsZero() {
		inc.DetectedAt = now
	}
	inc.CreatedAt = now
	inc.UpdatedAt = now

	_, err = s.db.Exec(ctx,
		`INSERT INTO incidents (id, dedupe_key, type, severity, status, title, detail,
		                        resource_type, resource_id, source, detected_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		inc.ID, inc.DedupeKey, inc.Type, inc.Severity, inc.Status, inc.Title, inc.Detail,
		inc.ResourceType, inc.ResourceID, inc.Source, inc.DetectedAt, inc.CreatedAt, inc.UpdatedAt,
	)
	if err != nil {
		return false, fmt.Errorf("create incident: %w", err)
	}

	// Add a "created" event.
	_, err = s.db.Exec(ctx,
		`INSERT INTO incident_events (id, incident_id, actor, action, detail, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		platform.NewID(), inc.ID, "system:"+inc.Source, "created", inc.Title, now,
	)
	if err != nil {
		return true, fmt.Errorf("create incident event: %w", err)
	}

	return true, nil
}

// GetByID returns an incident by ID.
func (s *IncidentService) GetByID(ctx context.Context, id string) (*model.Incident, error) {
	var inc model.Incident
	err := s.db.QueryRow(ctx,
		`SELECT id, dedupe_key, type, severity, status, title, detail,
		        resource_type, resource_id, source, assigned_to, resolution,
		        detected_at, resolved_at, escalated_at, created_at, updated_at
		 FROM incidents WHERE id = $1`, id,
	).Scan(&inc.ID, &inc.DedupeKey, &inc.Type, &inc.Severity,
		&inc.Status, &inc.Title, &inc.Detail, &inc.ResourceType,
		&inc.ResourceID, &inc.Source, &inc.AssignedTo, &inc.Resolution,
		&inc.DetectedAt, &inc.ResolvedAt, &inc.EscalatedAt,
		&inc.CreatedAt, &inc.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get incident: %w", err)
	}
	return &inc, nil
}

// List returns incidents with optional filters, paginated.
func (s *IncidentService) List(ctx context.Context, filters IncidentFilters, limit int, cursor string) ([]model.Incident, bool, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `SELECT id, dedupe_key, type, severity, status, title, detail,
	                  resource_type, resource_id, source, assigned_to, resolution,
	                  detected_at, resolved_at, escalated_at, created_at, updated_at
	           FROM incidents`

	var conditions []string
	var args []any
	argN := 1

	if filters.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argN))
		args = append(args, filters.Status)
		argN++
	}
	if filters.Severity != "" {
		conditions = append(conditions, fmt.Sprintf("severity = $%d", argN))
		args = append(args, filters.Severity)
		argN++
	}
	if filters.Type != "" {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argN))
		args = append(args, filters.Type)
		argN++
	}
	if filters.ResourceType != "" {
		conditions = append(conditions, fmt.Sprintf("resource_type = $%d", argN))
		args = append(args, filters.ResourceType)
		argN++
	}
	if filters.ResourceID != "" {
		conditions = append(conditions, fmt.Sprintf("resource_id = $%d", argN))
		args = append(args, filters.ResourceID)
		argN++
	}
	if filters.Source != "" {
		conditions = append(conditions, fmt.Sprintf("source = $%d", argN))
		args = append(args, filters.Source)
		argN++
	}
	if cursor != "" {
		conditions = append(conditions, fmt.Sprintf("created_at < (SELECT created_at FROM incidents WHERE id = $%d)", argN))
		args = append(args, cursor)
		argN++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argN)
	args = append(args, limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()

	var incidents []model.Incident
	for rows.Next() {
		var inc model.Incident
		if err := rows.Scan(&inc.ID, &inc.DedupeKey, &inc.Type, &inc.Severity,
			&inc.Status, &inc.Title, &inc.Detail, &inc.ResourceType,
			&inc.ResourceID, &inc.Source, &inc.AssignedTo, &inc.Resolution,
			&inc.DetectedAt, &inc.ResolvedAt, &inc.EscalatedAt,
			&inc.CreatedAt, &inc.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan incident: %w", err)
		}
		incidents = append(incidents, inc)
	}

	hasMore := len(incidents) > limit
	if hasMore {
		incidents = incidents[:limit]
	}
	return incidents, hasMore, nil
}

// Update updates mutable fields on an incident.
func (s *IncidentService) Update(ctx context.Context, id string, status, severity, assignedTo *string) error {
	var sets []string
	var args []any
	argN := 1

	if status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d", argN))
		args = append(args, *status)
		argN++
	}
	if severity != nil {
		sets = append(sets, fmt.Sprintf("severity = $%d", argN))
		args = append(args, *severity)
		argN++
	}
	if assignedTo != nil {
		sets = append(sets, fmt.Sprintf("assigned_to = $%d", argN))
		args = append(args, *assignedTo)
		argN++
	}

	if len(sets) == 0 {
		return nil
	}

	sets = append(sets, fmt.Sprintf("updated_at = $%d", argN))
	args = append(args, time.Now())
	argN++

	args = append(args, id)
	query := fmt.Sprintf("UPDATE incidents SET %s WHERE id = $%d", strings.Join(sets, ", "), argN)

	_, err := s.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update incident: %w", err)
	}
	return nil
}

// Resolve marks an incident as resolved.
func (s *IncidentService) Resolve(ctx context.Context, id, resolution, actor string) error {
	now := time.Now()
	_, err := s.db.Exec(ctx,
		`UPDATE incidents SET status = $1, resolution = $2, resolved_at = $3, updated_at = $3 WHERE id = $4`,
		model.IncidentResolved, resolution, now, id,
	)
	if err != nil {
		return fmt.Errorf("resolve incident: %w", err)
	}

	_, err = s.db.Exec(ctx,
		`INSERT INTO incident_events (id, incident_id, actor, action, detail, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		platform.NewID(), id, actor, "resolved", resolution, now,
	)
	if err != nil {
		return fmt.Errorf("create resolve event: %w", err)
	}
	return nil
}

// Escalate marks an incident as escalated.
func (s *IncidentService) Escalate(ctx context.Context, id, reason, actor string) error {
	now := time.Now()
	_, err := s.db.Exec(ctx,
		`UPDATE incidents SET status = $1, escalated_at = $2, updated_at = $2 WHERE id = $3`,
		model.IncidentEscalated, now, id,
	)
	if err != nil {
		return fmt.Errorf("escalate incident: %w", err)
	}

	_, err = s.db.Exec(ctx,
		`INSERT INTO incident_events (id, incident_id, actor, action, detail, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		platform.NewID(), id, actor, "escalated", reason, now,
	)
	if err != nil {
		return fmt.Errorf("create escalate event: %w", err)
	}
	return nil
}

// Cancel marks an incident as cancelled (false positive).
func (s *IncidentService) Cancel(ctx context.Context, id, reason, actor string) error {
	now := time.Now()
	_, err := s.db.Exec(ctx,
		`UPDATE incidents SET status = $1, resolution = $2, updated_at = $3 WHERE id = $4`,
		model.IncidentCancelled, reason, now, id,
	)
	if err != nil {
		return fmt.Errorf("cancel incident: %w", err)
	}

	_, err = s.db.Exec(ctx,
		`INSERT INTO incident_events (id, incident_id, actor, action, detail, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		platform.NewID(), id, actor, "cancelled", reason, now,
	)
	if err != nil {
		return fmt.Errorf("create cancel event: %w", err)
	}
	return nil
}

// AutoResolve resolves all open incidents matching a resource and type prefix.
func (s *IncidentService) AutoResolve(ctx context.Context, resourceType, resourceID, typePrefix, resolution string) (int, error) {
	now := time.Now()
	rows, err := s.db.Query(ctx,
		`UPDATE incidents SET status = $1, resolution = $2, resolved_at = $3, updated_at = $3
		 WHERE resource_type = $4 AND resource_id = $5 AND type LIKE $6
		   AND status NOT IN ('resolved', 'cancelled')
		 RETURNING id`,
		model.IncidentResolved, resolution, now, resourceType, resourceID, typePrefix+"%",
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
		_, _ = s.db.Exec(ctx,
			`INSERT INTO incident_events (id, incident_id, actor, action, detail, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			platform.NewID(), incID, "system:auto-resolve", "resolved", resolution, now,
		)
		count++
	}
	return count, nil
}

// AddEvent adds a timeline event to an incident.
func (s *IncidentService) AddEvent(ctx context.Context, evt *model.IncidentEvent) error {
	evt.ID = platform.NewID()
	evt.CreatedAt = time.Now()

	if evt.Metadata == nil {
		evt.Metadata = []byte("{}")
	}

	_, err := s.db.Exec(ctx,
		`INSERT INTO incident_events (id, incident_id, actor, action, detail, metadata, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		evt.ID, evt.IncidentID, evt.Actor, evt.Action, evt.Detail, evt.Metadata, evt.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("add incident event: %w", err)
	}
	return nil
}

// ListEvents returns events for an incident, ordered chronologically.
func (s *IncidentService) ListEvents(ctx context.Context, incidentID string, limit int, cursor string) ([]model.IncidentEvent, bool, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var query string
	var args []any

	if cursor != "" {
		query = `SELECT id, incident_id, actor, action, detail, metadata, created_at
		         FROM incident_events
		         WHERE incident_id = $1 AND created_at > (SELECT created_at FROM incident_events WHERE id = $2)
		         ORDER BY created_at ASC LIMIT $3`
		args = []any{incidentID, cursor, limit + 1}
	} else {
		query = `SELECT id, incident_id, actor, action, detail, metadata, created_at
		         FROM incident_events
		         WHERE incident_id = $1
		         ORDER BY created_at ASC LIMIT $2`
		args = []any{incidentID, limit + 1}
	}

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list incident events: %w", err)
	}
	defer rows.Close()

	var events []model.IncidentEvent
	for rows.Next() {
		var evt model.IncidentEvent
		if err := rows.Scan(&evt.ID, &evt.IncidentID, &evt.Actor, &evt.Action,
			&evt.Detail, &evt.Metadata, &evt.CreatedAt); err != nil {
			return nil, false, fmt.Errorf("scan incident event: %w", err)
		}
		events = append(events, evt)
	}

	hasMore := len(events) > limit
	if hasMore {
		events = events[:limit]
	}
	return events, hasMore, nil
}

// ListGapsByIncident returns capability gaps linked to an incident.
func (s *IncidentService) ListGapsByIncident(ctx context.Context, incidentID string) ([]model.CapabilityGap, error) {
	rows, err := s.db.Query(ctx,
		`SELECT g.id, g.tool_name, g.description, g.category, g.occurrences, g.status,
		        g.implemented_at, g.created_at, g.updated_at
		 FROM capability_gaps g
		 JOIN incident_capability_gaps icg ON g.id = icg.gap_id
		 WHERE icg.incident_id = $1
		 ORDER BY g.occurrences DESC`, incidentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list gaps by incident: %w", err)
	}
	defer rows.Close()

	var gaps []model.CapabilityGap
	for rows.Next() {
		var g model.CapabilityGap
		if err := rows.Scan(&g.ID, &g.ToolName, &g.Description, &g.Category, &g.Occurrences,
			&g.Status, &g.ImplementedAt, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan capability gap: %w", err)
		}
		gaps = append(gaps, g)
	}
	return gaps, rows.Err()
}

// IncidentFilters holds optional filters for listing incidents.
type IncidentFilters struct {
	Status       string
	Severity     string
	Type         string
	ResourceType string
	ResourceID   string
	Source       string
}
