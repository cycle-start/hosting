package activity

import (
	"context"
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
	Table  string
	ID     string
	Status string
}

// UpdateResourceStatus sets the status of a resource row in the given table.
func (a *CoreDB) UpdateResourceStatus(ctx context.Context, params UpdateResourceStatusParams) error {
	query := fmt.Sprintf("UPDATE %s SET status = $1, updated_at = now() WHERE id = $2", params.Table)
	_, err := a.db.Exec(ctx, query, params.Status, params.ID)
	return err
}

// GetTenantByID retrieves a tenant by its ID.
func (a *CoreDB) GetTenantByID(ctx context.Context, id string) (*model.Tenant, error) {
	var t model.Tenant
	err := a.db.QueryRow(ctx,
		`SELECT id, name, region_id, cluster_id, shard_id, uid, sftp_enabled, status, created_at, updated_at
		 FROM tenants WHERE id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.RegionID, &t.ClusterID, &t.ShardID, &t.UID, &t.SFTPEnabled, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get tenant by id: %w", err)
	}
	return &t, nil
}

// GetWebrootByID retrieves a webroot by its ID.
func (a *CoreDB) GetWebrootByID(ctx context.Context, id string) (*model.Webroot, error) {
	var w model.Webroot
	err := a.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, runtime, runtime_version, runtime_config, public_folder, status, created_at, updated_at
		 FROM webroots WHERE id = $1`, id,
	).Scan(&w.ID, &w.TenantID, &w.Name, &w.Runtime, &w.RuntimeVersion, &w.RuntimeConfig, &w.PublicFolder, &w.Status, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get webroot by id: %w", err)
	}
	return &w, nil
}

// GetFQDNByID retrieves an FQDN by its ID.
func (a *CoreDB) GetFQDNByID(ctx context.Context, id string) (*model.FQDN, error) {
	var f model.FQDN
	err := a.db.QueryRow(ctx,
		`SELECT id, fqdn, webroot_id, ssl_enabled, status, created_at, updated_at
		 FROM fqdns WHERE id = $1`, id,
	).Scan(&f.ID, &f.FQDN, &f.WebrootID, &f.SSLEnabled, &f.Status, &f.CreatedAt, &f.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get fqdn by id: %w", err)
	}
	return &f, nil
}

// GetZoneByID retrieves a zone by its ID.
func (a *CoreDB) GetZoneByID(ctx context.Context, id string) (*model.Zone, error) {
	var z model.Zone
	err := a.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, region_id, status, created_at, updated_at
		 FROM zones WHERE id = $1`, id,
	).Scan(&z.ID, &z.TenantID, &z.Name, &z.RegionID, &z.Status, &z.CreatedAt, &z.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get zone by id: %w", err)
	}
	return &z, nil
}

// GetZoneByName retrieves a zone by its name. Used for auto-DNS lookups.
func (a *CoreDB) GetZoneByName(ctx context.Context, name string) (*model.Zone, error) {
	var z model.Zone
	err := a.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, region_id, status, created_at, updated_at
		 FROM zones WHERE name = $1 AND status = $2`, name, model.StatusActive,
	).Scan(&z.ID, &z.TenantID, &z.Name, &z.RegionID, &z.Status, &z.CreatedAt, &z.UpdatedAt)
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
		`SELECT id, zone_id, type, name, content, ttl, priority, managed_by, source_fqdn_id, status, created_at, updated_at
		 FROM zone_records WHERE id = $1`, id,
	).Scan(&r.ID, &r.ZoneID, &r.Type, &r.Name, &r.Content, &r.TTL, &r.Priority, &r.ManagedBy, &r.SourceFQDNID, &r.Status, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get zone record by id: %w", err)
	}
	return &r, nil
}

