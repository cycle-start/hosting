package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type DatabaseAccessRuleService struct {
	db DB
	tc temporalclient.Client
}

func NewDatabaseAccessRuleService(db DB, tc temporalclient.Client) *DatabaseAccessRuleService {
	return &DatabaseAccessRuleService{db: db, tc: tc}
}

func (s *DatabaseAccessRuleService) Create(ctx context.Context, rule *model.DatabaseAccessRule) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO database_access_rules (id, database_id, cidr, description, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		rule.ID, rule.DatabaseID, rule.CIDR, rule.Description,
		rule.Status, rule.CreatedAt, rule.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert database access rule: %w", err)
	}

	tenantID, err := resolveTenantIDFromDatabase(ctx, s.db, rule.DatabaseID)
	if err != nil {
		return fmt.Errorf("resolve tenant for database access rule: %w", err)
	}
	if err := signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "SyncDatabaseAccessWorkflow",
		WorkflowID:   fmt.Sprintf("create-db-access-rule-%s", rule.ID),
		Arg:          rule.DatabaseID,
	}); err != nil {
		return fmt.Errorf("signal SyncDatabaseAccessWorkflow: %w", err)
	}

	return nil
}

func (s *DatabaseAccessRuleService) GetByID(ctx context.Context, id string) (*model.DatabaseAccessRule, error) {
	var r model.DatabaseAccessRule
	err := s.db.QueryRow(ctx,
		`SELECT id, database_id, cidr, description, status, status_message, created_at, updated_at
		 FROM database_access_rules WHERE id = $1`, id,
	).Scan(&r.ID, &r.DatabaseID, &r.CIDR, &r.Description,
		&r.Status, &r.StatusMessage, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get database access rule %s: %w", id, err)
	}
	return &r, nil
}

func (s *DatabaseAccessRuleService) ListByDatabase(ctx context.Context, databaseID string, limit int, cursor string) ([]model.DatabaseAccessRule, bool, error) {
	query := `SELECT id, database_id, cidr, description, status, status_message, created_at, updated_at FROM database_access_rules WHERE database_id = $1`
	args := []any{databaseID}
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
		return nil, false, fmt.Errorf("list database access rules for database %s: %w", databaseID, err)
	}
	defer rows.Close()

	var rules []model.DatabaseAccessRule
	for rows.Next() {
		var r model.DatabaseAccessRule
		if err := rows.Scan(&r.ID, &r.DatabaseID, &r.CIDR, &r.Description,
			&r.Status, &r.StatusMessage, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan database access rule: %w", err)
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate database access rules: %w", err)
	}

	hasMore := len(rules) > limit
	if hasMore {
		rules = rules[:limit]
	}
	return rules, hasMore, nil
}

func (s *DatabaseAccessRuleService) Delete(ctx context.Context, id string) error {
	var databaseID string
	err := s.db.QueryRow(ctx,
		"UPDATE database_access_rules SET status = $1, updated_at = now() WHERE id = $2 RETURNING database_id",
		model.StatusDeleting, id,
	).Scan(&databaseID)
	if err != nil {
		return fmt.Errorf("set database access rule %s status to deleting: %w", id, err)
	}

	tenantID, err := resolveTenantIDFromDatabaseAccessRule(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("resolve tenant for database access rule: %w", err)
	}
	if err := signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "SyncDatabaseAccessWorkflow",
		WorkflowID:   workflowID("db-access-rule-del", id, id),
		Arg:          databaseID,
	}); err != nil {
		return fmt.Errorf("signal SyncDatabaseAccessWorkflow: %w", err)
	}

	return nil
}

func (s *DatabaseAccessRuleService) Retry(ctx context.Context, id string) error {
	var status, databaseID string
	err := s.db.QueryRow(ctx, "SELECT status, database_id FROM database_access_rules WHERE id = $1", id).Scan(&status, &databaseID)
	if err != nil {
		return fmt.Errorf("get database access rule status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("database access rule %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE database_access_rules SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set database access rule %s status to provisioning: %w", id, err)
	}
	tenantID, err := resolveTenantIDFromDatabaseAccessRule(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("resolve tenant for database access rule: %w", err)
	}
	return signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "SyncDatabaseAccessWorkflow",
		WorkflowID:   workflowID("db-access-rule-retry", id, id),
		Arg:          databaseID,
	})
}
