package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type DatabaseService struct {
	db DB
	tc temporalclient.Client
}

func NewDatabaseService(db DB, tc temporalclient.Client) *DatabaseService {
	return &DatabaseService{db: db, tc: tc}
}

func (s *DatabaseService) Create(ctx context.Context, database *model.Database) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO databases (id, tenant_id, name, shard_id, node_id, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		database.ID, database.TenantID, database.Name, database.ShardID, database.NodeID,
		database.Status, database.CreatedAt, database.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert database: %w", err)
	}

	workflowID := fmt.Sprintf("database-%s", database.ID)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "CreateDatabaseWorkflow", database.ID)
	if err != nil {
		return fmt.Errorf("start CreateDatabaseWorkflow: %w", err)
	}

	return nil
}

func (s *DatabaseService) GetByID(ctx context.Context, id string) (*model.Database, error) {
	var d model.Database
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, shard_id, node_id, status, created_at, updated_at
		 FROM databases WHERE id = $1`, id,
	).Scan(&d.ID, &d.TenantID, &d.Name, &d.ShardID, &d.NodeID,
		&d.Status, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get database %s: %w", id, err)
	}
	return &d, nil
}

func (s *DatabaseService) ListByTenant(ctx context.Context, tenantID string, params request.ListParams) ([]model.Database, bool, error) {
	query := `SELECT id, tenant_id, name, shard_id, node_id, status, created_at, updated_at FROM databases WHERE tenant_id = $1 AND status != 'deleted'`
	args := []any{tenantID}
	argIdx := 2

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
		return nil, false, fmt.Errorf("list databases for tenant %s: %w", tenantID, err)
	}
	defer rows.Close()

	var databases []model.Database
	for rows.Next() {
		var d model.Database
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.ShardID, &d.NodeID,
			&d.Status, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan database: %w", err)
		}
		databases = append(databases, d)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate databases: %w", err)
	}

	hasMore := len(databases) > params.Limit
	if hasMore {
		databases = databases[:params.Limit]
	}
	return databases, hasMore, nil
}

func (s *DatabaseService) ListByShard(ctx context.Context, shardID string, limit int, cursor string) ([]model.Database, bool, error) {
	query := `SELECT id, tenant_id, name, shard_id, node_id, status, created_at, updated_at FROM databases WHERE shard_id = $1`
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
		return nil, false, fmt.Errorf("list databases for shard %s: %w", shardID, err)
	}
	defer rows.Close()

	var databases []model.Database
	for rows.Next() {
		var d model.Database
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.ShardID, &d.NodeID,
			&d.Status, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan database: %w", err)
		}
		databases = append(databases, d)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate databases: %w", err)
	}

	hasMore := len(databases) > limit
	if hasMore {
		databases = databases[:limit]
	}
	return databases, hasMore, nil
}

func (s *DatabaseService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE databases SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusDeleting, id,
	)
	if err != nil {
		return fmt.Errorf("set database %s status to deleting: %w", id, err)
	}

	workflowID := fmt.Sprintf("database-%s", id)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "DeleteDatabaseWorkflow", id)
	if err != nil {
		return fmt.Errorf("start DeleteDatabaseWorkflow: %w", err)
	}

	return nil
}

func (s *DatabaseService) Migrate(ctx context.Context, id string, targetShardID string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE databases SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusProvisioning, id,
	)
	if err != nil {
		return fmt.Errorf("set database %s status to provisioning: %w", id, err)
	}

	workflowID := fmt.Sprintf("migrate-database-%s", id)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "MigrateDatabaseWorkflow", MigrateDatabaseParams{
		DatabaseID:    id,
		TargetShardID: targetShardID,
	})
	if err != nil {
		return fmt.Errorf("start MigrateDatabaseWorkflow: %w", err)
	}

	return nil
}

// MigrateDatabaseParams holds parameters for the MigrateDatabaseWorkflow.
type MigrateDatabaseParams struct {
	DatabaseID    string `json:"database_id"`
	TargetShardID string `json:"target_shard_id"`
}

func (s *DatabaseService) ReassignTenant(ctx context.Context, id string, tenantID *string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE databases SET tenant_id = $1, updated_at = now() WHERE id = $2",
		tenantID, id,
	)
	if err != nil {
		return fmt.Errorf("reassign database %s to tenant: %w", id, err)
	}
	return nil
}
