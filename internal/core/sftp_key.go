package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

// SFTPKeyService manages SFTP SSH key operations against the core database.
type SFTPKeyService struct {
	db DB
	tc temporalclient.Client
}

// NewSFTPKeyService creates a new SFTPKeyService.
func NewSFTPKeyService(db DB, tc temporalclient.Client) *SFTPKeyService {
	return &SFTPKeyService{db: db, tc: tc}
}

// Create inserts a new SFTP key and starts the provisioning workflow.
func (s *SFTPKeyService) Create(ctx context.Context, key *model.SFTPKey) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO sftp_keys (id, tenant_id, name, public_key, fingerprint, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		key.ID, key.TenantID, key.Name, key.PublicKey, key.Fingerprint,
		key.Status, key.CreatedAt, key.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert sftp key: %w", err)
	}

	if err := signalProvision(ctx, s.tc, key.TenantID, model.ProvisionTask{
		WorkflowName: "AddSFTPKeyWorkflow",
		WorkflowID:   workflowID("sftp-key", key.Name, key.ID),
		Arg:          key.ID,
	}); err != nil {
		return fmt.Errorf("start AddSFTPKeyWorkflow: %w", err)
	}

	return nil
}

// GetByID retrieves an SFTP key by its ID.
func (s *SFTPKeyService) GetByID(ctx context.Context, id string) (*model.SFTPKey, error) {
	var k model.SFTPKey
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, public_key, fingerprint, status, status_message, created_at, updated_at
		 FROM sftp_keys WHERE id = $1`, id,
	).Scan(&k.ID, &k.TenantID, &k.Name, &k.PublicKey, &k.Fingerprint,
		&k.Status, &k.StatusMessage, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get sftp key %s: %w", id, err)
	}
	return &k, nil
}

// ListByTenant retrieves SFTP keys for a tenant with cursor-based pagination.
func (s *SFTPKeyService) ListByTenant(ctx context.Context, tenantID string, limit int, cursor string) ([]model.SFTPKey, bool, error) {
	query := `SELECT id, tenant_id, name, public_key, fingerprint, status, status_message, created_at, updated_at FROM sftp_keys WHERE tenant_id = $1`
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
		return nil, false, fmt.Errorf("list sftp keys for tenant %s: %w", tenantID, err)
	}
	defer rows.Close()

	var keys []model.SFTPKey
	for rows.Next() {
		var k model.SFTPKey
		if err := rows.Scan(&k.ID, &k.TenantID, &k.Name, &k.PublicKey, &k.Fingerprint,
			&k.Status, &k.StatusMessage, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan sftp key: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate sftp keys: %w", err)
	}

	hasMore := len(keys) > limit
	if hasMore {
		keys = keys[:limit]
	}
	return keys, hasMore, nil
}

// Delete sets the key status to deleting and starts the removal workflow.
func (s *SFTPKeyService) Delete(ctx context.Context, id string) error {
	var name string
	err := s.db.QueryRow(ctx,
		"UPDATE sftp_keys SET status = $1, updated_at = now() WHERE id = $2 RETURNING name",
		model.StatusDeleting, id,
	).Scan(&name)
	if err != nil {
		return fmt.Errorf("set sftp key %s status to deleting: %w", id, err)
	}

	tenantID, err := resolveTenantIDFromSFTPKey(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("delete sftp key: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "RemoveSFTPKeyWorkflow",
		WorkflowID:   workflowID("sftp-key", name, id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("start RemoveSFTPKeyWorkflow: %w", err)
	}

	return nil
}

func (s *SFTPKeyService) Retry(ctx context.Context, id string) error {
	var status, name string
	err := s.db.QueryRow(ctx, "SELECT status, name FROM sftp_keys WHERE id = $1", id).Scan(&status, &name)
	if err != nil {
		return fmt.Errorf("get sftp key status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("sftp key %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE sftp_keys SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set sftp key %s status to provisioning: %w", id, err)
	}
	tenantID, err := resolveTenantIDFromSFTPKey(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("retry sftp key: %w", err)
	}
	return signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "AddSFTPKeyWorkflow",
		WorkflowID:   workflowID("sftp-key", name, id),
		Arg:          id,
	})
}
