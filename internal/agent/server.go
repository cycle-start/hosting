package agent

// Reconciliation model:
// 1. At startup, the agent queries core DB for its own node record to determine its shard_id
// 2. It then queries all tenants assigned to that shard
// 3. For each tenant, it converges local state (nginx configs, PHP-FPM pools, etc.)
// 4. It periodically re-queries to pick up new assignments or removals
// 5. Within a shard, all nodes converge to the same state for resilience

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/edvin/hosting/internal/agent/runtime"
	agentv1 "github.com/edvin/hosting/proto/agent/v1"
)

// Config holds the configuration for the node agent server.
type Config struct {
	MySQLDSN       string
	NginxConfigDir string
	WebStorageDir  string
	HomeBaseDir    string
	CertDir        string
	ValkeyConfigDir string
	ValkeyDataDir   string
	// InitSystem selects the service manager implementation.
	// "systemd" for production (VMs/bare metal), "direct" for Docker dev.
	InitSystem string
	// ShardName is the name of the shard this node belongs to.
	// Used in nginx X-Shard response headers for debugging.
	ShardName string
}

// Server implements the agentv1.NodeAgentServer gRPC service.
// It delegates operations to specialized managers for each subsystem.
type Server struct {
	agentv1.UnimplementedNodeAgentServer
	logger   zerolog.Logger
	tenant   *TenantManager
	webroot  *WebrootManager
	nginx    *NginxManager
	database *DatabaseManager
	valkey   *ValkeyManager
	runtimes map[string]runtime.Manager

	// ShardID is the shard this node belongs to, resolved at startup from the core DB.
	ShardID *uuid.UUID
}

// NewServer creates a new gRPC node agent server.
func NewServer(logger zerolog.Logger, cfg Config) *Server {
	var svcMgr runtime.ServiceManager
	switch cfg.InitSystem {
	case "systemd":
		svcMgr = runtime.NewSystemdManager(logger)
	default:
		svcMgr = runtime.NewDirectManager(logger)
	}

	runtimes := map[string]runtime.Manager{
		"php":    runtime.NewPHP(logger, svcMgr),
		"node":   runtime.NewNode(logger, svcMgr),
		"python": runtime.NewPython(logger, svcMgr),
		"ruby":   runtime.NewRuby(logger, svcMgr),
		"static": runtime.NewStatic(logger),
	}
	nginxMgr := NewNginxManager(logger, cfg)
	if cfg.ShardName != "" {
		nginxMgr.SetShardName(cfg.ShardName)
	}

	return &Server{
		logger:   logger.With().Str("component", "agent-server").Logger(),
		tenant:   NewTenantManager(logger, cfg),
		webroot:  NewWebrootManager(logger, cfg),
		nginx:    nginxMgr,
		database: NewDatabaseManager(logger, cfg),
		valkey:   NewValkeyManager(logger, cfg, svcMgr),
		runtimes: runtimes,
	}
}

// TenantManager returns the server's tenant manager.
func (s *Server) TenantManager() *TenantManager { return s.tenant }

// WebrootManager returns the server's webroot manager.
func (s *Server) WebrootManager() *WebrootManager { return s.webroot }

// NginxManager returns the server's nginx manager.
func (s *Server) NginxManager() *NginxManager { return s.nginx }

// DatabaseManager returns the server's database manager.
func (s *Server) DatabaseManager() *DatabaseManager { return s.database }

// ValkeyManager returns the server's valkey manager.
func (s *Server) ValkeyManager() *ValkeyManager { return s.valkey }

// Runtimes returns the server's runtime managers.
func (s *Server) Runtimes() map[string]runtime.Manager { return s.runtimes }

// --------------------------------------------------------------------------
// Tenant management
// --------------------------------------------------------------------------

// CreateTenant provisions a new Linux user account for a tenant.
func (s *Server) CreateTenant(ctx context.Context, req *agentv1.CreateTenantRequest) (*agentv1.CreateTenantResponse, error) {
	info := req.GetTenant()
	s.logger.Info().
		Str("tenant", info.GetName()).
		Int32("uid", info.GetUid()).
		Msg("CreateTenant RPC")

	if err := s.tenant.Create(ctx, info); err != nil {
		s.logger.Error().Err(err).Str("tenant", info.GetName()).Msg("CreateTenant failed")
		return nil, err
	}

	return &agentv1.CreateTenantResponse{}, nil
}

