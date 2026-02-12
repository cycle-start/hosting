package activity

import (
	"context"
	"fmt"

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
