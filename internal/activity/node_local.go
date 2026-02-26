package activity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/rs/zerolog"
	"go.temporal.io/sdk/temporal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/edvin/hosting/internal/agent"
	"github.com/edvin/hosting/internal/agent/runtime"
)

// grpcStatusError is the interface implemented by gRPC status errors.
// Used with errors.As to extract gRPC status from wrapped errors.
type grpcStatusError interface {
	GRPCStatus() *status.Status
}

// asNonRetryable checks whether err (or any error in its chain) is a gRPC
// status error with a deterministic error code. If so it wraps the error as a
// Temporal non-retryable application error so that the activity is not
// retried — these failures will never succeed on retry. All other errors
// (including codes.Internal) are returned unchanged and may be retried.
func asNonRetryable(err error) error {
	if err == nil {
		return nil
	}
	var se grpcStatusError
	if errors.As(err, &se) {
		code := se.GRPCStatus().Code()
		switch code {
		case codes.InvalidArgument, codes.NotFound, codes.AlreadyExists, codes.FailedPrecondition:
			return temporal.NewNonRetryableApplicationError(
				se.GRPCStatus().Message(),
				code.String(),
				err,
			)
		}
	}
	return err
}

// NodeLocal contains activities that execute locally on the node using manager
// structs directly. This replaces the gRPC-based NodeGRPC and NodeGRPCDynamic
// activities — routing is handled by Temporal task queues instead of gRPC addresses.
type NodeLocal struct {
	logger   zerolog.Logger
	tenant   *agent.TenantManager
	webroot  *agent.WebrootManager
	nginx    *agent.NginxManager
	database *agent.DatabaseManager
	valkey   *agent.ValkeyManager
	s3       *agent.S3Manager
	ssh      *agent.SSHManager
	cron     *agent.CronManager
	daemon    *agent.DaemonManager
	tenantULA *agent.TenantULAManager
	wireguard *agent.WireGuardManager
	runtimes  map[string]runtime.Manager
}

// NewNodeLocal creates a new NodeLocal activity struct.
func NewNodeLocal(
	logger zerolog.Logger,
	tenant *agent.TenantManager,
	webroot *agent.WebrootManager,
	nginx *agent.NginxManager,
	database *agent.DatabaseManager,
	valkey *agent.ValkeyManager,
	s3 *agent.S3Manager,
	ssh *agent.SSHManager,
	cron *agent.CronManager,
	daemon *agent.DaemonManager,
	tenantULA *agent.TenantULAManager,
	wireguard *agent.WireGuardManager,
	runtimes map[string]runtime.Manager,
) *NodeLocal {
	return &NodeLocal{
		logger:    logger.With().Str("component", "node-local-activity").Logger(),
		tenant:    tenant,
		webroot:   webroot,
		nginx:     nginx,
		database:  database,
		valkey:    valkey,
		s3:        s3,
		ssh:       ssh,
		cron:      cron,
		daemon:    daemon,
		tenantULA: tenantULA,
		wireguard: wireguard,
		runtimes:  runtimes,
	}
}

// --------------------------------------------------------------------------
// Tenant activities
// --------------------------------------------------------------------------

// CreateTenant creates a tenant locally on this node.
func (a *NodeLocal) CreateTenant(ctx context.Context, params CreateTenantParams) error {
	a.logger.Info().Str("tenant", params.ID).Msg("CreateTenant")
	return asNonRetryable(a.tenant.Create(ctx, &agent.TenantInfo{
		ID:             params.ID,
		Name:           params.Name,
		UID:            int32(params.UID),
		SFTPEnabled:    params.SFTPEnabled,
		SSHEnabled:     params.SSHEnabled,
		DiskQuotaBytes: params.DiskQuotaBytes,
	}))
}

// UpdateTenant updates a tenant locally on this node.
func (a *NodeLocal) UpdateTenant(ctx context.Context, params UpdateTenantParams) error {
	a.logger.Info().Str("tenant", params.ID).Msg("UpdateTenant")
	return asNonRetryable(a.tenant.Update(ctx, &agent.TenantInfo{
		ID:          params.ID,
		Name:        params.Name,
		UID:         int32(params.UID),
		SFTPEnabled: params.SFTPEnabled,
		SSHEnabled:  params.SSHEnabled,
	}))
}

// SuspendTenant suspends a tenant locally on this node.
func (a *NodeLocal) SuspendTenant(ctx context.Context, name string) error {
	a.logger.Info().Str("tenant", name).Msg("SuspendTenant")
	return asNonRetryable(a.tenant.Suspend(ctx, name))
}

// UnsuspendTenant unsuspends a tenant locally on this node.
func (a *NodeLocal) UnsuspendTenant(ctx context.Context, name string) error {
	a.logger.Info().Str("tenant", name).Msg("UnsuspendTenant")
	return asNonRetryable(a.tenant.Unsuspend(ctx, name))
}

// DeleteTenant deletes a tenant locally on this node.
func (a *NodeLocal) DeleteTenant(ctx context.Context, name string) error {
	a.logger.Info().Str("tenant", name).Msg("DeleteTenant")
	return asNonRetryable(a.tenant.Delete(ctx, name))
}

// --------------------------------------------------------------------------
// Webroot activities
// --------------------------------------------------------------------------

// CreateWebroot creates a webroot locally on this node.
func (a *NodeLocal) CreateWebroot(ctx context.Context, params CreateWebrootParams) error {
	a.logger.Info().Str("tenant", params.TenantName).Str("webroot", params.Name).Msg("CreateWebroot")

	info := &runtime.WebrootInfo{
		ID:             params.ID,
		TenantName:     params.TenantName,
		Name:           params.Name,
		Runtime:        params.Runtime,
		RuntimeVersion: params.RuntimeVersion,
		RuntimeConfig:  params.RuntimeConfig,
		PublicFolder:   params.PublicFolder,
		EnvVars:        params.EnvVars,
	}

	fqdns := make([]*agent.FQDNInfo, len(params.FQDNs))
	for i, f := range params.FQDNs {
		fqdns[i] = &agent.FQDNInfo{
			FQDN:       f.FQDN,
			WebrootID:  f.WebrootID,
			SSLEnabled: f.SSLEnabled,
		}
	}

	// Create webroot directories.
	if err := a.webroot.Create(ctx, info); err != nil {
		return asNonRetryable(fmt.Errorf("create webroot: %w", err))
	}

	// Write env file and .envrc (env vars + PHP version selection).
	if err := writeWebrootEnv(params.TenantName, params.Name, params.EnvFileName, params.Runtime, params.RuntimeVersion, params.EnvVars); err != nil {
		return asNonRetryable(fmt.Errorf("write env file: %w", err))
	}

	// Configure and start runtime.
	rt, ok := a.runtimes[info.Runtime]
	if !ok {
		return fmt.Errorf("unsupported runtime: %s", info.Runtime)
	}
	if err := rt.Configure(ctx, info); err != nil {
		return asNonRetryable(fmt.Errorf("configure runtime: %w", err))
	}
	if err := rt.Start(ctx, info); err != nil {
		return asNonRetryable(fmt.Errorf("start runtime: %w", err))
	}

	// Convert daemon proxy info for nginx generation.
	daemonProxies := make([]agent.DaemonProxyInfo, len(params.Daemons))
	for i, d := range params.Daemons {
		daemonProxies[i] = agent.DaemonProxyInfo{ProxyPath: d.ProxyPath, Port: d.Port, TargetIP: d.TargetIP, ProxyURL: d.ProxyURL}
	}

	// Generate and write nginx config.
	nginxConfig, err := a.nginx.GenerateConfig(info, fqdns, daemonProxies...)
	if err != nil {
		return asNonRetryable(fmt.Errorf("generate nginx config: %w", err))
	}
	if err := a.nginx.WriteConfig(info.TenantName, info.Name, nginxConfig); err != nil {
		return asNonRetryable(fmt.Errorf("write nginx config: %w", err))
	}

	// Reload nginx.
	if err := a.nginx.Reload(ctx); err != nil {
		return asNonRetryable(fmt.Errorf("reload nginx: %w", err))
	}

	return nil
}

