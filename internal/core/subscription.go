package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type SubscriptionService struct {
	db DB
	tc temporalclient.Client
}

func NewSubscriptionService(db DB, tc temporalclient.Client) *SubscriptionService {
	return &SubscriptionService{db: db, tc: tc}
}

func (s *SubscriptionService) Create(ctx context.Context, sub *model.Subscription) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO subscriptions (id, tenant_id, name, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		sub.ID, sub.TenantID, sub.Name, sub.Status, sub.CreatedAt, sub.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert subscription: %w", err)
	}
	return nil
}

func (s *SubscriptionService) GetByID(ctx context.Context, id string) (*model.Subscription, error) {
	var sub model.Subscription
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, status, created_at, updated_at
		 FROM subscriptions WHERE id = $1`, id,
	).Scan(&sub.ID, &sub.TenantID, &sub.Name, &sub.Status, &sub.CreatedAt, &sub.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get subscription %s: %w", id, err)
	}
	return &sub, nil
}

func (s *SubscriptionService) ListByTenant(ctx context.Context, tenantID string, limit int, cursor string) ([]model.Subscription, bool, error) {
	query := `SELECT id, tenant_id, name, status, created_at, updated_at FROM subscriptions WHERE tenant_id = $1`
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
		return nil, false, fmt.Errorf("list subscriptions for tenant %s: %w", tenantID, err)
	}
	defer rows.Close()

	var subs []model.Subscription
	for rows.Next() {
		var sub model.Subscription
		if err := rows.Scan(&sub.ID, &sub.TenantID, &sub.Name, &sub.Status, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan subscription: %w", err)
		}
		subs = append(subs, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate subscriptions: %w", err)
	}

	hasMore := len(subs) > limit
	if hasMore {
		subs = subs[:limit]
	}
	return subs, hasMore, nil
}

func (s *SubscriptionService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE subscriptions SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusDeleting, id,
	)
	if err != nil {
		return fmt.Errorf("set subscription %s status to deleting: %w", id, err)
	}

	var tenantID string
	if err := s.db.QueryRow(ctx, "SELECT tenant_id FROM subscriptions WHERE id = $1", id).Scan(&tenantID); err != nil {
		return fmt.Errorf("resolve tenant for subscription: %w", err)
	}

	if err := signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteSubscriptionWorkflow",
		WorkflowID:   fmt.Sprintf("delete-subscription-%s", id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("signal DeleteSubscriptionWorkflow: %w", err)
	}

	return nil
}
