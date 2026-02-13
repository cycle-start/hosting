package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type DatabaseUserService struct {
	db DB
	tc temporalclient.Client
}

func NewDatabaseUserService(db DB, tc temporalclient.Client) *DatabaseUserService {
	return &DatabaseUserService{db: db, tc: tc}
}

func (s *DatabaseUserService) Create(ctx context.Context, user *model.DatabaseUser) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO database_users (id, database_id, username, password, privileges, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		user.ID, user.DatabaseID, user.Username, user.Password,
		user.Privileges, user.Status, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert database user: %w", err)
	}

	tenantID, err := resolveTenantIDFromDatabase(ctx, s.db, user.DatabaseID)
	if err != nil {
		return fmt.Errorf("create database user: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "CreateDatabaseUserWorkflow",
		WorkflowID:   fmt.Sprintf("database-user-%s", user.ID),
		Arg:          user.ID,
	}); err != nil {
		return fmt.Errorf("start CreateDatabaseUserWorkflow: %w", err)
	}

	return nil
}

func (s *DatabaseUserService) GetByID(ctx context.Context, id string) (*model.DatabaseUser, error) {
	var u model.DatabaseUser
	err := s.db.QueryRow(ctx,
		`SELECT id, database_id, username, password, privileges, status, created_at, updated_at
		 FROM database_users WHERE id = $1`, id,
	).Scan(&u.ID, &u.DatabaseID, &u.Username, &u.Password,
		&u.Privileges, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get database user %s: %w", id, err)
	}
	return &u, nil
}

func (s *DatabaseUserService) ListByDatabase(ctx context.Context, dbID string, limit int, cursor string) ([]model.DatabaseUser, bool, error) {
	query := `SELECT id, database_id, username, password, privileges, status, created_at, updated_at FROM database_users WHERE database_id = $1`
	args := []any{dbID}
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
		return nil, false, fmt.Errorf("list database users for database %s: %w", dbID, err)
	}
	defer rows.Close()

	var users []model.DatabaseUser
	for rows.Next() {
		var u model.DatabaseUser
		if err := rows.Scan(&u.ID, &u.DatabaseID, &u.Username, &u.Password,
			&u.Privileges, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan database user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate database users: %w", err)
	}

	hasMore := len(users) > limit
	if hasMore {
		users = users[:limit]
	}
	return users, hasMore, nil
}

func (s *DatabaseUserService) Update(ctx context.Context, user *model.DatabaseUser) error {
	_, err := s.db.Exec(ctx,
		`UPDATE database_users SET username = $1, password = $2, privileges = $3, updated_at = now()
		 WHERE id = $4`,
		user.Username, user.Password, user.Privileges, user.ID,
	)
	if err != nil {
		return fmt.Errorf("update database user %s: %w", user.ID, err)
	}

	tenantID, err := resolveTenantIDFromDatabase(ctx, s.db, user.DatabaseID)
	if err != nil {
		return fmt.Errorf("update database user: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "UpdateDatabaseUserWorkflow",
		WorkflowID:   fmt.Sprintf("database-user-%s", user.ID),
		Arg:          user.ID,
	}); err != nil {
		return fmt.Errorf("start UpdateDatabaseUserWorkflow: %w", err)
	}

	return nil
}

func (s *DatabaseUserService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE database_users SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusDeleting, id,
	)
	if err != nil {
		return fmt.Errorf("set database user %s status to deleting: %w", id, err)
	}

	tenantID, err := resolveTenantIDFromDatabaseUser(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("delete database user: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteDatabaseUserWorkflow",
		WorkflowID:   fmt.Sprintf("database-user-%s", id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("start DeleteDatabaseUserWorkflow: %w", err)
	}

	return nil
}