// UpdateWebroot updates a webroot locally on this node.
func (a *NodeLocal) UpdateWebroot(ctx context.Context, params UpdateWebrootParams) error {
	a.logger.Info().Str("tenant", params.TenantName).Str("webroot", params.Name).Msg("UpdateWebroot")

	info := &runtime.WebrootInfo{
		ID:             params.ID,
		TenantName:     params.TenantName,
		Name:           params.Name,
		Runtime:        params.Runtime,
		RuntimeVersion: params.RuntimeVersion,
		RuntimeConfig:  params.RuntimeConfig,
		PublicFolder:   params.PublicFolder,
		EnvVars:        params.EnvVars,
	}

	fqdns := make([]*agent.FQDNInfo, len(params.FQDNs))
	for i, f := range params.FQDNs {
		fqdns[i] = &agent.FQDNInfo{
			FQDN:       f.FQDN,
			WebrootID:  f.WebrootID,
			SSLEnabled: f.SSLEnabled,
		}
	}

	// Update webroot directories.
	if err := a.webroot.Update(ctx, info); err != nil {
		return asNonRetryable(fmt.Errorf("update webroot: %w", err))
	}

	// Write env file and .envrc (env vars + PHP version selection).
	if err := writeWebrootEnv(params.TenantName, params.Name, params.EnvFileName, params.Runtime, params.RuntimeVersion, params.EnvVars); err != nil {
		return asNonRetryable(fmt.Errorf("write env file: %w", err))
	}

	// Reconfigure and reload runtime.
	rt, ok := a.runtimes[info.Runtime]
	if !ok {
		return fmt.Errorf("unsupported runtime: %s", info.Runtime)
	}
	if err := rt.Configure(ctx, info); err != nil {
		return asNonRetryable(fmt.Errorf("configure runtime: %w", err))
	}
	if err := rt.Reload(ctx, info); err != nil {
		return asNonRetryable(fmt.Errorf("reload runtime: %w", err))
	}

	// Convert daemon proxy info for nginx generation.
	daemonProxies := make([]agent.DaemonProxyInfo, len(params.Daemons))
	for i, d := range params.Daemons {
		daemonProxies[i] = agent.DaemonProxyInfo{ProxyPath: d.ProxyPath, Port: d.Port, TargetIP: d.TargetIP, ProxyURL: d.ProxyURL}
	}

	// Regenerate and write nginx config.
	nginxConfig, err := a.nginx.GenerateConfig(info, fqdns, daemonProxies...)
	if err != nil {
		return asNonRetryable(fmt.Errorf("generate nginx config: %w", err))
	}
	if err := a.nginx.WriteConfig(info.TenantName, info.Name, nginxConfig); err != nil {
		return asNonRetryable(fmt.Errorf("write nginx config: %w", err))
	}

	// Reload nginx.
	if err := a.nginx.Reload(ctx); err != nil {
		return asNonRetryable(fmt.Errorf("reload nginx: %w", err))
	}

	return nil
}

// DeleteWebroot deletes a webroot locally on this node.
func (a *NodeLocal) DeleteWebroot(ctx context.Context, tenantName, webrootName string) error {
	a.logger.Info().Str("tenant", tenantName).Str("webroot", webrootName).Msg("DeleteWebroot")

	// Remove nginx config (tolerate missing files).
	if err := a.nginx.RemoveConfig(tenantName, webrootName); err != nil && !os.IsNotExist(err) {
		return asNonRetryable(fmt.Errorf("remove nginx config: %w", err))
	}

	// Reload nginx.
	if err := a.nginx.Reload(ctx); err != nil {
		return asNonRetryable(fmt.Errorf("reload nginx: %w", err))
	}

	// Remove runtimes (try all, only one will match).
	wrInfo := &runtime.WebrootInfo{TenantName: tenantName, Name: webrootName}
	for _, rt := range a.runtimes {
		_ = rt.Remove(ctx, wrInfo)
	}

	// Remove webroot directories.
	if err := a.webroot.Delete(ctx, tenantName, webrootName); err != nil {
		return asNonRetryable(fmt.Errorf("delete webroot: %w", err))
	}

	return nil
}

// --------------------------------------------------------------------------
// Runtime / Nginx activities
// --------------------------------------------------------------------------

// ConfigureRuntime configures and starts a runtime for a webroot.
func (a *NodeLocal) ConfigureRuntime(ctx context.Context, params ConfigureRuntimeParams) error {
	a.logger.Info().Str("runtime", params.Runtime).Str("webroot", params.Name).Msg("ConfigureRuntime")

	info := &runtime.WebrootInfo{
		ID:             params.ID,
		TenantName:     params.TenantName,
		Name:           params.Name,
		Runtime:        params.Runtime,
		RuntimeVersion: params.RuntimeVersion,
		RuntimeConfig:  params.RuntimeConfig,
		PublicFolder:   params.PublicFolder,
		EnvVars:        params.EnvVars,
	}

	rt, ok := a.runtimes[info.Runtime]
	if !ok {
		return fmt.Errorf("unsupported runtime: %s", info.Runtime)
	}
	if err := rt.Configure(ctx, info); err != nil {
		return asNonRetryable(fmt.Errorf("configure runtime: %w", err))
	}
	if err := rt.Start(ctx, info); err != nil {
		return asNonRetryable(fmt.Errorf("start runtime: %w", err))
	}
	return nil
}

// CleanOrphanedConfigs removes nginx config files that are not in the expected set.
func (a *NodeLocal) CleanOrphanedConfigs(ctx context.Context, input CleanOrphanedConfigsInput) (CleanOrphanedConfigsResult, error) {
	a.logger.Info().Int("expected_count", len(input.ExpectedConfigs)).Msg("CleanOrphanedConfigs")
	removed, err := a.nginx.CleanOrphanedConfigs(input.ExpectedConfigs)
	if err != nil {
		return CleanOrphanedConfigsResult{}, asNonRetryable(fmt.Errorf("clean orphaned configs: %w", err))
	}
	return CleanOrphanedConfigsResult{Removed: removed}, nil
}

