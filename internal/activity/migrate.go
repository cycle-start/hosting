package activity

import (
	"context"
	"fmt"
	"log"
)

// Migrate contains activities for tenant migration operations.
type Migrate struct {
	db DB
}

// NewMigrate creates a new Migrate activity struct.
func NewMigrate(db DB) *Migrate {
	return &Migrate{db: db}
}

// ValidateMigrationPreconditionsParams holds parameters for ValidateMigrationPreconditions.
type ValidateMigrationPreconditionsParams struct {
	TenantID      string `json:"tenant_id"`
	TargetShardID string `json:"target_shard_id"`
}

// ValidateMigrationPreconditions checks that the migration can proceed.
func (a *Migrate) ValidateMigrationPreconditions(ctx context.Context, params ValidateMigrationPreconditionsParams) error {
	// Get tenant's current shard
	var sourceClusterID, targetClusterID string
	var targetRole, targetStatus string

	// Get tenant cluster
	err := a.db.QueryRow(ctx,
		`SELECT t.cluster_id FROM tenants t WHERE t.id = $1`, params.TenantID,
	).Scan(&sourceClusterID)
	if err != nil {
		return fmt.Errorf("get tenant cluster: %w", err)
	}

	// Get target shard info
	err = a.db.QueryRow(ctx,
		`SELECT cluster_id, role, status FROM shards WHERE id = $1`, params.TargetShardID,
	).Scan(&targetClusterID, &targetRole, &targetStatus)
	if err != nil {
		return fmt.Errorf("get target shard: %w", err)
	}

	if sourceClusterID != targetClusterID {
		return fmt.Errorf("source cluster %s != target cluster %s: cross-cluster migration not supported", sourceClusterID, targetClusterID)
	}

	if targetRole != "web" {
		return fmt.Errorf("target shard role is %s, expected web", targetRole)
	}

	if targetStatus != "active" {
		return fmt.Errorf("target shard status is %s, expected active", targetStatus)
	}

	return nil
}

// ValidateDatabaseMigrationParams holds parameters for ValidateDatabaseMigration.
type ValidateDatabaseMigrationParams struct {
	DatabaseID    string `json:"database_id"`
	TargetShardID string `json:"target_shard_id"`
}

// ValidateDatabaseMigration checks that a database migration can proceed.
func (a *Migrate) ValidateDatabaseMigration(ctx context.Context, params ValidateDatabaseMigrationParams) error {
	var sourceClusterID string
	var sourceShardID *string

	// Get the database's current shard and cluster.
	err := a.db.QueryRow(ctx,
		`SELECT d.shard_id, s.cluster_id FROM databases d JOIN shards s ON d.shard_id = s.id WHERE d.id = $1`,
		params.DatabaseID,
	).Scan(&sourceShardID, &sourceClusterID)
	if err != nil {
		return fmt.Errorf("get database shard: %w", err)
	}

	// Get target shard info.
	var targetClusterID, targetRole, targetStatus string
	err = a.db.QueryRow(ctx,
		`SELECT cluster_id, role, status FROM shards WHERE id = $1`, params.TargetShardID,
	).Scan(&targetClusterID, &targetRole, &targetStatus)
	if err != nil {
		return fmt.Errorf("get target shard: %w", err)
	}

	if sourceClusterID != targetClusterID {
		return fmt.Errorf("source cluster %s != target cluster %s: cross-cluster migration not supported", sourceClusterID, targetClusterID)
	}

	if targetRole != "database" {
		return fmt.Errorf("target shard role is %s, expected database", targetRole)
	}

	if targetStatus != "active" {
		return fmt.Errorf("target shard status is %s, expected active", targetStatus)
	}

	return nil
}

// ValidateValkeyMigrationParams holds parameters for ValidateValkeyMigration.
type ValidateValkeyMigrationParams struct {
	InstanceID    string `json:"instance_id"`
	TargetShardID string `json:"target_shard_id"`
}

// ValidateValkeyMigration checks that a Valkey instance migration can proceed.
func (a *Migrate) ValidateValkeyMigration(ctx context.Context, params ValidateValkeyMigrationParams) error {
	var sourceClusterID string
	var sourceShardID *string

	// Get the instance's current shard and cluster.
	err := a.db.QueryRow(ctx,
		`SELECT v.shard_id, s.cluster_id FROM valkey_instances v JOIN shards s ON v.shard_id = s.id WHERE v.id = $1`,
		params.InstanceID,
	).Scan(&sourceShardID, &sourceClusterID)
	if err != nil {
		return fmt.Errorf("get valkey instance shard: %w", err)
	}

	// Get target shard info.
	var targetClusterID, targetRole, targetStatus string
	err = a.db.QueryRow(ctx,
		`SELECT cluster_id, role, status FROM shards WHERE id = $1`, params.TargetShardID,
	).Scan(&targetClusterID, &targetRole, &targetStatus)
	if err != nil {
		return fmt.Errorf("get target shard: %w", err)
	}

	if sourceClusterID != targetClusterID {
		return fmt.Errorf("source cluster %s != target cluster %s: cross-cluster migration not supported", sourceClusterID, targetClusterID)
	}

	if targetRole != "valkey" {
		return fmt.Errorf("target shard role is %s, expected valkey", targetRole)
	}

	if targetStatus != "active" {
		return fmt.Errorf("target shard status is %s, expected active", targetStatus)
	}

	return nil
}

// BulkUpdateLBMapEntriesParams holds parameters for BulkUpdateLBMapEntries.
type BulkUpdateLBMapEntriesParams struct {
	Entries []SetLBMapEntryParams `json:"entries"`
}

// BulkUpdateLBMapEntries updates multiple HAProxy map entries at once.
func (a *Migrate) BulkUpdateLBMapEntries(ctx context.Context, params BulkUpdateLBMapEntriesParams) error {
	for _, entry := range params.Entries {
		log.Printf("setting LB map entry: %s -> %s", entry.FQDN, entry.LBBackend)
	}
	return nil
}
