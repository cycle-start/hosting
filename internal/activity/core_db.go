package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
}

// CoreDB contains activities that read from and update the core database.
type CoreDB struct {
	db DB
}

// NewCoreDB creates a new CoreDB activity struct.
func NewCoreDB(db DB) *CoreDB {
	return &CoreDB{db: db}
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

// ListValkeyInstancesByTenantID retrieves all valkey instances for a tenant.
func (a *CoreDB) ListValkeyInstancesByTenantID(ctx context.Context, tenantID string) ([]model.ValkeyInstance, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, name, shard_id, port, max_memory_mb, password, status, status_message, suspend_reason, created_at, updated_at
		 FROM valkey_instances WHERE tenant_id = $1`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list valkey instances by tenant: %w", err)
	}
	defer rows.Close()

	var instances []model.ValkeyInstance
	for rows.Next() {
		var v model.ValkeyInstance
		if err := rows.Scan(&v.ID, &v.TenantID, &v.Name, &v.ShardID, &v.Port, &v.MaxMemoryMB,
			&v.Password, &v.Status, &v.StatusMessage, &v.SuspendReason, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan valkey instance row: %w", err)
		}
		instances = append(instances, v)
	}
	return instances, rows.Err()
}

// ListS3BucketsByTenantID retrieves all S3 buckets for a tenant.
func (a *CoreDB) ListS3BucketsByTenantID(ctx context.Context, tenantID string) ([]model.S3Bucket, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, name, shard_id, public, quota_bytes, status, status_message, suspend_reason, created_at, updated_at
		 FROM s3_buckets WHERE tenant_id = $1`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list s3 buckets by tenant: %w", err)
	}
	defer rows.Close()

	var buckets []model.S3Bucket
	for rows.Next() {
		var b model.S3Bucket
		if err := rows.Scan(&b.ID, &b.TenantID, &b.Name, &b.ShardID,
			&b.Public, &b.QuotaBytes, &b.Status, &b.StatusMessage, &b.SuspendReason, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan s3 bucket row: %w", err)
		}
		buckets = append(buckets, b)
	}
	return buckets, rows.Err()
}

// ListZonesByTenantID retrieves all zones for a tenant.
func (a *CoreDB) ListZonesByTenantID(ctx context.Context, tenantID string) ([]model.Zone, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, brand_id, tenant_id, name, region_id, status, status_message, suspend_reason, created_at, updated_at
		 FROM zones WHERE tenant_id = $1`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list zones by tenant: %w", err)
	}
	defer rows.Close()

	var zones []model.Zone
	for rows.Next() {
		var z model.Zone
		if err := rows.Scan(&z.ID, &z.BrandID, &z.TenantID, &z.Name, &z.RegionID, &z.Status, &z.StatusMessage, &z.SuspendReason, &z.CreatedAt, &z.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan zone row: %w", err)
		}
		zones = append(zones, z)
	}
	return zones, rows.Err()
}

// GetBrandByID retrieves a brand by its ID.
func (a *CoreDB) GetBrandByID(ctx context.Context, id string) (*model.Brand, error) {
	var b model.Brand
	err := a.db.QueryRow(ctx,
		`SELECT id, name, base_hostname, primary_ns, secondary_ns, hostmaster_email, mail_hostname, spf_includes, dkim_selector, dkim_public_key, dmarc_policy, status, created_at, updated_at
		 FROM brands WHERE id = $1`, id,
	).Scan(&b.ID, &b.Name, &b.BaseHostname, &b.PrimaryNS, &b.SecondaryNS,
		&b.HostmasterEmail, &b.MailHostname, &b.SPFIncludes, &b.DKIMSelector,
		&b.DKIMPublicKey, &b.DMARCPolicy, &b.Status, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get brand by id: %w", err)
	}
	return &b, nil
}

// GetTenantByID retrieves a tenant by its ID.
func (a *CoreDB) GetTenantByID(ctx context.Context, id string) (*model.Tenant, error) {
	var t model.Tenant
	err := a.db.QueryRow(ctx,
		`SELECT id, name, brand_id, region_id, cluster_id, shard_id, uid, sftp_enabled, ssh_enabled, disk_quota_bytes, status, status_message, suspend_reason, created_at, updated_at
		 FROM tenants WHERE id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.BrandID, &t.RegionID, &t.ClusterID, &t.ShardID, &t.UID, &t.SFTPEnabled, &t.SSHEnabled, &t.DiskQuotaBytes, &t.Status, &t.StatusMessage, &t.SuspendReason, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get tenant by id: %w", err)
	}
	return &t, nil
}

// GetWebrootByID retrieves a webroot by its ID.
func (a *CoreDB) GetWebrootByID(ctx context.Context, id string) (*model.Webroot, error) {
	var w model.Webroot
	err := a.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, runtime, runtime_version, runtime_config, public_folder, status, status_message, suspend_reason, created_at, updated_at
		 FROM webroots WHERE id = $1`, id,
	).Scan(&w.ID, &w.TenantID, &w.Name, &w.Runtime, &w.RuntimeVersion, &w.RuntimeConfig, &w.PublicFolder, &w.Status, &w.StatusMessage, &w.SuspendReason, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get webroot by id: %w", err)
	}
	return &w, nil
}

// GetFQDNByID retrieves an FQDN by its ID.
func (a *CoreDB) GetFQDNByID(ctx context.Context, id string) (*model.FQDN, error) {
	var f model.FQDN
	err := a.db.QueryRow(ctx,
		`SELECT id, fqdn, webroot_id, ssl_enabled, status, status_message, created_at, updated_at
		 FROM fqdns WHERE id = $1`, id,
	).Scan(&f.ID, &f.FQDN, &f.WebrootID, &f.SSLEnabled, &f.Status, &f.StatusMessage, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get fqdn by id: %w", err)
	}
	return &f, nil
}

// GetZoneByID retrieves a zone by its ID.
func (a *CoreDB) GetZoneByID(ctx context.Context, id string) (*model.Zone, error) {
	var z model.Zone
	err := a.db.QueryRow(ctx,
		`SELECT id, brand_id, tenant_id, name, region_id, status, status_message, suspend_reason, created_at, updated_at
		 FROM zones WHERE id = $1`, id,
	).Scan(&z.ID, &z.BrandID, &z.TenantID, &z.Name, &z.RegionID, &z.Status, &z.StatusMessage, &z.SuspendReason, &z.CreatedAt, &z.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get zone by id: %w", err)
	}
	return &z, nil
}

// GetZoneByName retrieves a zone by its name. Used for auto-DNS lookups.
func (a *CoreDB) GetZoneByName(ctx context.Context, name string) (*model.Zone, error) {
	var z model.Zone
	err := a.db.QueryRow(ctx,
		`SELECT id, brand_id, tenant_id, name, region_id, status, status_message, suspend_reason, created_at, updated_at
		 FROM zones WHERE name = $1 AND status = $2`, name, model.StatusActive,
	).Scan(&z.ID, &z.BrandID, &z.TenantID, &z.Name, &z.RegionID, &z.Status, &z.StatusMessage, &z.SuspendReason, &z.CreatedAt, &z.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get zone by name: %w", err)
	}
	return &z, nil
}

// GetZoneRecordByID retrieves a zone record by its ID.
func (a *CoreDB) GetZoneRecordByID(ctx context.Context, id string) (*model.ZoneRecord, error) {
	var r model.ZoneRecord
	err := a.db.QueryRow(ctx,
		`SELECT id, zone_id, type, name, content, ttl, priority, managed_by, source_fqdn_id, status, status_message, created_at, updated_at
		 FROM zone_records WHERE id = $1`, id,
	).Scan(&r.ID, &r.ZoneID, &r.Type, &r.Name, &r.Content, &r.TTL, &r.Priority, &r.ManagedBy, &r.SourceFQDNID, &r.Status, &r.StatusMessage, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get zone record by id: %w", err)
	}
	return &r, nil
}

// GetDatabaseByID retrieves a database by its ID.
func (a *CoreDB) GetDatabaseByID(ctx context.Context, id string) (*model.Database, error) {
	var d model.Database
	err := a.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, shard_id, node_id, status, status_message, suspend_reason, created_at, updated_at
		 FROM databases WHERE id = $1`, id,
	).Scan(&d.ID, &d.TenantID, &d.Name, &d.ShardID, &d.NodeID, &d.Status, &d.StatusMessage, &d.SuspendReason, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get database by id: %w", err)
	}
	return &d, nil
}

// GetDatabaseUserByID retrieves a database user by its ID.
func (a *CoreDB) GetDatabaseUserByID(ctx context.Context, id string) (*model.DatabaseUser, error) {
	var u model.DatabaseUser
	err := a.db.QueryRow(ctx,
		`SELECT id, database_id, username, password, privileges, status, status_message, created_at, updated_at
		 FROM database_users WHERE id = $1`, id,
	).Scan(&u.ID, &u.DatabaseID, &u.Username, &u.Password, &u.Privileges, &u.Status, &u.StatusMessage, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get database user by id: %w", err)
	}
	return &u, nil
}

// GetCertificateByID retrieves a certificate by its ID.
func (a *CoreDB) GetCertificateByID(ctx context.Context, id string) (*model.Certificate, error) {
	var c model.Certificate
	err := a.db.QueryRow(ctx,
		`SELECT id, fqdn_id, type, cert_pem, key_pem, chain_pem, issued_at, expires_at, status, status_message, is_active, created_at, updated_at
		 FROM certificates WHERE id = $1`, id,
	).Scan(&c.ID, &c.FQDNID, &c.Type, &c.CertPEM, &c.KeyPEM, &c.ChainPEM, &c.IssuedAt, &c.ExpiresAt, &c.Status, &c.StatusMessage, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get certificate by id: %w", err)
	}
	return &c, nil
}

// GetClusterByID retrieves a cluster by its ID.
func (a *CoreDB) GetClusterByID(ctx context.Context, id string) (*model.Cluster, error) {
	var c model.Cluster
	err := a.db.QueryRow(ctx,
		`SELECT id, region_id, name, config, status, spec, created_at, updated_at
		 FROM clusters WHERE id = $1`, id,
	).Scan(&c.ID, &c.RegionID, &c.Name, &c.Config, &c.Status, &c.Spec, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get cluster by id: %w", err)
	}
	return &c, nil
}