// CleanOrphanedDaemonConfigs removes supervisor daemon configs not in the expected set.
func (a *NodeLocal) CleanOrphanedDaemonConfigs(ctx context.Context, input CleanOrphanedDaemonConfigsInput) (CleanOrphanedDaemonConfigsResult, error) {
	a.logger.Info().Int("expected_count", len(input.ExpectedConfigs)).Msg("CleanOrphanedDaemonConfigs")
	removed, err := a.daemon.CleanOrphanedConfigs(input.ExpectedConfigs)
	if err != nil {
		return CleanOrphanedDaemonConfigsResult{}, asNonRetryable(fmt.Errorf("clean orphaned daemon configs: %w", err))
	}
	return CleanOrphanedDaemonConfigsResult{Removed: removed}, nil
}

// CleanOrphanedFPMPools removes PHP-FPM pool configs not in the expected set.
func (a *NodeLocal) CleanOrphanedFPMPools(ctx context.Context, input CleanOrphanedFPMPoolsInput) (CleanOrphanedFPMPoolsResult, error) {
	a.logger.Info().Int("expected_count", len(input.ExpectedPools)).Msg("CleanOrphanedFPMPools")
	phpMgr, ok := a.runtimes["php"].(*runtime.PHP)
	if !ok {
		return CleanOrphanedFPMPoolsResult{}, nil
	}
	removed, err := phpMgr.CleanOrphanedPools(input.ExpectedPools)
	if err != nil {
		return CleanOrphanedFPMPoolsResult{}, asNonRetryable(fmt.Errorf("clean orphaned fpm pools: %w", err))
	}
	return CleanOrphanedFPMPoolsResult{Removed: removed}, nil
}

// RestartSupervisord restarts the supervisord service via systemctl.
// This is needed after removing stale daemon configs that prevented supervisord from starting.
func (a *NodeLocal) RestartSupervisord(ctx context.Context) error {
	a.logger.Info().Msg("RestartSupervisord")
	cmd := exec.CommandContext(ctx, "systemctl", "restart", "supervisor")
	if output, err := cmd.CombinedOutput(); err != nil {
		return asNonRetryable(fmt.Errorf("restart supervisor: %s: %w", string(output), err))
	}
	return nil
}

// ReloadNginx tests and reloads the nginx configuration.
func (a *NodeLocal) ReloadNginx(ctx context.Context) error {
	a.logger.Info().Msg("ReloadNginx")
	return asNonRetryable(a.nginx.Reload(ctx))
}

// ReloadPHPFPM gracefully reloads all PHP-FPM services.
func (a *NodeLocal) ReloadPHPFPM(ctx context.Context) error {
	a.logger.Info().Msg("ReloadPHPFPM")
	phpMgr, ok := a.runtimes["php"].(*runtime.PHP)
	if !ok {
		return nil
	}
	return phpMgr.ReloadAll(ctx)
}

// --------------------------------------------------------------------------
// Database activities
// --------------------------------------------------------------------------

// CreateDatabase creates a MySQL database locally on this node.
func (a *NodeLocal) CreateDatabase(ctx context.Context, name string) error {
	a.logger.Info().Str("database", name).Msg("CreateDatabase")
	return asNonRetryable(a.database.CreateDatabase(ctx, name))
}

// DeleteDatabase drops a MySQL database locally on this node.
func (a *NodeLocal) DeleteDatabase(ctx context.Context, name string) error {
	a.logger.Info().Str("database", name).Msg("DeleteDatabase")
	return asNonRetryable(a.database.DeleteDatabase(ctx, name))
}

// CreateDatabaseUser creates a MySQL user locally on this node.
func (a *NodeLocal) CreateDatabaseUser(ctx context.Context, params CreateDatabaseUserParams) error {
	a.logger.Info().Str("username", params.Username).Msg("CreateDatabaseUser")
	return asNonRetryable(a.database.CreateUser(ctx, params.DatabaseName, params.Username, params.Password, params.Privileges))
}

// UpdateDatabaseUser updates a MySQL user locally on this node.
func (a *NodeLocal) UpdateDatabaseUser(ctx context.Context, params UpdateDatabaseUserParams) error {
	a.logger.Info().Str("username", params.Username).Msg("UpdateDatabaseUser")
	return asNonRetryable(a.database.UpdateUser(ctx, params.DatabaseName, params.Username, params.Password, params.Privileges))
}

// DeleteDatabaseUser drops a MySQL user locally on this node.
func (a *NodeLocal) DeleteDatabaseUser(ctx context.Context, dbName, username string) error {
	a.logger.Info().Str("username", username).Msg("DeleteDatabaseUser")
	return asNonRetryable(a.database.DeleteUser(ctx, dbName, username))
}

// ConfigureReplication sets up this node as a replica of the given primary.
func (a *NodeLocal) ConfigureReplication(ctx context.Context, params ConfigureReplicationParams) error {
	a.logger.Info().Str("primary", params.PrimaryHost).Msg("ConfigureReplication")
	return asNonRetryable(a.database.ConfigureReplication(ctx, params.PrimaryHost, params.ReplUser, params.ReplPassword))
}

// SetReadOnly makes this MySQL instance read-only or read-write.
func (a *NodeLocal) SetReadOnly(ctx context.Context, readOnly bool) error {
	a.logger.Info().Bool("read_only", readOnly).Msg("SetReadOnly")
	return asNonRetryable(a.database.SetReadOnly(ctx, readOnly))
}

// GetReplicationStatus returns the current replication status of this node.
func (a *NodeLocal) GetReplicationStatus(ctx context.Context) (*agent.ReplicationStatus, error) {
	a.logger.Info().Msg("GetReplicationStatus")
	status, err := a.database.GetReplicationStatus(ctx)
	if err != nil {
		return nil, asNonRetryable(err)
	}
	return status, nil
}

// StopReplication stops replication on this node.
func (a *NodeLocal) StopReplication(ctx context.Context) error {
	a.logger.Info().Msg("StopReplication")
	return asNonRetryable(a.database.StopReplication(ctx))
}

// --------------------------------------------------------------------------
// Valkey activities
// --------------------------------------------------------------------------

// CreateValkeyInstance creates a Valkey instance locally on this node.
func (a *NodeLocal) CreateValkeyInstance(ctx context.Context, params CreateValkeyInstanceParams) error {
	a.logger.Info().Str("instance", params.Name).Msg("CreateValkeyInstance")
	return asNonRetryable(a.valkey.CreateInstance(ctx, params.Name, params.Port, params.Password, params.MaxMemoryMB))
}

// DeleteValkeyInstance deletes a Valkey instance locally on this node.
func (a *NodeLocal) DeleteValkeyInstance(ctx context.Context, params DeleteValkeyInstanceParams) error {
	a.logger.Info().Str("instance", params.Name).Msg("DeleteValkeyInstance")
	return asNonRetryable(a.valkey.DeleteInstance(ctx, params.Name, params.Port))
}

// CreateValkeyUser creates a Valkey ACL user locally on this node.
func (a *NodeLocal) CreateValkeyUser(ctx context.Context, params CreateValkeyUserParams) error {
	a.logger.Info().Str("username", params.Username).Msg("CreateValkeyUser")
	return asNonRetryable(a.valkey.CreateUser(ctx, params.InstanceName, params.Port, params.Username, params.Password, params.Privileges, params.KeyPattern))
}

