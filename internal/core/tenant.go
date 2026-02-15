package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type TenantService struct {
	db DB
	tc temporalclient.Client
}

func NewTenantService(db DB, tc temporalclient.Client) *TenantService {
	return &TenantService{db: db, tc: tc}
}

func (s *TenantService) Create(ctx context.Context, tenant *model.Tenant) error {
	uid, err := s.NextUID(ctx)
	if err != nil {
		return fmt.Errorf("create tenant: %w", err)
	}
	tenant.UID = uid

	_, err = s.db.Exec(ctx,
		`INSERT INTO tenants (id, name, brand_id, region_id, cluster_id, shard_id, uid, sftp_enabled, ssh_enabled, disk_quota_bytes, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		tenant.ID, tenant.Name, tenant.BrandID, tenant.RegionID, tenant.ClusterID, tenant.ShardID, tenant.UID,
		tenant.SFTPEnabled, tenant.SSHEnabled, tenant.DiskQuotaBytes, tenant.Status, tenant.CreatedAt, tenant.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert tenant: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenant.ID, model.ProvisionTask{
		WorkflowName: "CreateTenantWorkflow",
		WorkflowID:   fmt.Sprintf("tenant-%s", tenant.ID),
		Arg:          tenant.ID,
		ResourceType: "tenant",
		ResourceID:   tenant.ID,
	}); err != nil {
		return fmt.Errorf("start CreateTenantWorkflow: %w", err)
	}

	return nil
}

func (s *TenantService) GetByID(ctx context.Context, id string) (*model.Tenant, error) {
	var t model.Tenant
	err := s.db.QueryRow(ctx,
		`SELECT t.id, t.name, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.ssh_enabled, t.disk_quota_bytes, t.status, t.status_message, t.suspend_reason, t.created_at, t.updated_at,
		        r.name, c.name, s.name
		 FROM tenants t
		 JOIN regions r ON r.id = t.region_id
		 JOIN clusters c ON c.id = t.cluster_id
		 LEFT JOIN shards s ON s.id = t.shard_id
		 WHERE t.id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.BrandID, &t.RegionID, &t.ClusterID, &t.ShardID, &t.UID,
		&t.SFTPEnabled, &t.SSHEnabled, &t.DiskQuotaBytes, &t.Status, &t.StatusMessage, &t.SuspendReason, &t.CreatedAt, &t.UpdatedAt,
		&t.RegionName, &t.ClusterName, &t.ShardName)
	if err != nil {
		return nil, fmt.Errorf("get tenant %s: %w", id, err)
	}
	return &t, nil
}

func (s *TenantService) List(ctx context.Context, params request.ListParams) ([]model.Tenant, bool, error) {
	query := `SELECT t.id, t.name, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.ssh_enabled, t.disk_quota_bytes, t.status, t.status_message, t.suspend_reason, t.created_at, t.updated_at, r.name, c.name, s.name FROM tenants t JOIN regions r ON r.id = t.region_id JOIN clusters c ON c.id = t.cluster_id LEFT JOIN shards s ON s.id = t.shard_id WHERE true`
	args := []any{}
	argIdx := 1

	if params.Search != "" {
		query += fmt.Sprintf(` AND (t.id ILIKE $%d OR t.name ILIKE $%d)`, argIdx, argIdx)
		args = append(args, "%"+params.Search+"%")
		argIdx++
	}
	if params.Status != "" {
		query += fmt.Sprintf(` AND t.status = $%d`, argIdx)
		args = append(args, params.Status)
		argIdx++
	}
	if params.Cursor != "" {
		query += fmt.Sprintf(` AND t.id > $%d`, argIdx)
		args = append(args, params.Cursor)
		argIdx++
	}
	if len(params.BrandIDs) > 0 {
		query += fmt.Sprintf(` AND t.brand_id = ANY($%d)`, argIdx)
		args = append(args, params.BrandIDs)
		argIdx++
	}

	sortCol := "t.created_at"
	switch params.Sort {
	case "status":
		sortCol = "t.status"
	case "created_at":
		sortCol = "t.created_at"
	}
	order := "DESC"
	if params.Order == "asc" {
		order = "ASC"
	}
	query += fmt.Sprintf(` ORDER BY %s %s`, sortCol, order)
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, params.Limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var tenants []model.Tenant
	for rows.Next() {
		var t model.Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.BrandID, &t.RegionID, &t.ClusterID, &t.ShardID, &t.UID,
			&t.SFTPEnabled, &t.SSHEnabled, &t.DiskQuotaBytes, &t.Status, &t.StatusMessage, &t.SuspendReason, &t.CreatedAt, &t.UpdatedAt,
			&t.RegionName, &t.ClusterName, &t.ShardName); err != nil {
			return nil, false, fmt.Errorf("scan tenant: %w", err)
		}
		tenants = append(tenants, t)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate tenants: %w", err)
	}

	hasMore := len(tenants) > params.Limit
	if hasMore {
		tenants = tenants[:params.Limit]
	}
	return tenants, hasMore, nil
}