// GetClusterLBAddresses retrieves all LB addresses for a cluster.
func (a *CoreDB) GetClusterLBAddresses(ctx context.Context, clusterID string) ([]model.ClusterLBAddress, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, cluster_id, address::text, family, label, created_at
		 FROM cluster_lb_addresses WHERE cluster_id = $1 ORDER BY family, address`, clusterID)
	if err != nil {
		return nil, fmt.Errorf("get cluster LB addresses: %w", err)
	}
	defer rows.Close()
	var addrs []model.ClusterLBAddress
	for rows.Next() {
		var a model.ClusterLBAddress
		if err := rows.Scan(&a.ID, &a.ClusterID, &a.Address, &a.Family, &a.Label, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan cluster LB address: %w", err)
		}
		addrs = append(addrs, a)
	}
	return addrs, nil
}

// GetShardByID retrieves a shard by its ID.
func (a *CoreDB) GetShardByID(ctx context.Context, id string) (*model.Shard, error) {
	var s model.Shard
	err := a.db.QueryRow(ctx,
		`SELECT id, cluster_id, name, role, lb_backend, config, status, status_message, created_at, updated_at
		 FROM shards WHERE id = $1`, id,
	).Scan(&s.ID, &s.ClusterID, &s.Name, &s.Role, &s.LBBackend, &s.Config, &s.Status, &s.StatusMessage, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get shard by id: %w", err)
	}
	return &s, nil
}

// GetNodesByClusterAndRole retrieves all nodes in a cluster with the specified role.
func (a *CoreDB) GetNodesByClusterAndRole(ctx context.Context, clusterID string, role string) ([]model.Node, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, cluster_id, hostname, ip_address::text, ip6_address::text, roles, status, created_at, updated_at
		 FROM nodes WHERE cluster_id = $1 AND $2 = ANY(roles) AND status = $3`, clusterID, role, model.StatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("get nodes by cluster and role: %w", err)
	}
	defer rows.Close()

	var nodes []model.Node
	for rows.Next() {
		var n model.Node
		if err := rows.Scan(&n.ID, &n.ClusterID, &n.Hostname, &n.IPAddress, &n.IP6Address, &n.Roles, &n.Status, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan node row: %w", err)
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// GetFQDNsByWebrootID retrieves all FQDNs bound to a webroot.
func (a *CoreDB) GetFQDNsByWebrootID(ctx context.Context, webrootID string) ([]model.FQDN, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, fqdn, webroot_id, ssl_enabled, status, status_message, created_at, updated_at
		 FROM fqdns WHERE webroot_id = $1`, webrootID,
	)
	if err != nil {
		return nil, fmt.Errorf("get fqdns by webroot id: %w", err)
	}
	defer rows.Close()

	var fqdns []model.FQDN
	for rows.Next() {
		var f model.FQDN
		if err := rows.Scan(&f.ID, &f.FQDN, &f.WebrootID, &f.SSLEnabled, &f.Status, &f.StatusMessage, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan fqdn row: %w", err)
		}
		fqdns = append(fqdns, f)
	}
	return fqdns, rows.Err()
}

// CreateCertificateParams holds parameters for creating a certificate record.
type CreateCertificateParams struct {
	ID     string
	FQDNID string
	Type   string
}

// CreateCertificate inserts a new certificate row in pending state.
func (a *CoreDB) CreateCertificate(ctx context.Context, params CreateCertificateParams) error {
	_, err := a.db.Exec(ctx,
		`INSERT INTO certificates (id, fqdn_id, type, status, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, false, now(), now())`,
		params.ID, params.FQDNID, params.Type, model.StatusPending,
	)
	return err
}

// ListTenantsByShard retrieves all tenants assigned to a shard.
func (a *CoreDB) ListTenantsByShard(ctx context.Context, shardID string) ([]model.Tenant, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, name, brand_id, region_id, cluster_id, shard_id, uid, sftp_enabled, ssh_enabled, disk_quota_bytes, status, status_message, suspend_reason, created_at, updated_at
		 FROM tenants WHERE shard_id = $1 ORDER BY id`, shardID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tenants by shard: %w", err)
	}
	defer rows.Close()

	var tenants []model.Tenant
	for rows.Next() {
		var t model.Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.BrandID, &t.RegionID, &t.ClusterID, &t.ShardID, &t.UID, &t.SFTPEnabled, &t.SSHEnabled, &t.DiskQuotaBytes, &t.Status, &t.StatusMessage, &t.SuspendReason, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan tenant row: %w", err)
		}
		tenants = append(tenants, t)
	}
	return tenants, rows.Err()
}

// ListNodesByShard retrieves all nodes assigned to a shard via the join table.
func (a *CoreDB) ListNodesByShard(ctx context.Context, shardID string) ([]model.Node, error) {
	rows, err := a.db.Query(ctx,
		`SELECT n.id, n.cluster_id, n.hostname, n.ip_address::text, n.ip6_address::text, n.roles, n.status, n.created_at, n.updated_at,
		        nsa.shard_id, nsa.shard_index
		 FROM nodes n
		 JOIN node_shard_assignments nsa ON n.id = nsa.node_id
		 WHERE nsa.shard_id = $1
		 ORDER BY n.hostname`, shardID,
	)
	if err != nil {
		return nil, fmt.Errorf("list nodes by shard: %w", err)
	}
	defer rows.Close()

	var nodes []model.Node
	for rows.Next() {
		var n model.Node
		var joinShardID string
		var joinShardIndex int
		if err := rows.Scan(&n.ID, &n.ClusterID, &n.Hostname, &n.IPAddress, &n.IP6Address, &n.Roles, &n.Status, &n.CreatedAt, &n.UpdatedAt,
			&joinShardID, &joinShardIndex); err != nil {
			return nil, fmt.Errorf("scan node row: %w", err)
		}
		n.ShardID = &joinShardID
		n.ShardIndex = &joinShardIndex
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// GetTenantServicesByTenantID retrieves all services for a tenant.
func (a *CoreDB) GetTenantServicesByTenantID(ctx context.Context, tenantID string) ([]model.TenantService, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, service, node_id, hostname, enabled, status, created_at, updated_at
		 FROM tenant_services WHERE tenant_id = $1 AND enabled = true`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("get tenant services: %w", err)
	}
	defer rows.Close()

	var services []model.TenantService
	for rows.Next() {
		var s model.TenantService
		if err := rows.Scan(&s.ID, &s.TenantID, &s.Service, &s.NodeID, &s.Hostname, &s.Enabled, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan tenant service row: %w", err)
		}
		services = append(services, s)
	}
	return services, rows.Err()
}

// GetNodeByID retrieves a node by its ID.
func (a *CoreDB) GetNodeByID(ctx context.Context, id string) (*model.Node, error) {
	var n model.Node
	err := a.db.QueryRow(ctx,
		`SELECT id, cluster_id, hostname, ip_address::text, ip6_address::text, roles, status, created_at, updated_at
		 FROM nodes WHERE id = $1`, id,
	).Scan(&n.ID, &n.ClusterID, &n.Hostname, &n.IPAddress, &n.IP6Address,
		&n.Roles, &n.Status, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get node by id: %w", err)
	}
	return &n, nil
}


// UpdateTenantShardID updates the shard assignment for a tenant.
func (a *CoreDB) UpdateTenantShardID(ctx context.Context, tenantID string, shardID string) error {
	_, err := a.db.Exec(ctx,
		`UPDATE tenants SET shard_id = $1, updated_at = now() WHERE id = $2`, shardID, tenantID)
	return err
}

// UpdateDatabaseShardID updates the shard assignment for a database.
func (a *CoreDB) UpdateDatabaseShardID(ctx context.Context, databaseID string, shardID string) error {
	_, err := a.db.Exec(ctx,
		`UPDATE databases SET shard_id = $1, updated_at = now() WHERE id = $2`, shardID, databaseID)
	return err
}

// UpdateValkeyInstanceShardID updates the shard assignment for a valkey instance.
func (a *CoreDB) UpdateValkeyInstanceShardID(ctx context.Context, instanceID string, shardID string) error {
	_, err := a.db.Exec(ctx,
		`UPDATE valkey_instances SET shard_id = $1, updated_at = now() WHERE id = $2`, shardID, instanceID)
	return err
}

// ListWebrootsByTenantID retrieves all webroots for a tenant.
func (a *CoreDB) ListWebrootsByTenantID(ctx context.Context, tenantID string) ([]model.Webroot, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, name, runtime, runtime_version, runtime_config, public_folder, env_file_name, env_shell_source, service_hostname_enabled, status, status_message, suspend_reason, created_at, updated_at
		 FROM webroots WHERE tenant_id = $1`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list webroots by tenant: %w", err)
	}
	defer rows.Close()

	var webroots []model.Webroot
	for rows.Next() {
		var w model.Webroot
		if err := rows.Scan(&w.ID, &w.TenantID, &w.Name, &w.Runtime, &w.RuntimeVersion, &w.RuntimeConfig, &w.PublicFolder, &w.EnvFileName, &w.EnvShellSource, &w.ServiceHostnameEnabled, &w.Status, &w.StatusMessage, &w.SuspendReason, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan webroot row: %w", err)
		}
		webroots = append(webroots, w)
	}
	return webroots, rows.Err()
}

// ListDatabasesByTenantID retrieves all databases for a tenant.
func (a *CoreDB) ListDatabasesByTenantID(ctx context.Context, tenantID string) ([]model.Database, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, name, shard_id, node_id, status, status_message, suspend_reason, created_at, updated_at
		 FROM databases WHERE tenant_id = $1`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list databases by tenant: %w", err)
	}
	defer rows.Close()

	var databases []model.Database
	for rows.Next() {
		var d model.Database
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.ShardID, &d.NodeID, &d.Status, &d.StatusMessage, &d.SuspendReason, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan database row: %w", err)
		}
		databases = append(databases, d)
	}
	return databases, rows.Err()
}

// GetValkeyInstanceByID retrieves a valkey instance by its ID.
func (a *CoreDB) GetValkeyInstanceByID(ctx context.Context, id string) (*model.ValkeyInstance, error) {
	var v model.ValkeyInstance
	err := a.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, shard_id, port, max_memory_mb, password, status, status_message, suspend_reason, created_at, updated_at
		 FROM valkey_instances WHERE id = $1`, id,
	).Scan(&v.ID, &v.TenantID, &v.Name, &v.ShardID, &v.Port, &v.MaxMemoryMB,
		&v.Password, &v.Status, &v.StatusMessage, &v.SuspendReason, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get valkey instance by id: %w", err)
	}
	return &v, nil
}

// GetValkeyUserByID retrieves a valkey user by its ID.
func (a *CoreDB) GetValkeyUserByID(ctx context.Context, id string) (*model.ValkeyUser, error) {
	var u model.ValkeyUser
	err := a.db.QueryRow(ctx,
		`SELECT id, valkey_instance_id, username, password, privileges, key_pattern, status, status_message, created_at, updated_at
		 FROM valkey_users WHERE id = $1`, id,
	).Scan(&u.ID, &u.ValkeyInstanceID, &u.Username, &u.Password,
		&u.Privileges, &u.KeyPattern, &u.Status, &u.StatusMessage, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get valkey user by id: %w", err)
	}
	return &u, nil
}

// GetEmailAccountByID retrieves an email account by its ID.
func (a *CoreDB) GetEmailAccountByID(ctx context.Context, id string) (*model.EmailAccount, error) {
	var acct model.EmailAccount
	err := a.db.QueryRow(ctx,
		`SELECT id, fqdn_id, address, display_name, quota_bytes, status, status_message, created_at, updated_at
		 FROM email_accounts WHERE id = $1`, id,
	).Scan(&acct.ID, &acct.FQDNID, &acct.Address, &acct.DisplayName, &acct.QuotaBytes, &acct.Status, &acct.StatusMessage, &acct.CreatedAt, &acct.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get email account by id: %w", err)
	}
	return &acct, nil
}

// CountActiveEmailAccountsByFQDN returns the number of active (non-deleted) email accounts for an FQDN.
func (a *CoreDB) CountActiveEmailAccountsByFQDN(ctx context.Context, fqdnID string) (int, error) {
	var count int
	err := a.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM email_accounts WHERE fqdn_id = $1`, fqdnID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active email accounts by fqdn: %w", err)
	}
	return count, nil
}


// CreateShard inserts a new shard record.
func (a *CoreDB) CreateShard(ctx context.Context, s *model.Shard) error {
	_, err := a.db.Exec(ctx,
		`INSERT INTO shards (id, cluster_id, name, role, lb_backend, config, status, status_message, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		s.ID, s.ClusterID, s.Name, s.Role, s.LBBackend, s.Config, s.Status, s.StatusMessage, s.CreatedAt, s.UpdatedAt,
	)
	return err
}

// CreateNode inserts a new node record.
func (a *CoreDB) CreateNode(ctx context.Context, n *model.Node) error {
	_, err := a.db.Exec(ctx,
		`INSERT INTO nodes (id, cluster_id, hostname, ip_address, ip6_address, roles, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		n.ID, n.ClusterID, n.Hostname, n.IPAddress, n.IP6Address, n.Roles, n.Status, n.CreatedAt, n.UpdatedAt,
	)
	return err
}


// ListDatabasesByShard retrieves all databases assigned to a shard (excluding deleted).
func (a *CoreDB) ListDatabasesByShard(ctx context.Context, shardID string) ([]model.Database, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, name, shard_id, node_id, status, status_message, suspend_reason, created_at, updated_at
		 FROM databases WHERE shard_id = $1 ORDER BY name`, shardID,
	)
	if err != nil {
		return nil, fmt.Errorf("list databases by shard: %w", err)
	}
	defer rows.Close()

	var databases []model.Database
	for rows.Next() {
		var d model.Database
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.ShardID, &d.NodeID, &d.Status, &d.StatusMessage, &d.SuspendReason, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan database row: %w", err)
		}
		databases = append(databases, d)
	}
	return databases, rows.Err()
}

