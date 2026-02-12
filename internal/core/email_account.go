package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type EmailAccountService struct {
	db DB
	tc temporalclient.Client
}

func NewEmailAccountService(db DB, tc temporalclient.Client) *EmailAccountService {
	return &EmailAccountService{db: db, tc: tc}
}

func (s *EmailAccountService) Create(ctx context.Context, a *model.EmailAccount) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO email_accounts (id, fqdn_id, address, display_name, quota_bytes, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		a.ID, a.FQDNID, a.Address, a.DisplayName, a.QuotaBytes, a.Status, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert email account: %w", err)
	}

	workflowID := fmt.Sprintf("email-account-%s", a.ID)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "CreateEmailAccountWorkflow", a.ID)
	if err != nil {
		return fmt.Errorf("start CreateEmailAccountWorkflow: %w", err)
	}

	return nil
}

func (s *EmailAccountService) GetByID(ctx context.Context, id string) (*model.EmailAccount, error) {
	var a model.EmailAccount
	err := s.db.QueryRow(ctx,
		`SELECT id, fqdn_id, address, display_name, quota_bytes, status, created_at, updated_at
		 FROM email_accounts WHERE id = $1`, id,
	).Scan(&a.ID, &a.FQDNID, &a.Address, &a.DisplayName, &a.QuotaBytes, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get email account %s: %w", id, err)
	}
	return &a, nil
}

func (s *EmailAccountService) ListByFQDN(ctx context.Context, fqdnID string) ([]model.EmailAccount, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, fqdn_id, address, display_name, quota_bytes, status, created_at, updated_at
		 FROM email_accounts WHERE fqdn_id = $1 ORDER BY address`, fqdnID,
	)
	if err != nil {
		return nil, fmt.Errorf("list email accounts for fqdn %s: %w", fqdnID, err)
	}
	defer rows.Close()

	var accounts []model.EmailAccount
	for rows.Next() {
		var a model.EmailAccount
		if err := rows.Scan(&a.ID, &a.FQDNID, &a.Address, &a.DisplayName, &a.QuotaBytes, &a.Status, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan email account: %w", err)
		}
		accounts = append(accounts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate email accounts: %w", err)
	}
	return accounts, nil
}

func (s *EmailAccountService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE email_accounts SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusDeleting, id,
	)
	if err != nil {
		return fmt.Errorf("set email account %s status to deleting: %w", id, err)
	}

	workflowID := fmt.Sprintf("email-account-%s", id)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "DeleteEmailAccountWorkflow", id)
	if err != nil {
		return fmt.Errorf("start DeleteEmailAccountWorkflow: %w", err)
	}

	return nil
}