func (s *TenantService) ListByShard(ctx context.Context, shardID string, limit int, cursor string) ([]model.Tenant, bool, error) {
	query := `SELECT t.id, t.name, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.ssh_enabled, t.disk_quota_bytes, t.status, t.status_message, t.suspend_reason, t.created_at, t.updated_at, r.name, c.name, s.name FROM tenants t JOIN regions r ON r.id = t.region_id JOIN clusters c ON c.id = t.cluster_id LEFT JOIN shards s ON s.id = t.shard_id WHERE t.shard_id = $1`
	args := []any{shardID}
	argIdx := 2

	if cursor != "" {
		query += fmt.Sprintf(` AND t.id > $%d`, argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query += ` ORDER BY t.id`
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list tenants for shard %s: %w", shardID, err)
	}
	defer rows.Close()

	var tenants []model.Tenant
	for rows.Next() {
		var t model.Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.BrandID, &t.RegionID, &t.ClusterID, &t.ShardID, &t.UID,
			&t.SFTPEnabled, &t.SSHEnabled, &t.DiskQuotaBytes, &t.Status, &t.StatusMessage, &t.SuspendReason, &t.CreatedAt, &t.UpdatedAt,
			&t.RegionName, &t.ClusterName, &t.ShardName); err != nil {
			return nil, false, fmt.Errorf("scan tenant: %w", err)
		}
		tenants = append(tenants, t)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate tenants: %w", err)
	}

	hasMore := len(tenants) > limit
	if hasMore {
		tenants = tenants[:limit]
	}
	return tenants, hasMore, nil
}

func (s *TenantService) Update(ctx context.Context, tenant *model.Tenant) error {
	_, err := s.db.Exec(ctx,
		`UPDATE tenants SET region_id = $1, cluster_id = $2, shard_id = $3, sftp_enabled = $4, ssh_enabled = $5, disk_quota_bytes = $6, status = $7, updated_at = now()
		 WHERE id = $8`,
		tenant.RegionID, tenant.ClusterID, tenant.ShardID, tenant.SFTPEnabled, tenant.SSHEnabled, tenant.DiskQuotaBytes, tenant.Status, tenant.ID,
	)
	if err != nil {
		return fmt.Errorf("update tenant %s: %w", tenant.ID, err)
	}

	if err := signalProvision(ctx, s.tc, tenant.ID, model.ProvisionTask{
		WorkflowName: "UpdateTenantWorkflow",
		WorkflowID:   fmt.Sprintf("tenant-%s", tenant.ID),
		Arg:          tenant.ID,
		ResourceType: "tenant",
		ResourceID:   tenant.ID,
	}); err != nil {
		return fmt.Errorf("start UpdateTenantWorkflow: %w", err)
	}

	return nil
}

func (s *TenantService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE tenants SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusDeleting, id,
	)
	if err != nil {
		return fmt.Errorf("set tenant %s status to deleting: %w", id, err)
	}

	if err := signalProvision(ctx, s.tc, id, model.ProvisionTask{
		WorkflowName: "DeleteTenantWorkflow",
		WorkflowID:   fmt.Sprintf("tenant-%s", id),
		Arg:          id,
		ResourceType: "tenant",
		ResourceID:   id,
	}); err != nil {
		return fmt.Errorf("start DeleteTenantWorkflow: %w", err)
	}

	return nil
}

