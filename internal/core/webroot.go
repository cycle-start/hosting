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
		`INSERT INTO webroots (id, tenant_id, subscription_id, runtime, runtime_version, runtime_config, public_folder, env_file_name, service_hostname_enabled, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		webroot.ID, webroot.TenantID, webroot.SubscriptionID, webroot.Runtime, webroot.RuntimeVersion,
		webroot.RuntimeConfig, webroot.PublicFolder, webroot.EnvFileName,
		webroot.ServiceHostnameEnabled, webroot.Status, webroot.CreatedAt, webroot.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert webroot: %w", err)
	}

	if err := signalProvision(ctx, s.tc, s.db, webroot.TenantID, model.ProvisionTask{
		WorkflowName: "CreateWebrootWorkflow",
		WorkflowID:   fmt.Sprintf("create-webroot-%s", webroot.ID),
		Arg:          webroot.ID,
	}); err != nil {
		return fmt.Errorf("signal CreateWebrootWorkflow: %w", err)
	}

	return nil
}

func (s *WebrootService) GetByID(ctx context.Context, id string) (*model.Webroot, error) {
	var w model.Webroot
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, subscription_id, runtime, runtime_version, runtime_config, public_folder, env_file_name, service_hostname_enabled, status, status_message, suspend_reason, created_at, updated_at
		 FROM webroots WHERE id = $1`, id,
	).Scan(&w.ID, &w.TenantID, &w.SubscriptionID, &w.Runtime, &w.RuntimeVersion,
		&w.RuntimeConfig, &w.PublicFolder, &w.EnvFileName,
		&w.ServiceHostnameEnabled, &w.Status, &w.StatusMessage, &w.SuspendReason, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get webroot %s: %w", id, err)
	}
	return &w, nil
}

func (s *WebrootService) ListByTenant(ctx context.Context, tenantID string, limit int, cursor string) ([]model.Webroot, bool, error) {
	query := `SELECT id, tenant_id, subscription_id, runtime, runtime_version, runtime_config, public_folder, env_file_name, service_hostname_enabled, status, status_message, suspend_reason, created_at, updated_at FROM webroots WHERE tenant_id = $1`
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
		if err := rows.Scan(&w.ID, &w.TenantID, &w.SubscriptionID, &w.Runtime, &w.RuntimeVersion,
			&w.RuntimeConfig, &w.PublicFolder, &w.EnvFileName,
			&w.ServiceHostnameEnabled, &w.Status, &w.StatusMessage, &w.SuspendReason, &w.CreatedAt, &w.UpdatedAt); err != nil {
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
		`UPDATE webroots SET runtime = $1, runtime_version = $2, runtime_config = $3,
		 public_folder = $4, env_file_name = $5, service_hostname_enabled = $6, status = $7, updated_at = now() WHERE id = $8`,
		webroot.Runtime, webroot.RuntimeVersion, webroot.RuntimeConfig,
		webroot.PublicFolder, webroot.EnvFileName, webroot.ServiceHostnameEnabled, webroot.Status, webroot.ID,
	)
	if err != nil {
		return fmt.Errorf("update webroot %s: %w", webroot.ID, err)
	}

	if err := signalProvision(ctx, s.tc, s.db, webroot.TenantID, model.ProvisionTask{
		WorkflowName: "UpdateWebrootWorkflow",
		WorkflowID:   workflowID("webroot", webroot.ID),
		Arg:          webroot.ID,
	}); err != nil {
		return fmt.Errorf("signal UpdateWebrootWorkflow: %w", err)
	}

	return nil
}

func (s *WebrootService) Delete(ctx context.Context, id string) error {
	var tenantID string
	err := s.db.QueryRow(ctx,
		"UPDATE webroots SET status = $1, updated_at = now() WHERE id = $2 RETURNING tenant_id",
		model.StatusDeleting, id,
	).Scan(&tenantID)
	if err != nil {
		return fmt.Errorf("set webroot %s status to deleting: %w", id, err)
	}

	if err := signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteWebrootWorkflow",
		WorkflowID:   workflowID("webroot", id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("signal DeleteWebrootWorkflow: %w", err)
	}

	return nil
}

func (s *WebrootService) Retry(ctx context.Context, id string) error {
	var status, tenantID string
	err := s.db.QueryRow(ctx, "SELECT status, tenant_id FROM webroots WHERE id = $1", id).Scan(&status, &tenantID)
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
	return signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "CreateWebrootWorkflow",
		WorkflowID:   workflowID("webroot", id),
		Arg:          id,
	})
}