// UpdateTenant modifies an existing tenant user configuration.
func (s *Server) UpdateTenant(ctx context.Context, req *agentv1.UpdateTenantRequest) (*agentv1.UpdateTenantResponse, error) {
	info := req.GetTenant()
	s.logger.Info().
		Str("tenant", info.GetName()).
		Bool("sftp_enabled", info.GetSftpEnabled()).
		Msg("UpdateTenant RPC")

	if err := s.tenant.Update(ctx, info); err != nil {
		s.logger.Error().Err(err).Str("tenant", info.GetName()).Msg("UpdateTenant failed")
		return nil, err
	}

	return &agentv1.UpdateTenantResponse{}, nil
}

// SuspendTenant locks a tenant user account.
func (s *Server) SuspendTenant(ctx context.Context, req *agentv1.SuspendTenantRequest) (*agentv1.SuspendTenantResponse, error) {
	name := req.GetTenantName()
	s.logger.Info().Str("tenant", name).Msg("SuspendTenant RPC")

	if err := s.tenant.Suspend(ctx, name); err != nil {
		s.logger.Error().Err(err).Str("tenant", name).Msg("SuspendTenant failed")
		return nil, err
	}

	return &agentv1.SuspendTenantResponse{}, nil
}

// UnsuspendTenant unlocks a tenant user account.
func (s *Server) UnsuspendTenant(ctx context.Context, req *agentv1.UnsuspendTenantRequest) (*agentv1.UnsuspendTenantResponse, error) {
	name := req.GetTenantName()
	s.logger.Info().Str("tenant", name).Msg("UnsuspendTenant RPC")

	if err := s.tenant.Unsuspend(ctx, name); err != nil {
		s.logger.Error().Err(err).Str("tenant", name).Msg("UnsuspendTenant failed")
		return nil, err
	}

	return &agentv1.UnsuspendTenantResponse{}, nil
}

// DeleteTenant removes a tenant user account and its home directory.
func (s *Server) DeleteTenant(ctx context.Context, req *agentv1.DeleteTenantRequest) (*agentv1.DeleteTenantResponse, error) {
	name := req.GetTenantName()
	s.logger.Info().Str("tenant", name).Msg("DeleteTenant RPC")

	if err := s.tenant.Delete(ctx, name); err != nil {
		s.logger.Error().Err(err).Str("tenant", name).Msg("DeleteTenant failed")
		return nil, err
	}

	return &agentv1.DeleteTenantResponse{}, nil
}

// --------------------------------------------------------------------------
// Webroot management
// --------------------------------------------------------------------------

// CreateWebroot provisions a webroot directory, nginx config, and runtime.
func (s *Server) CreateWebroot(ctx context.Context, req *agentv1.CreateWebrootRequest) (*agentv1.CreateWebrootResponse, error) {
	info := req.GetWebroot()
	fqdns := req.GetFqdns()
	s.logger.Info().
		Str("tenant", info.GetTenantName()).
		Str("webroot", info.GetName()).
		Str("runtime", info.GetRuntime()).
		Msg("CreateWebroot RPC")

	// Create the webroot directories and symlink.
	if err := s.webroot.Create(ctx, info); err != nil {
		s.logger.Error().Err(err).Str("webroot", info.GetName()).Msg("CreateWebroot: webroot creation failed")
		return nil, err
	}

	// Configure the runtime for this webroot.
	rt, ok := s.runtimes[info.GetRuntime()]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported runtime: %s", info.GetRuntime())
	}
	if err := rt.Configure(ctx, info); err != nil {
		s.logger.Error().Err(err).Str("webroot", info.GetName()).Msg("CreateWebroot: runtime configure failed")
		return nil, status.Errorf(codes.Internal, "configure runtime: %v", err)
	}
	if err := rt.Start(ctx, info); err != nil {
		s.logger.Error().Err(err).Str("webroot", info.GetName()).Msg("CreateWebroot: runtime start failed")
		return nil, status.Errorf(codes.Internal, "start runtime: %v", err)
	}

	// Generate and write nginx configuration.
	nginxConfig, err := s.nginx.GenerateConfig(info, fqdns)
	if err != nil {
		s.logger.Error().Err(err).Str("webroot", info.GetName()).Msg("CreateWebroot: nginx config generation failed")
		return nil, status.Errorf(codes.Internal, "generate nginx config: %v", err)
	}
	if err := s.nginx.WriteConfig(info.GetTenantName(), info.GetName(), nginxConfig); err != nil {
		s.logger.Error().Err(err).Str("webroot", info.GetName()).Msg("CreateWebroot: nginx config write failed")
		return nil, err
	}

	// Reload nginx to apply the new configuration.
	if err := s.nginx.Reload(ctx); err != nil {
		s.logger.Error().Err(err).Str("webroot", info.GetName()).Msg("CreateWebroot: nginx reload failed")
		return nil, err
	}

	return &agentv1.CreateWebrootResponse{}, nil
}