func (s *TenantService) Suspend(ctx context.Context, id string, reason string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE tenants SET status = $1, suspend_reason = $2, updated_at = now() WHERE id = $3",
		model.StatusSuspended, reason, id,
	)
	if err != nil {
		return fmt.Errorf("set tenant %s status to suspended: %w", id, err)
	}

	if err := signalProvision(ctx, s.tc, id, model.ProvisionTask{
		WorkflowName: "SuspendTenantWorkflow",
		WorkflowID:   fmt.Sprintf("tenant-%s", id),
		Arg:          id,
		ResourceType: "tenant",
		ResourceID:   id,
	}); err != nil {
		return fmt.Errorf("start SuspendTenantWorkflow: %w", err)
	}

	return nil
}

func (s *TenantService) Unsuspend(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE tenants SET status = $1, suspend_reason = '', updated_at = now() WHERE id = $2",
		model.StatusPending, id,
	)
	if err != nil {
		return fmt.Errorf("set tenant %s status to pending: %w", id, err)
	}

	if err := signalProvision(ctx, s.tc, id, model.ProvisionTask{
		WorkflowName: "UnsuspendTenantWorkflow",
		WorkflowID:   fmt.Sprintf("tenant-%s", id),
		Arg:          id,
		ResourceType: "tenant",
		ResourceID:   id,
	}); err != nil {
		return fmt.Errorf("start UnsuspendTenantWorkflow: %w", err)
	}

	return nil
}

func (s *TenantService) Migrate(ctx context.Context, id string, targetShardID string, migrateZones, migrateFQDNs bool) error {
	_, err := s.db.Exec(ctx,
		"UPDATE tenants SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusProvisioning, id,
	)
	if err != nil {
		return fmt.Errorf("set tenant %s status to provisioning: %w", id, err)
	}

	if err := signalProvision(ctx, s.tc, id, model.ProvisionTask{
		WorkflowName: "MigrateTenantWorkflow",
		WorkflowID:   fmt.Sprintf("migrate-tenant-%s", id),
		Arg: MigrateTenantParams{
			TenantID:      id,
			TargetShardID: targetShardID,
			MigrateZones:  migrateZones,
			MigrateFQDNs:  migrateFQDNs,
		},
		ResourceType: "tenant",
		ResourceID:   id,
	}); err != nil {
		return fmt.Errorf("start MigrateTenantWorkflow: %w", err)
	}

	return nil
}

// MigrateTenantParams holds parameters for the MigrateTenantWorkflow.
type MigrateTenantParams struct {
	TenantID      string `json:"tenant_id"`
	TargetShardID string `json:"target_shard_id"`
	MigrateZones  bool   `json:"migrate_zones"`
	MigrateFQDNs  bool   `json:"migrate_fqdns"`
}

