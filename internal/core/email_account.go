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

	tenantID, err := resolveTenantIDFromFQDN(ctx, s.db, a.FQDNID)
	if err != nil {
		return fmt.Errorf("create email account: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "CreateEmailAccountWorkflow",
		WorkflowID:   workflowID("email-account", a.Address, a.ID),
		Arg:          a.ID,
		ResourceType: "email-account",
		ResourceID:   a.ID,
	}); err != nil {
		return fmt.Errorf("start CreateEmailAccountWorkflow: %w", err)
	}

	return nil
}

func (s *EmailAccountService) GetByID(ctx context.Context, id string) (*model.EmailAccount, error) {
	var a model.EmailAccount
	err := s.db.QueryRow(ctx,
		`SELECT id, fqdn_id, address, display_name, quota_bytes, status, status_message, created_at, updated_at
		 FROM email_accounts WHERE id = $1`, id,
	).Scan(&a.ID, &a.FQDNID, &a.Address, &a.DisplayName, &a.QuotaBytes, &a.Status, &a.StatusMessage, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get email account %s: %w", id, err)
	}
	return &a, nil
}

func (s *EmailAccountService) ListByFQDN(ctx context.Context, fqdnID string, limit int, cursor string) ([]model.EmailAccount, bool, error) {
	query := `SELECT id, fqdn_id, address, display_name, quota_bytes, status, status_message, created_at, updated_at FROM email_accounts WHERE fqdn_id = $1`
	args := []any{fqdnID}
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
		return nil, false, fmt.Errorf("list email accounts for fqdn %s: %w", fqdnID, err)
	}
	defer rows.Close()

	var accounts []model.EmailAccount
	for rows.Next() {
		var a model.EmailAccount
		if err := rows.Scan(&a.ID, &a.FQDNID, &a.Address, &a.DisplayName, &a.QuotaBytes, &a.Status, &a.StatusMessage, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan email account: %w", err)
		}
		accounts = append(accounts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate email accounts: %w", err)
	}

	hasMore := len(accounts) > limit
	if hasMore {
		accounts = accounts[:limit]
	}
	return accounts, hasMore, nil
}

func (s *EmailAccountService) ListByTenant(ctx context.Context, tenantID string, limit int, cursor string) ([]model.EmailAccount, bool, error) {
	query := `SELECT ea.id, ea.fqdn_id, ea.address, ea.display_name, ea.quota_bytes, ea.status, ea.status_message, ea.created_at, ea.updated_at
		 FROM email_accounts ea
		 JOIN fqdns f ON ea.fqdn_id = f.id
		 JOIN webroots w ON f.webroot_id = w.id
		 WHERE w.tenant_id = $1`
	args := []any{tenantID}
	argIdx := 2

	if cursor != "" {
		query += fmt.Sprintf(` AND ea.id > $%d`, argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query += ` ORDER BY ea.id`
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list email accounts for tenant %s: %w", tenantID, err)
	}
	defer rows.Close()

	var accounts []model.EmailAccount
	for rows.Next() {
		var a model.EmailAccount
		if err := rows.Scan(&a.ID, &a.FQDNID, &a.Address, &a.DisplayName, &a.QuotaBytes, &a.Status, &a.StatusMessage, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan email account: %w", err)
		}
		accounts = append(accounts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate email accounts: %w", err)
	}

	hasMore := len(accounts) > limit
	if hasMore {
		accounts = accounts[:limit]
	}
	return accounts, hasMore, nil
}

func (s *EmailAccountService) Delete(ctx context.Context, id string) error {
	var address string
	err := s.db.QueryRow(ctx,
		"UPDATE email_accounts SET status = $1, updated_at = now() WHERE id = $2 RETURNING address",
		model.StatusDeleting, id,
	).Scan(&address)
	if err != nil {
		return fmt.Errorf("set email account %s status to deleting: %w", id, err)
	}

	tenantID, err := resolveTenantIDFromEmailAccount(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("delete email account: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteEmailAccountWorkflow",
		WorkflowID:   workflowID("email-account", address, id),
		Arg:          id,
		ResourceType: "email-account",
		ResourceID:   id,
	}); err != nil {
		return fmt.Errorf("start DeleteEmailAccountWorkflow: %w", err)
	}

	return nil
}

func (s *EmailAccountService) Retry(ctx context.Context, id string) error {
	var status, address string
	err := s.db.QueryRow(ctx, "SELECT status, address FROM email_accounts WHERE id = $1", id).Scan(&status, &address)
	if err != nil {
		return fmt.Errorf("get email account status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("email account %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE email_accounts SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set email account %s status to provisioning: %w", id, err)
	}
	tenantID, err := resolveTenantIDFromEmailAccount(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("retry email account: %w", err)
	}
	return signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "CreateEmailAccountWorkflow",
		WorkflowID:   workflowID("email-account", address, id),
		Arg:          id,
		ResourceType: "email-account",
		ResourceID:   id,
	})
}
