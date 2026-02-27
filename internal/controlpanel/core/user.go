package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/controlpanel/model"
)

type UserService struct {
	db DB
}

func NewUserService(db DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) GetByID(ctx context.Context, id string) (*model.User, error) {
	var u model.User
	err := s.db.QueryRow(ctx,
		`SELECT id, partner_id, email, password_hash, display_name, locale, last_customer_id, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.PartnerID, &u.Email, &u.PasswordHash,
		&u.DisplayName, &u.Locale, &u.LastCustomerID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user %s: %w", id, err)
	}
	return &u, nil
}

func (s *UserService) UpdateLocale(ctx context.Context, id, locale string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE users SET locale = $1, updated_at = now() WHERE id = $2`, locale, id)
	if err != nil {
		return fmt.Errorf("update user locale: %w", err)
	}
	return nil
}

func (s *UserService) UpdateLastCustomer(ctx context.Context, id, customerID string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE users SET last_customer_id = $1, updated_at = now() WHERE id = $2`, customerID, id)
	if err != nil {
		return fmt.Errorf("update user last customer: %w", err)
	}
	return nil
}

func (s *UserService) UpdateDisplayName(ctx context.Context, id string, name *string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE users SET display_name = $1, updated_at = now() WHERE id = $2`, name, id)
	if err != nil {
		return fmt.Errorf("update user display name: %w", err)
	}
	return nil
}
