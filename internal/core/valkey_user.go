package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type ValkeyUserService struct {
	db DB
	tc temporalclient.Client
}

func NewValkeyUserService(db DB, tc temporalclient.Client) *ValkeyUserService {
	return &ValkeyUserService{db: db, tc: tc}
}

func (s *ValkeyUserService) Create(ctx context.Context, user *model.ValkeyUser) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO valkey_users (id, valkey_instance_id, username, password, privileges, key_pattern, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		user.ID, user.ValkeyInstanceID, user.Username, user.Password,
		user.Privileges, user.KeyPattern, user.Status, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert valkey user: %w", err)
	}

	tenantID, err := resolveTenantIDFromValkeyInstance(ctx, s.db, user.ValkeyInstanceID)
	if err != nil {
		return fmt.Errorf("resolve tenant for valkey user: %w", err)
	}
	if err := signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "CreateValkeyUserWorkflow",
		WorkflowID:   fmt.Sprintf("create-valkey-user-%s", user.ID),
		Arg:          user.ID,
	}); err != nil {
		return fmt.Errorf("signal CreateValkeyUserWorkflow: %w", err)
	}

	return nil
}

func (s *ValkeyUserService) GetByID(ctx context.Context, id string) (*model.ValkeyUser, error) {
	var u model.ValkeyUser
	err := s.db.QueryRow(ctx,
		`SELECT id, valkey_instance_id, username, password, privileges, key_pattern, status, status_message, created_at, updated_at
		 FROM valkey_users WHERE id = $1`, id,
	).Scan(&u.ID, &u.ValkeyInstanceID, &u.Username, &u.Password,
		&u.Privileges, &u.KeyPattern, &u.Status, &u.StatusMessage, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get valkey user %s: %w", id, err)
	}
	return &u, nil
}

func (s *ValkeyUserService) ListByInstance(ctx context.Context, instanceID string, limit int, cursor string) ([]model.ValkeyUser, bool, error) {
	query := `SELECT id, valkey_instance_id, username, password, privileges, key_pattern, status, status_message, created_at, updated_at FROM valkey_users WHERE valkey_instance_id = $1`
	args := []any{instanceID}
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
		return nil, false, fmt.Errorf("list valkey users for instance %s: %w", instanceID, err)
	}
	defer rows.Close()

	var users []model.ValkeyUser
	for rows.Next() {
		var u model.ValkeyUser
		if err := rows.Scan(&u.ID, &u.ValkeyInstanceID, &u.Username, &u.Password,
			&u.Privileges, &u.KeyPattern, &u.Status, &u.StatusMessage, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan valkey user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate valkey users: %w", err)
	}

	hasMore := len(users) > limit
	if hasMore {
		users = users[:limit]
	}
	return users, hasMore, nil
}

func (s *ValkeyUserService) Update(ctx context.Context, user *model.ValkeyUser) error {
	_, err := s.db.Exec(ctx,
		`UPDATE valkey_users SET username = $1, password = $2, privileges = $3, key_pattern = $4, updated_at = now()
		 WHERE id = $5`,
		user.Username, user.Password, user.Privileges, user.KeyPattern, user.ID,
	)
	if err != nil {
		return fmt.Errorf("update valkey user %s: %w", user.ID, err)
	}

	tenantID, err := resolveTenantIDFromValkeyUser(ctx, s.db, user.ID)
	if err != nil {
		return fmt.Errorf("resolve tenant for valkey user: %w", err)
	}
	if err := signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "UpdateValkeyUserWorkflow",
		WorkflowID:   workflowID("valkey-user", user.ID),
		Arg:          user.ID,
	}); err != nil {
		return fmt.Errorf("signal UpdateValkeyUserWorkflow: %w", err)
	}

	return nil
}

func (s *ValkeyUserService) Delete(ctx context.Context, id string) error {
	var username string
	err := s.db.QueryRow(ctx,
		"UPDATE valkey_users SET status = $1, updated_at = now() WHERE id = $2 RETURNING username",
		model.StatusDeleting, id,
	).Scan(&username)
	if err != nil {
		return fmt.Errorf("set valkey user %s status to deleting: %w", id, err)
	}

	tenantID, err := resolveTenantIDFromValkeyUser(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("resolve tenant for valkey user: %w", err)
	}
	if err := signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteValkeyUserWorkflow",
		WorkflowID:   workflowID("valkey-user", id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("signal DeleteValkeyUserWorkflow: %w", err)
	}

	return nil
}

func (s *ValkeyUserService) Retry(ctx context.Context, id string) error {
	var status, username string
	err := s.db.QueryRow(ctx, "SELECT status, username FROM valkey_users WHERE id = $1", id).Scan(&status, &username)
	if err != nil {
		return fmt.Errorf("get valkey user status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("valkey user %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE valkey_users SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set valkey user %s status to provisioning: %w", id, err)
	}
	tenantID, err := resolveTenantIDFromValkeyUser(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("resolve tenant for valkey user: %w", err)
	}
	return signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "CreateValkeyUserWorkflow",
		WorkflowID:   workflowID("valkey-user", id),
		Arg:          id,
	})
}
