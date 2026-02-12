package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type EmailAliasService struct {
	db DB
	tc temporalclient.Client
}

func NewEmailAliasService(db DB, tc temporalclient.Client) *EmailAliasService {
	return &EmailAliasService{db: db, tc: tc}
}

func (s *EmailAliasService) Create(ctx context.Context, a *model.EmailAlias) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO email_aliases (id, email_account_id, address, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		a.ID, a.EmailAccountID, a.Address, a.Status, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert email alias: %w", err)
	}

	workflowID := fmt.Sprintf("email-alias-%s", a.ID)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "CreateEmailAliasWorkflow", a.ID)
	if err != nil {
		return fmt.Errorf("start CreateEmailAliasWorkflow: %w", err)
	}

	return nil
}

func (s *EmailAliasService) GetByID(ctx context.Context, id string) (*model.EmailAlias, error) {
	var a model.EmailAlias
	err := s.db.QueryRow(ctx,
		`SELECT id, email_account_id, address, status, created_at, updated_at
		 FROM email_aliases WHERE id = $1`, id,
	).Scan(&a.ID, &a.EmailAccountID, &a.Address, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get email alias %s: %w", id, err)
	}
	return &a, nil
}

func (s *EmailAliasService) ListByAccountID(ctx context.Context, accountID string, limit int, cursor string) ([]model.EmailAlias, bool, error) {
	query := `SELECT id, email_account_id, address, status, created_at, updated_at FROM email_aliases WHERE email_account_id = $1`
	args := []any{accountID}
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
		return nil, false, fmt.Errorf("list email aliases for account %s: %w", accountID, err)
	}
	defer rows.Close()

	var aliases []model.EmailAlias
	for rows.Next() {
		var a model.EmailAlias
		if err := rows.Scan(&a.ID, &a.EmailAccountID, &a.Address, &a.Status, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan email alias: %w", err)
		}
		aliases = append(aliases, a)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate email aliases: %w", err)
	}

	hasMore := len(aliases) > limit
	if hasMore {
		aliases = aliases[:limit]
	}
	return aliases, hasMore, nil
}

func (s *EmailAliasService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE email_aliases SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusDeleting, id,
	)
	if err != nil {
		return fmt.Errorf("set email alias %s status to deleting: %w", id, err)
	}

	workflowID := fmt.Sprintf("email-alias-%s", id)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "DeleteEmailAliasWorkflow", id)
	if err != nil {
		return fmt.Errorf("start DeleteEmailAliasWorkflow: %w", err)
	}

	return nil
}