func (s *TenantService) ResourceSummary(ctx context.Context, tenantID string) (*model.TenantResourceSummary, error) {
	const query = `
		SELECT 'webroots' AS resource_type, status, count(*) FROM webroots WHERE tenant_id = $1 GROUP BY status
		UNION ALL
		SELECT 'fqdns', f.status, count(*) FROM fqdns f JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 GROUP BY f.status
		UNION ALL
		SELECT 'certificates', c.status, count(*) FROM certificates c JOIN fqdns f ON f.id = c.fqdn_id JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 GROUP BY c.status
		UNION ALL
		SELECT 'email_accounts', ea.status, count(*) FROM email_accounts ea JOIN fqdns f ON f.id = ea.fqdn_id JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 GROUP BY ea.status
		UNION ALL
		SELECT 'email_aliases', al.status, count(*) FROM email_aliases al JOIN email_accounts ea ON ea.id = al.email_account_id JOIN fqdns f ON f.id = ea.fqdn_id JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 GROUP BY al.status
		UNION ALL
		SELECT 'email_forwards', ef.status, count(*) FROM email_forwards ef JOIN email_accounts ea ON ea.id = ef.email_account_id JOIN fqdns f ON f.id = ea.fqdn_id JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 GROUP BY ef.status
		UNION ALL
		SELECT 'email_autoreplies', ar.status, count(*) FROM email_autoreplies ar JOIN email_accounts ea ON ea.id = ar.email_account_id JOIN fqdns f ON f.id = ea.fqdn_id JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 GROUP BY ar.status
		UNION ALL
		SELECT 'databases', status, count(*) FROM databases WHERE tenant_id = $1 GROUP BY status
		UNION ALL
		SELECT 'database_users', du.status, count(*) FROM database_users du JOIN databases d ON d.id = du.database_id WHERE d.tenant_id = $1 GROUP BY du.status
		UNION ALL
		SELECT 'zones', status, count(*) FROM zones WHERE tenant_id = $1 GROUP BY status
		UNION ALL
		SELECT 'zone_records', zr.status, count(*) FROM zone_records zr JOIN zones z ON z.id = zr.zone_id WHERE z.tenant_id = $1 GROUP BY zr.status
		UNION ALL
		SELECT 'valkey_instances', status, count(*) FROM valkey_instances WHERE tenant_id = $1 GROUP BY status
		UNION ALL
		SELECT 'valkey_users', vu.status, count(*) FROM valkey_users vu JOIN valkey_instances vi ON vi.id = vu.valkey_instance_id WHERE vi.tenant_id = $1 GROUP BY vu.status
		UNION ALL
		SELECT 'ssh_keys', status, count(*) FROM ssh_keys WHERE tenant_id = $1 GROUP BY status
		UNION ALL
		SELECT 'backups', status, count(*) FROM backups WHERE tenant_id = $1 GROUP BY status
		UNION ALL
		SELECT 'cron_jobs', status, count(*) FROM cron_jobs WHERE tenant_id = $1 GROUP BY status`

	rows, err := s.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("resource summary for tenant %s: %w", tenantID, err)
	}
	defer rows.Close()

	// Collect per-resource-type status counts.
	counts := map[string]model.ResourceStatusCounts{}
	for rows.Next() {
		var resourceType, status string
		var count int
		if err := rows.Scan(&resourceType, &status, &count); err != nil {
			return nil, fmt.Errorf("scan resource summary row: %w", err)
		}
		if counts[resourceType] == nil {
			counts[resourceType] = model.ResourceStatusCounts{}
		}
		counts[resourceType][status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resource summary: %w", err)
	}

	summary := &model.TenantResourceSummary{
		Webroots:         counts["webroots"],
		FQDNs:            counts["fqdns"],
		Certificates:     counts["certificates"],
		EmailAccounts:    counts["email_accounts"],
		EmailAliases:     counts["email_aliases"],
		EmailForwards:    counts["email_forwards"],
		EmailAutoReplies: counts["email_autoreplies"],
		Databases:        counts["databases"],
		DatabaseUsers:    counts["database_users"],
		Zones:            counts["zones"],
		ZoneRecords:      counts["zone_records"],
		ValkeyInstances:  counts["valkey_instances"],
		ValkeyUsers:      counts["valkey_users"],
		SSHKeys:          counts["ssh_keys"],
		Backups:          counts["backups"],
		CronJobs:         counts["cron_jobs"],
	}

	// Ensure nil maps become empty maps for clean JSON.
	ensureMap := func(m *model.ResourceStatusCounts) {
		if *m == nil {
			*m = model.ResourceStatusCounts{}
		}
	}
	ensureMap(&summary.Webroots)
	ensureMap(&summary.FQDNs)
	ensureMap(&summary.Certificates)
	ensureMap(&summary.EmailAccounts)
	ensureMap(&summary.EmailAliases)
	ensureMap(&summary.EmailForwards)
	ensureMap(&summary.EmailAutoReplies)
	ensureMap(&summary.Databases)
	ensureMap(&summary.DatabaseUsers)
	ensureMap(&summary.Zones)
	ensureMap(&summary.ZoneRecords)
	ensureMap(&summary.ValkeyInstances)
	ensureMap(&summary.ValkeyUsers)
	ensureMap(&summary.SSHKeys)
	ensureMap(&summary.Backups)
	ensureMap(&summary.CronJobs)

	// Compute aggregates.
	for _, m := range []model.ResourceStatusCounts{
		summary.Webroots, summary.FQDNs, summary.Certificates,
		summary.EmailAccounts, summary.EmailAliases, summary.EmailForwards, summary.EmailAutoReplies,
		summary.Databases, summary.DatabaseUsers,
		summary.Zones, summary.ZoneRecords,
		summary.ValkeyInstances, summary.ValkeyUsers,
		summary.SSHKeys, summary.Backups, summary.CronJobs,
	} {
		for status, count := range m {
			summary.Total += count
			switch status {
			case model.StatusPending:
				summary.Pending += count
			case model.StatusProvisioning:
				summary.Provisioning += count
			case model.StatusFailed:
				summary.Failed += count
			}
		}
	}

	return summary, nil
}

