package agent

import (
	"github.com/rs/zerolog"

	"github.com/edvin/hosting/internal/agent/runtime"
)

// Config holds the configuration for the node agent server.
type Config struct {
	MySQLDSN        string
	NginxConfigDir  string
	NginxLogDir     string
	NginxListenPort string // Port for nginx listen directives (default "80")
	WebStorageDir   string
	CertDir         string
	ValkeyConfigDir string
	ValkeyDataDir   string
	// InitSystem selects the service manager implementation.
	// "systemd" for production (VMs/bare metal), "direct" for Docker dev.
	InitSystem string
	// ShardName is the name of the shard this node belongs to.
	// Used in nginx X-Shard response headers for debugging.
	ShardName string
}

// Server coordinates the node agent's managers.
type Server struct {
	logger   zerolog.Logger
	tenant   *TenantManager
	webroot  *WebrootManager
	nginx    *NginxManager
	database *DatabaseManager
	valkey   *ValkeyManager
	ssh      *SSHManager
	cron     *CronManager
	daemon    *DaemonManager
	tenantULA *TenantULAManager
	runtimes  map[string]runtime.Manager

	// ShardID is the shard this node belongs to, resolved at startup from the core DB.
	ShardID string
}

// NewServer creates a new node agent server.
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
		ssh:      NewSSHManager(logger, cfg.WebStorageDir),
		cron:     NewCronManager(logger, cfg),
		daemon:    NewDaemonManager(logger, cfg),
		tenantULA: NewTenantULAManager(logger),
		runtimes:  runtimes,
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

// SSHManager returns the server's SSH manager.
func (s *Server) SSHManager() *SSHManager { return s.ssh }

// CronManager returns the server's cron manager.
func (s *Server) CronManager() *CronManager { return s.cron }

// DaemonManager returns the server's daemon manager.
func (s *Server) DaemonManager() *DaemonManager { return s.daemon }

// TenantULAManager returns the server's tenant ULA manager.
func (s *Server) TenantULAManager() *TenantULAManager { return s.tenantULA }

// Runtimes returns the server's runtime managers.
func (s *Server) Runtimes() map[string]runtime.Manager { return s.runtimes }