// ListValkeyInstancesByShard retrieves all valkey instances assigned to a shard (excluding deleted).
func (a *CoreDB) ListValkeyInstancesByShard(ctx context.Context, shardID string) ([]model.ValkeyInstance, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, name, shard_id, port, max_memory_mb, password, status, status_message, suspend_reason, created_at, updated_at
		 FROM valkey_instances WHERE shard_id = $1 ORDER BY name`, shardID,
	)
	if err != nil {
		return nil, fmt.Errorf("list valkey instances by shard: %w", err)
	}
	defer rows.Close()

	var instances []model.ValkeyInstance
	for rows.Next() {
		var v model.ValkeyInstance
		if err := rows.Scan(&v.ID, &v.TenantID, &v.Name, &v.ShardID, &v.Port, &v.MaxMemoryMB, &v.Password, &v.Status, &v.StatusMessage, &v.SuspendReason, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan valkey instance row: %w", err)
		}
		instances = append(instances, v)
	}
	return instances, rows.Err()
}

// ListDatabaseUsersByDatabaseID retrieves all users for a database (excluding deleted).
func (a *CoreDB) ListDatabaseUsersByDatabaseID(ctx context.Context, databaseID string) ([]model.DatabaseUser, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, database_id, username, password, privileges, status, status_message, created_at, updated_at
		 FROM database_users WHERE database_id = $1`, databaseID,
	)
	if err != nil {
		return nil, fmt.Errorf("list database users by database: %w", err)
	}
	defer rows.Close()

	var users []model.DatabaseUser
	for rows.Next() {
		var u model.DatabaseUser
		if err := rows.Scan(&u.ID, &u.DatabaseID, &u.Username, &u.Password, &u.Privileges, &u.Status, &u.StatusMessage, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan database user row: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// ListValkeyUsersByInstanceID retrieves all users for a valkey instance (excluding deleted).
func (a *CoreDB) ListValkeyUsersByInstanceID(ctx context.Context, instanceID string) ([]model.ValkeyUser, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, valkey_instance_id, username, password, privileges, key_pattern, status, status_message, created_at, updated_at
		 FROM valkey_users WHERE valkey_instance_id = $1`, instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list valkey users by instance: %w", err)
	}
	defer rows.Close()

	var users []model.ValkeyUser
	for rows.Next() {
		var u model.ValkeyUser
		if err := rows.Scan(&u.ID, &u.ValkeyInstanceID, &u.Username, &u.Password, &u.Privileges, &u.KeyPattern, &u.Status, &u.StatusMessage, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan valkey user row: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// GetEmailAliasByID retrieves an email alias by its ID.
func (a *CoreDB) GetEmailAliasByID(ctx context.Context, id string) (*model.EmailAlias, error) {
	var alias model.EmailAlias
	err := a.db.QueryRow(ctx,
		`SELECT id, email_account_id, address, status, status_message, created_at, updated_at
		 FROM email_aliases WHERE id = $1`, id,
	).Scan(&alias.ID, &alias.EmailAccountID, &alias.Address, &alias.Status, &alias.StatusMessage, &alias.CreatedAt, &alias.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get email alias by id: %w", err)
	}
	return &alias, nil
}

// GetEmailForwardByID retrieves an email forward by its ID.
func (a *CoreDB) GetEmailForwardByID(ctx context.Context, id string) (*model.EmailForward, error) {
	var fwd model.EmailForward
	err := a.db.QueryRow(ctx,
		`SELECT id, email_account_id, destination, keep_copy, status, status_message, created_at, updated_at
		 FROM email_forwards WHERE id = $1`, id,
	).Scan(&fwd.ID, &fwd.EmailAccountID, &fwd.Destination, &fwd.KeepCopy, &fwd.Status, &fwd.StatusMessage, &fwd.CreatedAt, &fwd.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get email forward by id: %w", err)
	}
	return &fwd, nil
}

// GetEmailAutoReplyByAccountID retrieves the auto-reply for an email account.
func (a *CoreDB) GetEmailAutoReplyByAccountID(ctx context.Context, accountID string) (*model.EmailAutoReply, error) {
	var ar model.EmailAutoReply
	err := a.db.QueryRow(ctx,
		`SELECT id, email_account_id, subject, body, start_date, end_date, enabled, status, status_message, created_at, updated_at
		 FROM email_autoreplies WHERE email_account_id = $1`, accountID,
	).Scan(&ar.ID, &ar.EmailAccountID, &ar.Subject, &ar.Body, &ar.StartDate, &ar.EndDate, &ar.Enabled, &ar.Status, &ar.StatusMessage, &ar.CreatedAt, &ar.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get email autoreply by account id: %w", err)
	}
	return &ar, nil
}

// GetEmailAutoReplyByID retrieves an email auto-reply by its ID.
func (a *CoreDB) GetEmailAutoReplyByID(ctx context.Context, id string) (*model.EmailAutoReply, error) {
	var ar model.EmailAutoReply
	err := a.db.QueryRow(ctx,
		`SELECT id, email_account_id, subject, body, start_date, end_date, enabled, status, status_message, created_at, updated_at
		 FROM email_autoreplies WHERE id = $1`, id,
	).Scan(&ar.ID, &ar.EmailAccountID, &ar.Subject, &ar.Body, &ar.StartDate, &ar.EndDate, &ar.Enabled, &ar.Status, &ar.StatusMessage, &ar.CreatedAt, &ar.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get email autoreply by id: %w", err)
	}
	return &ar, nil
}

// GetExpiringLECerts returns Let's Encrypt certificates expiring within the given number of days.
func (a *CoreDB) GetExpiringLECerts(ctx context.Context, daysBeforeExpiry int) ([]model.Certificate, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, fqdn_id, type, cert_pem, key_pem, chain_pem, issued_at, expires_at, status, status_message, is_active, created_at, updated_at
		 FROM certificates
		 WHERE type = $1 AND status = $2 AND is_active = true
		   AND expires_at <= now() + make_interval(days => $3)
		 ORDER BY expires_at ASC`,
		model.CertTypeLetsEncrypt, model.StatusActive, daysBeforeExpiry,
	)
	if err != nil {
		return nil, fmt.Errorf("get expiring LE certs: %w", err)
	}
	defer rows.Close()

	var certs []model.Certificate
	for rows.Next() {
		var c model.Certificate
		if err := rows.Scan(&c.ID, &c.FQDNID, &c.Type, &c.CertPEM, &c.KeyPEM, &c.ChainPEM,
			&c.IssuedAt, &c.ExpiresAt, &c.Status, &c.StatusMessage, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan expiring cert: %w", err)
		}
		certs = append(certs, c)
	}
	return certs, rows.Err()
}

// GetExpiredCerts returns certificates that have been expired for more than the given number of days.
func (a *CoreDB) GetExpiredCerts(ctx context.Context, daysAfterExpiry int) ([]model.Certificate, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, fqdn_id, type, cert_pem, key_pem, chain_pem, issued_at, expires_at, status, status_message, is_active, created_at, updated_at
		 FROM certificates
		 WHERE status = $1
		   AND expires_at < now() - make_interval(days => $2)
		 ORDER BY expires_at ASC`,
		model.StatusActive, daysAfterExpiry,
	)
	if err != nil {
		return nil, fmt.Errorf("get expired certs: %w", err)
	}
	defer rows.Close()

	var certs []model.Certificate
	for rows.Next() {
		var c model.Certificate
		if err := rows.Scan(&c.ID, &c.FQDNID, &c.Type, &c.CertPEM, &c.KeyPEM, &c.ChainPEM,
			&c.IssuedAt, &c.ExpiresAt, &c.Status, &c.StatusMessage, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan expired cert: %w", err)
		}
		certs = append(certs, c)
	}
	return certs, rows.Err()
}

// GetSSHKeyByID retrieves an SSH key by its ID.
func (a *CoreDB) GetSSHKeyByID(ctx context.Context, id string) (*model.SSHKey, error) {
	var k model.SSHKey
	err := a.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, public_key, fingerprint, status, status_message, created_at, updated_at
		 FROM ssh_keys WHERE id = $1`, id,
	).Scan(&k.ID, &k.TenantID, &k.Name, &k.PublicKey, &k.Fingerprint,
		&k.Status, &k.StatusMessage, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get SSH key by id: %w", err)
	}
	return &k, nil
}

// GetSSHKeysByTenant retrieves all active SSH keys for a tenant.
func (a *CoreDB) GetSSHKeysByTenant(ctx context.Context, tenantID string) ([]model.SSHKey, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, name, public_key, fingerprint, status, status_message, created_at, updated_at
		 FROM ssh_keys WHERE tenant_id = $1 AND status = $2`, tenantID, model.StatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("get SSH keys by tenant: %w", err)
	}
	defer rows.Close()

	var keys []model.SSHKey
	for rows.Next() {
		var k model.SSHKey
		if err := rows.Scan(&k.ID, &k.TenantID, &k.Name, &k.PublicKey, &k.Fingerprint,
			&k.Status, &k.StatusMessage, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan SSH key row: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// GetPlatformConfig retrieves a platform configuration value by key.
func (a *CoreDB) GetPlatformConfig(ctx context.Context, key string) (string, error) {
	var value string
	err := a.db.QueryRow(ctx, `SELECT value FROM platform_config WHERE key = $1`, key).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("get platform config %q: %w", key, err)
	}
	return value, nil
}

// GetBackupByID retrieves a backup by its ID.
func (a *CoreDB) GetBackupByID(ctx context.Context, id string) (*model.Backup, error) {
	var b model.Backup
	err := a.db.QueryRow(ctx,
		`SELECT id, tenant_id, type, source_id, source_name, storage_path, size_bytes, status, status_message, started_at, completed_at, created_at, updated_at
		 FROM backups WHERE id = $1`, id,
	).Scan(&b.ID, &b.TenantID, &b.Type, &b.SourceID, &b.SourceName,
		&b.StoragePath, &b.SizeBytes, &b.Status, &b.StatusMessage, &b.StartedAt,
		&b.CompletedAt, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get backup by id: %w", err)
	}
	return &b, nil
}

// UpdateBackupResultParams holds the parameters for UpdateBackupResult.
type UpdateBackupResultParams struct {
	ID          string
	StoragePath string
	SizeBytes   int64
	StartedAt   time.Time
	CompletedAt time.Time
}

// UpdateBackupResult updates a backup with its result after completion.
func (a *CoreDB) UpdateBackupResult(ctx context.Context, params UpdateBackupResultParams) error {
	_, err := a.db.Exec(ctx,
		`UPDATE backups SET storage_path = $1, size_bytes = $2, started_at = $3, completed_at = $4, updated_at = now() WHERE id = $5`,
		params.StoragePath, params.SizeBytes, params.StartedAt, params.CompletedAt, params.ID,
	)
	if err != nil {
		return fmt.Errorf("update backup result: %w", err)
	}
	return nil
}

// DeleteOldAuditLogs deletes audit log entries older than the specified number of days
// and returns the count of deleted rows.
func (a *CoreDB) DeleteOldAuditLogs(ctx context.Context, retentionDays int) (int64, error) {
	tag, err := a.db.Exec(ctx,
		"DELETE FROM audit_logs WHERE created_at < now() - make_interval(days => $1)", retentionDays)
	if err != nil {
		return 0, fmt.Errorf("delete old audit logs: %w", err)
	}
	return tag.RowsAffected(), nil
}

// GetS3BucketByID retrieves an S3 bucket by its ID.
func (a *CoreDB) GetS3BucketByID(ctx context.Context, id string) (*model.S3Bucket, error) {
	var b model.S3Bucket
	err := a.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, shard_id, public, quota_bytes, status, status_message, suspend_reason, created_at, updated_at
		 FROM s3_buckets WHERE id = $1`, id,
	).Scan(&b.ID, &b.TenantID, &b.Name, &b.ShardID,
		&b.Public, &b.QuotaBytes, &b.Status, &b.StatusMessage, &b.SuspendReason, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get s3 bucket by id: %w", err)
	}
	return &b, nil
}

// GetS3AccessKeyByID retrieves an S3 access key by its ID.
func (a *CoreDB) GetS3AccessKeyByID(ctx context.Context, id string) (*model.S3AccessKey, error) {
	var k model.S3AccessKey
	err := a.db.QueryRow(ctx,
		`SELECT id, s3_bucket_id, access_key_id, secret_access_key, permissions, status, status_message, created_at, updated_at
		 FROM s3_access_keys WHERE id = $1`, id,
	).Scan(&k.ID, &k.S3BucketID, &k.AccessKeyID, &k.SecretAccessKey,
		&k.Permissions, &k.Status, &k.StatusMessage, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get s3 access key by id: %w", err)
	}
	return &k, nil
}

