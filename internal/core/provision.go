package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

const taskQueue = "hosting-tasks"

// ctxKey is a context key type for callback URL propagation.
type ctxKey string

const callbackURLKey ctxKey = "callback_url"

// WithCallbackURL attaches a callback URL to the context for propagation
// to signalProvision without changing any service method signatures.
func WithCallbackURL(ctx context.Context, url string) context.Context {
	if url == "" {
		return ctx
	}
	return context.WithValue(ctx, callbackURLKey, url)
}

func callbackURLFromCtx(ctx context.Context) string {
	if url, ok := ctx.Value(callbackURLKey).(string); ok {
		return url
	}
	return ""
}

// skipWorkflowKey is a context key that suppresses workflow execution.
// Used during transactional nested creation so that child resources
// don't start their own workflows — the parent workflow spawns them instead.
type skipWorkflowKey struct{}

// WithSkipWorkflow returns a context that causes signalProvision to be a no-op.
func WithSkipWorkflow(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipWorkflowKey{}, true)
}

// workflowID builds a human-readable Temporal workflow ID from a resource type
// prefix and the resource's unique ID (short name).
func workflowID(prefix, id string) string {
	return fmt.Sprintf("%s-%s", prefix, id)
}

// signalProvision routes a workflow task through the per-tenant entity workflow.
// It uses SignalWithStartWorkflow to ensure sequential execution of all
// tenant-related workflows. If the context has WithSkipWorkflow set, this is a
// no-op. If tenantID is empty (for unassigned resources like tenant-less zones),
// the workflow is started directly without per-tenant serialization.
func signalProvision(ctx context.Context, tc temporalclient.Client, db DB, tenantID string, task model.ProvisionTask) error {
	if v, _ := ctx.Value(skipWorkflowKey{}).(bool); v {
		return nil
	}

	if tenantID == "" {
		// No tenant — start workflow directly.
		_, err := tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
			ID:        task.WorkflowID,
			TaskQueue: taskQueue,
		}, task.WorkflowName, task.Arg)
		return err
	}

	wfID := fmt.Sprintf("tenant-%s", tenantID)
	_, err := tc.SignalWithStartWorkflow(ctx, wfID, model.ProvisionSignalName, task,
		temporalclient.StartWorkflowOptions{
			ID:        wfID,
			TaskQueue: taskQueue,
		},
		"TenantProvisionWorkflow",
	)
	return err
}

// startWorkflow directly executes a Temporal workflow without per-tenant
// serialization. Used for non-tenant-scoped workflows (shard convergence, etc.).
func startWorkflow(ctx context.Context, tc temporalclient.Client, workflowName, wfID string, arg any) error {
	if v, _ := ctx.Value(skipWorkflowKey{}).(bool); v {
		return nil
	}
	_, err := tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        wfID,
		TaskQueue: taskQueue,
	}, workflowName, arg)
	return err
}

// --- Tenant ID resolvers ---
// These resolve the owning tenant ID for resources that don't have a direct
// tenant_id column, by following foreign key relationships.

func resolveTenantIDFromWebroot(ctx context.Context, db DB, webrootID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, "SELECT tenant_id FROM webroots WHERE id = $1", webrootID).Scan(&id)
	return id, err
}

func resolveTenantIDFromFQDN(ctx context.Context, db DB, fqdnID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, "SELECT tenant_id FROM fqdns WHERE id = $1", fqdnID).Scan(&id)
	return id, err
}

func resolveTenantIDFromDatabase(ctx context.Context, db DB, databaseID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, "SELECT tenant_id FROM databases WHERE id = $1", databaseID).Scan(&id)
	return id, err
}

func resolveTenantIDFromDatabaseUser(ctx context.Context, db DB, userID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, "SELECT d.tenant_id FROM databases d JOIN database_users du ON du.database_id = d.id WHERE du.id = $1", userID).Scan(&id)
	return id, err
}

func resolveTenantIDFromDatabaseAccessRule(ctx context.Context, db DB, ruleID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, "SELECT d.tenant_id FROM databases d JOIN database_access_rules dar ON dar.database_id = d.id WHERE dar.id = $1", ruleID).Scan(&id)
	return id, err
}

func resolveTenantIDFromValkeyInstance(ctx context.Context, db DB, instanceID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, "SELECT tenant_id FROM valkey_instances WHERE id = $1", instanceID).Scan(&id)
	return id, err
}

func resolveTenantIDFromValkeyUser(ctx context.Context, db DB, userID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, "SELECT vi.tenant_id FROM valkey_instances vi JOIN valkey_users vu ON vu.valkey_instance_id = vi.id WHERE vu.id = $1", userID).Scan(&id)
	return id, err
}

func resolveTenantIDFromS3Bucket(ctx context.Context, db DB, bucketID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, "SELECT tenant_id FROM s3_buckets WHERE id = $1", bucketID).Scan(&id)
	return id, err
}

func resolveTenantIDFromS3AccessKey(ctx context.Context, db DB, keyID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, "SELECT b.tenant_id FROM s3_buckets b JOIN s3_access_keys k ON k.s3_bucket_id = b.id WHERE k.id = $1", keyID).Scan(&id)
	return id, err
}

func resolveTenantIDFromEmailAccount(ctx context.Context, db DB, accountID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, "SELECT f.tenant_id FROM fqdns f JOIN email_accounts ea ON ea.fqdn_id = f.id WHERE ea.id = $1", accountID).Scan(&id)
	return id, err
}

func resolveTenantIDFromEmailAlias(ctx context.Context, db DB, aliasID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, "SELECT f.tenant_id FROM fqdns f JOIN email_accounts ea ON ea.fqdn_id = f.id JOIN email_aliases al ON al.email_account_id = ea.id WHERE al.id = $1", aliasID).Scan(&id)
	return id, err
}

func resolveTenantIDFromEmailForward(ctx context.Context, db DB, forwardID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, "SELECT f.tenant_id FROM fqdns f JOIN email_accounts ea ON ea.fqdn_id = f.id JOIN email_forwards ef ON ef.email_account_id = ea.id WHERE ef.id = $1", forwardID).Scan(&id)
	return id, err
}

func resolveTenantIDFromEmailAutoReply(ctx context.Context, db DB, autoReplyID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, "SELECT f.tenant_id FROM fqdns f JOIN email_accounts ea ON ea.fqdn_id = f.id JOIN email_autoreplies ar ON ar.email_account_id = ea.id WHERE ar.id = $1", autoReplyID).Scan(&id)
	return id, err
}

func resolveTenantIDFromCertificate(ctx context.Context, db DB, certID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, "SELECT f.tenant_id FROM fqdns f JOIN certificates c ON c.fqdn_id = f.id WHERE c.id = $1", certID).Scan(&id)
	return id, err
}

func resolveTenantIDFromZone(ctx context.Context, db DB, zoneID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, "SELECT tenant_id FROM zones WHERE id = $1", zoneID).Scan(&id)
	return id, err
}

func resolveTenantIDFromZoneRecord(ctx context.Context, db DB, recordID string) (string, error) {
	var id string
	err := db.QueryRow(ctx, "SELECT z.tenant_id FROM zones z JOIN zone_records zr ON zr.zone_id = z.id WHERE zr.id = $1", recordID).Scan(&id)
	return id, err
}
