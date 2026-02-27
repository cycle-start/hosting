package core

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/edvin/hosting/internal/crypto"
	"github.com/edvin/hosting/internal/model"
)

type DesiredStateService struct {
	db     DB
	kekHex string
}

func NewDesiredStateService(db DB, kekHex string) *DesiredStateService {
	return &DesiredStateService{db: db, kekHex: kekHex}
}

// GetForNode builds the complete desired state for a node across all its shard assignments.
func (s *DesiredStateService) GetForNode(ctx context.Context, nodeID string) (*model.DesiredState, error) {
	// Get all shard assignments for this node.
	rows, err := s.db.Query(ctx, `
		SELECT s.id, s.role
		FROM node_shard_assignments nsa
		JOIN shards s ON nsa.shard_id = s.id
		WHERE nsa.node_id = $1
		ORDER BY s.role`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("get node shard assignments: %w", err)
	}
	defer rows.Close()

	type shardInfo struct {
		id   string
		role string
	}
	var shardInfos []shardInfo
	for rows.Next() {
		var si shardInfo
		if err := rows.Scan(&si.id, &si.role); err != nil {
			return nil, fmt.Errorf("scan shard info: %w", err)
		}
		shardInfos = append(shardInfos, si)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate shard assignments: %w", err)
	}

	ds := &model.DesiredState{
		NodeID: nodeID,
	}

	for _, si := range shardInfos {
		ss := model.ShardState{
			ShardID:   si.id,
			ShardRole: si.role,
		}

		switch si.role {
		case "web":
			if err := s.loadWebState(ctx, si.id, &ss); err != nil {
				return nil, fmt.Errorf("load web state: %w", err)
			}
		case "database":
			if err := s.loadDatabaseState(ctx, si.id, &ss); err != nil {
				return nil, fmt.Errorf("load database state: %w", err)
			}
		case "valkey":
			if err := s.loadValkeyState(ctx, si.id, &ss); err != nil {
				return nil, fmt.Errorf("load valkey state: %w", err)
			}
		case "lb":
			if err := s.loadLBState(ctx, si.id, &ss); err != nil {
				return nil, fmt.Errorf("load lb state: %w", err)
			}
		case "storage":
			if err := s.loadStorageState(ctx, si.id, &ss); err != nil {
				return nil, fmt.Errorf("load storage state: %w", err)
			}
		}

		ds.Shards = append(ds.Shards, ss)
	}

	return ds, nil
}