// UpdateWebroot updates a webroot's directories, runtime, and nginx configuration.
func (s *Server) UpdateWebroot(ctx context.Context, req *agentv1.UpdateWebrootRequest) (*agentv1.UpdateWebrootResponse, error) {
	info := req.GetWebroot()
	fqdns := req.GetFqdns()
	s.logger.Info().
		Str("tenant", info.GetTenantName()).
		Str("webroot", info.GetName()).
		Msg("UpdateWebroot RPC")

	// Update the webroot directories.
	if err := s.webroot.Update(ctx, info); err != nil {
		s.logger.Error().Err(err).Str("webroot", info.GetName()).Msg("UpdateWebroot: webroot update failed")
		return nil, err
	}

	// Reconfigure the runtime.
	rt, ok := s.runtimes[info.GetRuntime()]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported runtime: %s", info.GetRuntime())
	}
	if err := rt.Configure(ctx, info); err != nil {
		s.logger.Error().Err(err).Str("webroot", info.GetName()).Msg("UpdateWebroot: runtime configure failed")
		return nil, status.Errorf(codes.Internal, "configure runtime: %v", err)
	}
	if err := rt.Reload(ctx, info); err != nil {
		s.logger.Error().Err(err).Str("webroot", info.GetName()).Msg("UpdateWebroot: runtime reload failed")
		return nil, status.Errorf(codes.Internal, "reload runtime: %v", err)
	}

	// Regenerate and write nginx configuration.
	nginxConfig, err := s.nginx.GenerateConfig(info, fqdns)
	if err != nil {
		s.logger.Error().Err(err).Str("webroot", info.GetName()).Msg("UpdateWebroot: nginx config generation failed")
		return nil, status.Errorf(codes.Internal, "generate nginx config: %v", err)
	}
	if err := s.nginx.WriteConfig(info.GetTenantName(), info.GetName(), nginxConfig); err != nil {
		s.logger.Error().Err(err).Str("webroot", info.GetName()).Msg("UpdateWebroot: nginx config write failed")
		return nil, err
	}

	// Reload nginx.
	if err := s.nginx.Reload(ctx); err != nil {
		s.logger.Error().Err(err).Str("webroot", info.GetName()).Msg("UpdateWebroot: nginx reload failed")
		return nil, err
	}

	return &agentv1.UpdateWebrootResponse{}, nil
}

// DeleteWebroot removes a webroot's runtime, nginx config, and directories.
func (s *Server) DeleteWebroot(ctx context.Context, req *agentv1.DeleteWebrootRequest) (*agentv1.DeleteWebrootResponse, error) {
	tenantName := req.GetTenantName()
	webrootName := req.GetWebrootName()
	s.logger.Info().
		Str("tenant", tenantName).
		Str("webroot", webrootName).
		Msg("DeleteWebroot RPC")

	// Remove nginx config first to stop serving traffic.
	if err := s.nginx.RemoveConfig(tenantName, webrootName); err != nil {
		s.logger.Error().Err(err).Msg("DeleteWebroot: nginx config removal failed")
		return nil, err
	}

	// Reload nginx to stop serving the removed site.
	if err := s.nginx.Reload(ctx); err != nil {
		s.logger.Error().Err(err).Msg("DeleteWebroot: nginx reload failed")
		return nil, err
	}

	// Stop and remove runtime for all possible runtime types. We attempt
	// removal for each runtime and ignore errors since only one will match.
	for name, rt := range s.runtimes {
		// Build a minimal WebrootInfo for the runtime manager.
		wrInfo := &agentv1.WebrootInfo{
			TenantName: tenantName,
			Name:       webrootName,
		}
		if err := rt.Remove(ctx, wrInfo); err != nil {
			s.logger.Debug().Err(err).Str("runtime", name).Msg("DeleteWebroot: runtime removal (may be expected)")
		}
	}

	// Remove the webroot directories and symlink.
	if err := s.webroot.Delete(ctx, tenantName, webrootName); err != nil {
		s.logger.Error().Err(err).Msg("DeleteWebroot: webroot deletion failed")
		return nil, err
	}

	return &agentv1.DeleteWebrootResponse{}, nil
}