// UpdateValkeyUser updates a Valkey ACL user locally on this node.
func (a *NodeLocal) UpdateValkeyUser(ctx context.Context, params UpdateValkeyUserParams) error {
	a.logger.Info().Str("username", params.Username).Msg("UpdateValkeyUser")
	return asNonRetryable(a.valkey.UpdateUser(ctx, params.InstanceName, params.Port, params.Username, params.Password, params.Privileges, params.KeyPattern))
}

// DeleteValkeyUser deletes a Valkey ACL user locally on this node.
func (a *NodeLocal) DeleteValkeyUser(ctx context.Context, params DeleteValkeyUserParams) error {
	a.logger.Info().Str("username", params.Username).Msg("DeleteValkeyUser")
	return asNonRetryable(a.valkey.DeleteUser(ctx, params.InstanceName, params.Port, params.Username))
}

// --------------------------------------------------------------------------
// Migration activities
// --------------------------------------------------------------------------

// DumpMySQLDatabase runs mysqldump and compresses the output to a gzipped file.
func (a *NodeLocal) DumpMySQLDatabase(ctx context.Context, params DumpMySQLDatabaseParams) error {
	a.logger.Info().Str("database", params.DatabaseName).Str("path", params.DumpPath).Msg("DumpMySQLDatabase")
	return asNonRetryable(a.database.DumpDatabase(ctx, params.DatabaseName, params.DumpPath))
}

// ImportMySQLDatabase imports a gzipped SQL dump into a MySQL database.
func (a *NodeLocal) ImportMySQLDatabase(ctx context.Context, params ImportMySQLDatabaseParams) error {
	a.logger.Info().Str("database", params.DatabaseName).Str("path", params.DumpPath).Msg("ImportMySQLDatabase")
	return asNonRetryable(a.database.ImportDatabase(ctx, params.DatabaseName, params.DumpPath))
}

// DumpValkeyData triggers a Valkey BGSAVE and copies the RDB file to the dump path.
func (a *NodeLocal) DumpValkeyData(ctx context.Context, params DumpValkeyDataParams) error {
	a.logger.Info().Str("instance", params.Name).Int("port", params.Port).Str("path", params.DumpPath).Msg("DumpValkeyData")
	return asNonRetryable(a.valkey.DumpData(ctx, params.Name, params.Port, params.Password, params.DumpPath))
}

// ImportValkeyData stops the instance, replaces the RDB file, and restarts.
func (a *NodeLocal) ImportValkeyData(ctx context.Context, params ImportValkeyDataParams) error {
	a.logger.Info().Str("instance", params.Name).Int("port", params.Port).Str("path", params.DumpPath).Msg("ImportValkeyData")
	return asNonRetryable(a.valkey.ImportData(ctx, params.Name, params.Port, params.DumpPath))
}

// CleanupMigrateFile removes a temporary migration file from the local filesystem.
func (a *NodeLocal) CleanupMigrateFile(ctx context.Context, path string) error {
	a.logger.Info().Str("path", path).Msg("CleanupMigrateFile")
	return os.Remove(path)
}

// --------------------------------------------------------------------------
// SSH Keys (authorized_keys)
// --------------------------------------------------------------------------

// SyncSSHKeys writes all public keys to the tenant's authorized_keys file.
func (a *NodeLocal) SyncSSHKeys(ctx context.Context, params SyncSSHKeysParams) error {
	a.logger.Info().Str("tenant", params.TenantName).Int("key_count", len(params.PublicKeys)).Msg("SyncSSHKeys")

	// authorized_keys lives in the tenant's home dir on CephFS.
	sshDir := filepath.Join(a.tenant.WebStorageDir(), params.TenantName, "home", ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("create .ssh dir for %s: %w", params.TenantName, err)
	}

	authKeysPath := filepath.Join(sshDir, "authorized_keys")

	content := ""
	for _, key := range params.PublicKeys {
		content += key + "\n"
	}

	if err := os.WriteFile(authKeysPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("write authorized_keys for %s: %w", params.TenantName, err)
	}

	return nil
}

// --------------------------------------------------------------------------
// SSH
// --------------------------------------------------------------------------

// SyncSSHConfig writes per-tenant sshd config and reloads sshd.
func (a *NodeLocal) SyncSSHConfig(ctx context.Context, params SyncSSHConfigParams) error {
	a.logger.Info().Str("tenant", params.TenantName).Bool("ssh", params.SSHEnabled).Bool("sftp", params.SFTPEnabled).Msg("SyncSSHConfig")
	return asNonRetryable(a.ssh.SyncConfig(ctx, &agent.TenantInfo{
		Name:        params.TenantName,
		SSHEnabled:  params.SSHEnabled,
		SFTPEnabled: params.SFTPEnabled,
	}))
}

// RemoveSSHConfig removes per-tenant sshd config and reloads sshd.
func (a *NodeLocal) RemoveSSHConfig(ctx context.Context, name string) error {
	a.logger.Info().Str("tenant", name).Msg("RemoveSSHConfig")
	return asNonRetryable(a.ssh.RemoveConfig(ctx, name))
}

// --------------------------------------------------------------------------
// SSL
// --------------------------------------------------------------------------

// InstallCertificate writes SSL certificate files to disk locally on this node.
func (a *NodeLocal) InstallCertificate(ctx context.Context, params InstallCertificateParams) error {
	a.logger.Info().Str("fqdn", params.FQDN).Msg("InstallCertificate")
	return asNonRetryable(a.nginx.InstallCertificate(ctx, &agent.CertificateInfo{
		FQDN:     params.FQDN,
		CertPEM:  params.CertPEM,
		KeyPEM:   params.KeyPEM,
		ChainPEM: params.ChainPEM,
	}))
}

// --------------------------------------------------------------------------
// Backup activities
// --------------------------------------------------------------------------

// CreateWebBackup creates a tar.gz backup of a webroot's storage directory.
func (a *NodeLocal) CreateWebBackup(ctx context.Context, params CreateWebBackupParams) (*BackupResult, error) {
	a.logger.Info().Str("tenant", params.TenantName).Str("webroot", params.WebrootName).Str("path", params.BackupPath).Msg("CreateWebBackup")

	sourceDir := fmt.Sprintf("/var/www/storage/%s/webroots/%s", params.TenantName, params.WebrootName)

	// Ensure backup directory exists.
	if err := os.MkdirAll(filepath.Dir(params.BackupPath), 0755); err != nil {
		return nil, fmt.Errorf("create backup directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, "tar", "czf", params.BackupPath, "-C", sourceDir, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("tar czf failed: %w: %s", err, string(out))
	}

	info, err := os.Stat(params.BackupPath)
	if err != nil {
		return nil, fmt.Errorf("stat backup file: %w", err)
	}

	return &BackupResult{
		StoragePath: params.BackupPath,
		SizeBytes:   info.Size(),
	}, nil
}

// RestoreWebBackup extracts a tar.gz backup to a webroot's storage directory.
func (a *NodeLocal) RestoreWebBackup(ctx context.Context, params RestoreWebBackupParams) error {
	a.logger.Info().Str("tenant", params.TenantName).Str("webroot", params.WebrootName).Str("path", params.BackupPath).Msg("RestoreWebBackup")

	targetDir := fmt.Sprintf("/var/www/storage/%s/webroots/%s", params.TenantName, params.WebrootName)

	cmd := exec.CommandContext(ctx, "tar", "xzf", params.BackupPath, "-C", targetDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar xzf failed: %w: %s", err, string(out))
	}

	return nil
}

// CreateMySQLBackup runs mysqldump and stores the compressed output.
func (a *NodeLocal) CreateMySQLBackup(ctx context.Context, params CreateMySQLBackupParams) (*BackupResult, error) {
	a.logger.Info().Str("database", params.DatabaseName).Str("path", params.BackupPath).Msg("CreateMySQLBackup")

	// Ensure backup directory exists.
	if err := os.MkdirAll(filepath.Dir(params.BackupPath), 0755); err != nil {
		return nil, fmt.Errorf("create backup directory: %w", err)
	}

	// Run: mysqldump {dbname} | gzip > {backupPath}
	cmd := exec.CommandContext(ctx, "bash", "-c",
		fmt.Sprintf("mysqldump %s | gzip > %s", params.DatabaseName, params.BackupPath))
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("mysqldump failed: %w: %s", err, string(out))
	}

	info, err := os.Stat(params.BackupPath)
	if err != nil {
		return nil, fmt.Errorf("stat backup file: %w", err)
	}

	return &BackupResult{
		StoragePath: params.BackupPath,
		SizeBytes:   info.Size(),
	}, nil
}

