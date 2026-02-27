package activity

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/edvin/hosting/internal/crypto"
	"github.com/edvin/hosting/internal/model"
)

// GetWebrootContext fetches a webroot and all related data needed by webroot workflows.
func (a *CoreDB) GetWebrootContext(ctx context.Context, webrootID string) (*WebrootContext, error) {
	var wc WebrootContext

	// JOIN webroots with tenants and brands.
	err := a.db.QueryRow(ctx,
		`SELECT w.id, w.tenant_id, w.runtime, w.runtime_version, w.runtime_config, w.public_folder, w.env_file_name, w.service_hostname_enabled, w.status, w.status_message, w.suspend_reason, w.created_at, w.updated_at,
		        t.id, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.ssh_enabled, t.disk_quota_bytes, t.status, t.status_message, t.suspend_reason, t.created_at, t.updated_at,
		        b.base_hostname
		 FROM webroots w
		 JOIN tenants t ON t.id = w.tenant_id
		 JOIN brands b ON b.id = t.brand_id
		 WHERE w.id = $1`, webrootID,
	).Scan(&wc.Webroot.ID, &wc.Webroot.TenantID, &wc.Webroot.Runtime, &wc.Webroot.RuntimeVersion, &wc.Webroot.RuntimeConfig, &wc.Webroot.PublicFolder, &wc.Webroot.EnvFileName, &wc.Webroot.ServiceHostnameEnabled, &wc.Webroot.Status, &wc.Webroot.StatusMessage, &wc.Webroot.SuspendReason, &wc.Webroot.CreatedAt, &wc.Webroot.UpdatedAt,
		&wc.Tenant.ID, &wc.Tenant.BrandID, &wc.Tenant.RegionID, &wc.Tenant.ClusterID, &wc.Tenant.ShardID, &wc.Tenant.UID, &wc.Tenant.SFTPEnabled, &wc.Tenant.SSHEnabled, &wc.Tenant.DiskQuotaBytes, &wc.Tenant.Status, &wc.Tenant.StatusMessage, &wc.Tenant.SuspendReason, &wc.Tenant.CreatedAt, &wc.Tenant.UpdatedAt,
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

	// Fetch env vars (decrypted) for this webroot.
	envVars, err := a.decryptEnvVars(ctx, []string{webrootID})
	if err != nil {
		return nil, fmt.Errorf("decrypt env vars: %w", err)
	}
	wc.EnvVars = envVars[webrootID]

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
		        w.id, w.tenant_id, w.runtime, w.runtime_version, w.runtime_config, w.public_folder, w.env_file_name, w.service_hostname_enabled, w.status, w.status_message, w.suspend_reason, w.created_at, w.updated_at,
		        t.id, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.ssh_enabled, t.disk_quota_bytes, t.status, t.status_message, t.suspend_reason, t.created_at, t.updated_at,
		        b.base_hostname
		 FROM fqdns f
		 JOIN webroots w ON w.id = f.webroot_id
		 JOIN tenants t ON t.id = w.tenant_id
		 JOIN brands b ON b.id = t.brand_id
		 WHERE f.id = $1`, fqdnID,
	).Scan(&fc.FQDN.ID, &fc.FQDN.FQDN, &fc.FQDN.WebrootID, &fc.FQDN.SSLEnabled, &fc.FQDN.Status, &fc.FQDN.StatusMessage, &fc.FQDN.CreatedAt, &fc.FQDN.UpdatedAt,
		&fc.Webroot.ID, &fc.Webroot.TenantID, &fc.Webroot.Runtime, &fc.Webroot.RuntimeVersion, &fc.Webroot.RuntimeConfig, &fc.Webroot.PublicFolder, &fc.Webroot.EnvFileName, &fc.Webroot.ServiceHostnameEnabled, &fc.Webroot.Status, &fc.Webroot.StatusMessage, &fc.Webroot.SuspendReason, &fc.Webroot.CreatedAt, &fc.Webroot.UpdatedAt,
		&fc.Tenant.ID, &fc.Tenant.BrandID, &fc.Tenant.RegionID, &fc.Tenant.ClusterID, &fc.Tenant.ShardID, &fc.Tenant.UID, &fc.Tenant.SFTPEnabled, &fc.Tenant.SSHEnabled, &fc.Tenant.DiskQuotaBytes, &fc.Tenant.Status, &fc.Tenant.StatusMessage, &fc.Tenant.SuspendReason, &fc.Tenant.CreatedAt, &fc.Tenant.UpdatedAt,
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
		`SELECT u.id, u.database_id, u.username, u.password_hash, u.privileges, u.status, u.status_message, u.created_at, u.updated_at,
		        d.id, d.tenant_id, d.shard_id, d.node_id, d.status, d.status_message, d.suspend_reason, d.created_at, d.updated_at
		 FROM database_users u
		 JOIN databases d ON d.id = u.database_id
		 WHERE u.id = $1`, userID,
	).Scan(&dc.User.ID, &dc.User.DatabaseID, &dc.User.Username, &dc.User.PasswordHash, &dc.User.Privileges, &dc.User.Status, &dc.User.StatusMessage, &dc.User.CreatedAt, &dc.User.UpdatedAt,
		&dc.Database.ID, &dc.Database.TenantID, &dc.Database.ShardID, &dc.Database.NodeID, &dc.Database.Status, &dc.Database.StatusMessage, &dc.Database.SuspendReason, &dc.Database.CreatedAt, &dc.Database.UpdatedAt)
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
		`SELECT u.id, u.valkey_instance_id, u.username, u.password_hash, u.privileges, u.key_pattern, u.status, u.status_message, u.created_at, u.updated_at,
		        i.id, i.tenant_id, i.shard_id, i.port, i.max_memory_mb, i.password_hash, i.status, i.status_message, i.suspend_reason, i.created_at, i.updated_at
		 FROM valkey_users u
		 JOIN valkey_instances i ON i.id = u.valkey_instance_id
		 WHERE u.id = $1`, userID,
	).Scan(&vc.User.ID, &vc.User.ValkeyInstanceID, &vc.User.Username, &vc.User.PasswordHash, &vc.User.Privileges, &vc.User.KeyPattern, &vc.User.Status, &vc.User.StatusMessage, &vc.User.CreatedAt, &vc.User.UpdatedAt,
		&vc.Instance.ID, &vc.Instance.TenantID, &vc.Instance.ShardID, &vc.Instance.Port, &vc.Instance.MaxMemoryMB, &vc.Instance.PasswordHash, &vc.Instance.Status, &vc.Instance.StatusMessage, &vc.Instance.SuspendReason, &vc.Instance.CreatedAt, &vc.Instance.UpdatedAt)
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
		        t.id, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.ssh_enabled, t.disk_quota_bytes, t.status, t.status_message, t.suspend_reason, t.created_at, t.updated_at
		 FROM backups b
		 JOIN tenants t ON t.id = b.tenant_id
		 WHERE b.id = $1`, backupID,
	).Scan(&bc.Backup.ID, &bc.Backup.TenantID, &bc.Backup.Type, &bc.Backup.SourceID, &bc.Backup.SourceName, &bc.Backup.StoragePath, &bc.Backup.SizeBytes, &bc.Backup.Status, &bc.Backup.StatusMessage, &bc.Backup.StartedAt, &bc.Backup.CompletedAt, &bc.Backup.CreatedAt, &bc.Backup.UpdatedAt,
		&bc.Tenant.ID, &bc.Tenant.BrandID, &bc.Tenant.RegionID, &bc.Tenant.ClusterID, &bc.Tenant.ShardID, &bc.Tenant.UID, &bc.Tenant.SFTPEnabled, &bc.Tenant.SSHEnabled, &bc.Tenant.DiskQuotaBytes, &bc.Tenant.Status, &bc.Tenant.StatusMessage, &bc.Tenant.SuspendReason, &bc.Tenant.CreatedAt, &bc.Tenant.UpdatedAt)
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
		`SELECT k.id, k.s3_bucket_id, k.access_key_id, k.secret_key_hash, k.permissions, k.status, k.status_message, k.created_at, k.updated_at,
		        b.id, b.tenant_id, b.shard_id, b.public, b.quota_bytes, b.status, b.status_message, b.suspend_reason, b.created_at, b.updated_at
		 FROM s3_access_keys k
		 JOIN s3_buckets b ON b.id = k.s3_bucket_id
		 WHERE k.id = $1`, keyID,
	).Scan(&sc.Key.ID, &sc.Key.S3BucketID, &sc.Key.AccessKeyID, &sc.Key.SecretKeyHash, &sc.Key.Permissions, &sc.Key.Status, &sc.Key.StatusMessage, &sc.Key.CreatedAt, &sc.Key.UpdatedAt,
		&sc.Bucket.ID, &sc.Bucket.TenantID, &sc.Bucket.ShardID, &sc.Bucket.Public, &sc.Bucket.QuotaBytes, &sc.Bucket.Status, &sc.Bucket.StatusMessage, &sc.Bucket.SuspendReason, &sc.Bucket.CreatedAt, &sc.Bucket.UpdatedAt)
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

