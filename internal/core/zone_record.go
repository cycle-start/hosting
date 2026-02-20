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
		`INSERT INTO zone_records (id, zone_id, type, name, content, ttl, priority, managed_by, source_type, source_fqdn_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		record.ID, record.ZoneID, record.Type, record.Name, record.Content,
		record.TTL, record.Priority, record.ManagedBy, record.SourceType, record.SourceFQDNID,
		record.Status, record.CreatedAt, record.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert zone record: %w", err)
	}

	zoneName, err := s.getZoneName(ctx, record.ZoneID)
	if err != nil {
		return fmt.Errorf("get zone name for record: %w", err)
	}

	tenantID, err := resolveTenantIDFromZone(ctx, s.db, record.ZoneID)
	if err != nil {
		return fmt.Errorf("resolve tenant for zone record: %w", err)
	}
	if err := signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "CreateZoneRecordWorkflow",
		WorkflowID:   fmt.Sprintf("create-zone-record-%s", record.ID),
		Arg: model.ZoneRecordParams{
			RecordID:  record.ID,
			Name:      record.Name,
			Type:      record.Type,
			Content:   record.Content,
			TTL:       record.TTL,
			Priority:  record.Priority,
			ManagedBy: record.ManagedBy,
			ZoneName:  zoneName,
		},
	}); err != nil {
		return fmt.Errorf("signal CreateZoneRecordWorkflow: %w", err)
	}

	return nil
}

func (s *ZoneRecordService) GetByID(ctx context.Context, id string) (*model.ZoneRecord, error) {
	var r model.ZoneRecord
	err := s.db.QueryRow(ctx,
		`SELECT id, zone_id, type, name, content, ttl, priority, managed_by, source_type, source_fqdn_id, status, status_message, created_at, updated_at
		 FROM zone_records WHERE id = $1`, id,
	).Scan(&r.ID, &r.ZoneID, &r.Type, &r.Name, &r.Content,
		&r.TTL, &r.Priority, &r.ManagedBy, &r.SourceType, &r.SourceFQDNID,
		&r.Status, &r.StatusMessage, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get zone record %s: %w", id, err)
	}
	return &r, nil
}

func (s *ZoneRecordService) ListByZone(ctx context.Context, zoneID string, limit int, cursor string) ([]model.ZoneRecord, bool, error) {
	query := `SELECT id, zone_id, type, name, content, ttl, priority, managed_by, source_type, source_fqdn_id, status, status_message, created_at, updated_at FROM zone_records WHERE zone_id = $1`
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
			&r.TTL, &r.Priority, &r.ManagedBy, &r.SourceType, &r.SourceFQDNID,
			&r.Status, &r.StatusMessage, &r.CreatedAt, &r.UpdatedAt); err != nil {
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

	zoneName, err := s.getZoneNameByRecord(ctx, record.ID)
	if err != nil {
		return fmt.Errorf("get zone name for record: %w", err)
	}

	tenantID, err := resolveTenantIDFromZoneRecord(ctx, s.db, record.ID)
	if err != nil {
		return fmt.Errorf("resolve tenant for zone record: %w", err)
	}
	if err := signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "UpdateZoneRecordWorkflow",
		WorkflowID:   workflowID("zone-record", record.Name+"/"+record.Type, record.ID),
		Arg: model.ZoneRecordParams{
			RecordID:  record.ID,
			Name:      record.Name,
			Type:      record.Type,
			Content:   record.Content,
			TTL:       record.TTL,
			Priority:  record.Priority,
			ManagedBy: record.ManagedBy,
			ZoneName:  zoneName,
		},
	}); err != nil {
		return fmt.Errorf("signal UpdateZoneRecordWorkflow: %w", err)
	}

	return nil
}

func (s *ZoneRecordService) Delete(ctx context.Context, id string) error {
	var name, rtype, content, managedBy string
	var ttl int
	var priority *int
	err := s.db.QueryRow(ctx,
		`UPDATE zone_records SET status = $1, updated_at = now() WHERE id = $2
		 RETURNING name, type, content, ttl, priority, managed_by`,
		model.StatusDeleting, id,
	).Scan(&name, &rtype, &content, &ttl, &priority, &managedBy)
	if err != nil {
		return fmt.Errorf("set zone record %s status to deleting: %w", id, err)
	}

	zoneName, err := s.getZoneNameByRecord(ctx, id)
	if err != nil {
		return fmt.Errorf("get zone name for record: %w", err)
	}

	tenantID, err := resolveTenantIDFromZoneRecord(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("resolve tenant for zone record: %w", err)
	}
	if err := signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteZoneRecordWorkflow",
		WorkflowID:   workflowID("zone-record", name+"/"+rtype, id),
		Arg: model.ZoneRecordParams{
			RecordID:  id,
			Name:      name,
			Type:      rtype,
			Content:   content,
			TTL:       ttl,
			Priority:  priority,
			ManagedBy: managedBy,
			ZoneName:  zoneName,
		},
	}); err != nil {
		return fmt.Errorf("signal DeleteZoneRecordWorkflow: %w", err)
	}

	return nil
}

func (s *ZoneRecordService) Retry(ctx context.Context, id string) error {
	var status string
	var r model.ZoneRecord
	err := s.db.QueryRow(ctx,
		`SELECT status, name, type, content, ttl, priority, managed_by, zone_id
		 FROM zone_records WHERE id = $1`, id,
	).Scan(&status, &r.Name, &r.Type, &r.Content, &r.TTL, &r.Priority, &r.ManagedBy, &r.ZoneID)
	if err != nil {
		return fmt.Errorf("get zone record status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("zone record %s is not in failed state (current: %s)", id, status)
	}

	_, err = s.db.Exec(ctx, "UPDATE zone_records SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set zone record %s status to provisioning: %w", id, err)
	}

	zoneName, err := s.getZoneName(ctx, r.ZoneID)
	if err != nil {
		return fmt.Errorf("get zone name for record: %w", err)
	}

	tenantID, err := resolveTenantIDFromZoneRecord(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("resolve tenant for zone record: %w", err)
	}
	return signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "CreateZoneRecordWorkflow",
		WorkflowID:   workflowID("zone-record", r.Name+"/"+r.Type, id),
		Arg: model.ZoneRecordParams{
			RecordID:  id,
			Name:      r.Name,
			Type:      r.Type,
			Content:   r.Content,
			TTL:       r.TTL,
			Priority:  r.Priority,
			ManagedBy: r.ManagedBy,
			ZoneName:  zoneName,
		},
	})
}

// getZoneName fetches the zone name by zone ID.
func (s *ZoneRecordService) getZoneName(ctx context.Context, zoneID string) (string, error) {
	var name string
	err := s.db.QueryRow(ctx, "SELECT name FROM zones WHERE id = $1", zoneID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("get zone name for zone %s: %w", zoneID, err)
	}
	return name, nil
}

// getZoneNameByRecord fetches the zone name via the record's zone_id.
func (s *ZoneRecordService) getZoneNameByRecord(ctx context.Context, recordID string) (string, error) {
	var name string
	err := s.db.QueryRow(ctx,
		`SELECT z.name FROM zones z JOIN zone_records r ON r.zone_id = z.id WHERE r.id = $1`, recordID,
	).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("get zone name for record %s: %w", recordID, err)
	}
	return name, nil
}
