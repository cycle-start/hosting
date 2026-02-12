package activity

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rs/zerolog"

	"github.com/edvin/hosting/internal/agent"
	"github.com/edvin/hosting/internal/agent/runtime"
	agentv1 "github.com/edvin/hosting/proto/agent/v1"
)

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
	runtimes map[string]runtime.Manager
}

// NewNodeLocal creates a new NodeLocal activity struct.
func NewNodeLocal(
	logger zerolog.Logger,
	tenant *agent.TenantManager,
	webroot *agent.WebrootManager,
	nginx *agent.NginxManager,
	database *agent.DatabaseManager,
	valkey *agent.ValkeyManager,
	runtimes map[string]runtime.Manager,
) *NodeLocal {
	return &NodeLocal{
		logger:   logger.With().Str("component", "node-local-activity").Logger(),
		tenant:   tenant,
		webroot:  webroot,
		nginx:    nginx,
		database: database,
		valkey:   valkey,
		runtimes: runtimes,
	}
}

// --------------------------------------------------------------------------
// Tenant activities
// --------------------------------------------------------------------------

// CreateTenant creates a tenant locally on this node.
func (a *NodeLocal) CreateTenant(ctx context.Context, params CreateTenantParams) error {
	a.logger.Info().Str("tenant", params.Name).Msg("CreateTenant")
	return a.tenant.Create(ctx, &agentv1.TenantInfo{
		Id:          params.ID,
		Name:        params.Name,
		Uid:         int32(params.UID),
		SftpEnabled: params.SFTPEnabled,
	})
}

// UpdateTenant updates a tenant locally on this node.
func (a *NodeLocal) UpdateTenant(ctx context.Context, params UpdateTenantParams) error {
	a.logger.Info().Str("tenant", params.Name).Msg("UpdateTenant")
	return a.tenant.Update(ctx, &agentv1.TenantInfo{
		Id:          params.ID,
		Name:        params.Name,
		Uid:         int32(params.UID),
		SftpEnabled: params.SFTPEnabled,
	})
}

// SuspendTenant suspends a tenant locally on this node.
func (a *NodeLocal) SuspendTenant(ctx context.Context, name string) error {
	a.logger.Info().Str("tenant", name).Msg("SuspendTenant")
	return a.tenant.Suspend(ctx, name)
}

// UnsuspendTenant unsuspends a tenant locally on this node.
func (a *NodeLocal) UnsuspendTenant(ctx context.Context, name string) error {
	a.logger.Info().Str("tenant", name).Msg("UnsuspendTenant")
	return a.tenant.Unsuspend(ctx, name)
}

// DeleteTenant deletes a tenant locally on this node.
func (a *NodeLocal) DeleteTenant(ctx context.Context, name string) error {
	a.logger.Info().Str("tenant", name).Msg("DeleteTenant")
	return a.tenant.Delete(ctx, name)
}

// --------------------------------------------------------------------------
// Webroot activities
// --------------------------------------------------------------------------

// CreateWebroot creates a webroot locally on this node.
func (a *NodeLocal) CreateWebroot(ctx context.Context, params CreateWebrootParams) error {
	a.logger.Info().Str("tenant", params.TenantName).Str("webroot", params.Name).Msg("CreateWebroot")

	info := &agentv1.WebrootInfo{
		Id:             params.ID,
		TenantName:     params.TenantName,
		Name:           params.Name,
		Runtime:        params.Runtime,
		RuntimeVersion: params.RuntimeVersion,
		RuntimeConfig:  params.RuntimeConfig,
		PublicFolder:   params.PublicFolder,
	}

	fqdns := make([]*agentv1.FQDNInfo, len(params.FQDNs))
	for i, f := range params.FQDNs {
		fqdns[i] = &agentv1.FQDNInfo{
			Fqdn:       f.FQDN,
			WebrootId:  f.WebrootID,
			SslEnabled: f.SSLEnabled,
		}
	}

	// Create webroot directories.
	if err := a.webroot.Create(ctx, info); err != nil {
		return fmt.Errorf("create webroot: %w", err)
	}

	// Configure and start runtime.
	rt, ok := a.runtimes[info.GetRuntime()]
	if !ok {
		return fmt.Errorf("unsupported runtime: %s", info.GetRuntime())
	}
	if err := rt.Configure(ctx, info); err != nil {
		return fmt.Errorf("configure runtime: %w", err)
	}
	if err := rt.Start(ctx, info); err != nil {
		return fmt.Errorf("start runtime: %w", err)
	}

	// Generate and write nginx config.
	nginxConfig, err := a.nginx.GenerateConfig(info, fqdns)
	if err != nil {
		return fmt.Errorf("generate nginx config: %w", err)
	}
	if err := a.nginx.WriteConfig(info.GetTenantName(), info.GetName(), nginxConfig); err != nil {
		return fmt.Errorf("write nginx config: %w", err)
	}

	// Reload nginx.
	if err := a.nginx.Reload(ctx); err != nil {
		return fmt.Errorf("reload nginx: %w", err)
	}

	return nil
}

