package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type TenantEgressRuleService struct {
	db DB
	tc temporalclient.Client
}

func NewTenantEgressRuleService(db DB, tc temporalclient.Client) *TenantEgressRuleService {
	return &TenantEgressRuleService{db: db, tc: tc}
}

func (s *TenantEgressRuleService) Create(ctx context.Context, rule *model.TenantEgressRule) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO tenant_egress_rules (id, tenant_id, cidr, description, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		rule.ID, rule.TenantID, rule.CIDR, rule.Description,
		rule.Status, rule.CreatedAt, rule.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert tenant egress rule: %w", err)
	}

	if err := signalProvision(ctx, s.tc, rule.TenantID, model.ProvisionTask{
		WorkflowName: "SyncEgressRulesWorkflow",
		WorkflowID:   workflowID("egress-rule", rule.CIDR, rule.ID),
		Arg:          rule.TenantID,
		ResourceType: "tenant-egress-rule",
		ResourceID:   rule.ID,
	}); err != nil {
		return fmt.Errorf("start SyncEgressRulesWorkflow: %w", err)
	}

	return nil
}

func (s *TenantEgressRuleService) GetByID(ctx context.Context, id string) (*model.TenantEgressRule, error) {
	var r model.TenantEgressRule
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, cidr, description, status, status_message, created_at, updated_at
		 FROM tenant_egress_rules WHERE id = $1`, id,
	).Scan(&r.ID, &r.TenantID, &r.CIDR, &r.Description,
		&r.Status, &r.StatusMessage, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get tenant egress rule %s: %w", id, err)
	}
	return &r, nil
}

func (s *TenantEgressRuleService) ListByTenant(ctx context.Context, tenantID string, limit int, cursor string) ([]model.TenantEgressRule, bool, error) {
	query := `SELECT id, tenant_id, cidr, description, status, status_message, created_at, updated_at FROM tenant_egress_rules WHERE tenant_id = $1`
	args := []any{tenantID}
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
		return nil, false, fmt.Errorf("list tenant egress rules for tenant %s: %w", tenantID, err)
	}
	defer rows.Close()

	var rules []model.TenantEgressRule
	for rows.Next() {
		var r model.TenantEgressRule
		if err := rows.Scan(&r.ID, &r.TenantID, &r.CIDR, &r.Description,
			&r.Status, &r.StatusMessage, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan tenant egress rule: %w", err)
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate tenant egress rules: %w", err)
	}

	hasMore := len(rules) > limit
	if hasMore {
		rules = rules[:limit]
	}
	return rules, hasMore, nil
}

func (s *TenantEgressRuleService) Delete(ctx context.Context, id string) error {
	var tenantID string
	err := s.db.QueryRow(ctx,
		"UPDATE tenant_egress_rules SET status = $1, updated_at = now() WHERE id = $2 RETURNING tenant_id",
		model.StatusDeleting, id,
	).Scan(&tenantID)
	if err != nil {
		return fmt.Errorf("set tenant egress rule %s status to deleting: %w", id, err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "SyncEgressRulesWorkflow",
		WorkflowID:   workflowID("egress-rule-del", id, id),
		Arg:          tenantID,
		ResourceType: "tenant-egress-rule",
		ResourceID:   id,
	}); err != nil {
		return fmt.Errorf("start SyncEgressRulesWorkflow: %w", err)
	}

	return nil
}

func (s *TenantEgressRuleService) Retry(ctx context.Context, id string) error {
	var status, tenantID string
	err := s.db.QueryRow(ctx, "SELECT status, tenant_id FROM tenant_egress_rules WHERE id = $1", id).Scan(&status, &tenantID)
	if err != nil {
		return fmt.Errorf("get tenant egress rule status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("tenant egress rule %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE tenant_egress_rules SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set tenant egress rule %s status to provisioning: %w", id, err)
	}
	return signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "SyncEgressRulesWorkflow",
		WorkflowID:   workflowID("egress-rule-retry", id, id),
		Arg:          tenantID,
		ResourceType: "tenant-egress-rule",
		ResourceID:   id,
	})
}
