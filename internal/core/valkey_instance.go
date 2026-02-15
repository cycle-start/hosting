package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

// MigrateValkeyInstanceParams holds parameters for the MigrateValkeyInstanceWorkflow.
type MigrateValkeyInstanceParams struct {
	InstanceID    string `json:"instance_id"`
	TargetShardID string `json:"target_shard_id"`
}

type ValkeyInstanceService struct {
	db DB
	tc temporalclient.Client
}

func NewValkeyInstanceService(db DB, tc temporalclient.Client) *ValkeyInstanceService {
	return &ValkeyInstanceService{db: db, tc: tc}
}

func (s *ValkeyInstanceService) Create(ctx context.Context, instance *model.ValkeyInstance) error {
	// Allocate port: MAX(port) + 1 within shard, starting from 6380.
	var nextPort int
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(port), 6379) + 1 FROM valkey_instances WHERE shard_id = $1`,
		instance.ShardID,
	).Scan(&nextPort)
	if err != nil {
		return fmt.Errorf("allocate valkey port: %w", err)
	}
	instance.Port = nextPort

	_, err = s.db.Exec(ctx,
		`INSERT INTO valkey_instances (id, tenant_id, name, shard_id, port, max_memory_mb, password, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		instance.ID, instance.TenantID, instance.Name, instance.ShardID, instance.Port,
		instance.MaxMemoryMB, instance.Password, instance.Status, instance.CreatedAt, instance.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert valkey instance: %w", err)
	}

	var tenantID string
	if instance.TenantID != nil {
		tenantID = *instance.TenantID
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "CreateValkeyInstanceWorkflow",
		WorkflowID:   workflowID("valkey-instance", instance.Name, instance.ID),
		Arg:          instance.ID,
		ResourceType: "valkey-instance",
		ResourceID:   instance.ID,
	}); err != nil {
		return fmt.Errorf("start CreateValkeyInstanceWorkflow: %w", err)
	}

	return nil
}

func (s *ValkeyInstanceService) GetByID(ctx context.Context, id string) (*model.ValkeyInstance, error) {
	var v model.ValkeyInstance
	err := s.db.QueryRow(ctx,
		`SELECT vi.id, vi.tenant_id, vi.name, vi.shard_id, vi.port, vi.max_memory_mb, vi.password, vi.status, vi.status_message, vi.suspend_reason, vi.created_at, vi.updated_at,
		        s.name
		 FROM valkey_instances vi
		 LEFT JOIN shards s ON s.id = vi.shard_id
		 WHERE vi.id = $1`, id,
	).Scan(&v.ID, &v.TenantID, &v.Name, &v.ShardID, &v.Port, &v.MaxMemoryMB,
		&v.Password, &v.Status, &v.StatusMessage, &v.SuspendReason, &v.CreatedAt, &v.UpdatedAt,
		&v.ShardName)
	if err != nil {
		return nil, fmt.Errorf("get valkey instance %s: %w", id, err)
	}
	return &v, nil
}

func (s *ValkeyInstanceService) ListByTenant(ctx context.Context, tenantID string, limit int, cursor string) ([]model.ValkeyInstance, bool, error) {
	query := `SELECT vi.id, vi.tenant_id, vi.name, vi.shard_id, vi.port, vi.max_memory_mb, vi.password, vi.status, vi.status_message, vi.suspend_reason, vi.created_at, vi.updated_at, s.name FROM valkey_instances vi LEFT JOIN shards s ON s.id = vi.shard_id WHERE vi.tenant_id = $1`
	args := []any{tenantID}
	argIdx := 2

	if cursor != "" {
		query += fmt.Sprintf(` AND vi.id > $%d`, argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query += ` ORDER BY vi.id`
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list valkey instances for tenant %s: %w", tenantID, err)
	}
	defer rows.Close()

	var instances []model.ValkeyInstance
	for rows.Next() {
		var v model.ValkeyInstance
		if err := rows.Scan(&v.ID, &v.TenantID, &v.Name, &v.ShardID, &v.Port, &v.MaxMemoryMB,
			&v.Password, &v.Status, &v.StatusMessage, &v.SuspendReason, &v.CreatedAt, &v.UpdatedAt,
			&v.ShardName); err != nil {
			return nil, false, fmt.Errorf("scan valkey instance: %w", err)
		}
		instances = append(instances, v)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate valkey instances: %w", err)
	}

	hasMore := len(instances) > limit
	if hasMore {
		instances = instances[:limit]
	}
	return instances, hasMore, nil
}

func (s *ValkeyInstanceService) Delete(ctx context.Context, id string) error {
	var name string
	err := s.db.QueryRow(ctx,
		"UPDATE valkey_instances SET status = $1, updated_at = now() WHERE id = $2 RETURNING name",
		model.StatusDeleting, id,
	).Scan(&name)
	if err != nil {
		return fmt.Errorf("set valkey instance %s status to deleting: %w", id, err)
	}

	tenantID, err := resolveTenantIDFromValkeyInstance(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("delete valkey instance: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteValkeyInstanceWorkflow",
		WorkflowID:   workflowID("valkey-instance", name, id),
		Arg:          id,
		ResourceType: "valkey-instance",
		ResourceID:   id,
	}); err != nil {
		return fmt.Errorf("start DeleteValkeyInstanceWorkflow: %w", err)
	}

	return nil
}

func (s *ValkeyInstanceService) Migrate(ctx context.Context, id string, targetShardID string) error {
	var name string
	err := s.db.QueryRow(ctx,
		"UPDATE valkey_instances SET status = $1, updated_at = now() WHERE id = $2 RETURNING name",
		model.StatusProvisioning, id,
	).Scan(&name)
	if err != nil {
		return fmt.Errorf("set valkey instance %s status to provisioning: %w", id, err)
	}

	tenantID, err := resolveTenantIDFromValkeyInstance(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("migrate valkey instance: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "MigrateValkeyInstanceWorkflow",
		WorkflowID:   workflowID("migrate-valkey-instance", name, id),
		Arg: MigrateValkeyInstanceParams{
			InstanceID:    id,
			TargetShardID: targetShardID,
		},
		ResourceType: "valkey-instance",
		ResourceID:   id,
	}); err != nil {
		return fmt.Errorf("start MigrateValkeyInstanceWorkflow: %w", err)
	}

	return nil
}

func (s *ValkeyInstanceService) ReassignTenant(ctx context.Context, id string, tenantID *string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE valkey_instances SET tenant_id = $1, updated_at = now() WHERE id = $2",
		tenantID, id,
	)
	if err != nil {
		return fmt.Errorf("reassign valkey instance %s to tenant: %w", id, err)
	}
	return nil
}

func (s *ValkeyInstanceService) Retry(ctx context.Context, id string) error {
	var status, name string
	err := s.db.QueryRow(ctx, "SELECT status, name FROM valkey_instances WHERE id = $1", id).Scan(&status, &name)
	if err != nil {
		return fmt.Errorf("get valkey instance status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("valkey instance %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE valkey_instances SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set valkey instance %s status to provisioning: %w", id, err)
	}
	tenantID, err := resolveTenantIDFromValkeyInstance(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("retry valkey instance: %w", err)
	}
	return signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "CreateValkeyInstanceWorkflow",
		WorkflowID:   workflowID("valkey-instance", name, id),
		Arg:          id,
		ResourceType: "valkey-instance",
		ResourceID:   id,
	})
}
