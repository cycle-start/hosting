package activity

import (
	"context"
	"encoding/json"
	"fmt"
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
	if params.Status == model.StatusActive || params.Status == model.StatusDeleted {
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

// GetBrandByID retrieves a brand by its ID.
func (a *CoreDB) GetBrandByID(ctx context.Context, id string) (*model.Brand, error) {
	var b model.Brand
	err := a.db.QueryRow(ctx,
		`SELECT id, name, base_hostname, primary_ns, secondary_ns, hostmaster_email, status, created_at, updated_at
		 FROM brands WHERE id = $1`, id,
	).Scan(&b.ID, &b.Name, &b.BaseHostname, &b.PrimaryNS, &b.SecondaryNS,
		&b.HostmasterEmail, &b.Status, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get brand by id: %w", err)
	}
	return &b, nil
}

// GetTenantByID retrieves a tenant by its ID.
func (a *CoreDB) GetTenantByID(ctx context.Context, id string) (*model.Tenant, error) {
	var t model.Tenant
	err := a.db.QueryRow(ctx,
		`SELECT id, brand_id, region_id, cluster_id, shard_id, uid, sftp_enabled, ssh_enabled, disk_quota_bytes, status, status_message, created_at, updated_at
		 FROM tenants WHERE id = $1`, id,
	).Scan(&t.ID, &t.BrandID, &t.RegionID, &t.ClusterID, &t.ShardID, &t.UID, &t.SFTPEnabled, &t.SSHEnabled, &t.DiskQuotaBytes, &t.Status, &t.StatusMessage, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get tenant by id: %w", err)
	}
	return &t, nil
}

// GetWebrootByID retrieves a webroot by its ID.
func (a *CoreDB) GetWebrootByID(ctx context.Context, id string) (*model.Webroot, error) {
	var w model.Webroot
	err := a.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, runtime, runtime_version, runtime_config, public_folder, status, status_message, created_at, updated_at
		 FROM webroots WHERE id = $1`, id,
	).Scan(&w.ID, &w.TenantID, &w.Name, &w.Runtime, &w.RuntimeVersion, &w.RuntimeConfig, &w.PublicFolder, &w.Status, &w.StatusMessage, &w.CreatedAt, &w.UpdatedAt)
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
		`SELECT id, brand_id, tenant_id, name, region_id, status, status_message, created_at, updated_at
		 FROM zones WHERE id = $1`, id,
	).Scan(&z.ID, &z.BrandID, &z.TenantID, &z.Name, &z.RegionID, &z.Status, &z.StatusMessage, &z.CreatedAt, &z.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get zone by id: %w", err)
	}
	return &z, nil
}

// GetZoneByName retrieves a zone by its name. Used for auto-DNS lookups.
func (a *CoreDB) GetZoneByName(ctx context.Context, name string) (*model.Zone, error) {
	var z model.Zone
	err := a.db.QueryRow(ctx,
		`SELECT id, brand_id, tenant_id, name, region_id, status, status_message, created_at, updated_at
		 FROM zones WHERE name = $1 AND status = $2`, name, model.StatusActive,
	).Scan(&z.ID, &z.BrandID, &z.TenantID, &z.Name, &z.RegionID, &z.Status, &z.StatusMessage, &z.CreatedAt, &z.UpdatedAt)
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
		`SELECT id, tenant_id, name, shard_id, node_id, status, status_message, created_at, updated_at
		 FROM databases WHERE id = $1`, id,
	).Scan(&d.ID, &d.TenantID, &d.Name, &d.ShardID, &d.NodeID, &d.Status, &d.StatusMessage, &d.CreatedAt, &d.UpdatedAt)
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
		`SELECT id, cluster_id, shard_id, hostname, ip_address::text, ip6_address::text, roles, status, created_at, updated_at
		 FROM nodes WHERE cluster_id = $1 AND $2 = ANY(roles) AND status = $3`, clusterID, role, model.StatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("get nodes by cluster and role: %w", err)
	}
	defer rows.Close()

	var nodes []model.Node
	for rows.Next() {
		var n model.Node
		if err := rows.Scan(&n.ID, &n.ClusterID, &n.ShardID, &n.Hostname, &n.IPAddress, &n.IP6Address, &n.Roles, &n.Status, &n.CreatedAt, &n.UpdatedAt); err != nil {
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
		 FROM fqdns WHERE webroot_id = $1 AND status != $2`, webrootID, model.StatusDeleted,
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
		`SELECT id, brand_id, region_id, cluster_id, shard_id, uid, sftp_enabled, ssh_enabled, disk_quota_bytes, status, status_message, created_at, updated_at
		 FROM tenants WHERE shard_id = $1 ORDER BY id`, shardID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tenants by shard: %w", err)
	}
	defer rows.Close()

	var tenants []model.Tenant
	for rows.Next() {
		var t model.Tenant
		if err := rows.Scan(&t.ID, &t.BrandID, &t.RegionID, &t.ClusterID, &t.ShardID, &t.UID, &t.SFTPEnabled, &t.SSHEnabled, &t.DiskQuotaBytes, &t.Status, &t.StatusMessage, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan tenant row: %w", err)
		}
		tenants = append(tenants, t)
	}
	return tenants, rows.Err()
}

// ListNodesByShard retrieves all nodes assigned to a shard.
func (a *CoreDB) ListNodesByShard(ctx context.Context, shardID string) ([]model.Node, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, cluster_id, shard_id, hostname, ip_address::text, ip6_address::text, roles, status, created_at, updated_at
		 FROM nodes WHERE shard_id = $1 ORDER BY hostname`, shardID,
	)
	if err != nil {
		return nil, fmt.Errorf("list nodes by shard: %w", err)
	}
	defer rows.Close()

	var nodes []model.Node
	for rows.Next() {
		var n model.Node
		if err := rows.Scan(&n.ID, &n.ClusterID, &n.ShardID, &n.Hostname, &n.IPAddress, &n.IP6Address, &n.Roles, &n.Status, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan node row: %w", err)
		}
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
		`SELECT id, cluster_id, shard_id, hostname, ip_address::text, ip6_address::text, roles, status, created_at, updated_at
		 FROM nodes WHERE id = $1`, id,
	).Scan(&n.ID, &n.ClusterID, &n.ShardID, &n.Hostname, &n.IPAddress, &n.IP6Address,
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
		`SELECT id, tenant_id, name, runtime, runtime_version, runtime_config, public_folder, status, status_message, created_at, updated_at
		 FROM webroots WHERE tenant_id = $1 AND status != $2`, tenantID, model.StatusDeleted,
	)
	if err != nil {
		return nil, fmt.Errorf("list webroots by tenant: %w", err)
	}
	defer rows.Close()

	var webroots []model.Webroot
	for rows.Next() {
		var w model.Webroot
		if err := rows.Scan(&w.ID, &w.TenantID, &w.Name, &w.Runtime, &w.RuntimeVersion, &w.RuntimeConfig, &w.PublicFolder, &w.Status, &w.StatusMessage, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan webroot row: %w", err)
		}
		webroots = append(webroots, w)
	}
	return webroots, rows.Err()
}

// ListDatabasesByTenantID retrieves all databases for a tenant.
func (a *CoreDB) ListDatabasesByTenantID(ctx context.Context, tenantID string) ([]model.Database, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, name, shard_id, node_id, status, status_message, created_at, updated_at
		 FROM databases WHERE tenant_id = $1 AND status != $2`, tenantID, model.StatusDeleted,
	)
	if err != nil {
		return nil, fmt.Errorf("list databases by tenant: %w", err)
	}
	defer rows.Close()

	var databases []model.Database
	for rows.Next() {
		var d model.Database
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.ShardID, &d.NodeID, &d.Status, &d.StatusMessage, &d.CreatedAt, &d.UpdatedAt); err != nil {
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
		`SELECT id, tenant_id, name, shard_id, port, max_memory_mb, password, status, status_message, created_at, updated_at
		 FROM valkey_instances WHERE id = $1`, id,
	).Scan(&v.ID, &v.TenantID, &v.Name, &v.ShardID, &v.Port, &v.MaxMemoryMB,
		&v.Password, &v.Status, &v.StatusMessage, &v.CreatedAt, &v.UpdatedAt)
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
		`SELECT COUNT(*) FROM email_accounts WHERE fqdn_id = $1 AND status != $2`, fqdnID, model.StatusDeleted,
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
		`INSERT INTO nodes (id, cluster_id, shard_id, hostname, ip_address, ip6_address, roles, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		n.ID, n.ClusterID, n.ShardID, n.Hostname, n.IPAddress, n.IP6Address, n.Roles, n.Status, n.CreatedAt, n.UpdatedAt,
	)
	return err
}


// ListDatabasesByShard retrieves all databases assigned to a shard (excluding deleted).
func (a *CoreDB) ListDatabasesByShard(ctx context.Context, shardID string) ([]model.Database, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, name, shard_id, node_id, status, status_message, created_at, updated_at
		 FROM databases WHERE shard_id = $1 AND status != $2 ORDER BY name`, shardID, model.StatusDeleted,
	)
	if err != nil {
		return nil, fmt.Errorf("list databases by shard: %w", err)
	}
	defer rows.Close()

	var databases []model.Database
	for rows.Next() {
		var d model.Database
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.ShardID, &d.NodeID, &d.Status, &d.StatusMessage, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan database row: %w", err)
		}
		databases = append(databases, d)
	}
	return databases, rows.Err()
}