// GetDatabaseByID retrieves a database by its ID.
func (a *CoreDB) GetDatabaseByID(ctx context.Context, id string) (*model.Database, error) {
	var d model.Database
	err := a.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, shard_id, node_id, status, created_at, updated_at
		 FROM databases WHERE id = $1`, id,
	).Scan(&d.ID, &d.TenantID, &d.Name, &d.ShardID, &d.NodeID, &d.Status, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get database by id: %w", err)
	}
	return &d, nil
}

// GetDatabaseUserByID retrieves a database user by its ID.
func (a *CoreDB) GetDatabaseUserByID(ctx context.Context, id string) (*model.DatabaseUser, error) {
	var u model.DatabaseUser
	err := a.db.QueryRow(ctx,
		`SELECT id, database_id, username, password, privileges, status, created_at, updated_at
		 FROM database_users WHERE id = $1`, id,
	).Scan(&u.ID, &u.DatabaseID, &u.Username, &u.Password, &u.Privileges, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get database user by id: %w", err)
	}
	return &u, nil
}

// GetCertificateByID retrieves a certificate by its ID.
func (a *CoreDB) GetCertificateByID(ctx context.Context, id string) (*model.Certificate, error) {
	var c model.Certificate
	err := a.db.QueryRow(ctx,
		`SELECT id, fqdn_id, type, cert_pem, key_pem, chain_pem, issued_at, expires_at, status, is_active, created_at, updated_at
		 FROM certificates WHERE id = $1`, id,
	).Scan(&c.ID, &c.FQDNID, &c.Type, &c.CertPEM, &c.KeyPEM, &c.ChainPEM, &c.IssuedAt, &c.ExpiresAt, &c.Status, &c.IsActive, &c.CreatedAt, &c.UpdatedAt)
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
		`SELECT id, cluster_id, name, role, lb_backend, config, status, created_at, updated_at
		 FROM shards WHERE id = $1`, id,
	).Scan(&s.ID, &s.ClusterID, &s.Name, &s.Role, &s.LBBackend, &s.Config, &s.Status, &s.CreatedAt, &s.UpdatedAt)
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
		`SELECT id, fqdn, webroot_id, ssl_enabled, status, created_at, updated_at
		 FROM fqdns WHERE webroot_id = $1 AND status != $2`, webrootID, model.StatusDeleted,
	)
	if err != nil {
		return nil, fmt.Errorf("get fqdns by webroot id: %w", err)
	}
	defer rows.Close()

	var fqdns []model.FQDN
	for rows.Next() {
		var f model.FQDN
		if err := rows.Scan(&f.ID, &f.FQDN, &f.WebrootID, &f.SSLEnabled, &f.Status, &f.CreatedAt, &f.UpdatedAt); err != nil {
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
		`SELECT id, name, region_id, cluster_id, shard_id, uid, sftp_enabled, status, created_at, updated_at
		 FROM tenants WHERE shard_id = $1 ORDER BY name`, shardID,
	)
	if err != nil {
		return nil, fmt.Errorf("list tenants by shard: %w", err)
	}
	defer rows.Close()

	var tenants []model.Tenant
	for rows.Next() {
		var t model.Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.RegionID, &t.ClusterID, &t.ShardID, &t.UID, &t.SFTPEnabled, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
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
		`SELECT id, tenant_id, name, runtime, runtime_version, runtime_config, public_folder, status, created_at, updated_at
		 FROM webroots WHERE tenant_id = $1 AND status != $2`, tenantID, model.StatusDeleted,
	)
	if err != nil {
		return nil, fmt.Errorf("list webroots by tenant: %w", err)
	}
	defer rows.Close()

	var webroots []model.Webroot
	for rows.Next() {
		var w model.Webroot
		if err := rows.Scan(&w.ID, &w.TenantID, &w.Name, &w.Runtime, &w.RuntimeVersion, &w.RuntimeConfig, &w.PublicFolder, &w.Status, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan webroot row: %w", err)
		}
		webroots = append(webroots, w)
	}
	return webroots, rows.Err()
}

// ListDatabasesByTenantID retrieves all databases for a tenant.
func (a *CoreDB) ListDatabasesByTenantID(ctx context.Context, tenantID string) ([]model.Database, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, name, shard_id, node_id, status, created_at, updated_at
		 FROM databases WHERE tenant_id = $1 AND status != $2`, tenantID, model.StatusDeleted,
	)
	if err != nil {
		return nil, fmt.Errorf("list databases by tenant: %w", err)
	}
	defer rows.Close()

	var databases []model.Database
	for rows.Next() {
		var d model.Database
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.ShardID, &d.NodeID, &d.Status, &d.CreatedAt, &d.UpdatedAt); err != nil {
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
		`SELECT id, tenant_id, name, shard_id, port, max_memory_mb, password, status, created_at, updated_at
		 FROM valkey_instances WHERE id = $1`, id,
	).Scan(&v.ID, &v.TenantID, &v.Name, &v.ShardID, &v.Port, &v.MaxMemoryMB,
		&v.Password, &v.Status, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get valkey instance by id: %w", err)
	}
	return &v, nil
}

