package activity

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
)

// ShardDesiredState holds all the data needed to converge a web shard,
// fetched in a single activity call instead of N+1 round-trips.
type ShardDesiredState struct {
	Tenants            []model.Tenant              `json:"tenants"`
	Webroots           map[string][]model.Webroot  `json:"webroots"`             // tenant ID -> webroots
	FQDNs              map[string][]FQDNParam      `json:"fqdns"`                // webroot ID -> FQDNs
	EnvVars            map[string]map[string]string `json:"env_vars"`            // webroot ID -> name -> value
	Daemons            map[string][]model.Daemon   `json:"daemons"`              // webroot ID -> daemons
	CronJobs           map[string][]model.CronJob  `json:"cron_jobs"`            // webroot ID -> cron jobs
	SSHKeys            map[string][]string         `json:"ssh_keys"`             // tenant ID -> public keys
	BrandBaseHostnames map[string]string           `json:"brand_base_hostnames"` // tenant ID -> brand base_hostname
}

// GetShardDesiredState fetches all data needed to converge a web shard in batch.
func (a *CoreDB) GetShardDesiredState(ctx context.Context, shardID string) (*ShardDesiredState, error) {
	result := &ShardDesiredState{
		Webroots:           make(map[string][]model.Webroot),
		FQDNs:              make(map[string][]FQDNParam),
		EnvVars:            make(map[string]map[string]string),
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
		`SELECT id, tenant_id, name, runtime, runtime_version, runtime_config, public_folder, env_file_name, service_hostname_enabled, status, status_message, suspend_reason, created_at, updated_at
		 FROM webroots WHERE tenant_id = ANY($1) AND status = $2`, tenantIDs, model.StatusActive)
	if err != nil {
		return nil, fmt.Errorf("batch list webroots: %w", err)
	}
	defer wrRows.Close()

	var webrootIDs []string
	for wrRows.Next() {
		var w model.Webroot
		if err := wrRows.Scan(&w.ID, &w.TenantID, &w.Name, &w.Runtime, &w.RuntimeVersion, &w.RuntimeConfig, &w.PublicFolder, &w.EnvFileName, &w.ServiceHostnameEnabled, &w.Status, &w.StatusMessage, &w.SuspendReason, &w.CreatedAt, &w.UpdatedAt); err != nil {
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

	// 4. Fetch and decrypt env vars for those webroots.
	envVars, err := a.decryptEnvVars(ctx, webrootIDs)
	if err != nil {
		return nil, fmt.Errorf("decrypt env vars: %w", err)
	}
	result.EnvVars = envVars

	// 5. Fetch all active FQDNs for those webroots.
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

	// 6. Fetch all daemons for those webroots.
	daemonRows, err := a.db.Query(ctx,
		`SELECT id, tenant_id, node_id, webroot_id, name, command, proxy_path, proxy_port,
		        num_procs, stop_signal, stop_wait_secs, max_memory_mb,
		        enabled, status, status_message, created_at, updated_at
		 FROM daemons WHERE webroot_id = ANY($1)`, webrootIDs)
	if err != nil {
		return nil, fmt.Errorf("batch list daemons: %w", err)
	}
	defer daemonRows.Close()

	for daemonRows.Next() {
		var d model.Daemon
		if err := daemonRows.Scan(&d.ID, &d.TenantID, &d.NodeID, &d.WebrootID, &d.Name, &d.Command,
			&d.ProxyPath, &d.ProxyPort,
			&d.NumProcs, &d.StopSignal, &d.StopWaitSecs, &d.MaxMemoryMB,
			&d.Enabled, &d.Status, &d.StatusMessage, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan daemon: %w", err)
		}
		result.Daemons[d.WebrootID] = append(result.Daemons[d.WebrootID], d)
	}
	if err := daemonRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daemons: %w", err)
	}

	// 7. Fetch all cron jobs for those webroots.
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

	// 8. Fetch all active SSH keys for those tenants.
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
		        num_procs, stop_signal, stop_wait_secs, max_memory_mb,
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
		if err := rows.Scan(&d.ID, &d.TenantID, &d.NodeID, &d.WebrootID, &d.Name, &d.Command,
			&d.ProxyPath, &d.ProxyPort,
			&d.NumProcs, &d.StopSignal, &d.StopWaitSecs, &d.MaxMemoryMB,
			&d.Enabled, &d.Status, &d.StatusMessage, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan daemon row: %w", err)
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