// RestoreMySQLBackup imports a gzipped mysqldump file into a database.
func (a *NodeLocal) RestoreMySQLBackup(ctx context.Context, params RestoreMySQLBackupParams) error {
	a.logger.Info().Str("database", params.DatabaseName).Str("path", params.BackupPath).Msg("RestoreMySQLBackup")

	// Run: gunzip -c {backupPath} | mysql {dbname}
	cmd := exec.CommandContext(ctx, "bash", "-c",
		fmt.Sprintf("gunzip -c %s | mysql %s", params.BackupPath, params.DatabaseName))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mysql restore failed: %w: %s", err, string(out))
	}

	return nil
}

// DeleteBackupFile removes a backup file from disk.
func (a *NodeLocal) DeleteBackupFile(ctx context.Context, storagePath string) error {
	a.logger.Info().Str("path", storagePath).Msg("DeleteBackupFile")
	return os.Remove(storagePath)
}

// --------------------------------------------------------------------------
// S3 activities
// --------------------------------------------------------------------------

// CreateS3Bucket creates an S3 bucket via RGW on this node.
func (a *NodeLocal) CreateS3Bucket(ctx context.Context, params CreateS3BucketParams) error {
	a.logger.Info().Str("tenant", params.TenantID).Str("bucket", params.Name).Msg("CreateS3Bucket")
	return asNonRetryable(a.s3.CreateBucket(ctx, params.TenantID, params.Name, params.QuotaBytes))
}

// DeleteS3Bucket deletes an S3 bucket via RGW on this node.
func (a *NodeLocal) DeleteS3Bucket(ctx context.Context, params DeleteS3BucketParams) error {
	a.logger.Info().Str("tenant", params.TenantID).Str("bucket", params.Name).Msg("DeleteS3Bucket")
	return asNonRetryable(a.s3.DeleteBucket(ctx, params.TenantID, params.Name))
}

// CreateS3AccessKey creates an S3 access key via RGW on this node.
func (a *NodeLocal) CreateS3AccessKey(ctx context.Context, params CreateS3AccessKeyParams) error {
	a.logger.Info().Str("tenant", params.TenantID).Str("access_key", params.AccessKeyID).Msg("CreateS3AccessKey")
	return asNonRetryable(a.s3.CreateAccessKey(ctx, params.TenantID, params.AccessKeyID, params.SecretAccessKey))
}

// DeleteS3AccessKey deletes an S3 access key via RGW on this node.
func (a *NodeLocal) DeleteS3AccessKey(ctx context.Context, params DeleteS3AccessKeyParams) error {
	a.logger.Info().Str("tenant", params.TenantID).Str("access_key", params.AccessKeyID).Msg("DeleteS3AccessKey")
	return asNonRetryable(a.s3.DeleteAccessKey(ctx, params.TenantID, params.AccessKeyID))
}

// UpdateS3BucketPolicy sets or removes a public-read policy on an S3 bucket.
func (a *NodeLocal) UpdateS3BucketPolicy(ctx context.Context, params UpdateS3BucketPolicyParams) error {
	a.logger.Info().Str("tenant", params.TenantID).Str("bucket", params.Name).Bool("public", params.Public).Msg("UpdateS3BucketPolicy")
	return asNonRetryable(a.s3.SetBucketPolicy(ctx, params.TenantID, params.Name, params.Public))
}

// --------------------------------------------------------------------------
// Cron job activities
// --------------------------------------------------------------------------

// CreateCronJobUnits writes systemd timer+service units for a cron job on this node.
func (a *NodeLocal) CreateCronJobUnits(ctx context.Context, params CreateCronJobParams) error {
	a.logger.Info().Str("cron_job", params.ID).Str("tenant", params.TenantName).Msg("CreateCronJobUnits")
	return a.cron.CreateUnits(ctx, &agent.CronJobInfo{
		ID:               params.ID,
		TenantName:       params.TenantName,
		WebrootName:      params.WebrootName,
		Name:             params.Name,
		Schedule:         params.Schedule,
		Command:          params.Command,
		WorkingDirectory: params.WorkingDirectory,
		TimeoutSeconds:   params.TimeoutSeconds,
		MaxMemoryMB:      params.MaxMemoryMB,
		EnvFileName:      params.EnvFileName,
	})
}

// UpdateCronJobUnits rewrites systemd units for a cron job on this node.
func (a *NodeLocal) UpdateCronJobUnits(ctx context.Context, params UpdateCronJobParams) error {
	a.logger.Info().Str("cron_job", params.ID).Str("tenant", params.TenantName).Msg("UpdateCronJobUnits")
	return a.cron.UpdateUnits(ctx, &agent.CronJobInfo{
		ID:               params.ID,
		TenantName:       params.TenantName,
		WebrootName:      params.WebrootName,
		Name:             params.Name,
		Schedule:         params.Schedule,
		Command:          params.Command,
		WorkingDirectory: params.WorkingDirectory,
		TimeoutSeconds:   params.TimeoutSeconds,
		MaxMemoryMB:      params.MaxMemoryMB,
		EnvFileName:      params.EnvFileName,
	})
}

// DeleteCronJobUnits stops, disables, and removes systemd units on this node.
func (a *NodeLocal) DeleteCronJobUnits(ctx context.Context, params DeleteCronJobParams) error {
	a.logger.Info().Str("cron_job", params.ID).Str("tenant", params.TenantName).Msg("DeleteCronJobUnits")
	return a.cron.DeleteUnits(ctx, &agent.CronJobInfo{
		ID:         params.ID,
		TenantName: params.TenantName,
	})
}

// EnableCronJobTimer starts the systemd timer on this node.
func (a *NodeLocal) EnableCronJobTimer(ctx context.Context, params CronJobTimerParams) error {
	a.logger.Info().Str("cron_job", params.ID).Str("tenant", params.TenantName).Msg("EnableCronJobTimer")
	return a.cron.EnableTimer(ctx, &agent.CronJobInfo{
		ID:         params.ID,
		TenantName: params.TenantName,
	})
}