// --------------------------------------------------------------------------
// Runtime management
// --------------------------------------------------------------------------

// ConfigureRuntime configures and starts the runtime for a webroot.
func (s *Server) ConfigureRuntime(ctx context.Context, req *agentv1.ConfigureRuntimeRequest) (*agentv1.ConfigureRuntimeResponse, error) {
	info := req.GetWebroot()
	s.logger.Info().
		Str("tenant", info.GetTenantName()).
		Str("webroot", info.GetName()).
		Str("runtime", info.GetRuntime()).
		Msg("ConfigureRuntime RPC")

	rt, ok := s.runtimes[info.GetRuntime()]
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported runtime: %s", info.GetRuntime())
	}

	if err := rt.Configure(ctx, info); err != nil {
		s.logger.Error().Err(err).Str("runtime", info.GetRuntime()).Msg("ConfigureRuntime failed")
		return nil, status.Errorf(codes.Internal, "configure runtime: %v", err)
	}

	if err := rt.Start(ctx, info); err != nil {
		s.logger.Error().Err(err).Str("runtime", info.GetRuntime()).Msg("ConfigureRuntime: start failed")
		return nil, status.Errorf(codes.Internal, "start runtime: %v", err)
	}

	return &agentv1.ConfigureRuntimeResponse{}, nil
}

// ReloadNginx tests and reloads the nginx configuration.
func (s *Server) ReloadNginx(ctx context.Context, _ *agentv1.ReloadNginxRequest) (*agentv1.ReloadNginxResponse, error) {
	s.logger.Info().Msg("ReloadNginx RPC")

	if err := s.nginx.Reload(ctx); err != nil {
		s.logger.Error().Err(err).Msg("ReloadNginx failed")
		return nil, err
	}

	return &agentv1.ReloadNginxResponse{}, nil
}

// --------------------------------------------------------------------------
// Database management
// --------------------------------------------------------------------------

// CreateDatabase creates a new MySQL database.
func (s *Server) CreateDatabase(ctx context.Context, req *agentv1.CreateDatabaseRequest) (*agentv1.CreateDatabaseResponse, error) {
	info := req.GetDatabase()
	s.logger.Info().Str("database", info.GetName()).Msg("CreateDatabase RPC")

	if err := s.database.CreateDatabase(ctx, info.GetName()); err != nil {
		s.logger.Error().Err(err).Str("database", info.GetName()).Msg("CreateDatabase failed")
		return nil, err
	}

	return &agentv1.CreateDatabaseResponse{}, nil
}

// DeleteDatabase drops a MySQL database.
func (s *Server) DeleteDatabase(ctx context.Context, req *agentv1.DeleteDatabaseRequest) (*agentv1.DeleteDatabaseResponse, error) {
	name := req.GetDatabaseName()
	s.logger.Info().Str("database", name).Msg("DeleteDatabase RPC")

	if err := s.database.DeleteDatabase(ctx, name); err != nil {
		s.logger.Error().Err(err).Str("database", name).Msg("DeleteDatabase failed")
		return nil, err
	}

	return &agentv1.DeleteDatabaseResponse{}, nil
}

