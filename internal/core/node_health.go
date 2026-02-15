package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
)

type NodeHealthService struct {
	db DB
}

func NewNodeHealthService(db DB) *NodeHealthService {
	return &NodeHealthService{db: db}
}

// UpsertHealth inserts or updates a node's health report.
func (s *NodeHealthService) UpsertHealth(ctx context.Context, health *model.NodeHealth) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO node_health (node_id, status, checks, reconciliation, reported_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (node_id) DO UPDATE SET
			status = EXCLUDED.status,
			checks = EXCLUDED.checks,
			reconciliation = EXCLUDED.reconciliation,
			reported_at = EXCLUDED.reported_at`,
		health.NodeID, health.Status, health.Checks, health.Reconciliation, health.ReportedAt)
	if err != nil {
		return fmt.Errorf("upsert node health for %s: %w", health.NodeID, err)
	}

	// Update last_health_at on the nodes table
	_, err = s.db.Exec(ctx, `UPDATE nodes SET last_health_at = $1 WHERE id = $2`, health.ReportedAt, health.NodeID)
	if err != nil {
		return fmt.Errorf("update last_health_at for node %s: %w", health.NodeID, err)
	}
	return nil
}

// GetHealth returns the latest health report for a node.
func (s *NodeHealthService) GetHealth(ctx context.Context, nodeID string) (*model.NodeHealth, error) {
	var h model.NodeHealth
	err := s.db.QueryRow(ctx, `
		SELECT node_id, status, checks, reconciliation, reported_at
		FROM node_health WHERE node_id = $1`, nodeID).
		Scan(&h.NodeID, &h.Status, &h.Checks, &h.Reconciliation, &h.ReportedAt)
	if err != nil {
		return nil, fmt.Errorf("get health for node %s: %w", nodeID, err)
	}
	return &h, nil
}

// CreateDriftEvents inserts drift events.
func (s *NodeHealthService) CreateDriftEvents(ctx context.Context, events []model.DriftEvent) error {
	if len(events) == 0 {
		return nil
	}

	for _, e := range events {
		_, err := s.db.Exec(ctx, `
			INSERT INTO drift_events (node_id, kind, resource, action, detail)
			VALUES ($1, $2, $3, $4, $5)`,
			e.NodeID, e.Kind, e.Resource, e.Action, e.Detail)
		if err != nil {
			return fmt.Errorf("insert drift event: %w", err)
		}
	}
	return nil
}

// ListDriftEvents returns recent drift events for a node.
func (s *NodeHealthService) ListDriftEvents(ctx context.Context, nodeID string, limit int) ([]model.DriftEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, node_id, kind, resource, action, detail, created_at
		FROM drift_events WHERE node_id = $1
		ORDER BY created_at DESC LIMIT $2`, nodeID, limit)
	if err != nil {
		return nil, fmt.Errorf("list drift events for node %s: %w", nodeID, err)
	}
	defer rows.Close()

	var events []model.DriftEvent
	for rows.Next() {
		var e model.DriftEvent
		if err := rows.Scan(&e.ID, &e.NodeID, &e.Kind, &e.Resource, &e.Action, &e.Detail, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan drift event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate drift events: %w", err)
	}
	return events, nil
}