// DisableCronJobTimer stops the systemd timer on this node.
func (a *NodeLocal) DisableCronJobTimer(ctx context.Context, params CronJobTimerParams) error {
	a.logger.Info().Str("cron_job", params.ID).Str("tenant", params.TenantName).Msg("DisableCronJobTimer")
	return a.cron.DisableTimer(ctx, &agent.CronJobInfo{
		ID:         params.ID,
		TenantName: params.TenantName,
	})
}

// --------------------------------------------------------------------------
// Daemon activities
// --------------------------------------------------------------------------

// CreateDaemonConfig writes a supervisord config for a daemon and starts it.
func (a *NodeLocal) CreateDaemonConfig(ctx context.Context, params CreateDaemonParams) error {
	a.logger.Info().Str("daemon", params.ID).Str("tenant", params.TenantName).Msg("CreateDaemonConfig")
	info := &agent.DaemonInfo{
		ID:           params.ID,
		TenantName:   params.TenantName,
		WebrootName:  params.WebrootName,
		Name:         params.Name,
		Command:      params.Command,
		ProxyPort:    params.ProxyPort,
		HostIP:       params.HostIP,
		NumProcs:     params.NumProcs,
		StopSignal:   params.StopSignal,
		StopWaitSecs: params.StopWaitSecs,
		MaxMemoryMB:  params.MaxMemoryMB,
		EnvFileName:  params.EnvFileName,
	}
	if err := a.daemon.Configure(ctx, info); err != nil {
		return asNonRetryable(fmt.Errorf("configure daemon: %w", err))
	}
	if err := a.daemon.Start(ctx, info); err != nil {
		return asNonRetryable(fmt.Errorf("start daemon: %w", err))
	}
	return nil
}

// UpdateDaemonConfig updates a supervisord config for a daemon and reloads it.
func (a *NodeLocal) UpdateDaemonConfig(ctx context.Context, params UpdateDaemonParams) error {
	a.logger.Info().Str("daemon", params.ID).Str("tenant", params.TenantName).Msg("UpdateDaemonConfig")
	info := &agent.DaemonInfo{
		ID:           params.ID,
		TenantName:   params.TenantName,
		WebrootName:  params.WebrootName,
		Name:         params.Name,
		Command:      params.Command,
		ProxyPort:    params.ProxyPort,
		HostIP:       params.HostIP,
		NumProcs:     params.NumProcs,
		StopSignal:   params.StopSignal,
		StopWaitSecs: params.StopWaitSecs,
		MaxMemoryMB:  params.MaxMemoryMB,
		EnvFileName:  params.EnvFileName,
	}
	if err := a.daemon.Configure(ctx, info); err != nil {
		return asNonRetryable(fmt.Errorf("configure daemon: %w", err))
	}
	if err := a.daemon.Reload(ctx, info); err != nil {
		return asNonRetryable(fmt.Errorf("reload daemon: %w", err))
	}
	return nil
}

// DeleteDaemonConfig stops and removes a daemon's supervisord config.
func (a *NodeLocal) DeleteDaemonConfig(ctx context.Context, params DeleteDaemonParams) error {
	a.logger.Info().Str("daemon", params.ID).Str("tenant", params.TenantName).Msg("DeleteDaemonConfig")
	return a.daemon.Remove(ctx, &agent.DaemonInfo{
		ID:          params.ID,
		TenantName:  params.TenantName,
		WebrootName: params.WebrootName,
		Name:        params.Name,
	})
}

// EnableDaemon starts a daemon on this node.
func (a *NodeLocal) EnableDaemon(ctx context.Context, params DaemonEnableParams) error {
	a.logger.Info().Str("daemon", params.ID).Str("tenant", params.TenantName).Msg("EnableDaemon")
	return a.daemon.Start(ctx, &agent.DaemonInfo{
		ID:          params.ID,
		TenantName:  params.TenantName,
		WebrootName: params.WebrootName,
		Name:        params.Name,
	})
}

// DisableDaemon stops a daemon on this node.
func (a *NodeLocal) DisableDaemon(ctx context.Context, params DaemonEnableParams) error {
	a.logger.Info().Str("daemon", params.ID).Str("tenant", params.TenantName).Msg("DisableDaemon")
	return a.daemon.Stop(ctx, &agent.DaemonInfo{
		ID:          params.ID,
		TenantName:  params.TenantName,
		WebrootName: params.WebrootName,
		Name:        params.Name,
	})
}

// --------------------------------------------------------------------------
// Tenant ULA address activities
// --------------------------------------------------------------------------

// ConfigureTenantAddresses adds the tenant's ULA IPv6 address to tenant0 and
// configures nftables binding restrictions.
func (a *NodeLocal) ConfigureTenantAddresses(ctx context.Context, params ConfigureTenantAddressesParams) error {
	a.logger.Info().Str("tenant", params.TenantName).Int("uid", params.TenantUID).Msg("ConfigureTenantAddresses")
	return a.tenantULA.Configure(ctx, &agent.TenantULAInfo{
		TenantName:   params.TenantName,
		TenantUID:    params.TenantUID,
		ClusterID:    params.ClusterID,
		NodeShardIdx: params.NodeShardIdx,
	})
}

// RemoveTenantAddresses removes the tenant's ULA IPv6 address from tenant0 and
// cleans up nftables binding restrictions.
func (a *NodeLocal) RemoveTenantAddresses(ctx context.Context, params ConfigureTenantAddressesParams) error {
	a.logger.Info().Str("tenant", params.TenantName).Int("uid", params.TenantUID).Msg("RemoveTenantAddresses")
	return a.tenantULA.Remove(ctx, &agent.TenantULAInfo{
		TenantName:   params.TenantName,
		TenantUID:    params.TenantUID,
		ClusterID:    params.ClusterID,
		NodeShardIdx: params.NodeShardIdx,
	})
}

// ConfigureULARoutes sets up cross-node IPv6 transit addresses and routes so
// nodes in a shard can reach each other's tenant ULA addresses.
func (a *NodeLocal) ConfigureULARoutes(ctx context.Context, params ConfigureULARoutesParams) error {
	a.logger.Info().Int("index", params.ThisNodeIndex).Ints("others", params.OtherNodeIndices).Msg("ConfigureULARoutes")
	return a.tenantULA.ConfigureRoutes(ctx, &agent.ULARoutesInfo{
		ClusterID:        params.ClusterID,
		ThisNodeIndex:    params.ThisNodeIndex,
		OtherNodeIndices: params.OtherNodeIndices,
	})
}

// ConfigureServiceTenantAddr adds a tenant's ULA address on a service node
// (DB/Valkey). Only adds the IPv6 to tenant0 + nftables ingress set, no
// UID-based binding rules.
func (a *NodeLocal) ConfigureServiceTenantAddr(ctx context.Context, params ConfigureTenantAddressesParams) error {
	a.logger.Info().Str("tenant", params.TenantName).Int("uid", params.TenantUID).Msg("ConfigureServiceTenantAddr")
	return a.tenantULA.ConfigureServiceAddr(ctx, &agent.TenantULAInfo{
		TenantName:   params.TenantName,
		TenantUID:    params.TenantUID,
		ClusterID:    params.ClusterID,
		NodeShardIdx: params.NodeShardIdx,
	})
}