// CreateDatabaseUser creates a MySQL user and grants privileges.
func (s *Server) CreateDatabaseUser(ctx context.Context, req *agentv1.CreateDatabaseUserRequest) (*agentv1.CreateDatabaseUserResponse, error) {
	user := req.GetUser()
	s.logger.Info().
		Str("database", user.GetDatabaseName()).
		Str("username", user.GetUsername()).
		Msg("CreateDatabaseUser RPC")

	if err := s.database.CreateUser(ctx, user.GetDatabaseName(), user.GetUsername(), user.GetPassword(), user.GetPrivileges()); err != nil {
		s.logger.Error().Err(err).Str("username", user.GetUsername()).Msg("CreateDatabaseUser failed")
		return nil, err
	}

	return &agentv1.CreateDatabaseUserResponse{}, nil
}

// UpdateDatabaseUser modifies a MySQL user's password and/or privileges.
func (s *Server) UpdateDatabaseUser(ctx context.Context, req *agentv1.UpdateDatabaseUserRequest) (*agentv1.UpdateDatabaseUserResponse, error) {
	user := req.GetUser()
	s.logger.Info().
		Str("database", user.GetDatabaseName()).
		Str("username", user.GetUsername()).
		Msg("UpdateDatabaseUser RPC")

	if err := s.database.UpdateUser(ctx, user.GetDatabaseName(), user.GetUsername(), user.GetPassword(), user.GetPrivileges()); err != nil {
		s.logger.Error().Err(err).Str("username", user.GetUsername()).Msg("UpdateDatabaseUser failed")
		return nil, err
	}

	return &agentv1.UpdateDatabaseUserResponse{}, nil
}

// DeleteDatabaseUser drops a MySQL user.
func (s *Server) DeleteDatabaseUser(ctx context.Context, req *agentv1.DeleteDatabaseUserRequest) (*agentv1.DeleteDatabaseUserResponse, error) {
	dbName := req.GetDatabaseName()
	username := req.GetUsername()
	s.logger.Info().
		Str("database", dbName).
		Str("username", username).
		Msg("DeleteDatabaseUser RPC")

	if err := s.database.DeleteUser(ctx, dbName, username); err != nil {
		s.logger.Error().Err(err).Str("username", username).Msg("DeleteDatabaseUser failed")
		return nil, err
	}

	return &agentv1.DeleteDatabaseUserResponse{}, nil
}

// --------------------------------------------------------------------------
// Valkey management
// --------------------------------------------------------------------------

// CreateValkeyInstance provisions a new Valkey instance.
func (s *Server) CreateValkeyInstance(ctx context.Context, req *agentv1.CreateValkeyInstanceRequest) (*agentv1.CreateValkeyInstanceResponse, error) {
	info := req.GetInstance()
	s.logger.Info().
		Str("instance", info.GetName()).
		Int32("port", info.GetPort()).
		Msg("CreateValkeyInstance RPC")

	if err := s.valkey.CreateInstance(ctx, info.GetName(), int(info.GetPort()), info.GetPassword(), int(info.GetMaxMemoryMb())); err != nil {
		s.logger.Error().Err(err).Str("instance", info.GetName()).Msg("CreateValkeyInstance failed")
		return nil, err
	}

	return &agentv1.CreateValkeyInstanceResponse{}, nil
}

// DeleteValkeyInstance removes a Valkey instance.
func (s *Server) DeleteValkeyInstance(ctx context.Context, req *agentv1.DeleteValkeyInstanceRequest) (*agentv1.DeleteValkeyInstanceResponse, error) {
	name := req.GetInstanceName()
	port := int(req.GetPort())
	s.logger.Info().Str("instance", name).Int("port", port).Msg("DeleteValkeyInstance RPC")

	if err := s.valkey.DeleteInstance(ctx, name, port); err != nil {
		s.logger.Error().Err(err).Str("instance", name).Msg("DeleteValkeyInstance failed")
		return nil, err
	}

	return &agentv1.DeleteValkeyInstanceResponse{}, nil
}