// UpdateWebroot updates a webroot locally on this node.
func (a *NodeLocal) UpdateWebroot(ctx context.Context, params UpdateWebrootParams) error {
	a.logger.Info().Str("tenant", params.TenantName).Str("webroot", params.Name).Msg("UpdateWebroot")

	info := &agentv1.WebrootInfo{
		Id:             params.ID,
		TenantName:     params.TenantName,
		Name:           params.Name,
		Runtime:        params.Runtime,
		RuntimeVersion: params.RuntimeVersion,
		RuntimeConfig:  params.RuntimeConfig,
		PublicFolder:   params.PublicFolder,
	}

	fqdns := make([]*agentv1.FQDNInfo, len(params.FQDNs))
	for i, f := range params.FQDNs {
		fqdns[i] = &agentv1.FQDNInfo{
			Fqdn:       f.FQDN,
			WebrootId:  f.WebrootID,
			SslEnabled: f.SSLEnabled,
		}
	}

	// Update webroot directories.
	if err := a.webroot.Update(ctx, info); err != nil {
		return fmt.Errorf("update webroot: %w", err)
	}

	// Reconfigure and reload runtime.
	rt, ok := a.runtimes[info.GetRuntime()]
	if !ok {
		return fmt.Errorf("unsupported runtime: %s", info.GetRuntime())
	}
	if err := rt.Configure(ctx, info); err != nil {
		return fmt.Errorf("configure runtime: %w", err)
	}
	if err := rt.Reload(ctx, info); err != nil {
		return fmt.Errorf("reload runtime: %w", err)
	}

	// Regenerate and write nginx config.
	nginxConfig, err := a.nginx.GenerateConfig(info, fqdns)
	if err != nil {
		return fmt.Errorf("generate nginx config: %w", err)
	}
	if err := a.nginx.WriteConfig(info.GetTenantName(), info.GetName(), nginxConfig); err != nil {
		return fmt.Errorf("write nginx config: %w", err)
	}

	// Reload nginx.
	if err := a.nginx.Reload(ctx); err != nil {
		return fmt.Errorf("reload nginx: %w", err)
	}

	return nil
}

// DeleteWebroot deletes a webroot locally on this node.
func (a *NodeLocal) DeleteWebroot(ctx context.Context, tenantName, webrootName string) error {
	a.logger.Info().Str("tenant", tenantName).Str("webroot", webrootName).Msg("DeleteWebroot")

	// Remove nginx config.
	if err := a.nginx.RemoveConfig(tenantName, webrootName); err != nil {
		return fmt.Errorf("remove nginx config: %w", err)
	}

	// Reload nginx.
	if err := a.nginx.Reload(ctx); err != nil {
		return fmt.Errorf("reload nginx: %w", err)
	}

	// Remove runtimes (try all, only one will match).
	wrInfo := &agentv1.WebrootInfo{TenantName: tenantName, Name: webrootName}
	for _, rt := range a.runtimes {
		_ = rt.Remove(ctx, wrInfo)
	}

	// Remove webroot directories.
	if err := a.webroot.Delete(ctx, tenantName, webrootName); err != nil {
		return fmt.Errorf("delete webroot: %w", err)
	}

	return nil
}

// --------------------------------------------------------------------------
// Runtime / Nginx activities
// --------------------------------------------------------------------------