// RemoveServiceTenantAddr removes a tenant's ULA address from a service node.
func (a *NodeLocal) RemoveServiceTenantAddr(ctx context.Context, params ConfigureTenantAddressesParams) error {
	a.logger.Info().Str("tenant", params.TenantName).Int("uid", params.TenantUID).Msg("RemoveServiceTenantAddr")
	return a.tenantULA.RemoveServiceAddr(ctx, &agent.TenantULAInfo{
		TenantName:   params.TenantName,
		TenantUID:    params.TenantUID,
		ClusterID:    params.ClusterID,
		NodeShardIdx: params.NodeShardIdx,
	})
}

// ConfigureULARoutesV2 sets up generalized cross-shard IPv6 transit addresses
// and routes, supporting routing between web, DB, and Valkey nodes.
func (a *NodeLocal) ConfigureULARoutesV2(ctx context.Context, params ConfigureULARoutesV2Params) error {
	a.logger.Info().Int("transit_index", params.ThisTransitIndex).Int("peer_count", len(params.Peers)).Msg("ConfigureULARoutesV2")
	peers := make([]agent.ULARoutePeer, len(params.Peers))
	for i, p := range params.Peers {
		peers[i] = agent.ULARoutePeer{
			PrefixIndex:  p.PrefixIndex,
			TransitIndex: p.TransitIndex,
		}
	}
	return a.tenantULA.ConfigureRoutesV2(ctx, &agent.ULARoutesInfoV2{
		ClusterID:        params.ClusterID,
		ThisTransitIndex: params.ThisTransitIndex,
		Peers:            peers,
	})
}

// --------------------------------------------------------------------------
// Egress rules activities
// --------------------------------------------------------------------------

// SyncEgressRules applies the full set of egress rules for a tenant via nftables.
func (a *NodeLocal) SyncEgressRules(ctx context.Context, params SyncEgressRulesParams) error {
	a.logger.Info().Int("uid", params.TenantUID).Int("rule_count", len(params.Rules)).Msg("SyncEgressRules")
	return asNonRetryable(a.tenantULA.SyncEgressRules(ctx, params.TenantUID, params.Rules))
}

// --------------------------------------------------------------------------
// Database access rules activities
// --------------------------------------------------------------------------

// SyncDatabaseUserHosts rebuilds MySQL users with host patterns based on access rules.
func (a *NodeLocal) SyncDatabaseUserHosts(ctx context.Context, params SyncDatabaseUserHostsParams) error {
	a.logger.Info().Str("database", params.DatabaseName).Int("user_count", len(params.Users)).Int("rule_count", len(params.AccessRules)).Msg("SyncDatabaseUserHosts")
	internalCIDR := params.InternalNetworkCIDR
	if internalCIDR == "" {
		internalCIDR = "10.0.0.0/8"
	}
	return asNonRetryable(a.database.SyncUserHosts(ctx, params.DatabaseName, params.Users, params.AccessRules, internalCIDR))
}

// --------------------------------------------------------------------------
// Env file helpers
// --------------------------------------------------------------------------

// writeWebrootEnv writes a .env file and .envrc for a webroot.
// The .envrc is always written when there are env vars or a PHP runtime version,
// so that direnv auto-loads the environment and selects the correct PHP binary.
func writeWebrootEnv(tenantName, webrootName, envFileName, runtimeName, runtimeVersion string, envVars map[string]string) error {
	if envFileName == "" {
		envFileName = ".env.hosting"
	}
	dir := filepath.Join("/var/www/storage", tenantName, "webroots", webrootName)
	envrcPath := filepath.Join(dir, ".envrc")
	envFilePath := filepath.Join(dir, envFileName)
	binDir := filepath.Join(dir, ".bin")

	isPHP := runtimeName == "php" && runtimeVersion != ""

	if len(envVars) == 0 && !isPHP {
		// Nothing to set up — clean up any previous files.
		_ = os.Remove(envFilePath)
		_ = os.Remove(envrcPath)
		_ = os.RemoveAll(binDir)
		return nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create webroot dir: %w", err)
	}

	// Look up tenant UID/GID for chowning files.
	var uid, gid int
	if u, err := user.Lookup(tenantName); err == nil {
		uid, _ = strconv.Atoi(u.Uid)
		gid, _ = strconv.Atoi(u.Gid)
	}

	// Write the .env file if there are vars.
	if len(envVars) > 0 {
		keys := make([]string, 0, len(envVars))
		for k := range envVars {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var b strings.Builder
		b.WriteString("# Auto-generated by hosting platform. Do not edit.\n")
		for _, k := range keys {
			b.WriteString(k)
			b.WriteString("=\"")
			b.WriteString(shellEscape(envVars[k]))
			b.WriteString("\"\n")
		}
		if err := os.WriteFile(envFilePath, []byte(b.String()), 0644); err != nil {
			return fmt.Errorf("write env file: %w", err)
		}
		_ = os.Chown(envFilePath, uid, gid)
	} else {
		_ = os.Remove(envFilePath)
	}

	// Create .bin/php symlink for the configured PHP version so that `php`
	// inside the webroot resolves to the right version (e.g. php8.3).
	if isPHP {
		_ = os.MkdirAll(binDir, 0755)
		phpLink := filepath.Join(binDir, "php")
		_ = os.Remove(phpLink)
		_ = os.Symlink("/usr/bin/php"+runtimeVersion, phpLink)
		_ = os.Chown(binDir, uid, gid)
		_ = os.Lchown(phpLink, uid, gid)
	} else {
		_ = os.RemoveAll(binDir)
	}

	// Build .envrc content.
	var envrc strings.Builder
	if isPHP {
		envrc.WriteString("PATH_add .bin\n")
		envrc.WriteString(fmt.Sprintf("export PHP_VERSION=%s\n", runtimeVersion))
	}
	if len(envVars) > 0 {
		envrc.WriteString(fmt.Sprintf("dotenv_if_exists %s\n", envFileName))
	}
	if err := os.WriteFile(envrcPath, []byte(envrc.String()), 0644); err != nil {
		return fmt.Errorf("write .envrc: %w", err)
	}
	_ = os.Chown(envrcPath, uid, gid)

	return nil
}

// shellEscape escapes a value for use in a double-quoted shell string.
func shellEscape(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		`$`, `\$`,
		"`", "\\`",
	)
	return r.Replace(s)
}


// --------------------------------------------------------------------------
// Disk usage activities
// --------------------------------------------------------------------------

// DiskUsage reports disk usage for a single mount point.
type DiskUsage struct {
	Path       string  `json:"path"`
	TotalBytes uint64  `json:"total_bytes"`
	UsedBytes  uint64  `json:"used_bytes"`
	FreeBytes  uint64  `json:"free_bytes"`
	UsedPct    float64 `json:"used_pct"`
}

// CephFSStatus reports the CephFS mount status on this node.
type CephFSStatus struct {
	Path    string `json:"path"`
	Mounted bool   `json:"mounted"`
	Error   string `json:"error,omitempty"`
}

