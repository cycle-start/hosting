package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/controlpanel/model"
)

type SubscriptionService struct {
	db DB
}

func NewSubscriptionService(db DB) *SubscriptionService {
	return &SubscriptionService{db: db}
}

// ListByCustomer returns all subscriptions for a customer.
func (s *SubscriptionService) ListByCustomer(ctx context.Context, customerID string) ([]model.Subscription, error) {
	rows, err := s.db.Query(ctx,
		`SELECT cs.id, cs.customer_id, cs.tenant_id, p.name, p.description, p.modules, cs.status, cs.updated_at
		 FROM customer_subscriptions cs
		 JOIN products p ON p.id = cs.product_id
		 WHERE cs.customer_id = $1
		 ORDER BY p.name`, customerID)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions for customer %s: %w", customerID, err)
	}
	defer rows.Close()

	var subs []model.Subscription
	for rows.Next() {
		var sub model.Subscription
		if err := rows.Scan(&sub.ID, &sub.CustomerID, &sub.TenantID, &sub.ProductName,
			&sub.ProductDescription, &sub.Modules, &sub.Status, &sub.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// ListByCustomerWithModule returns subscriptions that include a specific module.
func (s *SubscriptionService) ListByCustomerWithModule(ctx context.Context, customerID, module string) ([]model.Subscription, error) {
	rows, err := s.db.Query(ctx,
		`SELECT cs.id, cs.customer_id, cs.tenant_id, p.name, p.description, p.modules, cs.status, cs.updated_at
		 FROM customer_subscriptions cs
		 JOIN products p ON p.id = cs.product_id
		 WHERE cs.customer_id = $1 AND $2 = ANY(p.modules) AND cs.status = 'active'
		 ORDER BY p.name`, customerID, module)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions with module %s: %w", module, err)
	}
	defer rows.Close()

	var subs []model.Subscription
	for rows.Next() {
		var sub model.Subscription
		if err := rows.Scan(&sub.ID, &sub.CustomerID, &sub.TenantID, &sub.ProductName,
			&sub.ProductDescription, &sub.Modules, &sub.Status, &sub.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}