// ConfigureRuntime configures and starts a runtime for a webroot.
func (a *NodeLocal) ConfigureRuntime(ctx context.Context, params ConfigureRuntimeParams) error {
	a.logger.Info().Str("runtime", params.Runtime).Str("webroot", params.Name).Msg("ConfigureRuntime")

	info := &agentv1.WebrootInfo{
		Id:             params.ID,
		TenantName:     params.TenantName,
		Name:           params.Name,
		Runtime:        params.Runtime,
		RuntimeVersion: params.RuntimeVersion,
		RuntimeConfig:  params.RuntimeConfig,
		PublicFolder:   params.PublicFolder,
	}

	rt, ok := a.runtimes[info.GetRuntime()]
	if !ok {
		return fmt.Errorf("unsupported runtime: %s", info.GetRuntime())
	}
	if err := rt.Configure(ctx, info); err != nil {
		return fmt.Errorf("configure runtime: %w", err)
	}
	if err := rt.Start(ctx, info); err != nil {
		return fmt.Errorf("start runtime: %w", err)
	}
	return nil
}

// ReloadNginx tests and reloads the nginx configuration.
func (a *NodeLocal) ReloadNginx(ctx context.Context) error {
	a.logger.Info().Msg("ReloadNginx")
	return a.nginx.Reload(ctx)
}

// --------------------------------------------------------------------------
// Database activities
// --------------------------------------------------------------------------

// CreateDatabase creates a MySQL database locally on this node.
func (a *NodeLocal) CreateDatabase(ctx context.Context, name string) error {
	a.logger.Info().Str("database", name).Msg("CreateDatabase")
	return a.database.CreateDatabase(ctx, name)
}

// DeleteDatabase drops a MySQL database locally on this node.
func (a *NodeLocal) DeleteDatabase(ctx context.Context, name string) error {
	a.logger.Info().Str("database", name).Msg("DeleteDatabase")
	return a.database.DeleteDatabase(ctx, name)
}

// CreateDatabaseUser creates a MySQL user locally on this node.
func (a *NodeLocal) CreateDatabaseUser(ctx context.Context, params CreateDatabaseUserParams) error {
	a.logger.Info().Str("username", params.Username).Msg("CreateDatabaseUser")
	return a.database.CreateUser(ctx, params.DatabaseName, params.Username, params.Password, params.Privileges)
}

// UpdateDatabaseUser updates a MySQL user locally on this node.
func (a *NodeLocal) UpdateDatabaseUser(ctx context.Context, params UpdateDatabaseUserParams) error {
	a.logger.Info().Str("username", params.Username).Msg("UpdateDatabaseUser")
	return a.database.UpdateUser(ctx, params.DatabaseName, params.Username, params.Password, params.Privileges)
}

// DeleteDatabaseUser drops a MySQL user locally on this node.
func (a *NodeLocal) DeleteDatabaseUser(ctx context.Context, dbName, username string) error {
	a.logger.Info().Str("username", username).Msg("DeleteDatabaseUser")
	return a.database.DeleteUser(ctx, dbName, username)
}

// --------------------------------------------------------------------------
// Valkey activities
// --------------------------------------------------------------------------

// CreateValkeyInstance creates a Valkey instance locally on this node.
func (a *NodeLocal) CreateValkeyInstance(ctx context.Context, params CreateValkeyInstanceParams) error {
	a.logger.Info().Str("instance", params.Name).Msg("CreateValkeyInstance")
	return a.valkey.CreateInstance(ctx, params.Name, params.Port, params.Password, params.MaxMemoryMB)
}

// DeleteValkeyInstance deletes a Valkey instance locally on this node.
func (a *NodeLocal) DeleteValkeyInstance(ctx context.Context, params DeleteValkeyInstanceParams) error {
	a.logger.Info().Str("instance", params.Name).Msg("DeleteValkeyInstance")
	return a.valkey.DeleteInstance(ctx, params.Name, params.Port)
}

// CreateValkeyUser creates a Valkey ACL user locally on this node.
func (a *NodeLocal) CreateValkeyUser(ctx context.Context, params CreateValkeyUserParams) error {
	a.logger.Info().Str("username", params.Username).Msg("CreateValkeyUser")
	return a.valkey.CreateUser(ctx, params.InstanceName, params.Port, params.Username, params.Password, params.Privileges, params.KeyPattern)
}

// UpdateValkeyUser updates a Valkey ACL user locally on this node.
func (a *NodeLocal) UpdateValkeyUser(ctx context.Context, params UpdateValkeyUserParams) error {
	a.logger.Info().Str("username", params.Username).Msg("UpdateValkeyUser")
	return a.valkey.UpdateUser(ctx, params.InstanceName, params.Port, params.Username, params.Password, params.Privileges, params.KeyPattern)
}

