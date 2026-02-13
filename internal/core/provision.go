package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

const taskQueue = "hosting-tasks"

// signalProvision enqueues a provisioning task into the per-tenant
// orchestrator workflow. If the workflow is not running, it is started
// automatically via SignalWithStartWorkflow.
//
// When tenantID is empty (e.g. unassigned databases/zones/valkey instances),
// the task is executed directly as a standalone workflow.
func signalProvision(ctx context.Context, tc temporalclient.Client, tenantID string, task model.ProvisionTask) error {
	if tenantID == "" {
		_, err := tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
			ID:        task.WorkflowID,
			TaskQueue: taskQueue,
		}, task.WorkflowName, task.Arg)
		return err
	}

	workflowID := fmt.Sprintf("tenant-provision-%s", tenantID)
	_, err := tc.SignalWithStartWorkflow(ctx, workflowID, model.ProvisionSignalName, task,
		temporalclient.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: taskQueue,
		}, "TenantProvisionWorkflow")
	return err
}

// --- tenant_id resolution helpers ---
//
// These resolve the owning tenant for resources at various nesting levels.
// For nullable tenant_id columns (databases, zones, valkey_instances),
// an empty string is returned when the tenant is unassigned.

func resolveTenantIDFromWebroot(ctx context.Context, db DB, webrootID string) (string, error) {
	var tenantID string
	err := db.QueryRow(ctx, "SELECT tenant_id FROM webroots WHERE id = $1", webrootID).Scan(&tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant from webroot %s: %w", webrootID, err)
	}
	return tenantID, nil
}

func resolveTenantIDFromFQDN(ctx context.Context, db DB, fqdnID string) (string, error) {
	var tenantID string
	err := db.QueryRow(ctx,
		"SELECT w.tenant_id FROM fqdns f JOIN webroots w ON w.id = f.webroot_id WHERE f.id = $1",
		fqdnID).Scan(&tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant from fqdn %s: %w", fqdnID, err)
	}
	return tenantID, nil
}

func resolveTenantIDFromDatabase(ctx context.Context, db DB, databaseID string) (string, error) {
	var tenantID *string
	err := db.QueryRow(ctx, "SELECT tenant_id FROM databases WHERE id = $1", databaseID).Scan(&tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant from database %s: %w", databaseID, err)
	}
	if tenantID == nil {
		return "", nil
	}
	return *tenantID, nil
}

func resolveTenantIDFromDatabaseUser(ctx context.Context, db DB, userID string) (string, error) {
	var tenantID *string
	err := db.QueryRow(ctx,
		"SELECT d.tenant_id FROM database_users du JOIN databases d ON d.id = du.database_id WHERE du.id = $1",
		userID).Scan(&tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant from database user %s: %w", userID, err)
	}
	if tenantID == nil {
		return "", nil
	}
	return *tenantID, nil
}

func resolveTenantIDFromZone(ctx context.Context, db DB, zoneID string) (string, error) {
	var tenantID *string
	err := db.QueryRow(ctx, "SELECT tenant_id FROM zones WHERE id = $1", zoneID).Scan(&tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant from zone %s: %w", zoneID, err)
	}
	if tenantID == nil {
		return "", nil
	}
	return *tenantID, nil
}

func resolveTenantIDFromZoneRecord(ctx context.Context, db DB, recordID string) (string, error) {
	var tenantID *string
	err := db.QueryRow(ctx,
		"SELECT z.tenant_id FROM zone_records zr JOIN zones z ON z.id = zr.zone_id WHERE zr.id = $1",
		recordID).Scan(&tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant from zone record %s: %w", recordID, err)
	}
	if tenantID == nil {
		return "", nil
	}
	return *tenantID, nil
}

func resolveTenantIDFromValkeyInstance(ctx context.Context, db DB, instanceID string) (string, error) {
	var tenantID *string
	err := db.QueryRow(ctx, "SELECT tenant_id FROM valkey_instances WHERE id = $1", instanceID).Scan(&tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant from valkey instance %s: %w", instanceID, err)
	}
	if tenantID == nil {
		return "", nil
	}
	return *tenantID, nil
}