// GetValkeyUserByID retrieves a valkey user by its ID.
func (a *CoreDB) GetValkeyUserByID(ctx context.Context, id string) (*model.ValkeyUser, error) {
	var u model.ValkeyUser
	err := a.db.QueryRow(ctx,
		`SELECT id, valkey_instance_id, username, password, privileges, key_pattern, status, created_at, updated_at
		 FROM valkey_users WHERE id = $1`, id,
	).Scan(&u.ID, &u.ValkeyInstanceID, &u.Username, &u.Password,
		&u.Privileges, &u.KeyPattern, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get valkey user by id: %w", err)
	}
	return &u, nil
}

// GetEmailAccountByID retrieves an email account by its ID.
func (a *CoreDB) GetEmailAccountByID(ctx context.Context, id string) (*model.EmailAccount, error) {
	var acct model.EmailAccount
	err := a.db.QueryRow(ctx,
		`SELECT id, fqdn_id, address, display_name, quota_bytes, status, created_at, updated_at
		 FROM email_accounts WHERE id = $1`, id,
	).Scan(&acct.ID, &acct.FQDNID, &acct.Address, &acct.DisplayName, &acct.QuotaBytes, &acct.Status, &acct.CreatedAt, &acct.UpdatedAt)
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
		`INSERT INTO shards (id, cluster_id, name, role, lb_backend, config, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		s.ID, s.ClusterID, s.Name, s.Role, s.LBBackend, s.Config, s.Status, s.CreatedAt, s.UpdatedAt,
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
		`SELECT id, tenant_id, name, shard_id, node_id, status, created_at, updated_at
		 FROM databases WHERE shard_id = $1 AND status != $2 ORDER BY name`, shardID, model.StatusDeleted,
	)
	if err != nil {
		return nil, fmt.Errorf("list databases by shard: %w", err)
	}
	defer rows.Close()

	var databases []model.Database
	for rows.Next() {
		var d model.Database
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.ShardID, &d.NodeID, &d.Status, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan database row: %w", err)
		}
		databases = append(databases, d)
	}
	return databases, rows.Err()
}

// ListValkeyInstancesByShard retrieves all valkey instances assigned to a shard (excluding deleted).
func (a *CoreDB) ListValkeyInstancesByShard(ctx context.Context, shardID string) ([]model.ValkeyInstance, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, name, shard_id, port, max_memory_mb, password, status, created_at, updated_at
		 FROM valkey_instances WHERE shard_id = $1 AND status != $2 ORDER BY name`, shardID, model.StatusDeleted,
	)
	if err != nil {
		return nil, fmt.Errorf("list valkey instances by shard: %w", err)
	}
	defer rows.Close()

	var instances []model.ValkeyInstance
	for rows.Next() {
		var v model.ValkeyInstance
		if err := rows.Scan(&v.ID, &v.TenantID, &v.Name, &v.ShardID, &v.Port, &v.MaxMemoryMB, &v.Password, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan valkey instance row: %w", err)
		}
		instances = append(instances, v)
	}
	return instances, rows.Err()
}

