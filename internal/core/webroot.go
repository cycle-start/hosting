package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type WebrootService struct {
	db DB
	tc temporalclient.Client
}

func NewWebrootService(db DB, tc temporalclient.Client) *WebrootService {
	return &WebrootService{db: db, tc: tc}
}

func (s *WebrootService) Create(ctx context.Context, webroot *model.Webroot) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO webroots (id, tenant_id, name, runtime, runtime_version, runtime_config, public_folder, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		webroot.ID, webroot.TenantID, webroot.Name, webroot.Runtime, webroot.RuntimeVersion,
		webroot.RuntimeConfig, webroot.PublicFolder, webroot.Status, webroot.CreatedAt, webroot.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert webroot: %w", err)
	}

	if err := signalProvision(ctx, s.tc, webroot.TenantID, model.ProvisionTask{
		WorkflowName: "CreateWebrootWorkflow",
		WorkflowID:   fmt.Sprintf("webroot-%s", webroot.ID),
		Arg:          webroot.ID,
	}); err != nil {
		return fmt.Errorf("start CreateWebrootWorkflow: %w", err)
	}

	return nil
}

func (s *WebrootService) GetByID(ctx context.Context, id string) (*model.Webroot, error) {
	var w model.Webroot
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, runtime, runtime_version, runtime_config, public_folder, status, status_message, created_at, updated_at
		 FROM webroots WHERE id = $1`, id,
	).Scan(&w.ID, &w.TenantID, &w.Name, &w.Runtime, &w.RuntimeVersion,
		&w.RuntimeConfig, &w.PublicFolder, &w.Status, &w.StatusMessage, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get webroot %s: %w", id, err)
	}
	return &w, nil
}

func (s *WebrootService) ListByTenant(ctx context.Context, tenantID string, limit int, cursor string) ([]model.Webroot, bool, error) {
	query := `SELECT id, tenant_id, name, runtime, runtime_version, runtime_config, public_folder, status, status_message, created_at, updated_at FROM webroots WHERE tenant_id = $1`
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
		return nil, false, fmt.Errorf("list webroots for tenant %s: %w", tenantID, err)
	}
	defer rows.Close()

	var webroots []model.Webroot
	for rows.Next() {
		var w model.Webroot
		if err := rows.Scan(&w.ID, &w.TenantID, &w.Name, &w.Runtime, &w.RuntimeVersion,
			&w.RuntimeConfig, &w.PublicFolder, &w.Status, &w.StatusMessage, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan webroot: %w", err)
		}
		webroots = append(webroots, w)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate webroots: %w", err)
	}

	hasMore := len(webroots) > limit
	if hasMore {
		webroots = webroots[:limit]
	}
	return webroots, hasMore, nil
}

func (s *WebrootService) Update(ctx context.Context, webroot *model.Webroot) error {
	_, err := s.db.Exec(ctx,
		`UPDATE webroots SET name = $1, runtime = $2, runtime_version = $3, runtime_config = $4,
		 public_folder = $5, status = $6, updated_at = now() WHERE id = $7`,
		webroot.Name, webroot.Runtime, webroot.RuntimeVersion, webroot.RuntimeConfig,
		webroot.PublicFolder, webroot.Status, webroot.ID,
	)
	if err != nil {
		return fmt.Errorf("update webroot %s: %w", webroot.ID, err)
	}

	if err := signalProvision(ctx, s.tc, webroot.TenantID, model.ProvisionTask{
		WorkflowName: "UpdateWebrootWorkflow",
		WorkflowID:   fmt.Sprintf("webroot-%s", webroot.ID),
		Arg:          webroot.ID,
	}); err != nil {
		return fmt.Errorf("start UpdateWebrootWorkflow: %w", err)
	}

	return nil
}

func (s *WebrootService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE webroots SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusDeleting, id,
	)
	if err != nil {
		return fmt.Errorf("set webroot %s status to deleting: %w", id, err)
	}

	tenantID, err := resolveTenantIDFromWebroot(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("delete webroot: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteWebrootWorkflow",
		WorkflowID:   fmt.Sprintf("webroot-%s", id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("start DeleteWebrootWorkflow: %w", err)
	}

	return nil
}

func (s *WebrootService) Retry(ctx context.Context, id string) error {
	var status string
	err := s.db.QueryRow(ctx, "SELECT status FROM webroots WHERE id = $1", id).Scan(&status)
	if err != nil {
		return fmt.Errorf("get webroot status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("webroot %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE webroots SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set webroot %s status to provisioning: %w", id, err)
	}
	tenantID, err := resolveTenantIDFromWebroot(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("retry webroot: %w", err)
	}
	return signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "CreateWebrootWorkflow",
		WorkflowID:   fmt.Sprintf("webroot-%s", id),
		Arg:          id,
	})
}
