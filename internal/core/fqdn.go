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
		`INSERT INTO fqdns (id, fqdn, webroot_id, ssl_enabled, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		fqdn.ID, fqdn.FQDN, fqdn.WebrootID, fqdn.SSLEnabled, fqdn.Status,
		fqdn.CreatedAt, fqdn.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert fqdn: %w", err)
	}

	tenantID, err := resolveTenantIDFromWebroot(ctx, s.db, fqdn.WebrootID)
	if err != nil {
		return fmt.Errorf("create fqdn: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "BindFQDNWorkflow",
		WorkflowID:   fmt.Sprintf("fqdn-%s", fqdn.ID),
		Arg:          fqdn.ID,
	}); err != nil {
		return fmt.Errorf("start BindFQDNWorkflow: %w", err)
	}

	return nil
}

func (s *FQDNService) GetByID(ctx context.Context, id string) (*model.FQDN, error) {
	var f model.FQDN
	err := s.db.QueryRow(ctx,
		`SELECT id, fqdn, webroot_id, ssl_enabled, status, created_at, updated_at
		 FROM fqdns WHERE id = $1`, id,
	).Scan(&f.ID, &f.FQDN, &f.WebrootID, &f.SSLEnabled, &f.Status,
		&f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get fqdn %s: %w", id, err)
	}
	return &f, nil
}

func (s *FQDNService) ListByWebroot(ctx context.Context, webrootID string, limit int, cursor string) ([]model.FQDN, bool, error) {
	query := `SELECT id, fqdn, webroot_id, ssl_enabled, status, created_at, updated_at FROM fqdns WHERE webroot_id = $1`
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
		if err := rows.Scan(&f.ID, &f.FQDN, &f.WebrootID, &f.SSLEnabled, &f.Status,
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

func (s *FQDNService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE fqdns SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusDeleting, id,
	)
	if err != nil {
		return fmt.Errorf("set fqdn %s status to deleting: %w", id, err)
	}

	tenantID, err := resolveTenantIDFromFQDN(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("delete fqdn: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "UnbindFQDNWorkflow",
		WorkflowID:   fmt.Sprintf("fqdn-%s", id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("start UnbindFQDNWorkflow: %w", err)
	}

	return nil
}
