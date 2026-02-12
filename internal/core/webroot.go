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

	workflowID := fmt.Sprintf("webroot-%s", webroot.ID)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "CreateWebrootWorkflow", webroot.ID)
	if err != nil {
		return fmt.Errorf("start CreateWebrootWorkflow: %w", err)
	}

	return nil
}

func (s *WebrootService) GetByID(ctx context.Context, id string) (*model.Webroot, error) {
	var w model.Webroot
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, runtime, runtime_version, runtime_config, public_folder, status, created_at, updated_at
		 FROM webroots WHERE id = $1`, id,
	).Scan(&w.ID, &w.TenantID, &w.Name, &w.Runtime, &w.RuntimeVersion,
		&w.RuntimeConfig, &w.PublicFolder, &w.Status, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get webroot %s: %w", id, err)
	}
	return &w, nil
}

func (s *WebrootService) ListByTenant(ctx context.Context, tenantID string) ([]model.Webroot, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, tenant_id, name, runtime, runtime_version, runtime_config, public_folder, status, created_at, updated_at
		 FROM webroots WHERE tenant_id = $1 ORDER BY name`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list webroots for tenant %s: %w", tenantID, err)
	}
	defer rows.Close()

	var webroots []model.Webroot
	for rows.Next() {
		var w model.Webroot
		if err := rows.Scan(&w.ID, &w.TenantID, &w.Name, &w.Runtime, &w.RuntimeVersion,
			&w.RuntimeConfig, &w.PublicFolder, &w.Status, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan webroot: %w", err)
		}
		webroots = append(webroots, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate webroots: %w", err)
	}
	return webroots, nil
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

	workflowID := fmt.Sprintf("webroot-%s", webroot.ID)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "UpdateWebrootWorkflow", webroot.ID)
	if err != nil {
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

	workflowID := fmt.Sprintf("webroot-%s", id)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "DeleteWebrootWorkflow", id)
	if err != nil {
		return fmt.Errorf("start DeleteWebrootWorkflow: %w", err)
	}

	return nil
}