// GetWebrootContext fetches a webroot and its related tenant, FQDNs, and nodes in a single activity.
func (a *CoreDB) GetWebrootContext(ctx context.Context, webrootID string) (*WebrootContext, error) {
	var wc WebrootContext

	// JOIN webroots with tenants and brands.
	err := a.db.QueryRow(ctx,
		`SELECT w.id, w.tenant_id, w.name, w.runtime, w.runtime_version, w.runtime_config, w.public_folder, w.env_file_name, w.env_shell_source, w.service_hostname_enabled, w.status, w.status_message, w.suspend_reason, w.created_at, w.updated_at,
		        t.id, t.name, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.ssh_enabled, t.disk_quota_bytes, t.status, t.status_message, t.suspend_reason, t.created_at, t.updated_at,
		        b.base_hostname
		 FROM webroots w
		 JOIN tenants t ON t.id = w.tenant_id
		 JOIN brands b ON b.id = t.brand_id
		 WHERE w.id = $1`, webrootID,
	).Scan(&wc.Webroot.ID, &wc.Webroot.TenantID, &wc.Webroot.Name, &wc.Webroot.Runtime, &wc.Webroot.RuntimeVersion, &wc.Webroot.RuntimeConfig, &wc.Webroot.PublicFolder, &wc.Webroot.EnvFileName, &wc.Webroot.EnvShellSource, &wc.Webroot.ServiceHostnameEnabled, &wc.Webroot.Status, &wc.Webroot.StatusMessage, &wc.Webroot.SuspendReason, &wc.Webroot.CreatedAt, &wc.Webroot.UpdatedAt,
		&wc.Tenant.ID, &wc.Tenant.Name, &wc.Tenant.BrandID, &wc.Tenant.RegionID, &wc.Tenant.ClusterID, &wc.Tenant.ShardID, &wc.Tenant.UID, &wc.Tenant.SFTPEnabled, &wc.Tenant.SSHEnabled, &wc.Tenant.DiskQuotaBytes, &wc.Tenant.Status, &wc.Tenant.StatusMessage, &wc.Tenant.SuspendReason, &wc.Tenant.CreatedAt, &wc.Tenant.UpdatedAt,
		&wc.BrandBaseHostname)
	if err != nil {
		return nil, fmt.Errorf("get webroot context: %w", err)
	}

	// Fetch FQDNs for this webroot.
	fqdns, err := a.GetFQDNsByWebrootID(ctx, webrootID)
	if err != nil {
		return nil, err
	}
	wc.FQDNs = fqdns

	// Fetch nodes if tenant has a shard.
	if wc.Tenant.ShardID != nil {
		nodes, err := a.ListNodesByShard(ctx, *wc.Tenant.ShardID)
		if err != nil {
			return nil, err
		}
		wc.Nodes = nodes
	}

	// Fetch shard details and LB info for service hostname support.
	if wc.Tenant.ShardID != nil {
		shard, err := a.GetShardByID(ctx, *wc.Tenant.ShardID)
		if err != nil {
			return nil, fmt.Errorf("get shard for webroot context: %w", err)
		}
		wc.Shard = *shard

		lbAddresses, err := a.GetClusterLBAddresses(ctx, wc.Shard.ClusterID)
		if err != nil {
			return nil, fmt.Errorf("list lb addresses: %w", err)
		}
		wc.LBAddresses = lbAddresses

		lbNodes, err := a.GetNodesByClusterAndRole(ctx, wc.Shard.ClusterID, model.ShardRoleLB)
		if err != nil {
			return nil, fmt.Errorf("list lb nodes: %w", err)
		}
		wc.LBNodes = lbNodes
	}

	return &wc, nil
}

// GetFQDNContext fetches an FQDN and its related webroot, tenant, shard, nodes, and LB addresses.
func (a *CoreDB) GetFQDNContext(ctx context.Context, fqdnID string) (*FQDNContext, error) {
	var fc FQDNContext

	// JOIN fqdns -> webroots -> tenants -> brands.
	err := a.db.QueryRow(ctx,
		`SELECT f.id, f.fqdn, f.webroot_id, f.ssl_enabled, f.status, f.status_message, f.created_at, f.updated_at,
		        w.id, w.tenant_id, w.name, w.runtime, w.runtime_version, w.runtime_config, w.public_folder, w.env_file_name, w.env_shell_source, w.service_hostname_enabled, w.status, w.status_message, w.suspend_reason, w.created_at, w.updated_at,
		        t.id, t.name, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.ssh_enabled, t.disk_quota_bytes, t.status, t.status_message, t.suspend_reason, t.created_at, t.updated_at,
		        b.base_hostname
		 FROM fqdns f
		 JOIN webroots w ON w.id = f.webroot_id
		 JOIN tenants t ON t.id = w.tenant_id
		 JOIN brands b ON b.id = t.brand_id
		 WHERE f.id = $1`, fqdnID,
	).Scan(&fc.FQDN.ID, &fc.FQDN.FQDN, &fc.FQDN.WebrootID, &fc.FQDN.SSLEnabled, &fc.FQDN.Status, &fc.FQDN.StatusMessage, &fc.FQDN.CreatedAt, &fc.FQDN.UpdatedAt,
		&fc.Webroot.ID, &fc.Webroot.TenantID, &fc.Webroot.Name, &fc.Webroot.Runtime, &fc.Webroot.RuntimeVersion, &fc.Webroot.RuntimeConfig, &fc.Webroot.PublicFolder, &fc.Webroot.EnvFileName, &fc.Webroot.EnvShellSource, &fc.Webroot.ServiceHostnameEnabled, &fc.Webroot.Status, &fc.Webroot.StatusMessage, &fc.Webroot.SuspendReason, &fc.Webroot.CreatedAt, &fc.Webroot.UpdatedAt,
		&fc.Tenant.ID, &fc.Tenant.Name, &fc.Tenant.BrandID, &fc.Tenant.RegionID, &fc.Tenant.ClusterID, &fc.Tenant.ShardID, &fc.Tenant.UID, &fc.Tenant.SFTPEnabled, &fc.Tenant.SSHEnabled, &fc.Tenant.DiskQuotaBytes, &fc.Tenant.Status, &fc.Tenant.StatusMessage, &fc.Tenant.SuspendReason, &fc.Tenant.CreatedAt, &fc.Tenant.UpdatedAt,
		&fc.BrandBaseHostname)
	if err != nil {
		return nil, fmt.Errorf("get fqdn context: %w", err)
	}

	// Fetch LB addresses.
	lbAddresses, err := a.GetClusterLBAddresses(ctx, fc.Tenant.ClusterID)
	if err != nil {
		return nil, err
	}
	fc.LBAddresses = lbAddresses

	// Fetch LB nodes for this cluster.
	lbNodes, err := a.GetNodesByClusterAndRole(ctx, fc.Tenant.ClusterID, model.ShardRoleLB)
	if err != nil {
		return nil, err
	}
	fc.LBNodes = lbNodes

	// Fetch shard and nodes if tenant has a shard.
	if fc.Tenant.ShardID != nil {
		shard, err := a.GetShardByID(ctx, *fc.Tenant.ShardID)
		if err != nil {
			return nil, err
		}
		fc.Shard = *shard

		nodes, err := a.ListNodesByShard(ctx, *fc.Tenant.ShardID)
		if err != nil {
			return nil, err
		}
		fc.Nodes = nodes
	}

	return &fc, nil
}

// GetDatabaseUserContext fetches a database user and its related database and nodes.
func (a *CoreDB) GetDatabaseUserContext(ctx context.Context, userID string) (*DatabaseUserContext, error) {
	var dc DatabaseUserContext

	// JOIN database_users with databases.
	err := a.db.QueryRow(ctx,
		`SELECT u.id, u.database_id, u.username, u.password, u.privileges, u.status, u.status_message, u.created_at, u.updated_at,
		        d.id, d.tenant_id, d.name, d.shard_id, d.node_id, d.status, d.status_message, d.suspend_reason, d.created_at, d.updated_at
		 FROM database_users u
		 JOIN databases d ON d.id = u.database_id
		 WHERE u.id = $1`, userID,
	).Scan(&dc.User.ID, &dc.User.DatabaseID, &dc.User.Username, &dc.User.Password, &dc.User.Privileges, &dc.User.Status, &dc.User.StatusMessage, &dc.User.CreatedAt, &dc.User.UpdatedAt,
		&dc.Database.ID, &dc.Database.TenantID, &dc.Database.Name, &dc.Database.ShardID, &dc.Database.NodeID, &dc.Database.Status, &dc.Database.StatusMessage, &dc.Database.SuspendReason, &dc.Database.CreatedAt, &dc.Database.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get database user context: %w", err)
	}

	// Fetch nodes if database has a shard.
	if dc.Database.ShardID != nil {
		nodes, err := a.ListNodesByShard(ctx, *dc.Database.ShardID)
		if err != nil {
			return nil, err
		}
		dc.Nodes = nodes
	}

	return &dc, nil
}

// GetValkeyUserContext fetches a Valkey user and its related instance and nodes.
func (a *CoreDB) GetValkeyUserContext(ctx context.Context, userID string) (*ValkeyUserContext, error) {
	var vc ValkeyUserContext

	// JOIN valkey_users with valkey_instances.
	err := a.db.QueryRow(ctx,
		`SELECT u.id, u.valkey_instance_id, u.username, u.password, u.privileges, u.key_pattern, u.status, u.status_message, u.created_at, u.updated_at,
		        i.id, i.tenant_id, i.name, i.shard_id, i.port, i.max_memory_mb, i.password, i.status, i.status_message, i.suspend_reason, i.created_at, i.updated_at
		 FROM valkey_users u
		 JOIN valkey_instances i ON i.id = u.valkey_instance_id
		 WHERE u.id = $1`, userID,
	).Scan(&vc.User.ID, &vc.User.ValkeyInstanceID, &vc.User.Username, &vc.User.Password, &vc.User.Privileges, &vc.User.KeyPattern, &vc.User.Status, &vc.User.StatusMessage, &vc.User.CreatedAt, &vc.User.UpdatedAt,
		&vc.Instance.ID, &vc.Instance.TenantID, &vc.Instance.Name, &vc.Instance.ShardID, &vc.Instance.Port, &vc.Instance.MaxMemoryMB, &vc.Instance.Password, &vc.Instance.Status, &vc.Instance.StatusMessage, &vc.Instance.SuspendReason, &vc.Instance.CreatedAt, &vc.Instance.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get valkey user context: %w", err)
	}

	// Fetch nodes if instance has a shard.
	if vc.Instance.ShardID != nil {
		nodes, err := a.ListNodesByShard(ctx, *vc.Instance.ShardID)
		if err != nil {
			return nil, err
		}
		vc.Nodes = nodes
	}

	return &vc, nil
}

// GetZoneRecordContext fetches a zone record and its parent zone name.
func (a *CoreDB) GetZoneRecordContext(ctx context.Context, recordID string) (*ZoneRecordContext, error) {
	var zc ZoneRecordContext

	// JOIN zone_records with zones.
	err := a.db.QueryRow(ctx,
		`SELECT r.id, r.zone_id, r.type, r.name, r.content, r.ttl, r.priority, r.managed_by, r.source_type, r.source_fqdn_id, r.status, r.status_message, r.created_at, r.updated_at,
		        z.name
		 FROM zone_records r
		 JOIN zones z ON z.id = r.zone_id
		 WHERE r.id = $1`, recordID,
	).Scan(&zc.Record.ID, &zc.Record.ZoneID, &zc.Record.Type, &zc.Record.Name, &zc.Record.Content, &zc.Record.TTL, &zc.Record.Priority, &zc.Record.ManagedBy, &zc.Record.SourceType, &zc.Record.SourceFQDNID, &zc.Record.Status, &zc.Record.StatusMessage, &zc.Record.CreatedAt, &zc.Record.UpdatedAt,
		&zc.ZoneName)
	if err != nil {
		return nil, fmt.Errorf("get zone record context: %w", err)
	}

	return &zc, nil
}

