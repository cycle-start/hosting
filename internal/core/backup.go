package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type BackupService struct {
	db DB
	tc temporalclient.Client
}

func NewBackupService(db DB, tc temporalclient.Client) *BackupService {
	return &BackupService{db: db, tc: tc}
}

func (s *BackupService) Create(ctx context.Context, backup *model.Backup) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO backups (id, tenant_id, type, source_id, source_name, storage_path, size_bytes, status, started_at, completed_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		backup.ID, backup.TenantID, backup.Type, backup.SourceID, backup.SourceName,
		backup.StoragePath, backup.SizeBytes, backup.Status, backup.StartedAt,
		backup.CompletedAt, backup.CreatedAt, backup.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert backup: %w", err)
	}

	if err := signalProvision(ctx, s.tc, s.db, backup.TenantID, model.ProvisionTask{
		WorkflowName: "CreateBackupWorkflow",
		WorkflowID:   fmt.Sprintf("create-backup-%s", backup.ID),
		Arg:          backup.ID,
	}); err != nil {
		return fmt.Errorf("signal CreateBackupWorkflow: %w", err)
	}

	return nil
}

func (s *BackupService) GetByID(ctx context.Context, id string) (*model.Backup, error) {
	var b model.Backup
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, type, source_id, source_name, storage_path, size_bytes, status, status_message, started_at, completed_at, created_at, updated_at
		 FROM backups WHERE id = $1`, id,
	).Scan(&b.ID, &b.TenantID, &b.Type, &b.SourceID, &b.SourceName,
		&b.StoragePath, &b.SizeBytes, &b.Status, &b.StatusMessage, &b.StartedAt,
		&b.CompletedAt, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get backup %s: %w", id, err)
	}
	return &b, nil
}

func (s *BackupService) ListByTenant(ctx context.Context, tenantID string, limit int, cursor string) ([]model.Backup, bool, error) {
	query := `SELECT id, tenant_id, type, source_id, source_name, storage_path, size_bytes, status, status_message, started_at, completed_at, created_at, updated_at FROM backups WHERE tenant_id = $1`
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
		return nil, false, fmt.Errorf("list backups for tenant %s: %w", tenantID, err)
	}
	defer rows.Close()

	var backups []model.Backup
	for rows.Next() {
		var b model.Backup
		if err := rows.Scan(&b.ID, &b.TenantID, &b.Type, &b.SourceID, &b.SourceName,
			&b.StoragePath, &b.SizeBytes, &b.Status, &b.StatusMessage, &b.StartedAt,
			&b.CompletedAt, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan backup: %w", err)
		}
		backups = append(backups, b)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate backups: %w", err)
	}

	hasMore := len(backups) > limit
	if hasMore {
		backups = backups[:limit]
	}
	return backups, hasMore, nil
}

func (s *BackupService) Delete(ctx context.Context, id string) error {
	var name, tenantID string
	err := s.db.QueryRow(ctx,
		"UPDATE backups SET status = $1, updated_at = now() WHERE id = $2 RETURNING type || '/' || source_name, tenant_id",
		model.StatusDeleting, id,
	).Scan(&name, &tenantID)
	if err != nil {
		return fmt.Errorf("set backup %s status to deleting: %w", id, err)
	}

	if err := signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteBackupWorkflow",
		WorkflowID:   workflowID("backup-delete", name, id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("signal DeleteBackupWorkflow: %w", err)
	}

	return nil
}

func (s *BackupService) Restore(ctx context.Context, id string) error {
	// Validate backup exists and is active.
	backup, err := s.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get backup for restore: %w", err)
	}
	if backup.Status != model.StatusActive {
		return fmt.Errorf("backup %s is not active (status: %s)", id, backup.Status)
	}

	if err := signalProvision(ctx, s.tc, s.db, backup.TenantID, model.ProvisionTask{
		WorkflowName: "RestoreBackupWorkflow",
		WorkflowID:   workflowID("backup-restore", backup.Type+"/"+backup.SourceName, id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("signal RestoreBackupWorkflow: %w", err)
	}

	return nil
}

func (s *BackupService) Retry(ctx context.Context, id string) error {
	var status, name, tenantID string
	err := s.db.QueryRow(ctx, "SELECT status, type || '/' || source_name, tenant_id FROM backups WHERE id = $1", id).Scan(&status, &name, &tenantID)
	if err != nil {
		return fmt.Errorf("get backup status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("backup %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE backups SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set backup %s status to provisioning: %w", id, err)
	}
	return signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "CreateBackupWorkflow",
		WorkflowID:   workflowID("backup-create", name, id),
		Arg:          id,
	})
}