// CheckCephFSMount checks if CephFS is properly mounted at the web storage path.
func (a *NodeLocal) CheckCephFSMount(ctx context.Context) (*CephFSStatus, error) {
	path := "/var/www/storage"
	status := &CephFSStatus{Path: path}

	if os.Getenv("SKIP_CEPHFS_CHECK") == "1" {
		status.Mounted = true
		return status, nil
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		status.Error = fmt.Sprintf("statfs failed: %v", err)
		return status, nil
	}

	const cephFSMagic = 0x00C36400
	if stat.Type != cephFSMagic {
		status.Error = fmt.Sprintf("unexpected filesystem type 0x%X (expected CephFS 0x%X)", stat.Type, cephFSMagic)
		return status, nil
	}

	status.Mounted = true
	return status, nil
}

// GetDiskUsage returns disk usage for key mount points on the node.
func (a *NodeLocal) GetDiskUsage(ctx context.Context) ([]DiskUsage, error) {
	paths := []string{"/", "/var/lib/mysql", "/var/www/storage"}

	var results []DiskUsage
	seen := make(map[uint64]bool) // dedup by filesystem device ID

	for _, path := range paths {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(path, &stat); err != nil {
			// Path doesn't exist on this node type — skip.
			continue
		}

		// Deduplicate by device — multiple paths may be on the same filesystem.
		devID := uint64(stat.Fsid.X__val[0])<<32 | uint64(stat.Fsid.X__val[1])
		if seen[devID] {
			continue
		}
		seen[devID] = true

		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bavail * uint64(stat.Bsize) // available to unprivileged users
		used := total - free
		var usedPct float64
		if total > 0 {
			usedPct = float64(used) / float64(total) * 100
		}

		results = append(results, DiskUsage{
			Path:       path,
			TotalBytes: total,
			UsedBytes:  used,
			FreeBytes:  free,
			UsedPct:    usedPct,
		})
	}

	return results, nil
}

// --------------------------------------------------------------------------
// Resource usage collection activities
// --------------------------------------------------------------------------

// GetResourceUsageParams holds parameters for collecting per-resource usage.
type GetResourceUsageParams struct {
	Role string `json:"role"` // "web" or "database"
}

// ResourceUsageEntry represents per-resource disk usage collected from a node.
type ResourceUsageEntry struct {
	ResourceType string `json:"resource_type"` // "webroot" or "database"
	Name         string `json:"name"`          // "tenant/webroot" or "db_name"
	BytesUsed    int64  `json:"bytes_used"`
}

// GetResourceUsage collects per-resource disk usage on this node.
func (a *NodeLocal) GetResourceUsage(ctx context.Context, params GetResourceUsageParams) ([]ResourceUsageEntry, error) {
	a.logger.Info().Str("role", params.Role).Msg("GetResourceUsage")

	switch params.Role {
	case "web":
		return a.getWebResourceUsage(ctx)
	case "database":
		return a.getDatabaseResourceUsage(ctx)
	default:
		return nil, fmt.Errorf("unsupported role for resource usage: %s", params.Role)
	}
}

// getWebResourceUsage walks /var/www/storage/*/webroots/*/ and runs du -sb on each.
func (a *NodeLocal) getWebResourceUsage(ctx context.Context) ([]ResourceUsageEntry, error) {
	baseDir := "/var/www/storage"
	tenantDirs, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read storage dir: %w", err)
	}

	var entries []ResourceUsageEntry
	for _, tenantDir := range tenantDirs {
		if !tenantDir.IsDir() {
			continue
		}
		tenantName := tenantDir.Name()
		webrootsDir := filepath.Join(baseDir, tenantName, "webroots")
		webrootDirs, err := os.ReadDir(webrootsDir)
		if err != nil {
			continue // no webroots dir for this tenant
		}
		for _, wrDir := range webrootDirs {
			if !wrDir.IsDir() {
				continue
			}
			webrootName := wrDir.Name()
			webrootPath := filepath.Join(webrootsDir, webrootName)

			// Run du -sb to get total bytes.
			cmd := exec.CommandContext(ctx, "du", "-sb", webrootPath)
			out, err := cmd.Output()
			if err != nil {
				a.logger.Warn().Err(err).Str("path", webrootPath).Msg("du -sb failed")
				continue
			}
			parts := strings.Fields(string(out))
			if len(parts) < 1 {
				continue
			}
			bytesUsed, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				continue
			}

			entries = append(entries, ResourceUsageEntry{
				ResourceType: "webroot",
				Name:         tenantName + "/" + webrootName,
				BytesUsed:    bytesUsed,
			})
		}
	}

	return entries, nil
}

// getDatabaseResourceUsage queries MySQL information_schema for per-database sizes.
func (a *NodeLocal) getDatabaseResourceUsage(ctx context.Context) ([]ResourceUsageEntry, error) {
	cmd := exec.CommandContext(ctx, "mysql", "-N", "-B", "-e",
		"SELECT table_schema, SUM(data_length + index_length) FROM information_schema.tables WHERE table_schema NOT IN ('mysql','information_schema','performance_schema','sys') GROUP BY table_schema")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("mysql query failed: %w", err)
	}

	var entries []ResourceUsageEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		dbName := parts[0]
		bytesUsed, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		entries = append(entries, ResourceUsageEntry{
			ResourceType: "database",
			Name:         dbName,
			BytesUsed:    bytesUsed,
		})
	}

	return entries, nil
}

// --------------------------------------------------------------------------
// WireGuard activities
// --------------------------------------------------------------------------

// ConfigureWireGuardPeer adds a WireGuard peer to the gateway node.
func (a *NodeLocal) ConfigureWireGuardPeer(ctx context.Context, params ConfigureWireGuardPeerParams) error {
	if a.wireguard == nil {
		return nil // Not a gateway node.
	}
	return a.wireguard.AddPeer(ctx, agent.AddPeerParams{
		PublicKey:    params.PublicKey,
		PresharedKey: params.PresharedKey,
		AssignedIP:   params.AssignedIP,
		AllowedIPs:   params.AllowedIPs,
	})
}

// RemoveWireGuardPeer removes a WireGuard peer from the gateway node.
func (a *NodeLocal) RemoveWireGuardPeer(ctx context.Context, params RemoveWireGuardPeerParams) error {
	if a.wireguard == nil {
		return nil
	}
	return a.wireguard.RemovePeer(ctx, params.PublicKey, params.AssignedIP)
}

// GetWireGuardPublicKey returns the WireGuard server public key from the gateway node.
func (a *NodeLocal) GetWireGuardPublicKey(ctx context.Context) (string, error) {
	if a.wireguard == nil {
		return "", nil
	}
	return a.wireguard.GetPublicKey(ctx)
}

// SyncWireGuardPeers performs full convergence of all WireGuard peers on a gateway node.
func (a *NodeLocal) SyncWireGuardPeers(ctx context.Context, params SyncWireGuardPeersParams) error {
	if a.wireguard == nil {
		return nil
	}
	var agentPeers []agent.AddPeerParams
	for _, p := range params.Peers {
		agentPeers = append(agentPeers, agent.AddPeerParams{
			PublicKey:    p.PublicKey,
			PresharedKey: p.PresharedKey,
			AssignedIP:   p.AssignedIP,
			AllowedIPs:   p.AllowedIPs,
		})
	}
	return a.wireguard.SyncPeers(ctx, agentPeers)
}

