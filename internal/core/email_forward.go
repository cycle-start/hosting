package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type EmailForwardService struct {
	db DB
	tc temporalclient.Client
}

func NewEmailForwardService(db DB, tc temporalclient.Client) *EmailForwardService {
	return &EmailForwardService{db: db, tc: tc}
}

func (s *EmailForwardService) Create(ctx context.Context, f *model.EmailForward) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO email_forwards (id, email_account_id, destination, keep_copy, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		f.ID, f.EmailAccountID, f.Destination, f.KeepCopy, f.Status, f.CreatedAt, f.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert email forward: %w", err)
	}

	tenantID, err := resolveTenantIDFromEmailAccount(ctx, s.db, f.EmailAccountID)
	if err != nil {
		return fmt.Errorf("create email forward: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "CreateEmailForwardWorkflow",
		WorkflowID:   workflowID("email-forward", f.Destination, f.ID),
		Arg:          f.ID,
	}); err != nil {
		return fmt.Errorf("start CreateEmailForwardWorkflow: %w", err)
	}

	return nil
}

func (s *EmailForwardService) GetByID(ctx context.Context, id string) (*model.EmailForward, error) {
	var f model.EmailForward
	err := s.db.QueryRow(ctx,
		`SELECT id, email_account_id, destination, keep_copy, status, status_message, created_at, updated_at
		 FROM email_forwards WHERE id = $1`, id,
	).Scan(&f.ID, &f.EmailAccountID, &f.Destination, &f.KeepCopy, &f.Status, &f.StatusMessage, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get email forward %s: %w", id, err)
	}
	return &f, nil
}

func (s *EmailForwardService) ListByAccountID(ctx context.Context, accountID string, limit int, cursor string) ([]model.EmailForward, bool, error) {
	query := `SELECT id, email_account_id, destination, keep_copy, status, status_message, created_at, updated_at FROM email_forwards WHERE email_account_id = $1`
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
		return nil, false, fmt.Errorf("list email forwards for account %s: %w", accountID, err)
	}
	defer rows.Close()

	var forwards []model.EmailForward
	for rows.Next() {
		var f model.EmailForward
		if err := rows.Scan(&f.ID, &f.EmailAccountID, &f.Destination, &f.KeepCopy, &f.Status, &f.StatusMessage, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan email forward: %w", err)
		}
		forwards = append(forwards, f)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate email forwards: %w", err)
	}

	hasMore := len(forwards) > limit
	if hasMore {
		forwards = forwards[:limit]
	}
	return forwards, hasMore, nil
}

func (s *EmailForwardService) Delete(ctx context.Context, id string) error {
	var destination string
	err := s.db.QueryRow(ctx,
		"UPDATE email_forwards SET status = $1, updated_at = now() WHERE id = $2 RETURNING destination",
		model.StatusDeleting, id,
	).Scan(&destination)
	if err != nil {
		return fmt.Errorf("set email forward %s status to deleting: %w", id, err)
	}

	tenantID, err := resolveTenantIDFromEmailForward(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("delete email forward: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteEmailForwardWorkflow",
		WorkflowID:   workflowID("email-forward", destination, id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("start DeleteEmailForwardWorkflow: %w", err)
	}

	return nil
}

func (s *EmailForwardService) Retry(ctx context.Context, id string) error {
	var status, destination string
	err := s.db.QueryRow(ctx, "SELECT status, destination FROM email_forwards WHERE id = $1", id).Scan(&status, &destination)
	if err != nil {
		return fmt.Errorf("get email forward status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("email forward %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE email_forwards SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set email forward %s status to provisioning: %w", id, err)
	}
	tenantID, err := resolveTenantIDFromEmailForward(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("retry email forward: %w", err)
	}
	return signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "CreateEmailForwardWorkflow",
		WorkflowID:   workflowID("email-forward", destination, id),
		Arg:          id,
	})
}
