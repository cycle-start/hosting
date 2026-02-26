package activity

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/edvin/hosting/internal/model"
)

// DB defines the database operations used by activity structs.
// *pgxpool.Pool satisfies this interface.
type DB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// CoreDB contains activities that read from and update the core database.
type CoreDB struct {
	db     DB
	kekHex string
}

// NewCoreDB creates a new CoreDB activity struct.
func NewCoreDB(db DB, kekHex string) *CoreDB {
	return &CoreDB{db: db, kekHex: kekHex}
}

// UpdateResourceStatusParams holds the parameters for UpdateResourceStatus.
type UpdateResourceStatusParams struct {
	Table         string
	ID            string
	Status        string
	StatusMessage *string // nil = don't change; set to store message
}

// UpdateResourceStatus sets the status of a resource row in the given table.
func (a *CoreDB) UpdateResourceStatus(ctx context.Context, params UpdateResourceStatusParams) error {
	if params.Status == model.StatusDeleted {
		// Hard delete — remove the row entirely.
		query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", params.Table)
		_, err := a.db.Exec(ctx, query, params.ID)
		return err
	}
	if params.Status == model.StatusActive {
		// Clear status_message on success transitions.
		query := fmt.Sprintf("UPDATE %s SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", params.Table)
		_, err := a.db.Exec(ctx, query, params.Status, params.ID)
		return err
	}
	if params.StatusMessage != nil {
		query := fmt.Sprintf("UPDATE %s SET status = $1, status_message = $2, updated_at = now() WHERE id = $3", params.Table)
		_, err := a.db.Exec(ctx, query, params.Status, *params.StatusMessage, params.ID)
		return err
	}
	query := fmt.Sprintf("UPDATE %s SET status = $1, updated_at = now() WHERE id = $2", params.Table)
	_, err := a.db.Exec(ctx, query, params.Status, params.ID)
	return err
}

// SuspendResourceParams holds the parameters for SuspendResource and UnsuspendResource.
type SuspendResourceParams struct {
	Table  string `json:"table"`
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// SuspendResource sets a resource to suspended status with a reason.
func (a *CoreDB) SuspendResource(ctx context.Context, params SuspendResourceParams) error {
	query := fmt.Sprintf("UPDATE %s SET status = $1, suspend_reason = $2, updated_at = now() WHERE id = $3", params.Table)
	_, err := a.db.Exec(ctx, query, model.StatusSuspended, params.Reason, params.ID)
	return err
}

// UnsuspendResource sets a resource back to active status and clears the suspend reason.
func (a *CoreDB) UnsuspendResource(ctx context.Context, params SuspendResourceParams) error {
	query := fmt.Sprintf("UPDATE %s SET status = $1, suspend_reason = '', updated_at = now() WHERE id = $2", params.Table)
	_, err := a.db.Exec(ctx, query, model.StatusActive, params.ID)
	return err
}

// DeleteTenantDBRows bulk-deletes all database rows belonging to a tenant in
// correct FK order inside a single transaction. This is idempotent: rows
// already deleted by earlier workflow phases are simply no-ops.
func (a *CoreDB) DeleteTenantDBRows(ctx context.Context, tenantID string) error {
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Email features (via email_accounts → fqdns).
	emailAccountSubquery := `SELECT ea.id FROM email_accounts ea JOIN fqdns f ON ea.fqdn_id=f.id WHERE f.tenant_id=$1`
	fqdnSubquery := `SELECT id FROM fqdns WHERE tenant_id=$1`

	queries := []string{
		`DELETE FROM email_autoreplies WHERE account_id IN (` + emailAccountSubquery + `)`,
		`DELETE FROM email_forwards WHERE account_id IN (` + emailAccountSubquery + `)`,
		`DELETE FROM email_aliases WHERE account_id IN (` + emailAccountSubquery + `)`,
		`DELETE FROM email_accounts WHERE fqdn_id IN (` + fqdnSubquery + `)`,

		// Certificates and FQDNs (via webroots).
		`DELETE FROM certificates WHERE fqdn_id IN (` + fqdnSubquery + `)`,
		`DELETE FROM fqdns WHERE tenant_id=$1`,

		// Direct tenant children (web-shard).
		`DELETE FROM daemons WHERE tenant_id=$1`,
		`DELETE FROM cron_jobs WHERE tenant_id=$1`,
		`DELETE FROM webroots WHERE tenant_id=$1`,
		`DELETE FROM ssh_keys WHERE tenant_id=$1`,
		`DELETE FROM backups WHERE tenant_id=$1`,
		`DELETE FROM tenant_egress_rules WHERE tenant_id=$1`,
		`DELETE FROM resource_usage WHERE tenant_id=$1`,

		// OIDC sessions.
		`DELETE FROM oidc_auth_codes WHERE tenant_id=$1`,
		`DELETE FROM oidc_login_sessions WHERE tenant_id=$1`,

		// Cross-shard leftovers (idempotent — usually already deleted by Phase 1 workflows).
		`DELETE FROM database_users WHERE database_id IN (SELECT id FROM databases WHERE tenant_id=$1)`,
		`DELETE FROM databases WHERE tenant_id=$1`,
		`DELETE FROM valkey_users WHERE valkey_instance_id IN (SELECT id FROM valkey_instances WHERE tenant_id=$1)`,
		`DELETE FROM valkey_instances WHERE tenant_id=$1`,
		`DELETE FROM s3_access_keys WHERE bucket_id IN (SELECT id FROM s3_buckets WHERE tenant_id=$1)`,
		`DELETE FROM s3_buckets WHERE tenant_id=$1`,
		`DELETE FROM zone_records WHERE zone_id IN (SELECT id FROM zones WHERE tenant_id=$1)`,
		`DELETE FROM zones WHERE tenant_id=$1`,
		`DELETE FROM subscriptions WHERE tenant_id=$1`,

		// Finally, the tenant itself.
		`DELETE FROM tenants WHERE id=$1`,
	}

	for _, q := range queries {
		if _, err := tx.Exec(ctx, q, tenantID); err != nil {
			return fmt.Errorf("delete tenant rows: %w", err)
		}
	}

	return tx.Commit(ctx)
}