func (s *DesiredStateService) loadWebState(ctx context.Context, shardID string, ss *model.ShardState) error {
	// 1. Fetch all active/suspended tenants for this shard.
	rows, err := s.db.Query(ctx, `
		SELECT t.id, t.uid, t.sftp_enabled, t.ssh_enabled, t.status
		FROM tenants t
		WHERE t.shard_id = $1 AND t.status IN ('active', 'suspended')
		ORDER BY t.id`, shardID)
	if err != nil {
		return fmt.Errorf("query tenants: %w", err)
	}
	defer rows.Close()

	var tenants []model.DesiredTenant
	var tenantIDs []string
	for rows.Next() {
		var t model.DesiredTenant
		if err := rows.Scan(&t.ID, &t.UID, &t.SFTPEnabled, &t.SSHEnabled, &t.Status); err != nil {
			return fmt.Errorf("scan tenant: %w", err)
		}
		tenants = append(tenants, t)
		tenantIDs = append(tenantIDs, t.ID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tenants: %w", err)
	}

	if len(tenantIDs) == 0 {
		return nil
	}

	// 2. Batch-fetch all active webroots for those tenants.
	wrRows, err := s.db.Query(ctx, `
		SELECT id, tenant_id, runtime, runtime_version, runtime_config::text,
		       public_folder, env_file_name, status
		FROM webroots WHERE tenant_id = ANY($1) AND status = 'active'
		ORDER BY id`, tenantIDs)
	if err != nil {
		return fmt.Errorf("batch query webroots: %w", err)
	}
	defer wrRows.Close()

	type indexedWebroot struct {
		webroot  model.DesiredWebroot
		tenantID string
	}
	var webroots []indexedWebroot
	var webrootIDs []string
	webrootsByTenant := make(map[string][]int) // tenant ID -> indices in webroots slice
	for wrRows.Next() {
		var wr model.DesiredWebroot
		var tenantID string
		if err := wrRows.Scan(&wr.ID, &tenantID, &wr.Runtime, &wr.RuntimeVersion,
			&wr.RuntimeConfig, &wr.PublicFolder, &wr.EnvFileName, &wr.Status); err != nil {
			return fmt.Errorf("scan webroot: %w", err)
		}
		idx := len(webroots)
		webroots = append(webroots, indexedWebroot{webroot: wr, tenantID: tenantID})
		webrootIDs = append(webrootIDs, wr.ID)
		webrootsByTenant[tenantID] = append(webrootsByTenant[tenantID], idx)
	}
	if err := wrRows.Err(); err != nil {
		return fmt.Errorf("iterate webroots: %w", err)
	}

	if len(webrootIDs) > 0 {
		// 3. Batch-fetch all env vars for those webroots.
		envRows, err := s.db.Query(ctx, `
			SELECT webroot_id, name, value, is_secret
			FROM webroot_env_vars WHERE webroot_id = ANY($1)
			ORDER BY name`, webrootIDs)
		if err != nil {
			return fmt.Errorf("batch query env vars: %w", err)
		}
		defer envRows.Close()

		// Collect env var rows for decryption.
		type envRow struct {
			webrootID string
			tenantID  string
			name      string
			value     string
			isSecret  bool
		}
		var envVarRows []envRow
		for envRows.Next() {
			var r envRow
			if err := envRows.Scan(&r.webrootID, &r.name, &r.value, &r.isSecret); err != nil {
				return fmt.Errorf("scan env var: %w", err)
			}
			envVarRows = append(envVarRows, r)
		}
		if err := envRows.Err(); err != nil {
			return fmt.Errorf("iterate env vars: %w", err)
		}

		// Resolve tenant IDs for webroots that have secrets.
		webrootTenantMap := make(map[string]string) // webroot ID -> tenant ID
		for _, iwr := range webroots {
			webrootTenantMap[iwr.webroot.ID] = iwr.tenantID
		}

		// Decrypt secret values using KEK -> tenant DEK.
		var kek []byte
		if s.kekHex != "" {
			kek, _ = hex.DecodeString(s.kekHex)
		}
		tenantDEKs := make(map[string][]byte)
		envVarsByWebroot := make(map[string]map[string]string)
		for _, r := range envVarRows {
			if envVarsByWebroot[r.webrootID] == nil {
				envVarsByWebroot[r.webrootID] = make(map[string]string)
			}
			if !r.isSecret {
				envVarsByWebroot[r.webrootID][r.name] = r.value
				continue
			}
			if kek == nil {
				envVarsByWebroot[r.webrootID][r.name] = r.value // can't decrypt without KEK
				continue
			}
			tenantID := webrootTenantMap[r.webrootID]
			dek, ok := tenantDEKs[tenantID]
			if !ok {
				var encryptedDEK string
				err := s.db.QueryRow(ctx,
					`SELECT encrypted_dek FROM tenant_encryption_keys WHERE tenant_id = $1`, tenantID,
				).Scan(&encryptedDEK)
				if err != nil {
					return fmt.Errorf("get tenant encryption key for %s: %w", tenantID, err)
				}
				dek, err = crypto.Decrypt(encryptedDEK, kek)
				if err != nil {
					return fmt.Errorf("decrypt tenant DEK for %s: %w", tenantID, err)
				}
				tenantDEKs[tenantID] = dek
			}
			plaintext, err := crypto.Decrypt(r.value, dek)
			if err != nil {
				return fmt.Errorf("decrypt env var %s: %w", r.name, err)
			}
			envVarsByWebroot[r.webrootID][r.name] = string(plaintext)
		}

		// 4. Batch-fetch all active FQDNs for those webroots.
		fqdnRows, err := s.db.Query(ctx, `
			SELECT webroot_id, fqdn, ssl_enabled, status
			FROM fqdns WHERE webroot_id = ANY($1) AND status = 'active'
			ORDER BY fqdn`, webrootIDs)
		if err != nil {
			return fmt.Errorf("batch query fqdns: %w", err)
		}
		defer fqdnRows.Close()

		fqdnsByWebroot := make(map[string][]model.DesiredFQDN)
		for fqdnRows.Next() {
			var webrootID string
			var f model.DesiredFQDN
			if err := fqdnRows.Scan(&webrootID, &f.FQDN, &f.SSLEnabled, &f.Status); err != nil {
				return fmt.Errorf("scan fqdn: %w", err)
			}
			fqdnsByWebroot[webrootID] = append(fqdnsByWebroot[webrootID], f)
		}
		if err := fqdnRows.Err(); err != nil {
			return fmt.Errorf("iterate fqdns: %w", err)
		}

		// 5. Batch-fetch all cron jobs for those webroots.
		cronRows, err := s.db.Query(ctx, `
			SELECT webroot_id, id, enabled FROM cron_jobs
			WHERE webroot_id = ANY($1) AND status IN ('active', 'auto_disabled')
			ORDER BY id`, webrootIDs)
		if err != nil {
			return fmt.Errorf("batch query cron jobs: %w", err)
		}
		defer cronRows.Close()

		cronsByWebroot := make(map[string][]model.DesiredCronJob)
		for cronRows.Next() {
			var webrootID string
			var cj model.DesiredCronJob
			if err := cronRows.Scan(&webrootID, &cj.ID, &cj.Enabled); err != nil {
				return fmt.Errorf("scan cron job: %w", err)
			}
			cronsByWebroot[webrootID] = append(cronsByWebroot[webrootID], cj)
		}
		if err := cronRows.Err(); err != nil {
			return fmt.Errorf("iterate cron jobs: %w", err)
		}

		// Assemble webroots with their sub-resources.
		for i := range webroots {
			wr := &webroots[i].webroot
			if envVars := envVarsByWebroot[wr.ID]; len(envVars) > 0 {
				wr.EnvVars = envVars
			}
			wr.FQDNs = fqdnsByWebroot[wr.ID]
			wr.CronJobs = cronsByWebroot[wr.ID]
		}
	}

	// 6. Batch-fetch all active SSH keys for those tenants.
	sshRows, err := s.db.Query(ctx, `
		SELECT tenant_id, public_key FROM ssh_keys
		WHERE tenant_id = ANY($1) AND status = 'active'`, tenantIDs)
	if err != nil {
		return fmt.Errorf("batch query ssh keys: %w", err)
	}
	defer sshRows.Close()

	sshKeysByTenant := make(map[string][]string)
	for sshRows.Next() {
		var tenantID, key string
		if err := sshRows.Scan(&tenantID, &key); err != nil {
			return fmt.Errorf("scan ssh key: %w", err)
		}
		sshKeysByTenant[tenantID] = append(sshKeysByTenant[tenantID], key)
	}
	if err := sshRows.Err(); err != nil {
		return fmt.Errorf("iterate ssh keys: %w", err)
	}

	// Assemble tenants with their webroots and SSH keys.
	for i := range tenants {
		t := &tenants[i]
		for _, idx := range webrootsByTenant[t.ID] {
			t.Webroots = append(t.Webroots, webroots[idx].webroot)
		}
		t.SSHKeys = sshKeysByTenant[t.ID]
		ss.Tenants = append(ss.Tenants, *t)
	}

	return nil
}

func (s *DesiredStateService) loadDatabaseState(ctx context.Context, shardID string, ss *model.ShardState) error {
	rows, err := s.db.Query(ctx, `
		SELECT id, status FROM databases
		WHERE shard_id = $1 AND status = 'active'
		ORDER BY id`, shardID)
	if err != nil {
		return fmt.Errorf("query databases: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var d model.DesiredDatabase
		if err := rows.Scan(&d.ID, &d.Status); err != nil {
			return fmt.Errorf("scan database: %w", err)
		}

		userRows, err := s.db.Query(ctx, `
			SELECT id, username, password_hash, privileges, status FROM database_users
			WHERE database_id = $1 AND status = 'active'
			ORDER BY username`, d.ID)
		if err != nil {
			return fmt.Errorf("query database users for db %s: %w", d.ID, err)
		}
		for userRows.Next() {
			var u model.DesiredDBUser
			if err := userRows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Privileges, &u.Status); err != nil {
				userRows.Close()
				return fmt.Errorf("scan database user: %w", err)
			}
			d.Users = append(d.Users, u)
		}
		userRows.Close()

		ss.Databases = append(ss.Databases, d)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate databases: %w", err)
	}
	return nil
}

func (s *DesiredStateService) loadValkeyState(ctx context.Context, shardID string, ss *model.ShardState) error {
	rows, err := s.db.Query(ctx, `
		SELECT id, port, password_hash, max_memory_mb, status
		FROM valkey_instances WHERE shard_id = $1 AND status = 'active'
		ORDER BY id`, shardID)
	if err != nil {
		return fmt.Errorf("query valkey instances: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var vi model.DesiredValkeyInstance
		if err := rows.Scan(&vi.ID, &vi.Port, &vi.PasswordHash, &vi.MaxMemoryMB, &vi.Status); err != nil {
			return fmt.Errorf("scan valkey instance: %w", err)
		}

		userRows, err := s.db.Query(ctx, `
			SELECT id, username, password_hash, array_to_string(privileges, ','), key_pattern, status
			FROM valkey_users WHERE valkey_instance_id = $1 AND status = 'active'
			ORDER BY username`, vi.ID)
		if err != nil {
			return fmt.Errorf("query valkey users for instance %s: %w", vi.ID, err)
		}
		for userRows.Next() {
			var u model.DesiredValkeyUser
			if err := userRows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Privileges, &u.KeyPattern, &u.Status); err != nil {
				userRows.Close()
				return fmt.Errorf("scan valkey user: %w", err)
			}
			vi.Users = append(vi.Users, u)
		}
		userRows.Close()

		ss.ValkeyInstances = append(ss.ValkeyInstances, vi)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate valkey instances: %w", err)
	}
	return nil
}

func (s *DesiredStateService) loadLBState(ctx context.Context, shardID string, ss *model.ShardState) error {
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
		ss.FQDNMappings = append(ss.FQDNMappings, m)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate fqdn mappings: %w", err)
	}
	return nil
}

func (s *DesiredStateService) loadStorageState(ctx context.Context, shardID string, ss *model.ShardState) error {
	rows, err := s.db.Query(ctx, `
		SELECT id, tenant_id, status FROM s3_buckets
		WHERE shard_id = $1 AND status = 'active'
		ORDER BY id`, shardID)
	if err != nil {
		return fmt.Errorf("query s3 buckets: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var b model.DesiredS3Bucket
		var tenantID *string
		if err := rows.Scan(&b.ID, &tenantID, &b.Status); err != nil {
			return fmt.Errorf("scan s3 bucket: %w", err)
		}
		if tenantID != nil {
			b.TenantID = *tenantID
		}
		ss.S3Buckets = append(ss.S3Buckets, b)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate s3 buckets: %w", err)
	}
	return nil
}