// DeleteValkeyUser deletes a Valkey ACL user locally on this node.
func (a *NodeLocal) DeleteValkeyUser(ctx context.Context, params DeleteValkeyUserParams) error {
	a.logger.Info().Str("username", params.Username).Msg("DeleteValkeyUser")
	return a.valkey.DeleteUser(ctx, params.InstanceName, params.Port, params.Username)
}

// --------------------------------------------------------------------------
// Migration activities
// --------------------------------------------------------------------------

// DumpMySQLDatabase runs mysqldump and compresses the output to a gzipped file.
func (a *NodeLocal) DumpMySQLDatabase(ctx context.Context, params DumpMySQLDatabaseParams) error {
	a.logger.Info().Str("database", params.DatabaseName).Str("path", params.DumpPath).Msg("DumpMySQLDatabase")
	return a.database.DumpDatabase(ctx, params.DatabaseName, params.DumpPath)
}

// ImportMySQLDatabase imports a gzipped SQL dump into a MySQL database.
func (a *NodeLocal) ImportMySQLDatabase(ctx context.Context, params ImportMySQLDatabaseParams) error {
	a.logger.Info().Str("database", params.DatabaseName).Str("path", params.DumpPath).Msg("ImportMySQLDatabase")
	return a.database.ImportDatabase(ctx, params.DatabaseName, params.DumpPath)
}

// DumpValkeyData triggers a Valkey BGSAVE and copies the RDB file to the dump path.
func (a *NodeLocal) DumpValkeyData(ctx context.Context, params DumpValkeyDataParams) error {
	a.logger.Info().Str("instance", params.Name).Int("port", params.Port).Str("path", params.DumpPath).Msg("DumpValkeyData")
	return a.valkey.DumpData(ctx, params.Name, params.Port, params.Password, params.DumpPath)
}

// ImportValkeyData stops the instance, replaces the RDB file, and restarts.
func (a *NodeLocal) ImportValkeyData(ctx context.Context, params ImportValkeyDataParams) error {
	a.logger.Info().Str("instance", params.Name).Int("port", params.Port).Str("path", params.DumpPath).Msg("ImportValkeyData")
	return a.valkey.ImportData(ctx, params.Name, params.Port, params.DumpPath)
}

// CleanupMigrateFile removes a temporary migration file from the local filesystem.
func (a *NodeLocal) CleanupMigrateFile(ctx context.Context, path string) error {
	a.logger.Info().Str("path", path).Msg("CleanupMigrateFile")
	return os.Remove(path)
}

// --------------------------------------------------------------------------
// SFTP
// --------------------------------------------------------------------------

// SyncSFTPKeys writes all public keys to the tenant's authorized_keys file.
func (a *NodeLocal) SyncSFTPKeys(ctx context.Context, params SyncSFTPKeysParams) error {
	a.logger.Info().Str("tenant", params.TenantName).Int("key_count", len(params.PublicKeys)).Msg("SyncSFTPKeys")

	homeDir := fmt.Sprintf("/var/www/storage/%s", params.TenantName)
	authKeysPath := fmt.Sprintf("%s/.ssh/authorized_keys", homeDir)

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
// SSL
// --------------------------------------------------------------------------

// InstallCertificate writes SSL certificate files to disk locally on this node.
func (a *NodeLocal) InstallCertificate(ctx context.Context, params InstallCertificateParams) error {
	a.logger.Info().Str("fqdn", params.FQDN).Msg("InstallCertificate")
	return a.nginx.InstallCertificate(ctx, &agentv1.CertificateInfo{
		Fqdn:     params.FQDN,
		CertPem:  params.CertPEM,
		KeyPem:   params.KeyPEM,
		ChainPem: params.ChainPEM,
	})
}

// --------------------------------------------------------------------------
// Backup activities
// --------------------------------------------------------------------------

// CreateWebBackup creates a tar.gz backup of a webroot's storage directory.
func (a *NodeLocal) CreateWebBackup(ctx context.Context, params CreateWebBackupParams) (*BackupResult, error) {
	a.logger.Info().Str("tenant", params.TenantName).Str("webroot", params.WebrootName).Str("path", params.BackupPath).Msg("CreateWebBackup")

	sourceDir := fmt.Sprintf("/var/www/storage/%s/%s", params.TenantName, params.WebrootName)

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

	targetDir := fmt.Sprintf("/var/www/storage/%s/%s", params.TenantName, params.WebrootName)

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
