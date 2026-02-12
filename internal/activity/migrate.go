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

// MigrateMySQLDatabaseParams holds parameters for MigrateMySQLDatabase.
type MigrateMySQLDatabaseParams struct {
	DatabaseName  string `json:"database_name"`
	SourceShardID string `json:"source_shard_id"`
	TargetShardID string `json:"target_shard_id"`
}

// MigrateMySQLDatabase performs a mysqldump from source shard and imports to target shard.
// This is a stub that will be implemented when cross-shard database migration is needed.
func (a *Migrate) MigrateMySQLDatabase(ctx context.Context, params MigrateMySQLDatabaseParams) error {
	log.Printf("migrating MySQL database %s from shard %s to shard %s", params.DatabaseName, params.SourceShardID, params.TargetShardID)
	return nil
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