// ListDatabaseUsersByDatabaseID retrieves all users for a database (excluding deleted).
func (a *CoreDB) ListDatabaseUsersByDatabaseID(ctx context.Context, databaseID string) ([]model.DatabaseUser, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, database_id, username, password, privileges, status, created_at, updated_at
		 FROM database_users WHERE database_id = $1 AND status != $2`, databaseID, model.StatusDeleted,
	)
	if err != nil {
		return nil, fmt.Errorf("list database users by database: %w", err)
	}
	defer rows.Close()

	var users []model.DatabaseUser
	for rows.Next() {
		var u model.DatabaseUser
		if err := rows.Scan(&u.ID, &u.DatabaseID, &u.Username, &u.Password, &u.Privileges, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan database user row: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// ListValkeyUsersByInstanceID retrieves all users for a valkey instance (excluding deleted).
func (a *CoreDB) ListValkeyUsersByInstanceID(ctx context.Context, instanceID string) ([]model.ValkeyUser, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, valkey_instance_id, username, password, privileges, key_pattern, status, created_at, updated_at
		 FROM valkey_users WHERE valkey_instance_id = $1 AND status != $2`, instanceID, model.StatusDeleted,
	)
	if err != nil {
		return nil, fmt.Errorf("list valkey users by instance: %w", err)
	}
	defer rows.Close()

	var users []model.ValkeyUser
	for rows.Next() {
		var u model.ValkeyUser
		if err := rows.Scan(&u.ID, &u.ValkeyInstanceID, &u.Username, &u.Password, &u.Privileges, &u.KeyPattern, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
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
		`SELECT id, email_account_id, address, status, created_at, updated_at
		 FROM email_aliases WHERE id = $1`, id,
	).Scan(&alias.ID, &alias.EmailAccountID, &alias.Address, &alias.Status, &alias.CreatedAt, &alias.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get email alias by id: %w", err)
	}
	return &alias, nil
}

// GetEmailForwardByID retrieves an email forward by its ID.
func (a *CoreDB) GetEmailForwardByID(ctx context.Context, id string) (*model.EmailForward, error) {
	var fwd model.EmailForward
	err := a.db.QueryRow(ctx,
		`SELECT id, email_account_id, destination, keep_copy, status, created_at, updated_at
		 FROM email_forwards WHERE id = $1`, id,
	).Scan(&fwd.ID, &fwd.EmailAccountID, &fwd.Destination, &fwd.KeepCopy, &fwd.Status, &fwd.CreatedAt, &fwd.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get email forward by id: %w", err)
	}
	return &fwd, nil
}

// GetEmailAutoReplyByAccountID retrieves the auto-reply for an email account.
func (a *CoreDB) GetEmailAutoReplyByAccountID(ctx context.Context, accountID string) (*model.EmailAutoReply, error) {
	var ar model.EmailAutoReply
	err := a.db.QueryRow(ctx,
		`SELECT id, email_account_id, subject, body, start_date, end_date, enabled, status, created_at, updated_at
		 FROM email_autoreplies WHERE email_account_id = $1`, accountID,
	).Scan(&ar.ID, &ar.EmailAccountID, &ar.Subject, &ar.Body, &ar.StartDate, &ar.EndDate, &ar.Enabled, &ar.Status, &ar.CreatedAt, &ar.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get email autoreply by account id: %w", err)
	}
	return &ar, nil
}

