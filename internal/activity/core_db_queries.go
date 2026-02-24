package activity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/edvin/hosting/internal/model"
)

// ListValkeyInstancesByTenantID retrieves all valkey instances for a tenant.
func (a *CoreDB) ListValkeyInstancesByTenantID(ctx context.Context, tenantID string) ([]model.ValkeyInstance, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, shard_id, port, max_memory_mb, password, status, status_message, suspend_reason, created_at, updated_at
		 FROM valkey_instances WHERE tenant_id = $1`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list valkey instances by tenant: %w", err)
	}
	defer rows.Close()

	var instances []model.ValkeyInstance
	for rows.Next() {
		var v model.ValkeyInstance
		if err := rows.Scan(&v.ID, &v.TenantID, &v.ShardID, &v.Port, &v.MaxMemoryMB,
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
		`SELECT id, tenant_id, shard_id, public, quota_bytes, status, status_message, suspend_reason, created_at, updated_at
		 FROM s3_buckets WHERE tenant_id = $1`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list s3 buckets by tenant: %w", err)
	}
	defer rows.Close()

	var buckets []model.S3Bucket
	for rows.Next() {
		var b model.S3Bucket
		if err := rows.Scan(&b.ID, &b.TenantID, &b.ShardID,
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
		`SELECT id, brand_id, region_id, cluster_id, shard_id, uid, sftp_enabled, ssh_enabled, disk_quota_bytes, status, status_message, suspend_reason, created_at, updated_at
		 FROM tenants WHERE id = $1`, id,
	).Scan(&t.ID, &t.BrandID, &t.RegionID, &t.ClusterID, &t.ShardID, &t.UID, &t.SFTPEnabled, &t.SSHEnabled, &t.DiskQuotaBytes, &t.Status, &t.StatusMessage, &t.SuspendReason, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get tenant by id: %w", err)
	}
	return &t, nil
}

// GetWebrootByID retrieves a webroot by its ID.
func (a *CoreDB) GetWebrootByID(ctx context.Context, id string) (*model.Webroot, error) {
	var w model.Webroot
	err := a.db.QueryRow(ctx,
		`SELECT id, tenant_id, runtime, runtime_version, runtime_config, public_folder, status, status_message, suspend_reason, created_at, updated_at
		 FROM webroots WHERE id = $1`, id,
	).Scan(&w.ID, &w.TenantID, &w.Runtime, &w.RuntimeVersion, &w.RuntimeConfig, &w.PublicFolder, &w.Status, &w.StatusMessage, &w.SuspendReason, &w.CreatedAt, &w.UpdatedAt)
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
		`SELECT id, tenant_id, shard_id, node_id, status, status_message, suspend_reason, created_at, updated_at
		 FROM databases WHERE id = $1`, id,
	).Scan(&d.ID, &d.TenantID, &d.ShardID, &d.NodeID, &d.Status, &d.StatusMessage, &d.SuspendReason, &d.CreatedAt, &d.UpdatedAt)
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
		`SELECT id, brand_id, region_id, cluster_id, shard_id, uid, sftp_enabled, ssh_enabled, disk_quota_bytes, status, status_message, suspend_reason, created_at, updated_at
		 FROM tenants WHERE shard_id = $1 ORDER BY id`, shardID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tenants by shard: %w", err)
	}
	defer rows.Close()

	var tenants []model.Tenant
	for rows.Next() {
		var t model.Tenant
		if err := rows.Scan(&t.ID, &t.BrandID, &t.RegionID, &t.ClusterID, &t.ShardID, &t.UID, &t.SFTPEnabled, &t.SSHEnabled, &t.DiskQuotaBytes, &t.Status, &t.StatusMessage, &t.SuspendReason, &t.CreatedAt, &t.UpdatedAt); err != nil {
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
		`SELECT id, tenant_id, runtime, runtime_version, runtime_config, public_folder, env_file_name, service_hostname_enabled, status, status_message, suspend_reason, created_at, updated_at
		 FROM webroots WHERE tenant_id = $1`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list webroots by tenant: %w", err)
	}
	defer rows.Close()

	var webroots []model.Webroot
	for rows.Next() {
		var w model.Webroot
		if err := rows.Scan(&w.ID, &w.TenantID, &w.Runtime, &w.RuntimeVersion, &w.RuntimeConfig, &w.PublicFolder, &w.EnvFileName, &w.ServiceHostnameEnabled, &w.Status, &w.StatusMessage, &w.SuspendReason, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan webroot row: %w", err)
		}
		webroots = append(webroots, w)
	}
	return webroots, rows.Err()
}

// ListDatabasesByTenantID retrieves all databases for a tenant.
func (a *CoreDB) ListDatabasesByTenantID(ctx context.Context, tenantID string) ([]model.Database, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, shard_id, node_id, status, status_message, suspend_reason, created_at, updated_at
		 FROM databases WHERE tenant_id = $1`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list databases by tenant: %w", err)
	}
	defer rows.Close()

	var databases []model.Database
	for rows.Next() {
		var d model.Database
		if err := rows.Scan(&d.ID, &d.TenantID, &d.ShardID, &d.NodeID, &d.Status, &d.StatusMessage, &d.SuspendReason, &d.CreatedAt, &d.UpdatedAt); err != nil {
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
		`SELECT id, tenant_id, shard_id, port, max_memory_mb, password, status, status_message, suspend_reason, created_at, updated_at
		 FROM valkey_instances WHERE id = $1`, id,
	).Scan(&v.ID, &v.TenantID, &v.ShardID, &v.Port, &v.MaxMemoryMB,
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
		`SELECT id, tenant_id, shard_id, node_id, status, status_message, suspend_reason, created_at, updated_at
		 FROM databases WHERE shard_id = $1 ORDER BY id`, shardID,
	)
	if err != nil {
		return nil, fmt.Errorf("list databases by shard: %w", err)
	}
	defer rows.Close()

	var databases []model.Database
	for rows.Next() {
		var d model.Database
		if err := rows.Scan(&d.ID, &d.TenantID, &d.ShardID, &d.NodeID, &d.Status, &d.StatusMessage, &d.SuspendReason, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan database row: %w", err)
		}
		databases = append(databases, d)
	}
	return databases, rows.Err()
}

