package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type EmailAutoReplyService struct {
	db DB
	tc temporalclient.Client
}

func NewEmailAutoReplyService(db DB, tc temporalclient.Client) *EmailAutoReplyService {
	return &EmailAutoReplyService{db: db, tc: tc}
}

// Upsert creates or updates the auto-reply for an email account and starts
// the UpdateEmailAutoReplyWorkflow.
func (s *EmailAutoReplyService) Upsert(ctx context.Context, ar *model.EmailAutoReply) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO email_autoreplies (id, email_account_id, subject, body, start_date, end_date, enabled, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 ON CONFLICT (email_account_id) DO UPDATE SET
		   subject = EXCLUDED.subject,
		   body = EXCLUDED.body,
		   start_date = EXCLUDED.start_date,
		   end_date = EXCLUDED.end_date,
		   enabled = EXCLUDED.enabled,
		   status = EXCLUDED.status,
		   updated_at = EXCLUDED.updated_at`,
		ar.ID, ar.EmailAccountID, ar.Subject, ar.Body, ar.StartDate, ar.EndDate, ar.Enabled, ar.Status, ar.CreatedAt, ar.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert email autoreply: %w", err)
	}

	// Fetch the actual row (in case of conflict, the ID may differ).
	var actualID string
	err = s.db.QueryRow(ctx,
		`SELECT id FROM email_autoreplies WHERE email_account_id = $1`, ar.EmailAccountID,
	).Scan(&actualID)
	if err != nil {
		return fmt.Errorf("get autoreply id after upsert: %w", err)
	}
	ar.ID = actualID

	tenantID, err := resolveTenantIDFromEmailAccount(ctx, s.db, ar.EmailAccountID)
	if err != nil {
		return fmt.Errorf("upsert email autoreply: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "UpdateEmailAutoReplyWorkflow",
		WorkflowID:   workflowID("email-autoreply", ar.Subject, actualID),
		Arg:          actualID,
	}); err != nil {
		return fmt.Errorf("start UpdateEmailAutoReplyWorkflow: %w", err)
	}

	return nil
}

func (s *EmailAutoReplyService) GetByAccountID(ctx context.Context, accountID string) (*model.EmailAutoReply, error) {
	var ar model.EmailAutoReply
	err := s.db.QueryRow(ctx,
		`SELECT id, email_account_id, subject, body, start_date, end_date, enabled, status, status_message, created_at, updated_at
		 FROM email_autoreplies WHERE email_account_id = $1`, accountID,
	).Scan(&ar.ID, &ar.EmailAccountID, &ar.Subject, &ar.Body, &ar.StartDate, &ar.EndDate, &ar.Enabled, &ar.Status, &ar.StatusMessage, &ar.CreatedAt, &ar.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get email autoreply for account %s: %w", accountID, err)
	}
	return &ar, nil
}

func (s *EmailAutoReplyService) Delete(ctx context.Context, accountID string) error {
	var id, subject string
	err := s.db.QueryRow(ctx,
		`SELECT id, subject FROM email_autoreplies WHERE email_account_id = $1`, accountID,
	).Scan(&id, &subject)
	if err != nil {
		return fmt.Errorf("find email autoreply for account %s: %w", accountID, err)
	}

	_, err = s.db.Exec(ctx,
		"UPDATE email_autoreplies SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusDeleting, id,
	)
	if err != nil {
		return fmt.Errorf("set email autoreply %s status to deleting: %w", id, err)
	}

	tenantID, err := resolveTenantIDFromEmailAccount(ctx, s.db, accountID)
	if err != nil {
		return fmt.Errorf("delete email autoreply: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteEmailAutoReplyWorkflow",
		WorkflowID:   workflowID("email-autoreply", subject, id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("start DeleteEmailAutoReplyWorkflow: %w", err)
	}

	return nil
}

func (s *EmailAutoReplyService) Retry(ctx context.Context, id string) error {
	var status, accountID, subject string
	err := s.db.QueryRow(ctx, "SELECT status, email_account_id, subject FROM email_autoreplies WHERE id = $1", id).Scan(&status, &accountID, &subject)
	if err != nil {
		return fmt.Errorf("get email autoreply status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("email autoreply %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE email_autoreplies SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set email autoreply %s status to provisioning: %w", id, err)
	}
	tenantID, err := resolveTenantIDFromEmailAccount(ctx, s.db, accountID)
	if err != nil {
		return fmt.Errorf("retry email autoreply: %w", err)
	}
	return signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "UpdateEmailAutoReplyWorkflow",
		WorkflowID:   workflowID("email-autoreply", subject, id),
		Arg:          id,
	})
}