// ListValkeyInstancesByShard retrieves all valkey instances assigned to a shard (excluding deleted).
func (a *CoreDB) ListValkeyInstancesByShard(ctx context.Context, shardID string) ([]model.ValkeyInstance, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, name, shard_id, port, max_memory_mb, password, status, status_message, created_at, updated_at
		 FROM valkey_instances WHERE shard_id = $1 AND status != $2 ORDER BY name`, shardID, model.StatusDeleted,
	)
	if err != nil {
		return nil, fmt.Errorf("list valkey instances by shard: %w", err)
	}
	defer rows.Close()

	var instances []model.ValkeyInstance
	for rows.Next() {
		var v model.ValkeyInstance
		if err := rows.Scan(&v.ID, &v.TenantID, &v.Name, &v.ShardID, &v.Port, &v.MaxMemoryMB, &v.Password, &v.Status, &v.StatusMessage, &v.CreatedAt, &v.UpdatedAt); err != nil {
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
		 FROM database_users WHERE database_id = $1 AND status != $2`, databaseID, model.StatusDeleted,
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
		 FROM valkey_users WHERE valkey_instance_id = $1 AND status != $2`, instanceID, model.StatusDeleted,
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
		`SELECT id, tenant_id, name, shard_id, public, quota_bytes, status, status_message, created_at, updated_at
		 FROM s3_buckets WHERE id = $1`, id,
	).Scan(&b.ID, &b.TenantID, &b.Name, &b.ShardID,
		&b.Public, &b.QuotaBytes, &b.Status, &b.StatusMessage, &b.CreatedAt, &b.UpdatedAt)
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

	// JOIN webroots with tenants.
	err := a.db.QueryRow(ctx,
		`SELECT w.id, w.tenant_id, w.name, w.runtime, w.runtime_version, w.runtime_config, w.public_folder, w.status, w.status_message, w.created_at, w.updated_at,
		        t.id, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.status, t.status_message, t.created_at, t.updated_at
		 FROM webroots w
		 JOIN tenants t ON t.id = w.tenant_id
		 WHERE w.id = $1`, webrootID,
	).Scan(&wc.Webroot.ID, &wc.Webroot.TenantID, &wc.Webroot.Name, &wc.Webroot.Runtime, &wc.Webroot.RuntimeVersion, &wc.Webroot.RuntimeConfig, &wc.Webroot.PublicFolder, &wc.Webroot.Status, &wc.Webroot.StatusMessage, &wc.Webroot.CreatedAt, &wc.Webroot.UpdatedAt,
		&wc.Tenant.ID, &wc.Tenant.BrandID, &wc.Tenant.RegionID, &wc.Tenant.ClusterID, &wc.Tenant.ShardID, &wc.Tenant.UID, &wc.Tenant.SFTPEnabled, &wc.Tenant.Status, &wc.Tenant.StatusMessage, &wc.Tenant.CreatedAt, &wc.Tenant.UpdatedAt)
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

	return &wc, nil
}

// GetFQDNContext fetches an FQDN and its related webroot, tenant, shard, nodes, and LB addresses.
func (a *CoreDB) GetFQDNContext(ctx context.Context, fqdnID string) (*FQDNContext, error) {
	var fc FQDNContext

	// JOIN fqdns -> webroots -> tenants.
	err := a.db.QueryRow(ctx,
		`SELECT f.id, f.fqdn, f.webroot_id, f.ssl_enabled, f.status, f.status_message, f.created_at, f.updated_at,
		        w.id, w.tenant_id, w.name, w.runtime, w.runtime_version, w.runtime_config, w.public_folder, w.status, w.status_message, w.created_at, w.updated_at,
		        t.id, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.status, t.status_message, t.created_at, t.updated_at
		 FROM fqdns f
		 JOIN webroots w ON w.id = f.webroot_id
		 JOIN tenants t ON t.id = w.tenant_id
		 WHERE f.id = $1`, fqdnID,
	).Scan(&fc.FQDN.ID, &fc.FQDN.FQDN, &fc.FQDN.WebrootID, &fc.FQDN.SSLEnabled, &fc.FQDN.Status, &fc.FQDN.StatusMessage, &fc.FQDN.CreatedAt, &fc.FQDN.UpdatedAt,
		&fc.Webroot.ID, &fc.Webroot.TenantID, &fc.Webroot.Name, &fc.Webroot.Runtime, &fc.Webroot.RuntimeVersion, &fc.Webroot.RuntimeConfig, &fc.Webroot.PublicFolder, &fc.Webroot.Status, &fc.Webroot.StatusMessage, &fc.Webroot.CreatedAt, &fc.Webroot.UpdatedAt,
		&fc.Tenant.ID, &fc.Tenant.BrandID, &fc.Tenant.RegionID, &fc.Tenant.ClusterID, &fc.Tenant.ShardID, &fc.Tenant.UID, &fc.Tenant.SFTPEnabled, &fc.Tenant.Status, &fc.Tenant.StatusMessage, &fc.Tenant.CreatedAt, &fc.Tenant.UpdatedAt)
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
		        d.id, d.tenant_id, d.name, d.shard_id, d.node_id, d.status, d.status_message, d.created_at, d.updated_at
		 FROM database_users u
		 JOIN databases d ON d.id = u.database_id
		 WHERE u.id = $1`, userID,
	).Scan(&dc.User.ID, &dc.User.DatabaseID, &dc.User.Username, &dc.User.Password, &dc.User.Privileges, &dc.User.Status, &dc.User.StatusMessage, &dc.User.CreatedAt, &dc.User.UpdatedAt,
		&dc.Database.ID, &dc.Database.TenantID, &dc.Database.Name, &dc.Database.ShardID, &dc.Database.NodeID, &dc.Database.Status, &dc.Database.StatusMessage, &dc.Database.CreatedAt, &dc.Database.UpdatedAt)
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
		        i.id, i.tenant_id, i.name, i.shard_id, i.port, i.max_memory_mb, i.password, i.status, i.status_message, i.created_at, i.updated_at
		 FROM valkey_users u
		 JOIN valkey_instances i ON i.id = u.valkey_instance_id
		 WHERE u.id = $1`, userID,
	).Scan(&vc.User.ID, &vc.User.ValkeyInstanceID, &vc.User.Username, &vc.User.Password, &vc.User.Privileges, &vc.User.KeyPattern, &vc.User.Status, &vc.User.StatusMessage, &vc.User.CreatedAt, &vc.User.UpdatedAt,
		&vc.Instance.ID, &vc.Instance.TenantID, &vc.Instance.Name, &vc.Instance.ShardID, &vc.Instance.Port, &vc.Instance.MaxMemoryMB, &vc.Instance.Password, &vc.Instance.Status, &vc.Instance.StatusMessage, &vc.Instance.CreatedAt, &vc.Instance.UpdatedAt)
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
		`SELECT r.id, r.zone_id, r.type, r.name, r.content, r.ttl, r.priority, r.managed_by, r.source_fqdn_id, r.status, r.status_message, r.created_at, r.updated_at,
		        z.name
		 FROM zone_records r
		 JOIN zones z ON z.id = r.zone_id
		 WHERE r.id = $1`, recordID,
	).Scan(&zc.Record.ID, &zc.Record.ZoneID, &zc.Record.Type, &zc.Record.Name, &zc.Record.Content, &zc.Record.TTL, &zc.Record.Priority, &zc.Record.ManagedBy, &zc.Record.SourceFQDNID, &zc.Record.Status, &zc.Record.StatusMessage, &zc.Record.CreatedAt, &zc.Record.UpdatedAt,
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
		        t.id, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.status, t.status_message, t.created_at, t.updated_at
		 FROM backups b
		 JOIN tenants t ON t.id = b.tenant_id
		 WHERE b.id = $1`, backupID,
	).Scan(&bc.Backup.ID, &bc.Backup.TenantID, &bc.Backup.Type, &bc.Backup.SourceID, &bc.Backup.SourceName, &bc.Backup.StoragePath, &bc.Backup.SizeBytes, &bc.Backup.Status, &bc.Backup.StatusMessage, &bc.Backup.StartedAt, &bc.Backup.CompletedAt, &bc.Backup.CreatedAt, &bc.Backup.UpdatedAt,
		&bc.Tenant.ID, &bc.Tenant.BrandID, &bc.Tenant.RegionID, &bc.Tenant.ClusterID, &bc.Tenant.ShardID, &bc.Tenant.UID, &bc.Tenant.SFTPEnabled, &bc.Tenant.Status, &bc.Tenant.StatusMessage, &bc.Tenant.CreatedAt, &bc.Tenant.UpdatedAt)
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
		        b.id, b.tenant_id, b.name, b.shard_id, b.public, b.quota_bytes, b.status, b.status_message, b.created_at, b.updated_at
		 FROM s3_access_keys k
		 JOIN s3_buckets b ON b.id = k.s3_bucket_id
		 WHERE k.id = $1`, keyID,
	).Scan(&sc.Key.ID, &sc.Key.S3BucketID, &sc.Key.AccessKeyID, &sc.Key.SecretAccessKey, &sc.Key.Permissions, &sc.Key.Status, &sc.Key.StatusMessage, &sc.Key.CreatedAt, &sc.Key.UpdatedAt,
		&sc.Bucket.ID, &sc.Bucket.TenantID, &sc.Bucket.Name, &sc.Bucket.ShardID, &sc.Bucket.Public, &sc.Bucket.QuotaBytes, &sc.Bucket.Status, &sc.Bucket.StatusMessage, &sc.Bucket.CreatedAt, &sc.Bucket.UpdatedAt)
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

// GetStalwartContext resolves Stalwart connection info by traversing FQDN -> webroot -> tenant -> cluster.
func (a *CoreDB) GetStalwartContext(ctx context.Context, fqdnID string) (*StalwartContext, error) {
	var sc StalwartContext
	var clusterConfig []byte

	err := a.db.QueryRow(ctx,
		`SELECT f.id, f.fqdn, c.config
		 FROM fqdns f
		 JOIN webroots w ON w.id = f.webroot_id
		 JOIN tenants t ON t.id = w.tenant_id
		 JOIN clusters c ON c.id = t.cluster_id
		 WHERE f.id = $1`, fqdnID,
	).Scan(&sc.FQDNID, &sc.FQDN, &clusterConfig)
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
	sc.MailHostname = cfg.MailHostname

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
		        w.id, w.tenant_id, w.name, w.runtime, w.runtime_version, w.runtime_config, w.public_folder, w.status, w.status_message, w.created_at, w.updated_at,
		        t.id, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.status, t.status_message, t.created_at, t.updated_at
		 FROM cron_jobs c
		 JOIN webroots w ON w.id = c.webroot_id
		 JOIN tenants t ON t.id = c.tenant_id
		 WHERE c.id = $1`, cronJobID,
	).Scan(&cc.CronJob.ID, &cc.CronJob.TenantID, &cc.CronJob.WebrootID, &cc.CronJob.Name, &cc.CronJob.Schedule, &cc.CronJob.Command, &cc.CronJob.WorkingDirectory, &cc.CronJob.Enabled, &cc.CronJob.TimeoutSeconds, &cc.CronJob.MaxMemoryMB, &cc.CronJob.Status, &cc.CronJob.StatusMessage, &cc.CronJob.CreatedAt, &cc.CronJob.UpdatedAt,
		&cc.Webroot.ID, &cc.Webroot.TenantID, &cc.Webroot.Name, &cc.Webroot.Runtime, &cc.Webroot.RuntimeVersion, &cc.Webroot.RuntimeConfig, &cc.Webroot.PublicFolder, &cc.Webroot.Status, &cc.Webroot.StatusMessage, &cc.Webroot.CreatedAt, &cc.Webroot.UpdatedAt,
		&cc.Tenant.ID, &cc.Tenant.BrandID, &cc.Tenant.RegionID, &cc.Tenant.ClusterID, &cc.Tenant.ShardID, &cc.Tenant.UID, &cc.Tenant.SFTPEnabled, &cc.Tenant.Status, &cc.Tenant.StatusMessage, &cc.Tenant.CreatedAt, &cc.Tenant.UpdatedAt)
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
		 FROM cron_jobs WHERE webroot_id = $1 AND status != $2 ORDER BY name`, webrootID, model.StatusDeleted,
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
		`SELECT d.id, d.tenant_id, d.webroot_id, d.name, d.command, d.proxy_path, d.proxy_port,
		        d.num_procs, d.stop_signal, d.stop_wait_secs, d.max_memory_mb, d.environment,
		        d.enabled, d.status, d.status_message, d.created_at, d.updated_at,
		        w.id, w.tenant_id, w.name, w.runtime, w.runtime_version, w.runtime_config, w.public_folder, w.status, w.status_message, w.created_at, w.updated_at,
		        t.id, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.status, t.status_message, t.created_at, t.updated_at
		 FROM daemons d
		 JOIN webroots w ON w.id = d.webroot_id
		 JOIN tenants t ON t.id = d.tenant_id
		 WHERE d.id = $1`, daemonID,
	).Scan(&dc.Daemon.ID, &dc.Daemon.TenantID, &dc.Daemon.WebrootID, &dc.Daemon.Name, &dc.Daemon.Command,
		&dc.Daemon.ProxyPath, &dc.Daemon.ProxyPort,
		&dc.Daemon.NumProcs, &dc.Daemon.StopSignal, &dc.Daemon.StopWaitSecs, &dc.Daemon.MaxMemoryMB, &envJSON,
		&dc.Daemon.Enabled, &dc.Daemon.Status, &dc.Daemon.StatusMessage, &dc.Daemon.CreatedAt, &dc.Daemon.UpdatedAt,
		&dc.Webroot.ID, &dc.Webroot.TenantID, &dc.Webroot.Name, &dc.Webroot.Runtime, &dc.Webroot.RuntimeVersion, &dc.Webroot.RuntimeConfig, &dc.Webroot.PublicFolder, &dc.Webroot.Status, &dc.Webroot.StatusMessage, &dc.Webroot.CreatedAt, &dc.Webroot.UpdatedAt,
		&dc.Tenant.ID, &dc.Tenant.BrandID, &dc.Tenant.RegionID, &dc.Tenant.ClusterID, &dc.Tenant.ShardID, &dc.Tenant.UID, &dc.Tenant.SFTPEnabled, &dc.Tenant.Status, &dc.Tenant.StatusMessage, &dc.Tenant.CreatedAt, &dc.Tenant.UpdatedAt)
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
		`SELECT id, tenant_id, webroot_id, name, command, proxy_path, proxy_port,
		        num_procs, stop_signal, stop_wait_secs, max_memory_mb, environment,
		        enabled, status, status_message, created_at, updated_at
		 FROM daemons WHERE webroot_id = $1 AND status != $2 ORDER BY name`, webrootID, model.StatusDeleted,
	)
	if err != nil {
		return nil, fmt.Errorf("list daemons by webroot: %w", err)
	}
	defer rows.Close()

	var daemons []model.Daemon
	for rows.Next() {
		var d model.Daemon
		var envJSON []byte
		if err := rows.Scan(&d.ID, &d.TenantID, &d.WebrootID, &d.Name, &d.Command,
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

// ListDaemonsByTenant retrieves all active daemons for a tenant (used in convergence).
func (a *CoreDB) ListDaemonsByTenant(ctx context.Context, tenantID string) ([]model.Daemon, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, webroot_id, name, command, proxy_path, proxy_port,
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
		if err := rows.Scan(&d.ID, &d.TenantID, &d.WebrootID, &d.Name, &d.Command,
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
