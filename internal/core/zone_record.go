package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type ZoneRecordService struct {
	db DB
	tc temporalclient.Client
}

func NewZoneRecordService(db DB, tc temporalclient.Client) *ZoneRecordService {
	return &ZoneRecordService{db: db, tc: tc}
}

func (s *ZoneRecordService) Create(ctx context.Context, record *model.ZoneRecord) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO zone_records (id, zone_id, type, name, content, ttl, priority, managed_by, source_fqdn_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		record.ID, record.ZoneID, record.Type, record.Name, record.Content,
		record.TTL, record.Priority, record.ManagedBy, record.SourceFQDNID,
		record.Status, record.CreatedAt, record.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert zone record: %w", err)
	}

	workflowID := fmt.Sprintf("zone-record-%s", record.ID)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "CreateZoneRecordWorkflow", record.ID)
	if err != nil {
		return fmt.Errorf("start CreateZoneRecordWorkflow: %w", err)
	}

	return nil
}

func (s *ZoneRecordService) GetByID(ctx context.Context, id string) (*model.ZoneRecord, error) {
	var r model.ZoneRecord
	err := s.db.QueryRow(ctx,
		`SELECT id, zone_id, type, name, content, ttl, priority, managed_by, source_fqdn_id, status, created_at, updated_at
		 FROM zone_records WHERE id = $1`, id,
	).Scan(&r.ID, &r.ZoneID, &r.Type, &r.Name, &r.Content,
		&r.TTL, &r.Priority, &r.ManagedBy, &r.SourceFQDNID,
		&r.Status, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get zone record %s: %w", id, err)
	}
	return &r, nil
}

func (s *ZoneRecordService) ListByZone(ctx context.Context, zoneID string, limit int, cursor string) ([]model.ZoneRecord, bool, error) {
	query := `SELECT id, zone_id, type, name, content, ttl, priority, managed_by, source_fqdn_id, status, created_at, updated_at FROM zone_records WHERE zone_id = $1`
	args := []any{zoneID}
	argIdx := 2

	if cursor != "" {
		query += fmt.Sprintf(` AND id > $%d`, argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query += ` ORDER BY id`
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list zone records for zone %s: %w", zoneID, err)
	}
	defer rows.Close()

	var records []model.ZoneRecord
	for rows.Next() {
		var r model.ZoneRecord
		if err := rows.Scan(&r.ID, &r.ZoneID, &r.Type, &r.Name, &r.Content,
			&r.TTL, &r.Priority, &r.ManagedBy, &r.SourceFQDNID,
			&r.Status, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan zone record: %w", err)
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate zone records: %w", err)
	}

	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}
	return records, hasMore, nil
}

func (s *ZoneRecordService) Update(ctx context.Context, record *model.ZoneRecord) error {
	_, err := s.db.Exec(ctx,
		`UPDATE zone_records SET type = $1, name = $2, content = $3, ttl = $4, priority = $5, updated_at = now()
		 WHERE id = $6`,
		record.Type, record.Name, record.Content, record.TTL, record.Priority, record.ID,
	)
	if err != nil {
		return fmt.Errorf("update zone record %s: %w", record.ID, err)
	}

	workflowID := fmt.Sprintf("zone-record-%s", record.ID)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "UpdateZoneRecordWorkflow", record.ID)
	if err != nil {
		return fmt.Errorf("start UpdateZoneRecordWorkflow: %w", err)
	}

	return nil
}

func (s *ZoneRecordService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE zone_records SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusDeleting, id,
	)
	if err != nil {
		return fmt.Errorf("set zone record %s status to deleting: %w", id, err)
	}

	workflowID := fmt.Sprintf("zone-record-%s", id)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "DeleteZoneRecordWorkflow", id)
	if err != nil {
		return fmt.Errorf("start DeleteZoneRecordWorkflow: %w", err)
	}

	return nil
}
