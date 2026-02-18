package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

// SSHKeyService manages SSH key operations against the core database.
type SSHKeyService struct {
	db DB
	tc temporalclient.Client
}

// NewSSHKeyService creates a new SSHKeyService.
func NewSSHKeyService(db DB, tc temporalclient.Client) *SSHKeyService {
	return &SSHKeyService{db: db, tc: tc}
}

// Create inserts a new SSH key and starts the provisioning workflow.
func (s *SSHKeyService) Create(ctx context.Context, key *model.SSHKey) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO ssh_keys (id, tenant_id, name, public_key, fingerprint, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		key.ID, key.TenantID, key.Name, key.PublicKey, key.Fingerprint,
		key.Status, key.CreatedAt, key.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert SSH key: %w", err)
	}

	if err := signalProvision(ctx, s.tc, s.db, key.TenantID, model.ProvisionTask{
		WorkflowName: "AddSSHKeyWorkflow",
		WorkflowID:   fmt.Sprintf("create-ssh-key-%s", key.ID),
		Arg:          key.ID,
	}); err != nil {
		return fmt.Errorf("signal AddSSHKeyWorkflow: %w", err)
	}

	return nil
}

// GetByID retrieves an SSH key by its ID.
func (s *SSHKeyService) GetByID(ctx context.Context, id string) (*model.SSHKey, error) {
	var k model.SSHKey
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, public_key, fingerprint, status, status_message, created_at, updated_at
		 FROM ssh_keys WHERE id = $1`, id,
	).Scan(&k.ID, &k.TenantID, &k.Name, &k.PublicKey, &k.Fingerprint,
		&k.Status, &k.StatusMessage, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get SSH key %s: %w", id, err)
	}
	return &k, nil
}

// ListByTenant retrieves SSH keys for a tenant with cursor-based pagination.
func (s *SSHKeyService) ListByTenant(ctx context.Context, tenantID string, limit int, cursor string) ([]model.SSHKey, bool, error) {
	query := `SELECT id, tenant_id, name, public_key, fingerprint, status, status_message, created_at, updated_at FROM ssh_keys WHERE tenant_id = $1`
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
		return nil, false, fmt.Errorf("list SSH keys for tenant %s: %w", tenantID, err)
	}
	defer rows.Close()

	var keys []model.SSHKey
	for rows.Next() {
		var k model.SSHKey
		if err := rows.Scan(&k.ID, &k.TenantID, &k.Name, &k.PublicKey, &k.Fingerprint,
			&k.Status, &k.StatusMessage, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan SSH key: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate SSH keys: %w", err)
	}

	hasMore := len(keys) > limit
	if hasMore {
		keys = keys[:limit]
	}
	return keys, hasMore, nil
}

// Delete sets the key status to deleting and starts the removal workflow.
func (s *SSHKeyService) Delete(ctx context.Context, id string) error {
	var name, tenantID string
	err := s.db.QueryRow(ctx,
		"UPDATE ssh_keys SET status = $1, updated_at = now() WHERE id = $2 RETURNING name, tenant_id",
		model.StatusDeleting, id,
	).Scan(&name, &tenantID)
	if err != nil {
		return fmt.Errorf("set SSH key %s status to deleting: %w", id, err)
	}

	if err := signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "RemoveSSHKeyWorkflow",
		WorkflowID:   workflowID("ssh-key", name, id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("signal RemoveSSHKeyWorkflow: %w", err)
	}

	return nil
}

func (s *SSHKeyService) Retry(ctx context.Context, id string) error {
	var status, name, tenantID string
	err := s.db.QueryRow(ctx, "SELECT status, name, tenant_id FROM ssh_keys WHERE id = $1", id).Scan(&status, &name, &tenantID)
	if err != nil {
		return fmt.Errorf("get SSH key status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("SSH key %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE ssh_keys SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set SSH key %s status to provisioning: %w", id, err)
	}
	return signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "AddSSHKeyWorkflow",
		WorkflowID:   workflowID("ssh-key", name, id),
		Arg:          id,
	})
}
