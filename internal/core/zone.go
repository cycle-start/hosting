package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type ZoneService struct {
	db DB
	tc temporalclient.Client
}

func NewZoneService(db DB, tc temporalclient.Client) *ZoneService {
	return &ZoneService{db: db, tc: tc}
}

func (s *ZoneService) Create(ctx context.Context, zone *model.Zone) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO zones (id, tenant_id, name, region_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		zone.ID, zone.TenantID, zone.Name, zone.RegionID, zone.Status,
		zone.CreatedAt, zone.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert zone: %w", err)
	}

	workflowID := fmt.Sprintf("zone-%s", zone.ID)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "CreateZoneWorkflow", zone.ID)
	if err != nil {
		return fmt.Errorf("start CreateZoneWorkflow: %w", err)
	}

	return nil
}

func (s *ZoneService) GetByID(ctx context.Context, id string) (*model.Zone, error) {
	var z model.Zone
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, region_id, status, created_at, updated_at
		 FROM zones WHERE id = $1`, id,
	).Scan(&z.ID, &z.TenantID, &z.Name, &z.RegionID, &z.Status,
		&z.CreatedAt, &z.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get zone %s: %w", id, err)
	}
	return &z, nil
}

func (s *ZoneService) List(ctx context.Context) ([]model.Zone, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, tenant_id, name, region_id, status, created_at, updated_at
		 FROM zones ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list zones: %w", err)
	}
	defer rows.Close()

	var zones []model.Zone
	for rows.Next() {
		var z model.Zone
		if err := rows.Scan(&z.ID, &z.TenantID, &z.Name, &z.RegionID, &z.Status,
			&z.CreatedAt, &z.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan zone: %w", err)
		}
		zones = append(zones, z)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate zones: %w", err)
	}
	return zones, nil
}

func (s *ZoneService) Update(ctx context.Context, zone *model.Zone) error {
	_, err := s.db.Exec(ctx,
		`UPDATE zones SET tenant_id = $1, name = $2, region_id = $3, status = $4, updated_at = now()
		 WHERE id = $5`,
		zone.TenantID, zone.Name, zone.RegionID, zone.Status, zone.ID,
	)
	if err != nil {
		return fmt.Errorf("update zone %s: %w", zone.ID, err)
	}
	return nil
}

func (s *ZoneService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE zones SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusDeleting, id,
	)
	if err != nil {
		return fmt.Errorf("set zone %s status to deleting: %w", id, err)
	}

	workflowID := fmt.Sprintf("zone-%s", id)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "DeleteZoneWorkflow", id)
	if err != nil {
		return fmt.Errorf("start DeleteZoneWorkflow: %w", err)
	}

	return nil
}

func (s *ZoneService) ReassignTenant(ctx context.Context, id string, tenantID *string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE zones SET tenant_id = $1, updated_at = now() WHERE id = $2",
		tenantID, id,
	)
	if err != nil {
		return fmt.Errorf("reassign zone %s to tenant: %w", id, err)
	}
	return nil
}