// ListValkeyInstancesByShard retrieves all valkey instances assigned to a shard (excluding deleted).
func (a *CoreDB) ListValkeyInstancesByShard(ctx context.Context, shardID string) ([]model.ValkeyInstance, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, shard_id, port, max_memory_mb, password, status, status_message, suspend_reason, created_at, updated_at
		 FROM valkey_instances WHERE shard_id = $1 ORDER BY id`, shardID,
	)
	if err != nil {
		return nil, fmt.Errorf("list valkey instances by shard: %w", err)
	}
	defer rows.Close()

	var instances []model.ValkeyInstance
	for rows.Next() {
		var v model.ValkeyInstance
		if err := rows.Scan(&v.ID, &v.TenantID, &v.ShardID, &v.Port, &v.MaxMemoryMB, &v.Password, &v.Status, &v.StatusMessage, &v.SuspendReason, &v.CreatedAt, &v.UpdatedAt); err != nil {
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
// Returns an empty string (not an error) if the key does not exist.
func (a *CoreDB) GetPlatformConfig(ctx context.Context, key string) (string, error) {
	var value string
	err := a.db.QueryRow(ctx, `SELECT value FROM platform_config WHERE key = $1`, key).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
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
		`SELECT id, tenant_id, shard_id, public, quota_bytes, status, status_message, suspend_reason, created_at, updated_at
		 FROM s3_buckets WHERE id = $1`, id,
	).Scan(&b.ID, &b.TenantID, &b.ShardID,
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

// ListCronJobsByWebroot retrieves all cron jobs for a webroot (excluding deleted).
func (a *CoreDB) ListCronJobsByWebroot(ctx context.Context, webrootID string) ([]model.CronJob, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, webroot_id, schedule, command, working_directory, enabled, timeout_seconds, max_memory_mb, status, status_message, created_at, updated_at
		 FROM cron_jobs WHERE webroot_id = $1 ORDER BY id`, webrootID,
	)
	if err != nil {
		return nil, fmt.Errorf("list cron jobs by webroot: %w", err)
	}
	defer rows.Close()

	var jobs []model.CronJob
	for rows.Next() {
		var j model.CronJob
		if err := rows.Scan(&j.ID, &j.TenantID, &j.WebrootID, &j.Schedule, &j.Command, &j.WorkingDirectory, &j.Enabled, &j.TimeoutSeconds, &j.MaxMemoryMB, &j.Status, &j.StatusMessage, &j.CreatedAt, &j.UpdatedAt); err != nil {
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

// ListDaemonsByWebroot retrieves all daemons for a webroot (excluding deleted).
func (a *CoreDB) ListDaemonsByWebroot(ctx context.Context, webrootID string) ([]model.Daemon, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, node_id, webroot_id, command, proxy_path, proxy_port,
		        num_procs, stop_signal, stop_wait_secs, max_memory_mb,
		        enabled, status, status_message, created_at, updated_at
		 FROM daemons WHERE webroot_id = $1 ORDER BY id`, webrootID,
	)
	if err != nil {
		return nil, fmt.Errorf("list daemons by webroot: %w", err)
	}
	defer rows.Close()

	var daemons []model.Daemon
	for rows.Next() {
		var d model.Daemon
		if err := rows.Scan(&d.ID, &d.TenantID, &d.NodeID, &d.WebrootID, &d.Command,
			&d.ProxyPath, &d.ProxyPort,
			&d.NumProcs, &d.StopSignal, &d.StopWaitSecs, &d.MaxMemoryMB,
			&d.Enabled, &d.Status, &d.StatusMessage, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan daemon row: %w", err)
		}
		daemons = append(daemons, d)
	}
	return daemons, rows.Err()
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
		`SELECT id, tenant_id, node_id, webroot_id, command, proxy_path, proxy_port, num_procs, stop_signal, stop_wait_secs, max_memory_mb, enabled, status, status_message, created_at, updated_at
		 FROM daemons WHERE webroot_id = $1`, webrootID,
	)
	if err != nil {
		return nil, fmt.Errorf("list daemons by webroot: %w", err)
	}
	defer rows.Close()

	var daemons []model.Daemon
	for rows.Next() {
		var d model.Daemon
		if err := rows.Scan(&d.ID, &d.TenantID, &d.NodeID, &d.WebrootID, &d.Command, &d.ProxyPath, &d.ProxyPort, &d.NumProcs, &d.StopSignal, &d.StopWaitSecs, &d.MaxMemoryMB, &d.Enabled, &d.Status, &d.StatusMessage, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan daemon row: %w", err)
		}
		daemons = append(daemons, d)
	}
	return daemons, rows.Err()
}

// ListCronJobsByWebrootID retrieves all cron jobs for a webroot.
func (a *CoreDB) ListCronJobsByWebrootID(ctx context.Context, webrootID string) ([]model.CronJob, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, webroot_id, schedule, command, working_directory, enabled, timeout_seconds, max_memory_mb, consecutive_failures, max_failures, status, status_message, created_at, updated_at
		 FROM cron_jobs WHERE webroot_id = $1`, webrootID,
	)
	if err != nil {
		return nil, fmt.Errorf("list cron jobs by webroot: %w", err)
	}
	defer rows.Close()

	var jobs []model.CronJob
	for rows.Next() {
		var c model.CronJob
		if err := rows.Scan(&c.ID, &c.TenantID, &c.WebrootID, &c.Schedule, &c.Command, &c.WorkingDirectory, &c.Enabled, &c.TimeoutSeconds, &c.MaxMemoryMB, &c.ConsecutiveFailures, &c.MaxFailures, &c.Status, &c.StatusMessage, &c.CreatedAt, &c.UpdatedAt); err != nil {
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

// ListEmailAccountsByTenantID retrieves all email accounts for a tenant by
// joining through webroots → fqdns → email_accounts.
func (a *CoreDB) ListEmailAccountsByTenantID(ctx context.Context, tenantID string) ([]model.EmailAccount, error) {
	rows, err := a.db.Query(ctx,
		`SELECT ea.id, ea.fqdn_id, ea.address, ea.display_name, ea.quota_bytes, ea.status, ea.status_message, ea.created_at, ea.updated_at
		 FROM email_accounts ea
		 JOIN fqdns f ON ea.fqdn_id = f.id
		 JOIN webroots w ON f.webroot_id = w.id
		 WHERE w.tenant_id = $1`, tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list email accounts by tenant: %w", err)
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
			 WHERE t.id = $1 AND w.id = $2`, parts[0], parts[1],
		).Scan(&resourceID, &tenantID)
		if err != nil {
			return nil // skip unknown webroots
		}

	case "database":
		err := a.db.QueryRow(ctx,
			`SELECT id, tenant_id FROM databases WHERE id = $1`, params.Name,
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
