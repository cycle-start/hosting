package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type TenantService struct {
	db DB
	tc temporalclient.Client
}

func NewTenantService(db DB, tc temporalclient.Client) *TenantService {
	return &TenantService{db: db, tc: tc}
}

func (s *TenantService) Create(ctx context.Context, tenant *model.Tenant) error {
	uid, err := s.NextUID(ctx)
	if err != nil {
		return fmt.Errorf("create tenant: %w", err)
	}
	tenant.UID = uid

	_, err = s.db.Exec(ctx,
		`INSERT INTO tenants (id, name, region_id, cluster_id, shard_id, uid, sftp_enabled, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		tenant.ID, tenant.Name, tenant.RegionID, tenant.ClusterID, tenant.ShardID, tenant.UID,
		tenant.SFTPEnabled, tenant.Status, tenant.CreatedAt, tenant.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert tenant: %w", err)
	}

	workflowID := fmt.Sprintf("tenant-%s", tenant.ID)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "CreateTenantWorkflow", tenant.ID)
	if err != nil {
		return fmt.Errorf("start CreateTenantWorkflow: %w", err)
	}

	return nil
}

func (s *TenantService) GetByID(ctx context.Context, id string) (*model.Tenant, error) {
	var t model.Tenant
	err := s.db.QueryRow(ctx,
		`SELECT id, name, region_id, cluster_id, shard_id, uid, sftp_enabled, status, created_at, updated_at
		 FROM tenants WHERE id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.RegionID, &t.ClusterID, &t.ShardID, &t.UID,
		&t.SFTPEnabled, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get tenant %s: %w", id, err)
	}
	return &t, nil
}

func (s *TenantService) List(ctx context.Context, params request.ListParams) ([]model.Tenant, bool, error) {
	query := `SELECT id, name, region_id, cluster_id, shard_id, uid, sftp_enabled, status, created_at, updated_at FROM tenants WHERE status != 'deleted'`
	args := []any{}
	argIdx := 1

	if params.Search != "" {
		query += fmt.Sprintf(` AND name ILIKE $%d`, argIdx)
		args = append(args, "%"+params.Search+"%")
		argIdx++
	}
	if params.Status != "" {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, params.Status)
		argIdx++
	}
	if params.Cursor != "" {
		query += fmt.Sprintf(` AND id > $%d`, argIdx)
		args = append(args, params.Cursor)
		argIdx++
	}

	sortCol := "created_at"
	switch params.Sort {
	case "name":
		sortCol = "name"
	case "status":
		sortCol = "status"
	case "created_at":
		sortCol = "created_at"
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
		return nil, false, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []model.Tenant
	for rows.Next() {
		var t model.Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.RegionID, &t.ClusterID, &t.ShardID, &t.UID,
			&t.SFTPEnabled, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan tenant: %w", err)
		}
		tenants = append(tenants, t)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate tenants: %w", err)
	}

	hasMore := len(tenants) > params.Limit
	if hasMore {
		tenants = tenants[:params.Limit]
	}
	return tenants, hasMore, nil
}

func (s *TenantService) ListByShard(ctx context.Context, shardID string, limit int, cursor string) ([]model.Tenant, bool, error) {
	query := `SELECT id, name, region_id, cluster_id, shard_id, uid, sftp_enabled, status, created_at, updated_at FROM tenants WHERE shard_id = $1`
	args := []any{shardID}
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
		return nil, false, fmt.Errorf("list tenants for shard %s: %w", shardID, err)
	}
	defer rows.Close()

	var tenants []model.Tenant
	for rows.Next() {
		var t model.Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.RegionID, &t.ClusterID, &t.ShardID, &t.UID,
			&t.SFTPEnabled, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan tenant: %w", err)
		}
		tenants = append(tenants, t)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate tenants: %w", err)
	}

	hasMore := len(tenants) > limit
	if hasMore {
		tenants = tenants[:limit]
	}
	return tenants, hasMore, nil
}

func (s *TenantService) Update(ctx context.Context, tenant *model.Tenant) error {
	_, err := s.db.Exec(ctx,
		`UPDATE tenants SET name = $1, region_id = $2, cluster_id = $3, shard_id = $4, sftp_enabled = $5, status = $6, updated_at = now()
		 WHERE id = $7`,
		tenant.Name, tenant.RegionID, tenant.ClusterID, tenant.ShardID, tenant.SFTPEnabled, tenant.Status, tenant.ID,
	)
	if err != nil {
		return fmt.Errorf("update tenant %s: %w", tenant.ID, err)
	}

	workflowID := fmt.Sprintf("tenant-%s", tenant.ID)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "UpdateTenantWorkflow", tenant.ID)
	if err != nil {
		return fmt.Errorf("start UpdateTenantWorkflow: %w", err)
	}

	return nil
}

func (s *TenantService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE tenants SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusDeleting, id,
	)
	if err != nil {
		return fmt.Errorf("set tenant %s status to deleting: %w", id, err)
	}

	workflowID := fmt.Sprintf("tenant-%s", id)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "DeleteTenantWorkflow", id)
	if err != nil {
		return fmt.Errorf("start DeleteTenantWorkflow: %w", err)
	}

	return nil
}

func (s *TenantService) Suspend(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE tenants SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusSuspended, id,
	)
	if err != nil {
		return fmt.Errorf("set tenant %s status to suspended: %w", id, err)
	}

	workflowID := fmt.Sprintf("tenant-%s", id)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "SuspendTenantWorkflow", id)
	if err != nil {
		return fmt.Errorf("start SuspendTenantWorkflow: %w", err)
	}

	return nil
}

func (s *TenantService) Unsuspend(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE tenants SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusPending, id,
	)
	if err != nil {
		return fmt.Errorf("set tenant %s status to pending: %w", id, err)
	}

	workflowID := fmt.Sprintf("tenant-%s", id)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "UnsuspendTenantWorkflow", id)
	if err != nil {
		return fmt.Errorf("start UnsuspendTenantWorkflow: %w", err)
	}

	return nil
}

func (s *TenantService) Migrate(ctx context.Context, id string, targetShardID string, migrateZones, migrateFQDNs bool) error {
	_, err := s.db.Exec(ctx,
		"UPDATE tenants SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusProvisioning, id,
	)
	if err != nil {
		return fmt.Errorf("set tenant %s status to provisioning: %w", id, err)
	}

	workflowID := fmt.Sprintf("migrate-tenant-%s", id)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "MigrateTenantWorkflow", MigrateTenantParams{
		TenantID:      id,
		TargetShardID: targetShardID,
		MigrateZones:  migrateZones,
		MigrateFQDNs:  migrateFQDNs,
	})
	if err != nil {
		return fmt.Errorf("start MigrateTenantWorkflow: %w", err)
	}

	return nil
}

// MigrateTenantParams holds parameters for the MigrateTenantWorkflow.
type MigrateTenantParams struct {
	TenantID      string `json:"tenant_id"`
	TargetShardID string `json:"target_shard_id"`
	MigrateZones  bool   `json:"migrate_zones"`
	MigrateFQDNs  bool   `json:"migrate_fqdns"`
}

func (s *TenantService) NextUID(ctx context.Context) (int, error) {
	var uid int
	err := s.db.QueryRow(ctx, "SELECT nextval('tenant_uid_seq')").Scan(&uid)
	if err != nil {
		return 0, fmt.Errorf("next tenant uid: %w", err)
	}
	return uid, nil
}