// GetBackupContext fetches a backup and its related tenant and nodes.
func (a *CoreDB) GetBackupContext(ctx context.Context, backupID string) (*BackupContext, error) {
	var bc BackupContext

	// JOIN backups with tenants.
	err := a.db.QueryRow(ctx,
		`SELECT b.id, b.tenant_id, b.type, b.source_id, b.source_name, b.storage_path, b.size_bytes, b.status, b.status_message, b.started_at, b.completed_at, b.created_at, b.updated_at,
		        t.id, t.name, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.ssh_enabled, t.disk_quota_bytes, t.status, t.status_message, t.suspend_reason, t.created_at, t.updated_at
		 FROM backups b
		 JOIN tenants t ON t.id = b.tenant_id
		 WHERE b.id = $1`, backupID,
	).Scan(&bc.Backup.ID, &bc.Backup.TenantID, &bc.Backup.Type, &bc.Backup.SourceID, &bc.Backup.SourceName, &bc.Backup.StoragePath, &bc.Backup.SizeBytes, &bc.Backup.Status, &bc.Backup.StatusMessage, &bc.Backup.StartedAt, &bc.Backup.CompletedAt, &bc.Backup.CreatedAt, &bc.Backup.UpdatedAt,
		&bc.Tenant.ID, &bc.Tenant.Name, &bc.Tenant.BrandID, &bc.Tenant.RegionID, &bc.Tenant.ClusterID, &bc.Tenant.ShardID, &bc.Tenant.UID, &bc.Tenant.SFTPEnabled, &bc.Tenant.SSHEnabled, &bc.Tenant.DiskQuotaBytes, &bc.Tenant.Status, &bc.Tenant.StatusMessage, &bc.Tenant.SuspendReason, &bc.Tenant.CreatedAt, &bc.Tenant.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get backup context: %w", err)
	}

	// Fetch nodes if tenant has a shard.
	if bc.Tenant.ShardID != nil {
		nodes, err := a.ListNodesByShard(ctx, *bc.Tenant.ShardID)
		if err != nil {
			return nil, err
		}
		bc.Nodes = nodes
	}

	return &bc, nil
}

// GetS3AccessKeyContext fetches an S3 access key and its related bucket and nodes.
func (a *CoreDB) GetS3AccessKeyContext(ctx context.Context, keyID string) (*S3AccessKeyContext, error) {
	var sc S3AccessKeyContext

	// JOIN s3_access_keys with s3_buckets.
	err := a.db.QueryRow(ctx,
		`SELECT k.id, k.s3_bucket_id, k.access_key_id, k.secret_access_key, k.permissions, k.status, k.status_message, k.created_at, k.updated_at,
		        b.id, b.tenant_id, b.name, b.shard_id, b.public, b.quota_bytes, b.status, b.status_message, b.suspend_reason, b.created_at, b.updated_at
		 FROM s3_access_keys k
		 JOIN s3_buckets b ON b.id = k.s3_bucket_id
		 WHERE k.id = $1`, keyID,
	).Scan(&sc.Key.ID, &sc.Key.S3BucketID, &sc.Key.AccessKeyID, &sc.Key.SecretAccessKey, &sc.Key.Permissions, &sc.Key.Status, &sc.Key.StatusMessage, &sc.Key.CreatedAt, &sc.Key.UpdatedAt,
		&sc.Bucket.ID, &sc.Bucket.TenantID, &sc.Bucket.Name, &sc.Bucket.ShardID, &sc.Bucket.Public, &sc.Bucket.QuotaBytes, &sc.Bucket.Status, &sc.Bucket.StatusMessage, &sc.Bucket.SuspendReason, &sc.Bucket.CreatedAt, &sc.Bucket.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get s3 access key context: %w", err)
	}

	// Fetch nodes if bucket has a shard.
	if sc.Bucket.ShardID != nil {
		nodes, err := a.ListNodesByShard(ctx, *sc.Bucket.ShardID)
		if err != nil {
			return nil, err
		}
		sc.Nodes = nodes
	}

	return &sc, nil
}

// GetStalwartContext resolves Stalwart connection info by traversing FQDN -> webroot -> tenant -> cluster,
// and includes the brand's mail DNS configuration (SPF, DKIM, DMARC).
func (a *CoreDB) GetStalwartContext(ctx context.Context, fqdnID string) (*StalwartContext, error) {
	var sc StalwartContext
	var clusterConfig []byte

	err := a.db.QueryRow(ctx,
		`SELECT f.id, f.fqdn, c.config,
		 b.mail_hostname, b.spf_includes, b.dkim_selector, b.dkim_public_key, b.dmarc_policy
		 FROM fqdns f
		 JOIN webroots w ON w.id = f.webroot_id
		 JOIN tenants t ON t.id = w.tenant_id
		 JOIN clusters c ON c.id = t.cluster_id
		 JOIN brands b ON b.id = t.brand_id
		 WHERE f.id = $1`, fqdnID,
	).Scan(&sc.FQDNID, &sc.FQDN, &clusterConfig,
		&sc.MailHostname, &sc.SPFIncludes, &sc.DKIMSelector, &sc.DKIMPublicKey, &sc.DMARCPolicy)
	if err != nil {
		return nil, fmt.Errorf("get stalwart context: %w", err)
	}

	var cfg struct {
		StalwartURL   string `json:"stalwart_url"`
		StalwartToken string `json:"stalwart_token"`
		MailHostname  string `json:"mail_hostname"`
	}
	if err := json.Unmarshal(clusterConfig, &cfg); err != nil {
		return nil, fmt.Errorf("parse cluster config for stalwart: %w", err)
	}

	sc.StalwartURL = cfg.StalwartURL
	sc.StalwartToken = cfg.StalwartToken
	// Brand mail_hostname overrides cluster config mail_hostname if set.
	if sc.MailHostname == "" {
		sc.MailHostname = cfg.MailHostname
	}

	return &sc, nil
}

// FQDNMapping represents an active FQDN-to-shard-backend mapping for LB convergence.
type FQDNMapping struct {
	FQDN      string `json:"fqdn"`
	LBBackend string `json:"lb_backend"`
}

// ListActiveFQDNMappings returns all active FQDN-to-shard-backend mappings for a cluster.
// Used by LB shard convergence to populate the HAProxy map on LB nodes.
func (a *CoreDB) ListActiveFQDNMappings(ctx context.Context, clusterID string) ([]FQDNMapping, error) {
	rows, err := a.db.Query(ctx,
		`SELECT f.fqdn, s.lb_backend
		 FROM fqdns f
		 JOIN webroots w ON w.id = f.webroot_id
		 JOIN tenants t ON t.id = w.tenant_id
		 JOIN shards s ON s.id = t.shard_id
		 WHERE t.cluster_id = $1 AND f.status = $2`,
		clusterID, model.StatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("list active fqdn mappings: %w", err)
	}
	defer rows.Close()

	var mappings []FQDNMapping
	for rows.Next() {
		var m FQDNMapping
		if err := rows.Scan(&m.FQDN, &m.LBBackend); err != nil {
			return nil, fmt.Errorf("scan fqdn mapping: %w", err)
		}
		mappings = append(mappings, m)
	}
	return mappings, rows.Err()
}

