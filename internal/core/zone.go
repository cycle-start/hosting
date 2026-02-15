package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/api/request"
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
	// Auto-derive brand_id from tenant if not set.
	if zone.BrandID == "" && zone.TenantID != nil {
		var brandID string
		err := s.db.QueryRow(ctx, `SELECT brand_id FROM tenants WHERE id = $1`, *zone.TenantID).Scan(&brandID)
		if err != nil {
			return fmt.Errorf("get tenant brand_id for zone: %w", err)
		}
		zone.BrandID = brandID
	}

	_, err := s.db.Exec(ctx,
		`INSERT INTO zones (id, brand_id, tenant_id, name, region_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		zone.ID, zone.BrandID, zone.TenantID, zone.Name, zone.RegionID, zone.Status,
		zone.CreatedAt, zone.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert zone: %w", err)
	}

	var tenantID string
	if zone.TenantID != nil {
		tenantID = *zone.TenantID
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "CreateZoneWorkflow",
		WorkflowID:   workflowID("zone", zone.Name, zone.ID),
		Arg:          zone.ID,
		ResourceType: "zone",
		ResourceID:   zone.ID,
	}); err != nil {
		return fmt.Errorf("start CreateZoneWorkflow: %w", err)
	}

	return nil
}

func (s *ZoneService) GetByID(ctx context.Context, id string) (*model.Zone, error) {
	var z model.Zone
	err := s.db.QueryRow(ctx,
		`SELECT z.id, z.brand_id, z.tenant_id, z.name, z.region_id, z.status, z.status_message, z.suspend_reason, z.created_at, z.updated_at,
		        r.name
		 FROM zones z
		 JOIN regions r ON r.id = z.region_id
		 WHERE z.id = $1`, id,
	).Scan(&z.ID, &z.BrandID, &z.TenantID, &z.Name, &z.RegionID, &z.Status, &z.StatusMessage, &z.SuspendReason,
		&z.CreatedAt, &z.UpdatedAt,
		&z.RegionName)
	if err != nil {
		return nil, fmt.Errorf("get zone %s: %w", id, err)
	}
	return &z, nil
}

func (s *ZoneService) List(ctx context.Context, params request.ListParams) ([]model.Zone, bool, error) {
	query := `SELECT z.id, z.brand_id, z.tenant_id, z.name, z.region_id, z.status, z.status_message, z.suspend_reason, z.created_at, z.updated_at, r.name FROM zones z JOIN regions r ON r.id = z.region_id WHERE true`
	args := []any{}
	argIdx := 1

	if params.Search != "" {
		query += fmt.Sprintf(` AND z.name ILIKE $%d`, argIdx)
		args = append(args, "%"+params.Search+"%")
		argIdx++
	}
	if params.Status != "" {
		query += fmt.Sprintf(` AND z.status = $%d`, argIdx)
		args = append(args, params.Status)
		argIdx++
	}
	if params.Cursor != "" {
		query += fmt.Sprintf(` AND z.id > $%d`, argIdx)
		args = append(args, params.Cursor)
		argIdx++
	}
	if len(params.BrandIDs) > 0 {
		query += fmt.Sprintf(` AND z.brand_id = ANY($%d)`, argIdx)
		args = append(args, params.BrandIDs)
		argIdx++
	}

	sortCol := "z.created_at"
	switch params.Sort {
	case "name":
		sortCol = "z.name"
	case "status":
		sortCol = "z.status"
	case "created_at":
		sortCol = "z.created_at"
	}
	order := "DESC"
	if params.Order == "asc" {
		order = "ASC"
	}
	query += fmt.Sprintf(` ORDER BY %s %s`, sortCol, order)
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, params.Limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list zones: %w", err)
	}
	defer rows.Close()

	var zones []model.Zone
	for rows.Next() {
		var z model.Zone
		if err := rows.Scan(&z.ID, &z.BrandID, &z.TenantID, &z.Name, &z.RegionID, &z.Status, &z.StatusMessage, &z.SuspendReason,
			&z.CreatedAt, &z.UpdatedAt,
			&z.RegionName); err != nil {
			return nil, false, fmt.Errorf("scan zone: %w", err)
		}
		zones = append(zones, z)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate zones: %w", err)
	}

	hasMore := len(zones) > params.Limit
	if hasMore {
		zones = zones[:params.Limit]
	}
	return zones, hasMore, nil
}

func (s *ZoneService) Update(ctx context.Context, zone *model.Zone) error {
	_, err := s.db.Exec(ctx,
		`UPDATE zones SET brand_id = $1, tenant_id = $2, name = $3, region_id = $4, status = $5, updated_at = now()
		 WHERE id = $6`,
		zone.BrandID, zone.TenantID, zone.Name, zone.RegionID, zone.Status, zone.ID,
	)
	if err != nil {
		return fmt.Errorf("update zone %s: %w", zone.ID, err)
	}
	return nil
}

func (s *ZoneService) Delete(ctx context.Context, id string) error {
	var name string
	err := s.db.QueryRow(ctx,
		"UPDATE zones SET status = $1, updated_at = now() WHERE id = $2 RETURNING name",
		model.StatusDeleting, id,
	).Scan(&name)
	if err != nil {
		return fmt.Errorf("set zone %s status to deleting: %w", id, err)
	}

	tenantID, err := resolveTenantIDFromZone(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("delete zone: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteZoneWorkflow",
		WorkflowID:   workflowID("zone", name, id),
		Arg:          id,
		ResourceType: "zone",
		ResourceID:   id,
	}); err != nil {
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

func (s *ZoneService) Retry(ctx context.Context, id string) error {
	var status, name string
	err := s.db.QueryRow(ctx, "SELECT status, name FROM zones WHERE id = $1", id).Scan(&status, &name)
	if err != nil {
		return fmt.Errorf("get zone status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("zone %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE zones SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set zone %s status to provisioning: %w", id, err)
	}
	tenantID, err := resolveTenantIDFromZone(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("retry zone: %w", err)
	}
	return signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "CreateZoneWorkflow",
		WorkflowID:   workflowID("zone", name, id),
		Arg:          id,
		ResourceType: "zone",
		ResourceID:   id,
	})
}
