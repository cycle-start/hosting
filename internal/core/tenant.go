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
		`INSERT INTO tenants (id, brand_id, region_id, cluster_id, shard_id, uid, sftp_enabled, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		tenant.ID, tenant.BrandID, tenant.RegionID, tenant.ClusterID, tenant.ShardID, tenant.UID,
		tenant.SFTPEnabled, tenant.Status, tenant.CreatedAt, tenant.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert tenant: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenant.ID, model.ProvisionTask{
		WorkflowName: "CreateTenantWorkflow",
		WorkflowID:   fmt.Sprintf("tenant-%s", tenant.ID),
		Arg:          tenant.ID,
	}); err != nil {
		return fmt.Errorf("start CreateTenantWorkflow: %w", err)
	}

	return nil
}

func (s *TenantService) GetByID(ctx context.Context, id string) (*model.Tenant, error) {
	var t model.Tenant
	err := s.db.QueryRow(ctx,
		`SELECT id, brand_id, region_id, cluster_id, shard_id, uid, sftp_enabled, status, created_at, updated_at
		 FROM tenants WHERE id = $1`, id,
	).Scan(&t.ID, &t.BrandID, &t.RegionID, &t.ClusterID, &t.ShardID, &t.UID,
		&t.SFTPEnabled, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get tenant %s: %w", id, err)
	}
	return &t, nil
}

func (s *TenantService) List(ctx context.Context, params request.ListParams) ([]model.Tenant, bool, error) {
	query := `SELECT id, brand_id, region_id, cluster_id, shard_id, uid, sftp_enabled, status, created_at, updated_at FROM tenants WHERE status != 'deleted'`
	args := []any{}
	argIdx := 1

	if params.Search != "" {
		query += fmt.Sprintf(` AND id ILIKE $%d`, argIdx)
		args = append(args, "%"+params.Search+"%")
		argIdx++
	}
	if params.Status != "" {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, params.Status)
		argIdx++
	}
	if params.Cursor != "" {
		query += fmt.Sprintf(` AND id > $%d`, argIdx)
		args = append(args, params.Cursor)
		argIdx++
	}

	sortCol := "created_at"
	switch params.Sort {
	case "status":
		sortCol = "status"
	case "created_at":
		sortCol = "created_at"
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
		if err := rows.Scan(&t.ID, &t.BrandID, &t.RegionID, &t.ClusterID, &t.ShardID, &t.UID,
			&t.SFTPEnabled, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
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
	query := `SELECT id, brand_id, region_id, cluster_id, shard_id, uid, sftp_enabled, status, created_at, updated_at FROM tenants WHERE shard_id = $1`
	args := []any{shardID}
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
		return nil, false, fmt.Errorf("list tenants for shard %s: %w", shardID, err)
	}
	defer rows.Close()

	var tenants []model.Tenant
	for rows.Next() {
		var t model.Tenant
		if err := rows.Scan(&t.ID, &t.BrandID, &t.RegionID, &t.ClusterID, &t.ShardID, &t.UID,
			&t.SFTPEnabled, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
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
		`UPDATE tenants SET region_id = $1, cluster_id = $2, shard_id = $3, sftp_enabled = $4, status = $5, updated_at = now()
		 WHERE id = $6`,
		tenant.RegionID, tenant.ClusterID, tenant.ShardID, tenant.SFTPEnabled, tenant.Status, tenant.ID,
	)
	if err != nil {
		return fmt.Errorf("update tenant %s: %w", tenant.ID, err)
	}

	if err := signalProvision(ctx, s.tc, tenant.ID, model.ProvisionTask{
		WorkflowName: "UpdateTenantWorkflow",
		WorkflowID:   fmt.Sprintf("tenant-%s", tenant.ID),
		Arg:          tenant.ID,
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
	}); err != nil {
		return fmt.Errorf("start DeleteTenantWorkflow: %w", err)
	}

	return nil
}

func (s *TenantService) Suspend(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE tenants SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusSuspended, id,
	)
	if err != nil {
		return fmt.Errorf("set tenant %s status to suspended: %w", id, err)
	}

	if err := signalProvision(ctx, s.tc, id, model.ProvisionTask{
		WorkflowName: "SuspendTenantWorkflow",
		WorkflowID:   fmt.Sprintf("tenant-%s", id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("start SuspendTenantWorkflow: %w", err)
	}

	return nil
}

func (s *TenantService) Unsuspend(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE tenants SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusPending, id,
	)
	if err != nil {
		return fmt.Errorf("set tenant %s status to pending: %w", id, err)
	}

	if err := signalProvision(ctx, s.tc, id, model.ProvisionTask{
		WorkflowName: "UnsuspendTenantWorkflow",
		WorkflowID:   fmt.Sprintf("tenant-%s", id),
		Arg:          id,
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
		SELECT 'webroots' AS resource_type, status, count(*) FROM webroots WHERE tenant_id = $1 AND status != 'deleted' GROUP BY status
		UNION ALL
		SELECT 'fqdns', f.status, count(*) FROM fqdns f JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 AND f.status != 'deleted' GROUP BY f.status
		UNION ALL
		SELECT 'certificates', c.status, count(*) FROM certificates c JOIN fqdns f ON f.id = c.fqdn_id JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 AND c.status != 'deleted' GROUP BY c.status
		UNION ALL
		SELECT 'email_accounts', ea.status, count(*) FROM email_accounts ea JOIN fqdns f ON f.id = ea.fqdn_id JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 AND ea.status != 'deleted' GROUP BY ea.status
		UNION ALL
		SELECT 'email_aliases', al.status, count(*) FROM email_aliases al JOIN email_accounts ea ON ea.id = al.email_account_id JOIN fqdns f ON f.id = ea.fqdn_id JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 AND al.status != 'deleted' GROUP BY al.status
		UNION ALL
		SELECT 'email_forwards', ef.status, count(*) FROM email_forwards ef JOIN email_accounts ea ON ea.id = ef.email_account_id JOIN fqdns f ON f.id = ea.fqdn_id JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 AND ef.status != 'deleted' GROUP BY ef.status
		UNION ALL
		SELECT 'email_autoreplies', ar.status, count(*) FROM email_autoreplies ar JOIN email_accounts ea ON ea.id = ar.email_account_id JOIN fqdns f ON f.id = ea.fqdn_id JOIN webroots w ON w.id = f.webroot_id WHERE w.tenant_id = $1 AND ar.status != 'deleted' GROUP BY ar.status
		UNION ALL
		SELECT 'databases', status, count(*) FROM databases WHERE tenant_id = $1 AND status != 'deleted' GROUP BY status
		UNION ALL
		SELECT 'database_users', du.status, count(*) FROM database_users du JOIN databases d ON d.id = du.database_id WHERE d.tenant_id = $1 AND du.status != 'deleted' GROUP BY du.status
		UNION ALL
		SELECT 'zones', status, count(*) FROM zones WHERE tenant_id = $1 AND status != 'deleted' GROUP BY status
		UNION ALL
		SELECT 'zone_records', zr.status, count(*) FROM zone_records zr JOIN zones z ON z.id = zr.zone_id WHERE z.tenant_id = $1 AND zr.status != 'deleted' GROUP BY zr.status
		UNION ALL
		SELECT 'valkey_instances', status, count(*) FROM valkey_instances WHERE tenant_id = $1 AND status != 'deleted' GROUP BY status
		UNION ALL
		SELECT 'valkey_users', vu.status, count(*) FROM valkey_users vu JOIN valkey_instances vi ON vi.id = vu.valkey_instance_id WHERE vi.tenant_id = $1 AND vu.status != 'deleted' GROUP BY vu.status
		UNION ALL
		SELECT 'sftp_keys', status, count(*) FROM sftp_keys WHERE tenant_id = $1 AND status != 'deleted' GROUP BY status
		UNION ALL
		SELECT 'backups', status, count(*) FROM backups WHERE tenant_id = $1 AND status != 'deleted' GROUP BY status`

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
		SFTPKeys:         counts["sftp_keys"],
		Backups:          counts["backups"],
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
	ensureMap(&summary.SFTPKeys)
	ensureMap(&summary.Backups)

	// Compute aggregates.
	for _, m := range []model.ResourceStatusCounts{
		summary.Webroots, summary.FQDNs, summary.Certificates,
		summary.EmailAccounts, summary.EmailAliases, summary.EmailForwards, summary.EmailAutoReplies,
		summary.Databases, summary.DatabaseUsers,
		summary.Zones, summary.ZoneRecords,
		summary.ValkeyInstances, summary.ValkeyUsers,
		summary.SFTPKeys, summary.Backups,
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