// GetOldBackups returns active backups that are older than the specified number of days.
func (a *CoreDB) GetOldBackups(ctx context.Context, retentionDays int) ([]model.Backup, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, type, source_id, source_name, storage_path, size_bytes, status, status_message, started_at, completed_at, created_at, updated_at
		 FROM backups
		 WHERE status = $1
		   AND created_at < now() - make_interval(days => $2)
		 ORDER BY created_at ASC`,
		model.StatusActive, retentionDays,
	)
	if err != nil {
		return nil, fmt.Errorf("get old backups: %w", err)
	}
	defer rows.Close()

	var backups []model.Backup
	for rows.Next() {
		var b model.Backup
		if err := rows.Scan(&b.ID, &b.TenantID, &b.Type, &b.SourceID, &b.SourceName,
			&b.StoragePath, &b.SizeBytes, &b.Status, &b.StatusMessage, &b.StartedAt,
			&b.CompletedAt, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan old backup: %w", err)
		}
		backups = append(backups, b)
	}
	return backups, rows.Err()
}

// GetCronJobContext fetches a cron job and its related webroot, tenant, and nodes.
func (a *CoreDB) GetCronJobContext(ctx context.Context, cronJobID string) (*CronJobContext, error) {
	var cc CronJobContext

	// JOIN cron_jobs -> webroots -> tenants.
	err := a.db.QueryRow(ctx,
		`SELECT c.id, c.tenant_id, c.webroot_id, c.name, c.schedule, c.command, c.working_directory, c.enabled, c.timeout_seconds, c.max_memory_mb, c.status, c.status_message, c.created_at, c.updated_at,
		        w.id, w.tenant_id, w.name, w.runtime, w.runtime_version, w.runtime_config, w.public_folder, w.status, w.status_message, w.suspend_reason, w.created_at, w.updated_at,
		        t.id, t.name, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.ssh_enabled, t.disk_quota_bytes, t.status, t.status_message, t.suspend_reason, t.created_at, t.updated_at
		 FROM cron_jobs c
		 JOIN webroots w ON w.id = c.webroot_id
		 JOIN tenants t ON t.id = c.tenant_id
		 WHERE c.id = $1`, cronJobID,
	).Scan(&cc.CronJob.ID, &cc.CronJob.TenantID, &cc.CronJob.WebrootID, &cc.CronJob.Name, &cc.CronJob.Schedule, &cc.CronJob.Command, &cc.CronJob.WorkingDirectory, &cc.CronJob.Enabled, &cc.CronJob.TimeoutSeconds, &cc.CronJob.MaxMemoryMB, &cc.CronJob.Status, &cc.CronJob.StatusMessage, &cc.CronJob.CreatedAt, &cc.CronJob.UpdatedAt,
		&cc.Webroot.ID, &cc.Webroot.TenantID, &cc.Webroot.Name, &cc.Webroot.Runtime, &cc.Webroot.RuntimeVersion, &cc.Webroot.RuntimeConfig, &cc.Webroot.PublicFolder, &cc.Webroot.Status, &cc.Webroot.StatusMessage, &cc.Webroot.SuspendReason, &cc.Webroot.CreatedAt, &cc.Webroot.UpdatedAt,
		&cc.Tenant.ID, &cc.Tenant.Name, &cc.Tenant.BrandID, &cc.Tenant.RegionID, &cc.Tenant.ClusterID, &cc.Tenant.ShardID, &cc.Tenant.UID, &cc.Tenant.SFTPEnabled, &cc.Tenant.SSHEnabled, &cc.Tenant.DiskQuotaBytes, &cc.Tenant.Status, &cc.Tenant.StatusMessage, &cc.Tenant.SuspendReason, &cc.Tenant.CreatedAt, &cc.Tenant.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get cron job context: %w", err)
	}

	// Fetch nodes if tenant has a shard.
	if cc.Tenant.ShardID != nil {
		nodes, err := a.ListNodesByShard(ctx, *cc.Tenant.ShardID)
		if err != nil {
			return nil, err
		}
		cc.Nodes = nodes
	}

	return &cc, nil
}

// ListCronJobsByWebroot retrieves all cron jobs for a webroot (excluding deleted).
func (a *CoreDB) ListCronJobsByWebroot(ctx context.Context, webrootID string) ([]model.CronJob, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, webroot_id, name, schedule, command, working_directory, enabled, timeout_seconds, max_memory_mb, status, status_message, created_at, updated_at
		 FROM cron_jobs WHERE webroot_id = $1 ORDER BY name`, webrootID,
	)
	if err != nil {
		return nil, fmt.Errorf("list cron jobs by webroot: %w", err)
	}
	defer rows.Close()

	var jobs []model.CronJob
	for rows.Next() {
		var j model.CronJob
		if err := rows.Scan(&j.ID, &j.TenantID, &j.WebrootID, &j.Name, &j.Schedule, &j.Command, &j.WorkingDirectory, &j.Enabled, &j.TimeoutSeconds, &j.MaxMemoryMB, &j.Status, &j.StatusMessage, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan cron job row: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// ListShardsByRole retrieves all shards with the specified role.
func (a *CoreDB) ListShardsByRole(ctx context.Context, role string) ([]model.Shard, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, cluster_id, name, role, lb_backend, config, status, status_message, created_at, updated_at
		 FROM shards WHERE role = $1`, role,
	)
	if err != nil {
		return nil, fmt.Errorf("list shards by role: %w", err)
	}
	defer rows.Close()

	var shards []model.Shard
	for rows.Next() {
		var s model.Shard
		if err := rows.Scan(&s.ID, &s.ClusterID, &s.Name, &s.Role, &s.LBBackend, &s.Config, &s.Status, &s.StatusMessage, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan shard row: %w", err)
		}
		shards = append(shards, s)
	}
	return shards, rows.Err()
}

// UpdateShardConfig updates the config JSON for a shard.
func (a *CoreDB) UpdateShardConfig(ctx context.Context, params UpdateShardConfigParams) error {
	_, err := a.db.Exec(ctx,
		`UPDATE shards SET config = $1, updated_at = NOW() WHERE id = $2`,
		params.Config, params.ShardID,
	)
	return err
}

// GetDaemonContext fetches a daemon and its related webroot, tenant, and nodes.
func (a *CoreDB) GetDaemonContext(ctx context.Context, daemonID string) (*DaemonContext, error) {
	var dc DaemonContext
	var envJSON []byte

	// JOIN daemons -> webroots -> tenants.
	err := a.db.QueryRow(ctx,
		`SELECT d.id, d.tenant_id, d.node_id, d.webroot_id, d.name, d.command, d.proxy_path, d.proxy_port,
		        d.num_procs, d.stop_signal, d.stop_wait_secs, d.max_memory_mb, d.environment,
		        d.enabled, d.status, d.status_message, d.created_at, d.updated_at,
		        w.id, w.tenant_id, w.name, w.runtime, w.runtime_version, w.runtime_config, w.public_folder, w.status, w.status_message, w.suspend_reason, w.created_at, w.updated_at,
		        t.id, t.name, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.ssh_enabled, t.disk_quota_bytes, t.status, t.status_message, t.suspend_reason, t.created_at, t.updated_at
		 FROM daemons d
		 JOIN webroots w ON w.id = d.webroot_id
		 JOIN tenants t ON t.id = d.tenant_id
		 WHERE d.id = $1`, daemonID,
	).Scan(&dc.Daemon.ID, &dc.Daemon.TenantID, &dc.Daemon.NodeID, &dc.Daemon.WebrootID, &dc.Daemon.Name, &dc.Daemon.Command,
		&dc.Daemon.ProxyPath, &dc.Daemon.ProxyPort,
		&dc.Daemon.NumProcs, &dc.Daemon.StopSignal, &dc.Daemon.StopWaitSecs, &dc.Daemon.MaxMemoryMB, &envJSON,
		&dc.Daemon.Enabled, &dc.Daemon.Status, &dc.Daemon.StatusMessage, &dc.Daemon.CreatedAt, &dc.Daemon.UpdatedAt,
		&dc.Webroot.ID, &dc.Webroot.TenantID, &dc.Webroot.Name, &dc.Webroot.Runtime, &dc.Webroot.RuntimeVersion, &dc.Webroot.RuntimeConfig, &dc.Webroot.PublicFolder, &dc.Webroot.Status, &dc.Webroot.StatusMessage, &dc.Webroot.SuspendReason, &dc.Webroot.CreatedAt, &dc.Webroot.UpdatedAt,
		&dc.Tenant.ID, &dc.Tenant.Name, &dc.Tenant.BrandID, &dc.Tenant.RegionID, &dc.Tenant.ClusterID, &dc.Tenant.ShardID, &dc.Tenant.UID, &dc.Tenant.SFTPEnabled, &dc.Tenant.SSHEnabled, &dc.Tenant.DiskQuotaBytes, &dc.Tenant.Status, &dc.Tenant.StatusMessage, &dc.Tenant.SuspendReason, &dc.Tenant.CreatedAt, &dc.Tenant.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get daemon context: %w", err)
	}

	if len(envJSON) > 0 {
		_ = json.Unmarshal(envJSON, &dc.Daemon.Environment)
	}
	if dc.Daemon.Environment == nil {
		dc.Daemon.Environment = make(map[string]string)
	}

	// Fetch nodes if tenant has a shard.
	if dc.Tenant.ShardID != nil {
		nodes, err := a.ListNodesByShard(ctx, *dc.Tenant.ShardID)
		if err != nil {
			return nil, err
		}
		dc.Nodes = nodes
	}

	return &dc, nil
}

// ListDaemonsByWebroot retrieves all daemons for a webroot (excluding deleted).
func (a *CoreDB) ListDaemonsByWebroot(ctx context.Context, webrootID string) ([]model.Daemon, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, node_id, webroot_id, name, command, proxy_path, proxy_port,
		        num_procs, stop_signal, stop_wait_secs, max_memory_mb, environment,
		        enabled, status, status_message, created_at, updated_at
		 FROM daemons WHERE webroot_id = $1 ORDER BY name`, webrootID,
	)
	if err != nil {
		return nil, fmt.Errorf("list daemons by webroot: %w", err)
	}
	defer rows.Close()

	var daemons []model.Daemon
	for rows.Next() {
		var d model.Daemon
		var envJSON []byte
		if err := rows.Scan(&d.ID, &d.TenantID, &d.NodeID, &d.WebrootID, &d.Name, &d.Command,
			&d.ProxyPath, &d.ProxyPort,
			&d.NumProcs, &d.StopSignal, &d.StopWaitSecs, &d.MaxMemoryMB, &envJSON,
			&d.Enabled, &d.Status, &d.StatusMessage, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan daemon row: %w", err)
		}
		if len(envJSON) > 0 {
			_ = json.Unmarshal(envJSON, &d.Environment)
		}
		if d.Environment == nil {
			d.Environment = make(map[string]string)
		}
		daemons = append(daemons, d)
	}
	return daemons, rows.Err()
}

// ShardDesiredState holds all the data needed to converge a web shard,
// fetched in a single activity call instead of N+1 round-trips.
type ShardDesiredState struct {
	Tenants            []model.Tenant              `json:"tenants"`
	Webroots           map[string][]model.Webroot  `json:"webroots"`            // tenant ID -> webroots
	FQDNs              map[string][]FQDNParam      `json:"fqdns"`               // webroot ID -> FQDNs
	Daemons            map[string][]model.Daemon   `json:"daemons"`             // webroot ID -> daemons
	CronJobs           map[string][]model.CronJob  `json:"cron_jobs"`           // webroot ID -> cron jobs
	SSHKeys            map[string][]string         `json:"ssh_keys"`            // tenant ID -> public keys
	BrandBaseHostnames map[string]string           `json:"brand_base_hostnames"` // tenant ID -> brand base_hostname
}

// GetShardDesiredState fetches all data needed for web shard convergence in
// batch queries, replacing the N+1 per-tenant/per-webroot activity calls.
func (a *CoreDB) GetShardDesiredState(ctx context.Context, shardID string) (*ShardDesiredState, error) {
	result := &ShardDesiredState{
		Webroots:           make(map[string][]model.Webroot),
		FQDNs:              make(map[string][]FQDNParam),
		Daemons:            make(map[string][]model.Daemon),
		CronJobs:           make(map[string][]model.CronJob),
		SSHKeys:            make(map[string][]string),
		BrandBaseHostnames: make(map[string]string),
	}

	// 1. Fetch all tenants for shard.
	tenants, err := a.ListTenantsByShard(ctx, shardID)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	result.Tenants = tenants

	// Collect active tenant IDs.
	var tenantIDs []string
	for _, t := range tenants {
		if t.Status == model.StatusActive {
			tenantIDs = append(tenantIDs, t.ID)
		}
	}
	if len(tenantIDs) == 0 {
		return result, nil
	}

	// 2. Fetch brand base hostnames for tenants.
	brandRows, err := a.db.Query(ctx,
		`SELECT t.id, b.base_hostname FROM tenants t JOIN brands b ON b.id = t.brand_id WHERE t.id = ANY($1)`, tenantIDs)
	if err != nil {
		return nil, fmt.Errorf("batch list brand hostnames: %w", err)
	}
	defer brandRows.Close()
	for brandRows.Next() {
		var tid, bh string
		if err := brandRows.Scan(&tid, &bh); err != nil {
			return nil, fmt.Errorf("scan brand hostname: %w", err)
		}
		result.BrandBaseHostnames[tid] = bh
	}
	if err := brandRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate brand hostnames: %w", err)
	}

	// 3. Fetch all active webroots for those tenants.
	wrRows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, name, runtime, runtime_version, runtime_config, public_folder, env_file_name, env_shell_source, service_hostname_enabled, status, status_message, suspend_reason, created_at, updated_at
		 FROM webroots WHERE tenant_id = ANY($1) AND status = $2`, tenantIDs, model.StatusActive)
	if err != nil {
		return nil, fmt.Errorf("batch list webroots: %w", err)
	}
	defer wrRows.Close()

	var webrootIDs []string
	for wrRows.Next() {
		var w model.Webroot
		if err := wrRows.Scan(&w.ID, &w.TenantID, &w.Name, &w.Runtime, &w.RuntimeVersion, &w.RuntimeConfig, &w.PublicFolder, &w.EnvFileName, &w.EnvShellSource, &w.ServiceHostnameEnabled, &w.Status, &w.StatusMessage, &w.SuspendReason, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan webroot: %w", err)
		}
		result.Webroots[w.TenantID] = append(result.Webroots[w.TenantID], w)
		webrootIDs = append(webrootIDs, w.ID)
	}
	if err := wrRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate webroots: %w", err)
	}

	if len(webrootIDs) == 0 {
		return result, nil
	}

	// 4. Fetch all active FQDNs for those webroots.
	fqdnRows, err := a.db.Query(ctx,
		`SELECT fqdn, webroot_id, ssl_enabled
		 FROM fqdns WHERE webroot_id = ANY($1) AND status = $2`, webrootIDs, model.StatusActive)
	if err != nil {
		return nil, fmt.Errorf("batch list fqdns: %w", err)
	}
	defer fqdnRows.Close()

	for fqdnRows.Next() {
		var f FQDNParam
		if err := fqdnRows.Scan(&f.FQDN, &f.WebrootID, &f.SSLEnabled); err != nil {
			return nil, fmt.Errorf("scan fqdn: %w", err)
		}
		result.FQDNs[f.WebrootID] = append(result.FQDNs[f.WebrootID], f)
	}
	if err := fqdnRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate fqdns: %w", err)
	}

	// 5. Fetch all daemons for those webroots.
	daemonRows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, node_id, webroot_id, name, command, proxy_path, proxy_port,
		        num_procs, stop_signal, stop_wait_secs, max_memory_mb, environment,
		        enabled, status, status_message, created_at, updated_at
		 FROM daemons WHERE webroot_id = ANY($1)`, webrootIDs)
	if err != nil {
		return nil, fmt.Errorf("batch list daemons: %w", err)
	}
	defer daemonRows.Close()

	for daemonRows.Next() {
		var d model.Daemon
		var envJSON []byte
		if err := daemonRows.Scan(&d.ID, &d.TenantID, &d.NodeID, &d.WebrootID, &d.Name, &d.Command,
			&d.ProxyPath, &d.ProxyPort,
			&d.NumProcs, &d.StopSignal, &d.StopWaitSecs, &d.MaxMemoryMB, &envJSON,
			&d.Enabled, &d.Status, &d.StatusMessage, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan daemon: %w", err)
		}
		if len(envJSON) > 0 {
			_ = json.Unmarshal(envJSON, &d.Environment)
		}
		if d.Environment == nil {
			d.Environment = make(map[string]string)
		}
		result.Daemons[d.WebrootID] = append(result.Daemons[d.WebrootID], d)
	}
	if err := daemonRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daemons: %w", err)
	}

	// 6. Fetch all cron jobs for those webroots.
	cronRows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, webroot_id, name, schedule, command, working_directory, enabled, timeout_seconds, max_memory_mb, status, status_message, created_at, updated_at
		 FROM cron_jobs WHERE webroot_id = ANY($1)`, webrootIDs)
	if err != nil {
		return nil, fmt.Errorf("batch list cron jobs: %w", err)
	}
	defer cronRows.Close()

	for cronRows.Next() {
		var j model.CronJob
		if err := cronRows.Scan(&j.ID, &j.TenantID, &j.WebrootID, &j.Name, &j.Schedule, &j.Command, &j.WorkingDirectory, &j.Enabled, &j.TimeoutSeconds, &j.MaxMemoryMB, &j.Status, &j.StatusMessage, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan cron job: %w", err)
		}
		result.CronJobs[j.WebrootID] = append(result.CronJobs[j.WebrootID], j)
	}
	if err := cronRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cron jobs: %w", err)
	}

	// 7. Fetch all active SSH keys for those tenants.
	sshRows, err := a.db.Query(ctx,
		`SELECT tenant_id, public_key FROM ssh_keys WHERE tenant_id = ANY($1) AND status = $2`,
		tenantIDs, model.StatusActive)
	if err != nil {
		return nil, fmt.Errorf("batch list ssh keys: %w", err)
	}
	defer sshRows.Close()

	for sshRows.Next() {
		var tenantID, pubKey string
		if err := sshRows.Scan(&tenantID, &pubKey); err != nil {
			return nil, fmt.Errorf("scan ssh key: %w", err)
		}
		result.SSHKeys[tenantID] = append(result.SSHKeys[tenantID], pubKey)
	}
	if err := sshRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ssh keys: %w", err)
	}

	return result, nil
}

// ListDaemonsByTenant retrieves all active daemons for a tenant (used in convergence).
func (a *CoreDB) ListDaemonsByTenant(ctx context.Context, tenantID string) ([]model.Daemon, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, node_id, webroot_id, name, command, proxy_path, proxy_port,
		        num_procs, stop_signal, stop_wait_secs, max_memory_mb, environment,
		        enabled, status, status_message, created_at, updated_at
		 FROM daemons WHERE tenant_id = $1 AND status = $2 ORDER BY name`, tenantID, model.StatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("list daemons by tenant: %w", err)
	}
	defer rows.Close()

	var daemons []model.Daemon
	for rows.Next() {
		var d model.Daemon
		var envJSON []byte
		if err := rows.Scan(&d.ID, &d.TenantID, &d.NodeID, &d.WebrootID, &d.Name, &d.Command,
			&d.ProxyPath, &d.ProxyPort,
			&d.NumProcs, &d.StopSignal, &d.StopWaitSecs, &d.MaxMemoryMB, &envJSON,
			&d.Enabled, &d.Status, &d.StatusMessage, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan daemon row: %w", err)
		}
		if len(envJSON) > 0 {
			_ = json.Unmarshal(envJSON, &d.Environment)
		}
		if d.Environment == nil {
			d.Environment = make(map[string]string)
		}
		daemons = append(daemons, d)
	}
	return daemons, rows.Err()
}

