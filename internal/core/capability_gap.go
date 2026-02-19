package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
)

type CapabilityGapService struct {
	db DB
}

func NewCapabilityGapService(db DB) *CapabilityGapService {
	return &CapabilityGapService{db: db}
}

// Report creates a new capability gap or increments occurrences if tool_name already exists.
// If incidentID is provided, links the gap to that incident.
// Returns the gap and true if newly created, false if incremented.
func (s *CapabilityGapService) Report(ctx context.Context, toolName, description, category string, incidentID *string) (*model.CapabilityGap, bool, error) {
	now := time.Now()

	// Try to find existing gap by tool_name.
	var existing model.CapabilityGap
	err := s.db.QueryRow(ctx,
		`UPDATE capability_gaps SET occurrences = occurrences + 1, updated_at = $1
		 WHERE tool_name = $2
		 RETURNING id, tool_name, description, category, occurrences, status, implemented_at, created_at, updated_at`,
		now, toolName,
	).Scan(&existing.ID, &existing.ToolName, &existing.Description, &existing.Category,
		&existing.Occurrences, &existing.Status, &existing.ImplementedAt,
		&existing.CreatedAt, &existing.UpdatedAt)
	if err == nil {
		// Link to incident if provided.
		if incidentID != nil {
			_, _ = s.db.Exec(ctx,
				`INSERT INTO incident_capability_gaps (incident_id, gap_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				*incidentID, existing.ID,
			)
		}
		return &existing, false, nil
	}

	// Create new gap.
	gap := &model.CapabilityGap{
		ID:          platform.NewID(),
		ToolName:    toolName,
		Description: description,
		Category:    category,
		Occurrences: 1,
		Status:      model.GapOpen,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err = s.db.Exec(ctx,
		`INSERT INTO capability_gaps (id, tool_name, description, category, occurrences, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		gap.ID, gap.ToolName, gap.Description, gap.Category,
		gap.Occurrences, gap.Status, gap.CreatedAt, gap.UpdatedAt,
	)
	if err != nil {
		return nil, false, fmt.Errorf("create capability gap: %w", err)
	}

	// Link to incident if provided.
	if incidentID != nil {
		_, _ = s.db.Exec(ctx,
			`INSERT INTO incident_capability_gaps (incident_id, gap_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			*incidentID, gap.ID,
		)
	}

	return gap, true, nil
}

// GetByID returns a capability gap by ID.
func (s *CapabilityGapService) GetByID(ctx context.Context, id string) (*model.CapabilityGap, error) {
	var gap model.CapabilityGap
	err := s.db.QueryRow(ctx,
		`SELECT id, tool_name, description, category, occurrences, status, implemented_at, created_at, updated_at
		 FROM capability_gaps WHERE id = $1`, id,
	).Scan(&gap.ID, &gap.ToolName, &gap.Description, &gap.Category,
		&gap.Occurrences, &gap.Status, &gap.ImplementedAt,
		&gap.CreatedAt, &gap.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get capability gap: %w", err)
	}
	return &gap, nil
}

// List returns capability gaps with optional filters, sorted by occurrences desc.
func (s *CapabilityGapService) List(ctx context.Context, status, category string, limit int, cursor string) ([]model.CapabilityGap, bool, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := `SELECT id, tool_name, description, category, occurrences, status, implemented_at, created_at, updated_at
	           FROM capability_gaps`

	var conditions []string
	var args []any
	argN := 1

	if status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argN))
		args = append(args, status)
		argN++
	}
	if category != "" {
		conditions = append(conditions, fmt.Sprintf("category = $%d", argN))
		args = append(args, category)
		argN++
	}
	if cursor != "" {
		conditions = append(conditions, fmt.Sprintf("(occurrences, id) < (SELECT occurrences, id FROM capability_gaps WHERE id = $%d)", argN))
		args = append(args, cursor)
		argN++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += fmt.Sprintf(" ORDER BY occurrences DESC, id DESC LIMIT $%d", argN)
	args = append(args, limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list capability gaps: %w", err)
	}
	defer rows.Close()

	var gaps []model.CapabilityGap
	for rows.Next() {
		var gap model.CapabilityGap
		if err := rows.Scan(&gap.ID, &gap.ToolName, &gap.Description, &gap.Category,
			&gap.Occurrences, &gap.Status, &gap.ImplementedAt,
			&gap.CreatedAt, &gap.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan capability gap: %w", err)
		}
		gaps = append(gaps, gap)
	}

	hasMore := len(gaps) > limit
	if hasMore {
		gaps = gaps[:limit]
	}
	return gaps, hasMore, nil
}

// Update updates mutable fields on a capability gap.
func (s *CapabilityGapService) Update(ctx context.Context, id string, status *string) error {
	if status == nil {
		return nil
	}

	now := time.Now()
	var implementedAt *time.Time
	if *status == model.GapImplemented {
		implementedAt = &now
	}

	_, err := s.db.Exec(ctx,
		`UPDATE capability_gaps SET status = $1, implemented_at = $2, updated_at = $3 WHERE id = $4`,
		*status, implementedAt, now, id,
	)
	if err != nil {
		return fmt.Errorf("update capability gap: %w", err)
	}
	return nil
}