// GetCronJobContext fetches a cron job and its related webroot, tenant, and nodes.
func (a *CoreDB) GetCronJobContext(ctx context.Context, cronJobID string) (*CronJobContext, error) {
	var cc CronJobContext

	// JOIN cron_jobs -> webroots -> tenants.
	err := a.db.QueryRow(ctx,
		`SELECT c.id, c.tenant_id, c.webroot_id, c.schedule, c.command, c.working_directory, c.enabled, c.timeout_seconds, c.max_memory_mb, c.status, c.status_message, c.created_at, c.updated_at,
		        w.id, w.tenant_id, w.runtime, w.runtime_version, w.runtime_config, w.public_folder, w.env_file_name, w.status, w.status_message, w.suspend_reason, w.created_at, w.updated_at,
		        t.id, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.ssh_enabled, t.disk_quota_bytes, t.status, t.status_message, t.suspend_reason, t.created_at, t.updated_at
		 FROM cron_jobs c
		 JOIN webroots w ON w.id = c.webroot_id
		 JOIN tenants t ON t.id = c.tenant_id
		 WHERE c.id = $1`, cronJobID,
	).Scan(&cc.CronJob.ID, &cc.CronJob.TenantID, &cc.CronJob.WebrootID, &cc.CronJob.Schedule, &cc.CronJob.Command, &cc.CronJob.WorkingDirectory, &cc.CronJob.Enabled, &cc.CronJob.TimeoutSeconds, &cc.CronJob.MaxMemoryMB, &cc.CronJob.Status, &cc.CronJob.StatusMessage, &cc.CronJob.CreatedAt, &cc.CronJob.UpdatedAt,
		&cc.Webroot.ID, &cc.Webroot.TenantID, &cc.Webroot.Runtime, &cc.Webroot.RuntimeVersion, &cc.Webroot.RuntimeConfig, &cc.Webroot.PublicFolder, &cc.Webroot.EnvFileName, &cc.Webroot.Status, &cc.Webroot.StatusMessage, &cc.Webroot.SuspendReason, &cc.Webroot.CreatedAt, &cc.Webroot.UpdatedAt,
		&cc.Tenant.ID, &cc.Tenant.BrandID, &cc.Tenant.RegionID, &cc.Tenant.ClusterID, &cc.Tenant.ShardID, &cc.Tenant.UID, &cc.Tenant.SFTPEnabled, &cc.Tenant.SSHEnabled, &cc.Tenant.DiskQuotaBytes, &cc.Tenant.Status, &cc.Tenant.StatusMessage, &cc.Tenant.SuspendReason, &cc.Tenant.CreatedAt, &cc.Tenant.UpdatedAt)
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

// GetDaemonContext fetches a daemon and its related webroot, tenant, and nodes.
func (a *CoreDB) GetDaemonContext(ctx context.Context, daemonID string) (*DaemonContext, error) {
	var dc DaemonContext

	// JOIN daemons -> webroots -> tenants.
	err := a.db.QueryRow(ctx,
		`SELECT d.id, d.tenant_id, d.node_id, d.webroot_id, d.command, d.proxy_path, d.proxy_port,
		        d.num_procs, d.stop_signal, d.stop_wait_secs, d.max_memory_mb,
		        d.enabled, d.status, d.status_message, d.created_at, d.updated_at,
		        w.id, w.tenant_id, w.runtime, w.runtime_version, w.runtime_config, w.public_folder, w.env_file_name, w.status, w.status_message, w.suspend_reason, w.created_at, w.updated_at,
		        t.id, t.brand_id, t.region_id, t.cluster_id, t.shard_id, t.uid, t.sftp_enabled, t.ssh_enabled, t.disk_quota_bytes, t.status, t.status_message, t.suspend_reason, t.created_at, t.updated_at
		 FROM daemons d
		 JOIN webroots w ON w.id = d.webroot_id
		 JOIN tenants t ON t.id = d.tenant_id
		 WHERE d.id = $1`, daemonID,
	).Scan(&dc.Daemon.ID, &dc.Daemon.TenantID, &dc.Daemon.NodeID, &dc.Daemon.WebrootID, &dc.Daemon.Command,
		&dc.Daemon.ProxyPath, &dc.Daemon.ProxyPort,
		&dc.Daemon.NumProcs, &dc.Daemon.StopSignal, &dc.Daemon.StopWaitSecs, &dc.Daemon.MaxMemoryMB,
		&dc.Daemon.Enabled, &dc.Daemon.Status, &dc.Daemon.StatusMessage, &dc.Daemon.CreatedAt, &dc.Daemon.UpdatedAt,
		&dc.Webroot.ID, &dc.Webroot.TenantID, &dc.Webroot.Runtime, &dc.Webroot.RuntimeVersion, &dc.Webroot.RuntimeConfig, &dc.Webroot.PublicFolder, &dc.Webroot.EnvFileName, &dc.Webroot.Status, &dc.Webroot.StatusMessage, &dc.Webroot.SuspendReason, &dc.Webroot.CreatedAt, &dc.Webroot.UpdatedAt,
		&dc.Tenant.ID, &dc.Tenant.BrandID, &dc.Tenant.RegionID, &dc.Tenant.ClusterID, &dc.Tenant.ShardID, &dc.Tenant.UID, &dc.Tenant.SFTPEnabled, &dc.Tenant.SSHEnabled, &dc.Tenant.DiskQuotaBytes, &dc.Tenant.Status, &dc.Tenant.StatusMessage, &dc.Tenant.SuspendReason, &dc.Tenant.CreatedAt, &dc.Tenant.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get daemon context: %w", err)
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

// decryptEnvVars batch-fetches and decrypts env vars for the given webroot IDs.
// Returns webroot ID -> env var name -> plaintext value.
func (a *CoreDB) decryptEnvVars(ctx context.Context, webrootIDs []string) (map[string]map[string]string, error) {
	result := make(map[string]map[string]string)
	if len(webrootIDs) == 0 || a.kekHex == "" {
		return result, nil
	}

	kek, err := hex.DecodeString(a.kekHex)
	if err != nil {
		return nil, fmt.Errorf("decode kek: %w", err)
	}

	// Batch query all env vars.
	rows, err := a.db.Query(ctx,
		`SELECT e.webroot_id, e.name, e.value, e.is_secret, w.tenant_id
		 FROM webroot_env_vars e
		 JOIN webroots w ON w.id = e.webroot_id
		 WHERE e.webroot_id = ANY($1)`, webrootIDs)
	if err != nil {
		return nil, fmt.Errorf("query env vars: %w", err)
	}
	defer rows.Close()

	// Cache tenant DEKs to avoid repeated lookups.
	dekCache := make(map[string][]byte)

	for rows.Next() {
		var webrootID, name, value, tenantID string
		var isSecret bool
		if err := rows.Scan(&webrootID, &name, &value, &isSecret, &tenantID); err != nil {
			return nil, fmt.Errorf("scan env var: %w", err)
		}

		if isSecret {
			dek, ok := dekCache[tenantID]
			if !ok {
				var encryptedDEK string
				err := a.db.QueryRow(ctx,
					`SELECT encrypted_dek FROM tenant_encryption_keys WHERE tenant_id = $1`, tenantID,
				).Scan(&encryptedDEK)
				if err != nil {
					return nil, fmt.Errorf("get tenant dek for %s: %w", tenantID, err)
				}
				dek, err = crypto.Decrypt(encryptedDEK, kek)
				if err != nil {
					return nil, fmt.Errorf("decrypt tenant dek: %w", err)
				}
				dekCache[tenantID] = dek
			}
			plaintext, err := crypto.Decrypt(value, dek)
			if err != nil {
				return nil, fmt.Errorf("decrypt env var %s: %w", name, err)
			}
			value = string(plaintext)
		}

		if result[webrootID] == nil {
			result[webrootID] = make(map[string]string)
		}
		result[webrootID][name] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate env vars: %w", err)
	}

	return result, nil
}