// ListCronJobsByTenant retrieves all active cron jobs for a tenant (used in convergence).
func (a *CoreDB) ListCronJobsByTenant(ctx context.Context, tenantID string) ([]model.CronJob, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, webroot_id, name, schedule, command, working_directory, enabled, timeout_seconds, max_memory_mb, status, status_message, created_at, updated_at
		 FROM cron_jobs WHERE tenant_id = $1 AND status = $2 ORDER BY name`, tenantID, model.StatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("list cron jobs by tenant: %w", err)
	}
	defer rows.Close()

	var jobs []model.CronJob
	for rows.Next() {
		var j model.CronJob
		if err := rows.Scan(&j.ID, &j.TenantID, &j.WebrootID, &j.Name, &j.Schedule, &j.Command, &j.WorkingDirectory, &j.Enabled, &j.TimeoutSeconds, &j.MaxMemoryMB, &j.Status, &j.StatusMessage, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan cron job row: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// SetTenantEgressRulesProvisioning sets all pending/deleting rules for a tenant to provisioning.
func (a *CoreDB) SetTenantEgressRulesProvisioning(ctx context.Context, tenantID string) error {
	_, err := a.db.Exec(ctx,
		`UPDATE tenant_egress_rules SET status = 'provisioning', updated_at = now()
		 WHERE tenant_id = $1 AND status IN ('pending', 'deleting', 'failed')`, tenantID)
	if err != nil {
		return fmt.Errorf("set tenant egress rules provisioning: %w", err)
	}
	return nil
}

// GetActiveEgressRules returns all active + provisioning egress rules for a tenant.
func (a *CoreDB) GetActiveEgressRules(ctx context.Context, tenantID string) ([]model.TenantEgressRule, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, cidr, description, status, status_message, created_at, updated_at
		 FROM tenant_egress_rules
		 WHERE tenant_id = $1 AND status NOT IN ('deleting', 'deleted')
		 ORDER BY id`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get active egress rules: %w", err)
	}
	defer rows.Close()

	var rules []model.TenantEgressRule
	for rows.Next() {
		var r model.TenantEgressRule
		if err := rows.Scan(&r.ID, &r.TenantID, &r.CIDR, &r.Description,
			&r.Status, &r.StatusMessage, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan egress rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// FinalizeTenantEgressRules sets provisioning rules to active and hard-deletes deleting rules.
func (a *CoreDB) FinalizeTenantEgressRules(ctx context.Context, tenantID string) error {
	_, err := a.db.Exec(ctx,
		`UPDATE tenant_egress_rules SET status = 'active', updated_at = now()
		 WHERE tenant_id = $1 AND status = 'provisioning'`, tenantID)
	if err != nil {
		return fmt.Errorf("finalize tenant egress rules (active): %w", err)
	}
	// Rules that were being deleted get hard-deleted now that nftables is synced.
	_, err = a.db.Exec(ctx,
		`DELETE FROM tenant_egress_rules WHERE tenant_id = $1 AND status = 'deleting'`, tenantID)
	if err != nil {
		return fmt.Errorf("finalize tenant egress rules (delete): %w", err)
	}
	return nil
}

// SetDatabaseAccessRulesProvisioning sets all pending/deleting rules for a database to provisioning.
func (a *CoreDB) SetDatabaseAccessRulesProvisioning(ctx context.Context, databaseID string) error {
	_, err := a.db.Exec(ctx,
		`UPDATE database_access_rules SET status = 'provisioning', updated_at = now()
		 WHERE database_id = $1 AND status IN ('pending', 'deleting', 'failed')`, databaseID)
	if err != nil {
		return fmt.Errorf("set database access rules provisioning: %w", err)
	}
	return nil
}

// GetActiveDatabaseAccessRules returns all active + provisioning access rules for a database.
func (a *CoreDB) GetActiveDatabaseAccessRules(ctx context.Context, databaseID string) ([]model.DatabaseAccessRule, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, database_id, cidr, description, status, status_message, created_at, updated_at
		 FROM database_access_rules
		 WHERE database_id = $1 AND status NOT IN ('deleting', 'deleted')
		 ORDER BY id`, databaseID)
	if err != nil {
		return nil, fmt.Errorf("get active database access rules: %w", err)
	}
	defer rows.Close()

	var rules []model.DatabaseAccessRule
	for rows.Next() {
		var r model.DatabaseAccessRule
		if err := rows.Scan(&r.ID, &r.DatabaseID, &r.CIDR, &r.Description,
			&r.Status, &r.StatusMessage, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan database access rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// GetActiveDatabaseUsers returns all active database users for a database.
func (a *CoreDB) GetActiveDatabaseUsers(ctx context.Context, databaseID string) ([]model.DatabaseUser, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, database_id, username, password, privileges, status, status_message, created_at, updated_at
		 FROM database_users
		 WHERE database_id = $1 AND status = 'active'
		 ORDER BY id`, databaseID)
	if err != nil {
		return nil, fmt.Errorf("get active database users: %w", err)
	}
	defer rows.Close()

	var users []model.DatabaseUser
	for rows.Next() {
		var u model.DatabaseUser
		if err := rows.Scan(&u.ID, &u.DatabaseID, &u.Username, &u.Password,
			&u.Privileges, &u.Status, &u.StatusMessage, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan database user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// FinalizeDatabaseAccessRules sets provisioning rules to active and hard-deletes deleting rules.
func (a *CoreDB) FinalizeDatabaseAccessRules(ctx context.Context, databaseID string) error {
	_, err := a.db.Exec(ctx,
		`UPDATE database_access_rules SET status = 'active', updated_at = now()
		 WHERE database_id = $1 AND status = 'provisioning'`, databaseID)
	if err != nil {
		return fmt.Errorf("finalize database access rules (active): %w", err)
	}
	_, err = a.db.Exec(ctx,
		`DELETE FROM database_access_rules WHERE database_id = $1 AND status = 'deleting'`, databaseID)
	if err != nil {
		return fmt.Errorf("finalize database access rules (delete): %w", err)
	}
	return nil
}

// ListSSHKeysByTenantID retrieves all SSH keys for a tenant.
func (a *CoreDB) ListSSHKeysByTenantID(ctx context.Context, tenantID string) ([]model.SSHKey, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, name, public_key, fingerprint, status, status_message, created_at, updated_at
		 FROM ssh_keys WHERE tenant_id = $1`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list ssh keys by tenant: %w", err)
	}
	defer rows.Close()

	var keys []model.SSHKey
	for rows.Next() {
		var k model.SSHKey
		if err := rows.Scan(&k.ID, &k.TenantID, &k.Name, &k.PublicKey, &k.Fingerprint, &k.Status, &k.StatusMessage, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan ssh key row: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// ListBackupsByTenantID retrieves all backups for a tenant.
func (a *CoreDB) ListBackupsByTenantID(ctx context.Context, tenantID string) ([]model.Backup, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, type, source_id, source_name, storage_path, size_bytes, status, status_message, started_at, completed_at, created_at, updated_at
		 FROM backups WHERE tenant_id = $1`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list backups by tenant: %w", err)
	}
	defer rows.Close()

	var backups []model.Backup
	for rows.Next() {
		var b model.Backup
		if err := rows.Scan(&b.ID, &b.TenantID, &b.Type, &b.SourceID, &b.SourceName, &b.StoragePath, &b.SizeBytes, &b.Status, &b.StatusMessage, &b.StartedAt, &b.CompletedAt, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan backup row: %w", err)
		}
		backups = append(backups, b)
	}
	return backups, rows.Err()
}

// ListEgressRulesByTenantID retrieves all egress rules for a tenant.
func (a *CoreDB) ListEgressRulesByTenantID(ctx context.Context, tenantID string) ([]model.TenantEgressRule, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, cidr, description, status, status_message, created_at, updated_at
		 FROM tenant_egress_rules WHERE tenant_id = $1`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list egress rules by tenant: %w", err)
	}
	defer rows.Close()

	var rules []model.TenantEgressRule
	for rows.Next() {
		var r model.TenantEgressRule
		if err := rows.Scan(&r.ID, &r.TenantID, &r.CIDR, &r.Description, &r.Status, &r.StatusMessage, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan egress rule row: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// ListDatabaseAccessRulesByDatabaseID retrieves all access rules for a database.
func (a *CoreDB) ListDatabaseAccessRulesByDatabaseID(ctx context.Context, databaseID string) ([]model.DatabaseAccessRule, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, database_id, cidr, description, status, status_message, created_at, updated_at
		 FROM database_access_rules WHERE database_id = $1`, databaseID,
	)
	if err != nil {
		return nil, fmt.Errorf("list database access rules by database: %w", err)
	}
	defer rows.Close()

	var rules []model.DatabaseAccessRule
	for rows.Next() {
		var r model.DatabaseAccessRule
		if err := rows.Scan(&r.ID, &r.DatabaseID, &r.CIDR, &r.Description, &r.Status, &r.StatusMessage, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan database access rule row: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// ListS3AccessKeysByBucketID retrieves all access keys for an S3 bucket.
func (a *CoreDB) ListS3AccessKeysByBucketID(ctx context.Context, bucketID string) ([]model.S3AccessKey, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, s3_bucket_id, access_key_id, secret_access_key, permissions, status, status_message, created_at, updated_at
		 FROM s3_access_keys WHERE s3_bucket_id = $1`, bucketID,
	)
	if err != nil {
		return nil, fmt.Errorf("list s3 access keys by bucket: %w", err)
	}
	defer rows.Close()

	var keys []model.S3AccessKey
	for rows.Next() {
		var k model.S3AccessKey
		if err := rows.Scan(&k.ID, &k.S3BucketID, &k.AccessKeyID, &k.SecretAccessKey, &k.Permissions, &k.Status, &k.StatusMessage, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan s3 access key row: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// ListDaemonsByWebrootID retrieves all daemons for a webroot.
func (a *CoreDB) ListDaemonsByWebrootID(ctx context.Context, webrootID string) ([]model.Daemon, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, node_id, webroot_id, name, command, proxy_path, proxy_port, num_procs, stop_signal, stop_wait_secs, max_memory_mb, environment, enabled, status, status_message, created_at, updated_at
		 FROM daemons WHERE webroot_id = $1`, webrootID,
	)
	if err != nil {
		return nil, fmt.Errorf("list daemons by webroot: %w", err)
	}
	defer rows.Close()

	var daemons []model.Daemon
	for rows.Next() {
		var d model.Daemon
		if err := rows.Scan(&d.ID, &d.TenantID, &d.NodeID, &d.WebrootID, &d.Name, &d.Command, &d.ProxyPath, &d.ProxyPort, &d.NumProcs, &d.StopSignal, &d.StopWaitSecs, &d.MaxMemoryMB, &d.Environment, &d.Enabled, &d.Status, &d.StatusMessage, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan daemon row: %w", err)
		}
		daemons = append(daemons, d)
	}
	return daemons, rows.Err()
}

// ListCronJobsByWebrootID retrieves all cron jobs for a webroot.
func (a *CoreDB) ListCronJobsByWebrootID(ctx context.Context, webrootID string) ([]model.CronJob, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, webroot_id, name, schedule, command, working_directory, enabled, timeout_seconds, max_memory_mb, consecutive_failures, max_failures, status, status_message, created_at, updated_at
		 FROM cron_jobs WHERE webroot_id = $1`, webrootID,
	)
	if err != nil {
		return nil, fmt.Errorf("list cron jobs by webroot: %w", err)
	}
	defer rows.Close()

	var jobs []model.CronJob
	for rows.Next() {
		var c model.CronJob
		if err := rows.Scan(&c.ID, &c.TenantID, &c.WebrootID, &c.Name, &c.Schedule, &c.Command, &c.WorkingDirectory, &c.Enabled, &c.TimeoutSeconds, &c.MaxMemoryMB, &c.ConsecutiveFailures, &c.MaxFailures, &c.Status, &c.StatusMessage, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan cron job row: %w", err)
		}
		jobs = append(jobs, c)
	}
	return jobs, rows.Err()
}

// ListFQDNsByWebrootID retrieves all FQDNs for a webroot.
func (a *CoreDB) ListFQDNsByWebrootID(ctx context.Context, webrootID string) ([]model.FQDN, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, fqdn, webroot_id, ssl_enabled, status, status_message, created_at, updated_at
		 FROM fqdns WHERE webroot_id = $1`, webrootID,
	)
	if err != nil {
		return nil, fmt.Errorf("list fqdns by webroot: %w", err)
	}
	defer rows.Close()

	var fqdns []model.FQDN
	for rows.Next() {
		var f model.FQDN
		if err := rows.Scan(&f.ID, &f.FQDN, &f.WebrootID, &f.SSLEnabled, &f.Status, &f.StatusMessage, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan fqdn row: %w", err)
		}
		fqdns = append(fqdns, f)
	}
	return fqdns, rows.Err()
}

// ListEmailAccountsByFQDNID retrieves all email accounts for an FQDN.
func (a *CoreDB) ListEmailAccountsByFQDNID(ctx context.Context, fqdnID string) ([]model.EmailAccount, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, fqdn_id, address, display_name, quota_bytes, status, status_message, created_at, updated_at
		 FROM email_accounts WHERE fqdn_id = $1`, fqdnID,
	)
	if err != nil {
		return nil, fmt.Errorf("list email accounts by fqdn: %w", err)
	}
	defer rows.Close()

	var accounts []model.EmailAccount
	for rows.Next() {
		var a model.EmailAccount
		if err := rows.Scan(&a.ID, &a.FQDNID, &a.Address, &a.DisplayName, &a.QuotaBytes, &a.Status, &a.StatusMessage, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan email account row: %w", err)
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

// ListEmailAliasesByAccountID retrieves all aliases for an email account.
func (a *CoreDB) ListEmailAliasesByAccountID(ctx context.Context, accountID string) ([]model.EmailAlias, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, email_account_id, address, status, status_message, created_at, updated_at
		 FROM email_aliases WHERE email_account_id = $1`, accountID,
	)
	if err != nil {
		return nil, fmt.Errorf("list email aliases by account: %w", err)
	}
	defer rows.Close()

	var aliases []model.EmailAlias
	for rows.Next() {
		var al model.EmailAlias
		if err := rows.Scan(&al.ID, &al.EmailAccountID, &al.Address, &al.Status, &al.StatusMessage, &al.CreatedAt, &al.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan email alias row: %w", err)
		}
		aliases = append(aliases, al)
	}
	return aliases, rows.Err()
}

// ListEmailForwardsByAccountID retrieves all forwards for an email account.
func (a *CoreDB) ListEmailForwardsByAccountID(ctx context.Context, accountID string) ([]model.EmailForward, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, email_account_id, destination, keep_copy, status, status_message, created_at, updated_at
		 FROM email_forwards WHERE email_account_id = $1`, accountID,
	)
	if err != nil {
		return nil, fmt.Errorf("list email forwards by account: %w", err)
	}
	defer rows.Close()

	var forwards []model.EmailForward
	for rows.Next() {
		var f model.EmailForward
		if err := rows.Scan(&f.ID, &f.EmailAccountID, &f.Destination, &f.KeepCopy, &f.Status, &f.StatusMessage, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan email forward row: %w", err)
		}
		forwards = append(forwards, f)
	}
	return forwards, rows.Err()
}

// ListEmailAutoRepliesByAccountID retrieves all auto-replies for an email account.
func (a *CoreDB) ListEmailAutoRepliesByAccountID(ctx context.Context, accountID string) ([]model.EmailAutoReply, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, email_account_id, subject, body, start_date, end_date, enabled, status, status_message, created_at, updated_at
		 FROM email_autoreplies WHERE email_account_id = $1`, accountID,
	)
	if err != nil {
		return nil, fmt.Errorf("list email autoreplies by account: %w", err)
	}
	defer rows.Close()

	var replies []model.EmailAutoReply
	for rows.Next() {
		var r model.EmailAutoReply
		if err := rows.Scan(&r.ID, &r.EmailAccountID, &r.Subject, &r.Body, &r.StartDate, &r.EndDate, &r.Enabled, &r.Status, &r.StatusMessage, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan email autoreply row: %w", err)
		}
		replies = append(replies, r)
	}
	return replies, rows.Err()
}

// ListZoneRecordsByZoneID retrieves all zone records for a zone.
func (a *CoreDB) ListZoneRecordsByZoneID(ctx context.Context, zoneID string) ([]model.ZoneRecord, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, zone_id, type, name, content, ttl, priority, managed_by, source_type, source_fqdn_id, status, status_message, created_at, updated_at
		 FROM zone_records WHERE zone_id = $1`, zoneID,
	)
	if err != nil {
		return nil, fmt.Errorf("list zone records by zone: %w", err)
	}
	defer rows.Close()

	var records []model.ZoneRecord
	for rows.Next() {
		var r model.ZoneRecord
		if err := rows.Scan(&r.ID, &r.ZoneID, &r.Type, &r.Name, &r.Content, &r.TTL, &r.Priority, &r.ManagedBy, &r.SourceType, &r.SourceFQDNID, &r.Status, &r.StatusMessage, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan zone record row: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// ListActiveNodes returns all nodes with status "active".
func (a *CoreDB) ListActiveNodes(ctx context.Context) ([]model.Node, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, cluster_id, hostname, ip_address::text, ip6_address::text, roles, status, last_health_at, created_at, updated_at
		 FROM nodes WHERE status = $1 ORDER BY hostname`, model.StatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("list active nodes: %w", err)
	}
	defer rows.Close()

	var nodes []model.Node
	for rows.Next() {
		var n model.Node
		if err := rows.Scan(&n.ID, &n.ClusterID, &n.Hostname, &n.IPAddress, &n.IP6Address, &n.Roles, &n.Status, &n.LastHealthAt, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan active node: %w", err)
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// ExpiringCert holds minimal cert info for the health cron.
type ExpiringCert struct {
	ID        string    `json:"id"`
	FQDNID    string    `json:"fqdn_id"`
	Type      string    `json:"type"`
	ExpiresAt time.Time `json:"expires_at"`
}

// GetExpiringCerts returns all active certificates expiring within the given number of days.
func (a *CoreDB) GetExpiringCerts(ctx context.Context, days int) ([]ExpiringCert, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, fqdn_id, type, expires_at
		 FROM certificates
		 WHERE status = $1 AND is_active = true
		   AND expires_at <= now() + make_interval(days => $2)
		   AND expires_at > now()
		 ORDER BY expires_at ASC`,
		"active", days,
	)
	if err != nil {
		return nil, fmt.Errorf("get expiring certs: %w", err)
	}
	defer rows.Close()

	var certs []ExpiringCert
	for rows.Next() {
		var c ExpiringCert
		if err := rows.Scan(&c.ID, &c.FQDNID, &c.Type, &c.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scan expiring cert: %w", err)
		}
		certs = append(certs, c)
	}
	return certs, rows.Err()
}

// UpsertResourceUsageParams holds parameters for upserting a resource usage row.
type UpsertResourceUsageParams struct {
	ResourceType string `json:"resource_type"` // "webroot" or "database"
	Name         string `json:"name"`          // "tenant_name/webroot_name" or "db_name"
	BytesUsed    int64  `json:"bytes_used"`
}

// UpsertResourceUsage resolves a resource name to its ID and upserts a resource_usage row.
func (a *CoreDB) UpsertResourceUsage(ctx context.Context, params UpsertResourceUsageParams) error {
	var resourceID, tenantID string

	switch params.ResourceType {
	case "webroot":
		// Name is "tenant_name/webroot_name".
		parts := strings.SplitN(params.Name, "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid webroot name format: %s", params.Name)
		}
		err := a.db.QueryRow(ctx,
			`SELECT w.id, w.tenant_id
			 FROM webroots w JOIN tenants t ON t.id = w.tenant_id
			 WHERE t.name = $1 AND w.name = $2`, parts[0], parts[1],
		).Scan(&resourceID, &tenantID)
		if err != nil {
			return nil // skip unknown webroots
		}

	case "database":
		err := a.db.QueryRow(ctx,
			`SELECT id, tenant_id FROM databases WHERE name = $1`, params.Name,
		).Scan(&resourceID, &tenantID)
		if err != nil {
			return nil // skip unknown databases
		}

	default:
		return fmt.Errorf("unsupported resource type: %s", params.ResourceType)
	}

	_, err := a.db.Exec(ctx,
		`INSERT INTO resource_usage (id, resource_type, resource_id, tenant_id, bytes_used, collected_at)
		 VALUES ($1, $2, $3, $4, $5, now())
		 ON CONFLICT (resource_type, resource_id) DO UPDATE SET bytes_used = $5, collected_at = now()`,
		resourceID+"-usage", params.ResourceType, resourceID, tenantID, params.BytesUsed,
	)
	return err
}

// ListResourceUsageByTenantID retrieves all resource usage rows for a tenant.
func (a *CoreDB) ListResourceUsageByTenantID(ctx context.Context, tenantID string) ([]model.ResourceUsage, error) {
	rows, err := a.db.Query(ctx,
		`SELECT ru.id, ru.resource_type, ru.resource_id, ru.tenant_id, ru.bytes_used, ru.collected_at
		 FROM resource_usage ru WHERE ru.tenant_id = $1 ORDER BY ru.resource_type, ru.collected_at DESC`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list resource usage by tenant: %w", err)
	}
	defer rows.Close()

	var usages []model.ResourceUsage
	for rows.Next() {
		var u model.ResourceUsage
		if err := rows.Scan(&u.ID, &u.ResourceType, &u.ResourceID, &u.TenantID, &u.BytesUsed, &u.CollectedAt); err != nil {
			return nil, fmt.Errorf("scan resource usage: %w", err)
		}
		usages = append(usages, u)
	}
	return usages, rows.Err()
}
