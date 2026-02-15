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

	var tenantID string
	if database.TenantID != nil {
		tenantID = *database.TenantID
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "CreateDatabaseWorkflow",
		WorkflowID:   workflowID("database", database.Name, database.ID),
		Arg:          database.ID,
		ResourceType: "database",
		ResourceID:   database.ID,
	}); err != nil {
		return fmt.Errorf("start CreateDatabaseWorkflow: %w", err)
	}

	return nil
}

func (s *DatabaseService) GetByID(ctx context.Context, id string) (*model.Database, error) {
	var d model.Database
	err := s.db.QueryRow(ctx,
		`SELECT d.id, d.tenant_id, d.name, d.shard_id, d.node_id, d.status, d.status_message, d.suspend_reason, d.created_at, d.updated_at,
		        s.name
		 FROM databases d
		 LEFT JOIN shards s ON s.id = d.shard_id
		 WHERE d.id = $1`, id,
	).Scan(&d.ID, &d.TenantID, &d.Name, &d.ShardID, &d.NodeID,
		&d.Status, &d.StatusMessage, &d.SuspendReason, &d.CreatedAt, &d.UpdatedAt,
		&d.ShardName)
	if err != nil {
		return nil, fmt.Errorf("get database %s: %w", id, err)
	}
	return &d, nil
}

func (s *DatabaseService) ListByTenant(ctx context.Context, tenantID string, params request.ListParams) ([]model.Database, bool, error) {
	query := `SELECT d.id, d.tenant_id, d.name, d.shard_id, d.node_id, d.status, d.status_message, d.suspend_reason, d.created_at, d.updated_at, s.name FROM databases d LEFT JOIN shards s ON s.id = d.shard_id WHERE d.tenant_id = $1`
	args := []any{tenantID}
	argIdx := 2

	if params.Search != "" {
		query += fmt.Sprintf(` AND d.name ILIKE $%d`, argIdx)
		args = append(args, "%"+params.Search+"%")
		argIdx++
	}
	if params.Status != "" {
		query += fmt.Sprintf(` AND d.status = $%d`, argIdx)
		args = append(args, params.Status)
		argIdx++
	}
	if params.Cursor != "" {
		query += fmt.Sprintf(` AND d.id > $%d`, argIdx)
		args = append(args, params.Cursor)
		argIdx++
	}

	sortCol := "d.created_at"
	switch params.Sort {
	case "name":
		sortCol = "d.name"
	case "status":
		sortCol = "d.status"
	case "created_at":
		sortCol = "d.created_at"
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
			&d.Status, &d.StatusMessage, &d.SuspendReason, &d.CreatedAt, &d.UpdatedAt,
			&d.ShardName); err != nil {
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
	query := `SELECT d.id, d.tenant_id, d.name, d.shard_id, d.node_id, d.status, d.status_message, d.suspend_reason, d.created_at, d.updated_at, s.name FROM databases d LEFT JOIN shards s ON s.id = d.shard_id WHERE d.shard_id = $1`
	args := []any{shardID}
	argIdx := 2

	if cursor != "" {
		query += fmt.Sprintf(` AND d.id > $%d`, argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query += ` ORDER BY d.id`
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
			&d.Status, &d.StatusMessage, &d.SuspendReason, &d.CreatedAt, &d.UpdatedAt,
			&d.ShardName); err != nil {
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
	var name string
	err := s.db.QueryRow(ctx,
		"UPDATE databases SET status = $1, updated_at = now() WHERE id = $2 RETURNING name",
		model.StatusDeleting, id,
	).Scan(&name)
	if err != nil {
		return fmt.Errorf("set database %s status to deleting: %w", id, err)
	}

	tenantID, err := resolveTenantIDFromDatabase(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("delete database: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteDatabaseWorkflow",
		WorkflowID:   workflowID("database", name, id),
		Arg:          id,
		ResourceType: "database",
		ResourceID:   id,
	}); err != nil {
		return fmt.Errorf("start DeleteDatabaseWorkflow: %w", err)
	}

	return nil
}

func (s *DatabaseService) Migrate(ctx context.Context, id string, targetShardID string) error {
	var name string
	err := s.db.QueryRow(ctx,
		"UPDATE databases SET status = $1, updated_at = now() WHERE id = $2 RETURNING name",
		model.StatusProvisioning, id,
	).Scan(&name)
	if err != nil {
		return fmt.Errorf("set database %s status to provisioning: %w", id, err)
	}

	tenantID, err := resolveTenantIDFromDatabase(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "MigrateDatabaseWorkflow",
		WorkflowID:   workflowID("migrate-database", name, id),
		Arg: MigrateDatabaseParams{
			DatabaseID:    id,
			TargetShardID: targetShardID,
		},
		ResourceType: "database",
		ResourceID:   id,
	}); err != nil {
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

func (s *DatabaseService) Retry(ctx context.Context, id string) error {
	var status, name string
	err := s.db.QueryRow(ctx, "SELECT status, name FROM databases WHERE id = $1", id).Scan(&status, &name)
	if err != nil {
		return fmt.Errorf("get database status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("database %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE databases SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set database %s status to provisioning: %w", id, err)
	}
	tenantID, err := resolveTenantIDFromDatabase(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("retry database: %w", err)
	}
	return signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "CreateDatabaseWorkflow",
		WorkflowID:   workflowID("database", name, id),
		Arg:          id,
		ResourceType: "database",
		ResourceID:   id,
	})
}