func resolveTenantIDFromValkeyUser(ctx context.Context, db DB, userID string) (string, error) {
	var tenantID *string
	err := db.QueryRow(ctx,
		"SELECT vi.tenant_id FROM valkey_users vu JOIN valkey_instances vi ON vi.id = vu.valkey_instance_id WHERE vu.id = $1",
		userID).Scan(&tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant from valkey user %s: %w", userID, err)
	}
	if tenantID == nil {
		return "", nil
	}
	return *tenantID, nil
}

func resolveTenantIDFromSFTPKey(ctx context.Context, db DB, keyID string) (string, error) {
	var tenantID string
	err := db.QueryRow(ctx, "SELECT tenant_id FROM sftp_keys WHERE id = $1", keyID).Scan(&tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant from sftp key %s: %w", keyID, err)
	}
	return tenantID, nil
}

func resolveTenantIDFromBackup(ctx context.Context, db DB, backupID string) (string, error) {
	var tenantID string
	err := db.QueryRow(ctx, "SELECT tenant_id FROM backups WHERE id = $1", backupID).Scan(&tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant from backup %s: %w", backupID, err)
	}
	return tenantID, nil
}

func resolveTenantIDFromEmailAccount(ctx context.Context, db DB, accountID string) (string, error) {
	var tenantID string
	err := db.QueryRow(ctx,
		`SELECT w.tenant_id FROM email_accounts ea
		 JOIN fqdns f ON f.id = ea.fqdn_id
		 JOIN webroots w ON w.id = f.webroot_id
		 WHERE ea.id = $1`, accountID).Scan(&tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant from email account %s: %w", accountID, err)
	}
	return tenantID, nil
}

func resolveTenantIDFromEmailAlias(ctx context.Context, db DB, aliasID string) (string, error) {
	var tenantID string
	err := db.QueryRow(ctx,
		`SELECT w.tenant_id FROM email_aliases al
		 JOIN email_accounts ea ON ea.id = al.email_account_id
		 JOIN fqdns f ON f.id = ea.fqdn_id
		 JOIN webroots w ON w.id = f.webroot_id
		 WHERE al.id = $1`, aliasID).Scan(&tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant from email alias %s: %w", aliasID, err)
	}
	return tenantID, nil
}

func resolveTenantIDFromEmailForward(ctx context.Context, db DB, forwardID string) (string, error) {
	var tenantID string
	err := db.QueryRow(ctx,
		`SELECT w.tenant_id FROM email_forwards ef
		 JOIN email_accounts ea ON ea.id = ef.email_account_id
		 JOIN fqdns f ON f.id = ea.fqdn_id
		 JOIN webroots w ON w.id = f.webroot_id
		 WHERE ef.id = $1`, forwardID).Scan(&tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant from email forward %s: %w", forwardID, err)
	}
	return tenantID, nil
}

func resolveTenantIDFromS3Bucket(ctx context.Context, db DB, bucketID string) (string, error) {
	var tenantID *string
	err := db.QueryRow(ctx, "SELECT tenant_id FROM s3_buckets WHERE id = $1", bucketID).Scan(&tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant from s3 bucket %s: %w", bucketID, err)
	}
	if tenantID == nil {
		return "", nil
	}
	return *tenantID, nil
}

func resolveTenantIDFromS3AccessKey(ctx context.Context, db DB, keyID string) (string, error) {
	var tenantID *string
	err := db.QueryRow(ctx,
		"SELECT b.tenant_id FROM s3_access_keys k JOIN s3_buckets b ON b.id = k.s3_bucket_id WHERE k.id = $1",
		keyID).Scan(&tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve tenant from s3 access key %s: %w", keyID, err)
	}
	if tenantID == nil {
		return "", nil
	}
	return *tenantID, nil
}
