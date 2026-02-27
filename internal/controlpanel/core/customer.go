package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/controlpanel/model"
)

type CustomerService struct {
	db DB
}

func NewCustomerService(db DB) *CustomerService {
	return &CustomerService{db: db}
}

// ListByUser returns all customers the user has access to.
func (s *CustomerService) ListByUser(ctx context.Context, userID string) ([]model.Customer, error) {
	rows, err := s.db.Query(ctx,
		`SELECT c.id, c.partner_id, c.name, c.email, c.status, c.created_at, c.updated_at
		 FROM customers c
		 JOIN customer_users cu ON cu.customer_id = c.id
		 WHERE cu.user_id = $1
		 ORDER BY c.name`, userID)
	if err != nil {
		return nil, fmt.Errorf("list customers for user %s: %w", userID, err)
	}
	defer rows.Close()

	var customers []model.Customer
	for rows.Next() {
		var c model.Customer
		if err := rows.Scan(&c.ID, &c.PartnerID, &c.Name, &c.Email, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan customer: %w", err)
		}
		customers = append(customers, c)
	}
	return customers, rows.Err()
}

// UserHasAccess checks whether the user has access to the given customer.
func (s *CustomerService) UserHasAccess(ctx context.Context, userID, customerID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM customer_users WHERE user_id = $1 AND customer_id = $2)`,
		userID, customerID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check user access: %w", err)
	}
	return exists, nil
}

// GetCustomerIDByTenant maps a tenant ID to a customer ID via customer_subscriptions.
func (s *CustomerService) GetCustomerIDByTenant(ctx context.Context, tenantID string) (string, error) {
	var customerID string
	err := s.db.QueryRow(ctx,
		`SELECT customer_id FROM customer_subscriptions WHERE tenant_id = $1 LIMIT 1`, tenantID).Scan(&customerID)
	if err != nil {
		return "", fmt.Errorf("get customer for tenant %s: %w", tenantID, err)
	}
	return customerID, nil
}

// GetByID returns a single customer by ID.
func (s *CustomerService) GetByID(ctx context.Context, id string) (model.Customer, error) {
	var c model.Customer
	err := s.db.QueryRow(ctx,
		`SELECT id, partner_id, name, email, status, created_at, updated_at
		 FROM customers WHERE id = $1`, id).Scan(&c.ID, &c.PartnerID, &c.Name, &c.Email, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return c, fmt.Errorf("get customer %s: %w", id, err)
	}
	return c, nil
}

// GetCustomerIDBySubscription maps a subscription ID to a customer ID.
func (s *CustomerService) GetCustomerIDBySubscription(ctx context.Context, subscriptionID string) (string, error) {
	var customerID string
	err := s.db.QueryRow(ctx,
		`SELECT customer_id FROM customer_subscriptions WHERE id = $1`, subscriptionID).Scan(&customerID)
	if err != nil {
		return "", fmt.Errorf("get customer for subscription %s: %w", subscriptionID, err)
	}
	return customerID, nil
}
