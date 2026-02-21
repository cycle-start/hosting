package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type FQDNService struct {
	db DB
	tc temporalclient.Client
}

func NewFQDNService(db DB, tc temporalclient.Client) *FQDNService {
	return &FQDNService{db: db, tc: tc}
}

func (s *FQDNService) Create(ctx context.Context, fqdn *model.FQDN) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO fqdns (id, tenant_id, fqdn, webroot_id, ssl_enabled, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		fqdn.ID, fqdn.TenantID, fqdn.FQDN, fqdn.WebrootID, fqdn.SSLEnabled, fqdn.Status,
		fqdn.CreatedAt, fqdn.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert fqdn: %w", err)
	}

	if err := signalProvision(ctx, s.tc, s.db, fqdn.TenantID, model.ProvisionTask{
		WorkflowName: "BindFQDNWorkflow",
		WorkflowID:   fmt.Sprintf("create-fqdn-%s", fqdn.ID),
		Arg:          fqdn.ID,
	}); err != nil {
		return fmt.Errorf("signal BindFQDNWorkflow: %w", err)
	}

	return nil
}

func (s *FQDNService) GetByID(ctx context.Context, id string) (*model.FQDN, error) {
	var f model.FQDN
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, fqdn, webroot_id, ssl_enabled, status, status_message, created_at, updated_at
		 FROM fqdns WHERE id = $1`, id,
	).Scan(&f.ID, &f.TenantID, &f.FQDN, &f.WebrootID, &f.SSLEnabled, &f.Status, &f.StatusMessage,
		&f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get fqdn %s: %w", id, err)
	}
	return &f, nil
}

func (s *FQDNService) ListByWebroot(ctx context.Context, webrootID string, limit int, cursor string) ([]model.FQDN, bool, error) {
	query := `SELECT id, tenant_id, fqdn, webroot_id, ssl_enabled, status, status_message, created_at, updated_at FROM fqdns WHERE webroot_id = $1`
	args := []any{webrootID}
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
		return nil, false, fmt.Errorf("list fqdns for webroot %s: %w", webrootID, err)
	}
	defer rows.Close()

	var fqdns []model.FQDN
	for rows.Next() {
		var f model.FQDN
		if err := rows.Scan(&f.ID, &f.TenantID, &f.FQDN, &f.WebrootID, &f.SSLEnabled, &f.Status, &f.StatusMessage,
			&f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan fqdn: %w", err)
		}
		fqdns = append(fqdns, f)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate fqdns: %w", err)
	}

	hasMore := len(fqdns) > limit
	if hasMore {
		fqdns = fqdns[:limit]
	}
	return fqdns, hasMore, nil
}

func (s *FQDNService) ListByTenant(ctx context.Context, tenantID string, limit int, cursor string) ([]model.FQDN, bool, error) {
	query := `SELECT id, tenant_id, fqdn, webroot_id, ssl_enabled, status, status_message, created_at, updated_at FROM fqdns WHERE tenant_id = $1`
	args := []any{tenantID}
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
		return nil, false, fmt.Errorf("list fqdns for tenant %s: %w", tenantID, err)
	}
	defer rows.Close()

	var fqdns []model.FQDN
	for rows.Next() {
		var f model.FQDN
		if err := rows.Scan(&f.ID, &f.TenantID, &f.FQDN, &f.WebrootID, &f.SSLEnabled, &f.Status, &f.StatusMessage,
			&f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan fqdn: %w", err)
		}
		fqdns = append(fqdns, f)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate fqdns: %w", err)
	}

	hasMore := len(fqdns) > limit
	if hasMore {
		fqdns = fqdns[:limit]
	}
	return fqdns, hasMore, nil
}

func (s *FQDNService) Update(ctx context.Context, fqdn *model.FQDN) error {
	_, err := s.db.Exec(ctx,
		`UPDATE fqdns SET webroot_id = $1, ssl_enabled = $2, updated_at = now() WHERE id = $3`,
		fqdn.WebrootID, fqdn.SSLEnabled, fqdn.ID,
	)
	if err != nil {
		return fmt.Errorf("update fqdn %s: %w", fqdn.ID, err)
	}
	return nil
}

func (s *FQDNService) Delete(ctx context.Context, id string) error {
	var fqdnName, tenantID string
	err := s.db.QueryRow(ctx,
		"UPDATE fqdns SET status = $1, updated_at = now() WHERE id = $2 RETURNING fqdn, tenant_id",
		model.StatusDeleting, id,
	).Scan(&fqdnName, &tenantID)
	if err != nil {
		return fmt.Errorf("set fqdn %s status to deleting: %w", id, err)
	}

	if err := signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "UnbindFQDNWorkflow",
		WorkflowID:   workflowID("fqdn", fqdnName, id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("signal UnbindFQDNWorkflow: %w", err)
	}

	return nil
}

func (s *FQDNService) Retry(ctx context.Context, id string) error {
	var status, fqdnName, tenantID string
	err := s.db.QueryRow(ctx, "SELECT status, fqdn, tenant_id FROM fqdns WHERE id = $1", id).Scan(&status, &fqdnName, &tenantID)
	if err != nil {
		return fmt.Errorf("get fqdn status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("fqdn %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE fqdns SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set fqdn %s status to provisioning: %w", id, err)
	}
	return signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "CreateFQDNWorkflow",
		WorkflowID:   workflowID("fqdn", fqdnName, id),
		Arg:          id,
	})
}
