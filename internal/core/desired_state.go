package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
)

type DesiredStateService struct {
	db DB
}

func NewDesiredStateService(db DB) *DesiredStateService {
	return &DesiredStateService{db: db}
}

// GetForNode builds the complete desired state for a node.
func (s *DesiredStateService) GetForNode(ctx context.Context, nodeID string) (*model.DesiredState, error) {
	// Get node info including shard
	var shardID, shardRole string
	err := s.db.QueryRow(ctx, `
		SELECT s.id, s.role FROM nodes n
		JOIN shards s ON n.shard_id = s.id
		WHERE n.id = $1`, nodeID).Scan(&shardID, &shardRole)
	if err != nil {
		return nil, fmt.Errorf("get node shard info: %w", err)
	}

	ds := &model.DesiredState{
		NodeID:    nodeID,
		ShardID:   shardID,
		ShardRole: shardRole,
	}

	switch shardRole {
	case "web":
		if err := s.loadWebState(ctx, shardID, ds); err != nil {
			return nil, fmt.Errorf("load web state: %w", err)
		}
	case "database":
		if err := s.loadDatabaseState(ctx, shardID, ds); err != nil {
			return nil, fmt.Errorf("load database state: %w", err)
		}
	case "valkey":
		if err := s.loadValkeyState(ctx, shardID, ds); err != nil {
			return nil, fmt.Errorf("load valkey state: %w", err)
		}
	case "lb":
		if err := s.loadLBState(ctx, shardID, ds); err != nil {
			return nil, fmt.Errorf("load lb state: %w", err)
		}
	case "storage":
		if err := s.loadStorageState(ctx, shardID, ds); err != nil {
			return nil, fmt.Errorf("load storage state: %w", err)
		}
	}

	return ds, nil
}