func (s *TenantService) NextUID(ctx context.Context) (int, error) {
	var uid int
	err := s.db.QueryRow(ctx, "SELECT nextval('tenant_uid_seq')").Scan(&uid)
	if err != nil {
		return 0, fmt.Errorf("next tenant uid: %w", err)
	}
	return uid, nil
}

func (s *TenantService) Retry(ctx context.Context, id string) error {
	var status string
	err := s.db.QueryRow(ctx, "SELECT status FROM tenants WHERE id = $1", id).Scan(&status)
	if err != nil {
		return fmt.Errorf("get tenant status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("tenant %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE tenants SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set tenant %s status to provisioning: %w", id, err)
	}
	return signalProvision(ctx, s.tc, id, model.ProvisionTask{
		WorkflowName: "CreateTenantWorkflow",
		WorkflowID:   fmt.Sprintf("tenant-%s", id),
		Arg:          id,
		ResourceType: "tenant",
		ResourceID:   id,
	})
}

func (s *TenantService) RetryFailed(ctx context.Context, tenantID string) (int, error) {
	var exists bool
	err := s.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM tenants WHERE id = $1)", tenantID).Scan(&exists)
	if err != nil {
		return 0, fmt.Errorf("check tenant exists: %w", err)
	}
	if !exists {
		return 0, fmt.Errorf("tenant %s not found", tenantID)
	}

	type retrySpec struct {
		query          string
		table          string
		workflowName   string
		workflowPrefix string
	}

	specs := []retrySpec{
		{"SELECT id, name FROM webroots WHERE tenant_id = $1 AND status = 'failed'", "webroots", "CreateWebrootWorkflow", "webroot"},
		{`SELECT f.id, f.fqdn FROM fqdns f JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 AND f.status = 'failed'`, "fqdns", "CreateFQDNWorkflow", "fqdn"},
		{`SELECT c.id, f.fqdn FROM certificates c JOIN fqdns f ON f.id = c.fqdn_id JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 AND c.status = 'failed'`, "certificates", "UploadCustomCertWorkflow", "certificate"},
		{"SELECT id, name FROM zones WHERE tenant_id = $1 AND status = 'failed'", "zones", "CreateZoneWorkflow", "zone"},
		{`SELECT zr.id, zr.name || '/' || zr.type FROM zone_records zr JOIN zones z ON z.id = zr.zone_id WHERE z.tenant_id = $1 AND zr.status = 'failed'`, "zone_records", "CreateZoneRecordWorkflow", "zone-record"},
		{"SELECT id, name FROM databases WHERE tenant_id = $1 AND status = 'failed'", "databases", "CreateDatabaseWorkflow", "database"},
		{`SELECT du.id, du.username FROM database_users du JOIN databases d ON d.id = du.database_id WHERE d.tenant_id = $1 AND du.status = 'failed'`, "database_users", "CreateDatabaseUserWorkflow", "database-user"},
		{"SELECT id, name FROM valkey_instances WHERE tenant_id = $1 AND status = 'failed'", "valkey_instances", "CreateValkeyInstanceWorkflow", "valkey-instance"},
		{`SELECT vu.id, vu.username FROM valkey_users vu JOIN valkey_instances vi ON vi.id = vu.valkey_instance_id WHERE vi.tenant_id = $1 AND vu.status = 'failed'`, "valkey_users", "CreateValkeyUserWorkflow", "valkey-user"},
		{`SELECT ea.id, ea.address FROM email_accounts ea JOIN fqdns f ON f.id = ea.fqdn_id JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 AND ea.status = 'failed'`, "email_accounts", "CreateEmailAccountWorkflow", "email-account"},
		{`SELECT al.id, al.address FROM email_aliases al JOIN email_accounts ea ON ea.id = al.email_account_id JOIN fqdns f ON f.id = ea.fqdn_id JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 AND al.status = 'failed'`, "email_aliases", "CreateEmailAliasWorkflow", "email-alias"},
		{`SELECT ef.id, ef.destination FROM email_forwards ef JOIN email_accounts ea ON ea.id = ef.email_account_id JOIN fqdns f ON f.id = ea.fqdn_id JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 AND ef.status = 'failed'`, "email_forwards", "CreateEmailForwardWorkflow", "email-forward"},
		{`SELECT ar.id, ar.subject FROM email_autoreplies ar JOIN email_accounts ea ON ea.id = ar.email_account_id JOIN fqdns f ON f.id = ea.fqdn_id JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 AND ar.status = 'failed'`, "email_autoreplies", "UpdateEmailAutoReplyWorkflow", "email-autoreply"},
		{"SELECT id, name FROM ssh_keys WHERE tenant_id = $1 AND status = 'failed'", "ssh_keys", "AddSSHKeyWorkflow", "ssh-key"},
		{"SELECT id, name FROM s3_buckets WHERE tenant_id = $1 AND status = 'failed'", "s3_buckets", "CreateS3BucketWorkflow", "s3-bucket"},
		{`SELECT k.id, k.access_key_id FROM s3_access_keys k JOIN s3_buckets b ON b.id = k.s3_bucket_id WHERE b.tenant_id = $1 AND k.status = 'failed'`, "s3_access_keys", "CreateS3AccessKeyWorkflow", "s3-access-key"},
		{"SELECT id, type || '/' || source_name FROM backups WHERE tenant_id = $1 AND status = 'failed'", "backups", "CreateBackupWorkflow", "backup-create"},
		{"SELECT id, name FROM cron_jobs WHERE tenant_id = $1 AND status = 'failed'", "cron_jobs", "CreateCronJobWorkflow", "cron-job"},
	}

	type retryItem struct {
		id   string
		name string
	}

	count := 0
	for _, spec := range specs {
		rows, err := s.db.Query(ctx, spec.query, tenantID)
		if err != nil {
			return count, fmt.Errorf("query failed %s: %w", spec.table, err)
		}
		var items []retryItem
		for rows.Next() {
			var item retryItem
			if err := rows.Scan(&item.id, &item.name); err != nil {
				rows.Close()
				return count, fmt.Errorf("scan failed %s: %w", spec.table, err)
			}
			items = append(items, item)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return count, fmt.Errorf("iterate failed %s: %w", spec.table, err)
		}

		for _, item := range items {
			_, err := s.db.Exec(ctx, fmt.Sprintf("UPDATE %s SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", spec.table), model.StatusProvisioning, item.id)
			if err != nil {
				return count, fmt.Errorf("set %s %s to provisioning: %w", spec.table, item.id, err)
			}
			if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
				WorkflowName: spec.workflowName,
				WorkflowID:   workflowID(spec.workflowPrefix, item.name, item.id),
				Arg:          item.id,
				ResourceType: spec.workflowPrefix,
				ResourceID:   item.id,
			}); err != nil {
				return count, fmt.Errorf("start %s for %s: %w", spec.workflowName, item.id, err)
			}
			count++
		}
	}

	// Also retry the tenant itself if failed.
	var tenantStatus string
	err = s.db.QueryRow(ctx, "SELECT status FROM tenants WHERE id = $1", tenantID).Scan(&tenantStatus)
	if err != nil {
		return count, fmt.Errorf("get tenant status: %w", err)
	}
	if tenantStatus == model.StatusFailed {
		_, err = s.db.Exec(ctx, "UPDATE tenants SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, tenantID)
		if err != nil {
			return count, fmt.Errorf("set tenant to provisioning: %w", err)
		}
		if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
			WorkflowName: "CreateTenantWorkflow",
			WorkflowID:   fmt.Sprintf("tenant-%s", tenantID),
			Arg:          tenantID,
			ResourceType: "tenant",
			ResourceID:   tenantID,
		}); err != nil {
			return count, fmt.Errorf("start CreateTenantWorkflow: %w", err)
		}
		count++
	}

	return count, nil
}