// CreateValkeyUser creates a Valkey ACL user.
func (s *Server) CreateValkeyUser(ctx context.Context, req *agentv1.CreateValkeyUserRequest) (*agentv1.CreateValkeyUserResponse, error) {
	user := req.GetUser()
	s.logger.Info().
		Str("instance", user.GetInstanceName()).
		Str("username", user.GetUsername()).
		Msg("CreateValkeyUser RPC")

	if err := s.valkey.CreateUser(ctx, user.GetInstanceName(), int(user.GetPort()), user.GetUsername(), user.GetPassword(), user.GetPrivileges(), user.GetKeyPattern()); err != nil {
		s.logger.Error().Err(err).Str("username", user.GetUsername()).Msg("CreateValkeyUser failed")
		return nil, err
	}

	return &agentv1.CreateValkeyUserResponse{}, nil
}

// UpdateValkeyUser updates a Valkey ACL user.
func (s *Server) UpdateValkeyUser(ctx context.Context, req *agentv1.UpdateValkeyUserRequest) (*agentv1.UpdateValkeyUserResponse, error) {
	user := req.GetUser()
	s.logger.Info().
		Str("instance", user.GetInstanceName()).
		Str("username", user.GetUsername()).
		Msg("UpdateValkeyUser RPC")

	if err := s.valkey.UpdateUser(ctx, user.GetInstanceName(), int(user.GetPort()), user.GetUsername(), user.GetPassword(), user.GetPrivileges(), user.GetKeyPattern()); err != nil {
		s.logger.Error().Err(err).Str("username", user.GetUsername()).Msg("UpdateValkeyUser failed")
		return nil, err
	}

	return &agentv1.UpdateValkeyUserResponse{}, nil
}

// DeleteValkeyUser deletes a Valkey ACL user.
func (s *Server) DeleteValkeyUser(ctx context.Context, req *agentv1.DeleteValkeyUserRequest) (*agentv1.DeleteValkeyUserResponse, error) {
	name := req.GetInstanceName()
	username := req.GetUsername()
	s.logger.Info().
		Str("instance", name).
		Str("username", username).
		Msg("DeleteValkeyUser RPC")

	if err := s.valkey.DeleteUser(ctx, name, int(req.GetPort()), username); err != nil {
		s.logger.Error().Err(err).Str("username", username).Msg("DeleteValkeyUser failed")
		return nil, err
	}

	return &agentv1.DeleteValkeyUserResponse{}, nil
}

// --------------------------------------------------------------------------
// SSL
// --------------------------------------------------------------------------

// InstallCertificate writes SSL certificate files to disk.
func (s *Server) InstallCertificate(ctx context.Context, req *agentv1.InstallCertificateRequest) (*agentv1.InstallCertificateResponse, error) {
	cert := req.GetCertificate()
	s.logger.Info().Str("fqdn", cert.GetFqdn()).Msg("InstallCertificate RPC")

	if err := s.nginx.InstallCertificate(ctx, cert); err != nil {
		s.logger.Error().Err(err).Str("fqdn", cert.GetFqdn()).Msg("InstallCertificate failed")
		return nil, err
	}

	return &agentv1.InstallCertificateResponse{}, nil
}

// --------------------------------------------------------------------------
// Health
// --------------------------------------------------------------------------

// HealthCheck returns the health status of the node agent.
func (s *Server) HealthCheck(_ context.Context, _ *agentv1.HealthCheckRequest) (*agentv1.HealthCheckResponse, error) {
	s.logger.Debug().Msg("HealthCheck RPC")

	return &agentv1.HealthCheckResponse{
		Healthy: true,
		Message: "node agent is running",
	}, nil
}

// --------------------------------------------------------------------------
// Shard reconciliation
// --------------------------------------------------------------------------

// GetShardTenants returns all tenants assigned to this node's shard.
// This is a placeholder stub for shard-based reconciliation. The full
// implementation requires corresponding gRPC proto definitions for
// GetShardTenantsRequest and GetShardTenantsResponse messages. Once those
// are defined, this method will query the core DB for all tenants where
// shard_id matches the node's shard and return them for local convergence.
func (s *Server) GetShardTenants(ctx context.Context) error {
	if s.ShardID == nil {
		s.logger.Warn().Msg("GetShardTenants: shard_id not set, skipping reconciliation")
		return nil
	}
	s.logger.Info().Str("shard_id", s.ShardID.String()).Msg("GetShardTenants: stub called")
	// TODO: query core DB for tenants with shard_id = s.ShardID
	// TODO: return tenants via gRPC response once proto is defined
	return nil
}