// GetEmailAutoReplyByID retrieves an email auto-reply by its ID.
func (a *CoreDB) GetEmailAutoReplyByID(ctx context.Context, id string) (*model.EmailAutoReply, error) {
	var ar model.EmailAutoReply
	err := a.db.QueryRow(ctx,
		`SELECT id, email_account_id, subject, body, start_date, end_date, enabled, status, created_at, updated_at
		 FROM email_autoreplies WHERE id = $1`, id,
	).Scan(&ar.ID, &ar.EmailAccountID, &ar.Subject, &ar.Body, &ar.StartDate, &ar.EndDate, &ar.Enabled, &ar.Status, &ar.CreatedAt, &ar.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get email autoreply by id: %w", err)
	}
	return &ar, nil
}

// GetExpiringLECerts returns Let's Encrypt certificates expiring within the given number of days.
func (a *CoreDB) GetExpiringLECerts(ctx context.Context, daysBeforeExpiry int) ([]model.Certificate, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, fqdn_id, type, cert_pem, key_pem, chain_pem, issued_at, expires_at, status, is_active, created_at, updated_at
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
			&c.IssuedAt, &c.ExpiresAt, &c.Status, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan expiring cert: %w", err)
		}
		certs = append(certs, c)
	}
	return certs, rows.Err()
}

// GetExpiredCerts returns certificates that have been expired for more than the given number of days.
func (a *CoreDB) GetExpiredCerts(ctx context.Context, daysAfterExpiry int) ([]model.Certificate, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, fqdn_id, type, cert_pem, key_pem, chain_pem, issued_at, expires_at, status, is_active, created_at, updated_at
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
			&c.IssuedAt, &c.ExpiresAt, &c.Status, &c.IsActive, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan expired cert: %w", err)
		}
		certs = append(certs, c)
	}
	return certs, rows.Err()
}

// GetSFTPKeyByID retrieves an SFTP key by its ID.
func (a *CoreDB) GetSFTPKeyByID(ctx context.Context, id string) (*model.SFTPKey, error) {
	var k model.SFTPKey
	err := a.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, public_key, fingerprint, status, created_at, updated_at
		 FROM sftp_keys WHERE id = $1`, id,
	).Scan(&k.ID, &k.TenantID, &k.Name, &k.PublicKey, &k.Fingerprint,
		&k.Status, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get sftp key by id: %w", err)
	}
	return &k, nil
}

// GetSFTPKeysByTenant retrieves all active SFTP keys for a tenant.
func (a *CoreDB) GetSFTPKeysByTenant(ctx context.Context, tenantID string) ([]model.SFTPKey, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, name, public_key, fingerprint, status, created_at, updated_at
		 FROM sftp_keys WHERE tenant_id = $1 AND status = $2`, tenantID, model.StatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("get sftp keys by tenant: %w", err)
	}
	defer rows.Close()

	var keys []model.SFTPKey
	for rows.Next() {
		var k model.SFTPKey
		if err := rows.Scan(&k.ID, &k.TenantID, &k.Name, &k.PublicKey, &k.Fingerprint,
			&k.Status, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan sftp key row: %w", err)
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
		`SELECT id, tenant_id, type, source_id, source_name, storage_path, size_bytes, status, started_at, completed_at, created_at, updated_at
		 FROM backups WHERE id = $1`, id,
	).Scan(&b.ID, &b.TenantID, &b.Type, &b.SourceID, &b.SourceName,
		&b.StoragePath, &b.SizeBytes, &b.Status, &b.StartedAt,
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

// GetOldBackups returns active backups that are older than the specified number of days.
func (a *CoreDB) GetOldBackups(ctx context.Context, retentionDays int) ([]model.Backup, error) {
	rows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, type, source_id, source_name, storage_path, size_bytes, status, started_at, completed_at, created_at, updated_at
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
			&b.StoragePath, &b.SizeBytes, &b.Status, &b.StartedAt,
			&b.CompletedAt, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan old backup: %w", err)
		}
		backups = append(backups, b)
	}
	return backups, rows.Err()
}