func (s *DesiredStateService) loadWebState(ctx context.Context, shardID string, ds *model.DesiredState) error {
	// Get active/suspended tenants assigned to this shard
	rows, err := s.db.Query(ctx, `
		SELECT t.id, t.name, t.uid, t.sftp_enabled, t.ssh_enabled, t.status
		FROM tenants t
		WHERE t.shard_id = $1 AND t.status IN ('active', 'suspended')
		ORDER BY t.id`, shardID)
	if err != nil {
		return fmt.Errorf("query tenants: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var t model.DesiredTenant
		if err := rows.Scan(&t.ID, &t.Name, &t.UID, &t.SFTPEnabled, &t.SSHEnabled, &t.Status); err != nil {
			return fmt.Errorf("scan tenant: %w", err)
		}

		// Load webroots for this tenant
		wrRows, err := s.db.Query(ctx, `
			SELECT id, name, runtime, runtime_version, runtime_config::text, public_folder, status
			FROM webroots WHERE tenant_id = $1 AND status = 'active'
			ORDER BY name`, t.ID)
		if err != nil {
			return fmt.Errorf("query webroots for tenant %s: %w", t.ID, err)
		}
		for wrRows.Next() {
			var wr model.DesiredWebroot
			if err := wrRows.Scan(&wr.ID, &wr.Name, &wr.Runtime, &wr.RuntimeVersion, &wr.RuntimeConfig, &wr.PublicFolder, &wr.Status); err != nil {
				wrRows.Close()
				return fmt.Errorf("scan webroot: %w", err)
			}

			// Load FQDNs for this webroot
			fqdnRows, err := s.db.Query(ctx, `
				SELECT fqdn, ssl_enabled, status
				FROM fqdns WHERE webroot_id = $1 AND status = 'active'
				ORDER BY fqdn`, wr.ID)
			if err != nil {
				wrRows.Close()
				return fmt.Errorf("query fqdns for webroot %s: %w", wr.ID, err)
			}
			for fqdnRows.Next() {
				var f model.DesiredFQDN
				if err := fqdnRows.Scan(&f.FQDN, &f.SSLEnabled, &f.Status); err != nil {
					fqdnRows.Close()
					wrRows.Close()
					return fmt.Errorf("scan fqdn: %w", err)
				}
				wr.FQDNs = append(wr.FQDNs, f)
			}
			fqdnRows.Close()

			// Load cron jobs for this webroot
			cronRows, err := s.db.Query(ctx, `
				SELECT id, name, enabled FROM cron_jobs
				WHERE webroot_id = $1 AND status IN ('active', 'auto_disabled')
				ORDER BY id`, wr.ID)
			if err != nil {
				wrRows.Close()
				return fmt.Errorf("query cron jobs for webroot %s: %w", wr.ID, err)
			}
			for cronRows.Next() {
				var cj model.DesiredCronJob
				if err := cronRows.Scan(&cj.ID, &cj.Name, &cj.Enabled); err != nil {
					cronRows.Close()
					wrRows.Close()
					return fmt.Errorf("scan cron job: %w", err)
				}
				wr.CronJobs = append(wr.CronJobs, cj)
			}
			cronRows.Close()

			t.Webroots = append(t.Webroots, wr)
		}
		wrRows.Close()

		// Load SSH keys for tenant
		sshRows, err := s.db.Query(ctx, `
			SELECT public_key FROM ssh_keys WHERE tenant_id = $1 AND status = 'active'`, t.ID)
		if err != nil {
			return fmt.Errorf("query ssh keys for tenant %s: %w", t.ID, err)
		}
		for sshRows.Next() {
			var key string
			if err := sshRows.Scan(&key); err != nil {
				sshRows.Close()
				return fmt.Errorf("scan ssh key: %w", err)
			}
			t.SSHKeys = append(t.SSHKeys, key)
		}
		sshRows.Close()

		ds.Tenants = append(ds.Tenants, t)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tenants: %w", err)
	}
	return nil
}

func (s *DesiredStateService) loadDatabaseState(ctx context.Context, shardID string, ds *model.DesiredState) error {
	rows, err := s.db.Query(ctx, `
		SELECT id, name, status FROM databases
		WHERE shard_id = $1 AND status = 'active'
		ORDER BY name`, shardID)
	if err != nil {
		return fmt.Errorf("query databases: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var d model.DesiredDatabase
		if err := rows.Scan(&d.ID, &d.Name, &d.Status); err != nil {
			return fmt.Errorf("scan database: %w", err)
		}

		userRows, err := s.db.Query(ctx, `
			SELECT id, username, password, privileges, status FROM database_users
			WHERE database_id = $1 AND status = 'active'
			ORDER BY username`, d.ID)
		if err != nil {
			return fmt.Errorf("query database users for db %s: %w", d.ID, err)
		}
		for userRows.Next() {
			var u model.DesiredDBUser
			if err := userRows.Scan(&u.ID, &u.Username, &u.Password, &u.Privileges, &u.Status); err != nil {
				userRows.Close()
				return fmt.Errorf("scan database user: %w", err)
			}
			d.Users = append(d.Users, u)
		}
		userRows.Close()

		ds.Databases = append(ds.Databases, d)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate databases: %w", err)
	}
	return nil
}

func (s *DesiredStateService) loadValkeyState(ctx context.Context, shardID string, ds *model.DesiredState) error {
	rows, err := s.db.Query(ctx, `
		SELECT id, name, port, password, max_memory_mb, status
		FROM valkey_instances WHERE shard_id = $1 AND status = 'active'
		ORDER BY name`, shardID)
	if err != nil {
		return fmt.Errorf("query valkey instances: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var vi model.DesiredValkeyInstance
		if err := rows.Scan(&vi.ID, &vi.Name, &vi.Port, &vi.Password, &vi.MaxMemoryMB, &vi.Status); err != nil {
			return fmt.Errorf("scan valkey instance: %w", err)
		}

		userRows, err := s.db.Query(ctx, `
			SELECT id, username, password, array_to_string(privileges, ','), key_pattern, status
			FROM valkey_users WHERE valkey_instance_id = $1 AND status = 'active'
			ORDER BY username`, vi.ID)
		if err != nil {
			return fmt.Errorf("query valkey users for instance %s: %w", vi.ID, err)
		}
		for userRows.Next() {
			var u model.DesiredValkeyUser
			if err := userRows.Scan(&u.ID, &u.Username, &u.Password, &u.Privileges, &u.KeyPattern, &u.Status); err != nil {
				userRows.Close()
				return fmt.Errorf("scan valkey user: %w", err)
			}
			vi.Users = append(vi.Users, u)
		}
		userRows.Close()

		ds.ValkeyInstances = append(ds.ValkeyInstances, vi)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate valkey instances: %w", err)
	}
	return nil
}

func (s *DesiredStateService) loadLBState(ctx context.Context, shardID string, ds *model.DesiredState) error {
	// For LB shards, get all FQDNs that point to web shards in the same cluster.
	// The LB node serves all web shards in its cluster.
	rows, err := s.db.Query(ctx, `
		SELECT f.fqdn, s.lb_backend
		FROM fqdns f
		JOIN webroots w ON f.webroot_id = w.id
		JOIN tenants t ON w.tenant_id = t.id
		JOIN shards s ON t.shard_id = s.id
		JOIN shards lb ON lb.id = $1
		WHERE s.cluster_id = lb.cluster_id
			AND f.status = 'active'
			AND t.status IN ('active', 'suspended')
		ORDER BY f.fqdn`, shardID)
	if err != nil {
		return fmt.Errorf("query lb fqdn mappings: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var m model.DesiredFQDNMapping
		if err := rows.Scan(&m.FQDN, &m.LBBackend); err != nil {
			return fmt.Errorf("scan fqdn mapping: %w", err)
		}
		ds.FQDNMappings = append(ds.FQDNMappings, m)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate fqdn mappings: %w", err)
	}
	return nil
}

func (s *DesiredStateService) loadStorageState(ctx context.Context, shardID string, ds *model.DesiredState) error {
	rows, err := s.db.Query(ctx, `
		SELECT id, name, tenant_id, status FROM s3_buckets
		WHERE shard_id = $1 AND status = 'active'
		ORDER BY name`, shardID)
	if err != nil {
		return fmt.Errorf("query s3 buckets: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var b model.DesiredS3Bucket
		var tenantID *string
		if err := rows.Scan(&b.ID, &b.Name, &tenantID, &b.Status); err != nil {
			return fmt.Errorf("scan s3 bucket: %w", err)
		}
		if tenantID != nil {
			b.TenantID = *tenantID
		}
		ds.S3Buckets = append(ds.S3Buckets, b)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate s3 buckets: %w", err)
	}
	return nil
}
