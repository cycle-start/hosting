package activity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rs/zerolog"
	"go.temporal.io/sdk/temporal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/edvin/hosting/internal/agent"
	"github.com/edvin/hosting/internal/agent/runtime"
)

// isWorkerRuntime returns true if the runtime type has no HTTP interface.
func isWorkerRuntime(rt string) bool {
	return rt == "php-worker"
}

// grpcStatusError is the interface implemented by gRPC status errors.
// Used with errors.As to extract gRPC status from wrapped errors.
type grpcStatusError interface {
	GRPCStatus() *status.Status
}

// asNonRetryable checks whether err (or any error in its chain) is a gRPC
// status error with codes.InvalidArgument. If so it wraps the error as a
// Temporal non-retryable application error so that the activity is not
// retried — validation failures are deterministic and will never succeed
// on retry. All other errors are returned unchanged.
func asNonRetryable(err error) error {
	if err == nil {
		return nil
	}
	var se grpcStatusError
	if errors.As(err, &se) && se.GRPCStatus().Code() == codes.InvalidArgument {
		return temporal.NewNonRetryableApplicationError(
			se.GRPCStatus().Message(),
			"InvalidArgument",
			err,
		)
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
	s3 *agent.S3Manager,
	ssh *agent.SSHManager,
	cron *agent.CronManager,
	runtimes map[string]runtime.Manager,
) *NodeLocal {
	return &NodeLocal{
		logger:   logger.With().Str("component", "node-local-activity").Logger(),
		tenant:   tenant,
		webroot:  webroot,
		nginx:    nginx,
		database: database,
		valkey:   valkey,
		s3:       s3,
		ssh:      ssh,
		cron:     cron,
		runtimes: runtimes,
	}
}

// --------------------------------------------------------------------------
// Tenant activities
// --------------------------------------------------------------------------

// CreateTenant creates a tenant locally on this node.
func (a *NodeLocal) CreateTenant(ctx context.Context, params CreateTenantParams) error {
	a.logger.Info().Str("tenant", params.ID).Msg("CreateTenant")
	return asNonRetryable(a.tenant.Create(ctx, &agent.TenantInfo{
		ID:          params.ID,
		Name:        params.ID,
		UID:         int32(params.UID),
		SFTPEnabled: params.SFTPEnabled,
		SSHEnabled:  params.SSHEnabled,
	}))
}

// UpdateTenant updates a tenant locally on this node.
func (a *NodeLocal) UpdateTenant(ctx context.Context, params UpdateTenantParams) error {
	a.logger.Info().Str("tenant", params.ID).Msg("UpdateTenant")
	return asNonRetryable(a.tenant.Update(ctx, &agent.TenantInfo{
		ID:          params.ID,
		Name:        params.ID,
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

	// Skip nginx config for worker runtimes (no HTTP traffic).
	if isWorkerRuntime(info.Runtime) {
		return nil
	}

	// Generate and write nginx config.
	nginxConfig, err := a.nginx.GenerateConfig(info, fqdns)
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

	// Skip nginx config for worker runtimes (no HTTP traffic).
	if isWorkerRuntime(info.Runtime) {
		return nil
	}

	// Regenerate and write nginx config.
	nginxConfig, err := a.nginx.GenerateConfig(info, fqdns)
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

	// Remove nginx config (worker webroots have no nginx config, so tolerate missing files).
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

// ReloadNginx tests and reloads the nginx configuration.
func (a *NodeLocal) ReloadNginx(ctx context.Context) error {
	a.logger.Info().Msg("ReloadNginx")
	return asNonRetryable(a.nginx.Reload(ctx))
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
	a.logger.Info().Str("cron_job", params.ID).Str("tenant", params.TenantID).Msg("CreateCronJobUnits")
	return a.cron.CreateUnits(ctx, &agent.CronJobInfo{
		ID:               params.ID,
		TenantID:         params.TenantID,
		WebrootName:      params.WebrootName,
		Name:             params.Name,
		Schedule:         params.Schedule,
		Command:          params.Command,
		WorkingDirectory: params.WorkingDirectory,
		TimeoutSeconds:   params.TimeoutSeconds,
		MaxMemoryMB:      params.MaxMemoryMB,
	})
}

// UpdateCronJobUnits rewrites systemd units for a cron job on this node.
func (a *NodeLocal) UpdateCronJobUnits(ctx context.Context, params UpdateCronJobParams) error {
	a.logger.Info().Str("cron_job", params.ID).Str("tenant", params.TenantID).Msg("UpdateCronJobUnits")
	return a.cron.UpdateUnits(ctx, &agent.CronJobInfo{
		ID:               params.ID,
		TenantID:         params.TenantID,
		WebrootName:      params.WebrootName,
		Name:             params.Name,
		Schedule:         params.Schedule,
		Command:          params.Command,
		WorkingDirectory: params.WorkingDirectory,
		TimeoutSeconds:   params.TimeoutSeconds,
		MaxMemoryMB:      params.MaxMemoryMB,
	})
}

// DeleteCronJobUnits stops, disables, and removes systemd units on this node.
func (a *NodeLocal) DeleteCronJobUnits(ctx context.Context, params DeleteCronJobParams) error {
	a.logger.Info().Str("cron_job", params.ID).Str("tenant", params.TenantID).Msg("DeleteCronJobUnits")
	return a.cron.DeleteUnits(ctx, &agent.CronJobInfo{
		ID:       params.ID,
		TenantID: params.TenantID,
	})
}

// EnableCronJobTimer starts the systemd timer on this node.
func (a *NodeLocal) EnableCronJobTimer(ctx context.Context, params CronJobTimerParams) error {
	a.logger.Info().Str("cron_job", params.ID).Str("tenant", params.TenantID).Msg("EnableCronJobTimer")
	return a.cron.EnableTimer(ctx, &agent.CronJobInfo{
		ID:       params.ID,
		TenantID: params.TenantID,
	})
}

// DisableCronJobTimer stops the systemd timer on this node.
func (a *NodeLocal) DisableCronJobTimer(ctx context.Context, params CronJobTimerParams) error {
	a.logger.Info().Str("cron_job", params.ID).Str("tenant", params.TenantID).Msg("DisableCronJobTimer")
	return a.cron.DisableTimer(ctx, &agent.CronJobInfo{
		ID:       params.ID,
		TenantID: params.TenantID,
	})
}
