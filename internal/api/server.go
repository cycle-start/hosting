package api

import (
	"context"
	_ "embed"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	temporalclient "go.temporal.io/sdk/client"

	"github.com/edvin/hosting/internal/api/handler"
	mw "github.com/edvin/hosting/internal/api/middleware"
	"github.com/edvin/hosting/internal/config"
	"github.com/edvin/hosting/internal/core"
)

//go:embed docs/swagger.json
var swaggerJSON []byte

type Server struct {
	router         chi.Router
	logger         zerolog.Logger
	services       *core.Services
	corePool       *pgxpool.Pool
	temporalClient temporalclient.Client
	cfg            *config.Config
	auditLogger    *mw.AuditLogger
}

func NewServer(logger zerolog.Logger, coreDB *pgxpool.Pool, temporalClient temporalclient.Client, cfg *config.Config) *Server {
	services := core.NewServices(coreDB, temporalClient)
	auditLogger := mw.NewAuditLogger(coreDB, logger)

	s := &Server{
		router:         chi.NewRouter(),
		logger:         logger,
		services:       services,
		corePool:       coreDB,
		temporalClient: temporalClient,
		cfg:            cfg,
		auditLogger:    auditLogger,
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

func (s *Server) setupMiddleware() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(mw.RequestLogger(s.logger))
	s.router.Use(middleware.Recoverer)
	s.router.Use(mw.Metrics)
}

func (s *Server) setupRoutes() {
	// Prometheus metrics endpoint
	s.router.Handle("/metrics", promhttp.Handler())

	// Health check endpoints
	s.router.Get("/healthz", s.handleHealthz)
	s.router.Get("/readyz", s.handleReadyz)

	// API documentation (no auth required)
	s.router.Get("/docs/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(swaggerJSON)
	})
	s.router.Get("/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(scalarHTML))
	})

	s.router.Route("/api/v1", func(r chi.Router) {
		r.Use(mw.Auth(s.corePool))
		r.Use(s.auditLogger.Middleware)

		// Dashboard
		dashboard := handler.NewDashboard(s.services.Dashboard)
		r.Get("/dashboard/stats", dashboard.Stats)

		// Audit logs
		audit := handler.NewAudit(s.corePool)
		r.Get("/audit-logs", audit.List)

		// Platform config
		platformCfg := handler.NewPlatformConfig(s.services.PlatformConfig)
		r.Get("/platform/config", platformCfg.Get)
		r.Put("/platform/config", platformCfg.Update)

		// Regions
		region := handler.NewRegion(s.services.Region)
		r.Get("/regions", region.List)
		r.Post("/regions", region.Create)
		r.Get("/regions/{id}", region.Get)
		r.Put("/regions/{id}", region.Update)
		r.Delete("/regions/{id}", region.Delete)

		// Region runtimes
		r.Get("/regions/{id}/runtimes", region.ListRuntimes)
		r.Post("/regions/{id}/runtimes", region.AddRuntime)
		r.Delete("/regions/{id}/runtimes", region.RemoveRuntime)

		// Clusters
		cluster := handler.NewCluster(s.services.Cluster)
		r.Get("/regions/{regionID}/clusters", cluster.ListByRegion)
		r.Post("/regions/{regionID}/clusters", cluster.Create)
		r.Get("/clusters/{id}", cluster.Get)
		r.Put("/clusters/{id}", cluster.Update)
		r.Delete("/clusters/{id}", cluster.Delete)

		// Cluster LB addresses
		clusterLBAddress := handler.NewClusterLBAddressHandler(s.services.ClusterLBAddress)
		r.Get("/clusters/{clusterID}/lb-addresses", clusterLBAddress.List)
		r.Post("/clusters/{clusterID}/lb-addresses", clusterLBAddress.Create)
		r.Get("/lb-addresses/{id}", clusterLBAddress.Get)
		r.Delete("/lb-addresses/{id}", clusterLBAddress.Delete)

		// Shards
		shard := handler.NewShard(s.services.Shard)
		r.Get("/clusters/{clusterID}/shards", shard.ListByCluster)
		r.Post("/clusters/{clusterID}/shards", shard.Create)
		r.Get("/shards/{id}", shard.Get)
		r.Put("/shards/{id}", shard.Update)
		r.Delete("/shards/{id}", shard.Delete)
		r.Post("/shards/{id}/converge", shard.Converge)

		// Nodes
		node := handler.NewNode(s.services.Node)
		r.Get("/clusters/{clusterID}/nodes", node.ListByCluster)
		r.Post("/clusters/{clusterID}/nodes", node.Create)
		r.Get("/nodes/{id}", node.Get)
		r.Put("/nodes/{id}", node.Update)
		r.Delete("/nodes/{id}", node.Delete)

		// Tenants
		tenant := handler.NewTenant(s.services.Tenant)
		r.Get("/tenants", tenant.List)
		r.Post("/tenants", tenant.Create)
		r.Get("/tenants/{id}", tenant.Get)
		r.Put("/tenants/{id}", tenant.Update)
		r.Delete("/tenants/{id}", tenant.Delete)
		r.Post("/tenants/{id}/suspend", tenant.Suspend)
		r.Post("/tenants/{id}/unsuspend", tenant.Unsuspend)
		r.Post("/tenants/{id}/migrate", tenant.Migrate)
		r.Get("/tenants/{id}/resource-summary", tenant.ResourceSummary)

		// Webroots
		webroot := handler.NewWebroot(s.services.Webroot)
		r.Get("/tenants/{tenantID}/webroots", webroot.ListByTenant)
		r.Post("/tenants/{tenantID}/webroots", webroot.Create)
		r.Get("/webroots/{id}", webroot.Get)
		r.Put("/webroots/{id}", webroot.Update)
		r.Delete("/webroots/{id}", webroot.Delete)

		// FQDNs
		fqdn := handler.NewFQDN(s.services.FQDN)
		r.Get("/webroots/{webrootID}/fqdns", fqdn.ListByWebroot)
		r.Post("/webroots/{webrootID}/fqdns", fqdn.Create)
		r.Get("/fqdns/{id}", fqdn.Get)
		r.Delete("/fqdns/{id}", fqdn.Delete)

		// Certificates
		cert := handler.NewCertificate(s.services.Certificate)
		r.Get("/fqdns/{fqdnID}/certificates", cert.ListByFQDN)
		r.Post("/fqdns/{fqdnID}/certificates", cert.Upload)

		// Zones
		zone := handler.NewZone(s.services.Zone)
		r.Get("/zones", zone.List)
		r.Post("/zones", zone.Create)
		r.Get("/zones/{id}", zone.Get)
		r.Put("/zones/{id}", zone.Update)
		r.Delete("/zones/{id}", zone.Delete)
		r.Put("/zones/{id}/tenant", zone.ReassignTenant)

		// Zone records
		zoneRecord := handler.NewZoneRecord(s.services.ZoneRecord)
		r.Get("/zones/{zoneID}/records", zoneRecord.ListByZone)
		r.Post("/zones/{zoneID}/records", zoneRecord.Create)
		r.Get("/zone-records/{id}", zoneRecord.Get)
		r.Put("/zone-records/{id}", zoneRecord.Update)
		r.Delete("/zone-records/{id}", zoneRecord.Delete)

		// Databases
		database := handler.NewDatabase(s.services.Database)
		r.Get("/tenants/{tenantID}/databases", database.ListByTenant)
		r.Post("/tenants/{tenantID}/databases", database.Create)
		r.Get("/databases/{id}", database.Get)
		r.Delete("/databases/{id}", database.Delete)
		r.Post("/databases/{id}/migrate", database.Migrate)
		r.Put("/databases/{id}/tenant", database.ReassignTenant)

		// Database users
		dbUser := handler.NewDatabaseUser(s.services.DatabaseUser)
		r.Get("/databases/{databaseID}/users", dbUser.ListByDatabase)
		r.Post("/databases/{databaseID}/users", dbUser.Create)
		r.Get("/database-users/{id}", dbUser.Get)
		r.Put("/database-users/{id}", dbUser.Update)
		r.Delete("/database-users/{id}", dbUser.Delete)

		// Valkey instances
		valkeyInstance := handler.NewValkeyInstance(s.services.ValkeyInstance)
		r.Get("/tenants/{tenantID}/valkey-instances", valkeyInstance.ListByTenant)
		r.Post("/tenants/{tenantID}/valkey-instances", valkeyInstance.Create)
		r.Get("/valkey-instances/{id}", valkeyInstance.Get)
		r.Delete("/valkey-instances/{id}", valkeyInstance.Delete)
		r.Post("/valkey-instances/{id}/migrate", valkeyInstance.Migrate)
		r.Put("/valkey-instances/{id}/tenant", valkeyInstance.ReassignTenant)

		// Valkey users
		valkeyUser := handler.NewValkeyUser(s.services.ValkeyUser)
		r.Get("/valkey-instances/{instanceID}/users", valkeyUser.ListByInstance)
		r.Post("/valkey-instances/{instanceID}/users", valkeyUser.Create)
		r.Get("/valkey-users/{id}", valkeyUser.Get)
		r.Put("/valkey-users/{id}", valkeyUser.Update)
		r.Delete("/valkey-users/{id}", valkeyUser.Delete)

		// SFTP keys
		sftpKey := handler.NewSFTPKey(s.services.SFTPKey)
		r.Get("/tenants/{tenantID}/sftp-keys", sftpKey.ListByTenant)
		r.Post("/tenants/{tenantID}/sftp-keys", sftpKey.Create)
		r.Get("/sftp-keys/{id}", sftpKey.Get)
		r.Delete("/sftp-keys/{id}", sftpKey.Delete)

		// Email accounts
		emailAccount := handler.NewEmailAccount(s.services.EmailAccount)
		r.Get("/fqdns/{fqdnID}/email-accounts", emailAccount.ListByFQDN)
		r.Post("/fqdns/{fqdnID}/email-accounts", emailAccount.Create)
		r.Get("/email-accounts/{id}", emailAccount.Get)
		r.Delete("/email-accounts/{id}", emailAccount.Delete)

		// Email aliases
		emailAlias := handler.NewEmailAlias(s.services.EmailAlias)
		r.Get("/email-accounts/{id}/aliases", emailAlias.ListByAccount)
		r.Post("/email-accounts/{id}/aliases", emailAlias.Create)
		r.Get("/email-aliases/{aliasID}", emailAlias.Get)
		r.Delete("/email-aliases/{aliasID}", emailAlias.Delete)

		// Email forwards
		emailForward := handler.NewEmailForward(s.services.EmailForward)
		r.Get("/email-accounts/{id}/forwards", emailForward.ListByAccount)
		r.Post("/email-accounts/{id}/forwards", emailForward.Create)
		r.Get("/email-forwards/{forwardID}", emailForward.Get)
		r.Delete("/email-forwards/{forwardID}", emailForward.Delete)

		// Email auto-reply
		emailAutoReply := handler.NewEmailAutoReply(s.services.EmailAutoReply)
		r.Get("/email-accounts/{id}/autoreply", emailAutoReply.Get)
		r.Put("/email-accounts/{id}/autoreply", emailAutoReply.Put)
		r.Delete("/email-accounts/{id}/autoreply", emailAutoReply.Delete)

		// Backups
		backup := handler.NewBackup(s.services.Backup, s.services.Webroot, s.services.Database)
		r.Get("/tenants/{tenantID}/backups", backup.ListByTenant)
		r.Post("/tenants/{tenantID}/backups", backup.Create)
		r.Get("/backups/{id}", backup.Get)
		r.Delete("/backups/{id}", backup.Delete)
		r.Post("/backups/{id}/restore", backup.Restore)

		// API keys
		apiKey := handler.NewAPIKey(s.services.APIKey)
		r.Get("/api-keys", apiKey.List)
		r.Post("/api-keys", apiKey.Create)
		r.Get("/api-keys/{id}", apiKey.Get)
		r.Delete("/api-keys/{id}", apiKey.Revoke)
	})
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	checks := map[string]string{}
	healthy := true

	if err := s.corePool.Ping(ctx); err != nil {
		checks["core_db"] = err.Error()
		healthy = false
	} else {
		checks["core_db"] = "ok"
	}

	if _, err := s.temporalClient.CheckHealth(ctx, &temporalclient.CheckHealthRequest{}); err != nil {
		checks["temporal"] = err.Error()
		healthy = false
	} else {
		checks["temporal"] = "ok"
	}

	w.Header().Set("Content-Type", "application/json")
	if healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	json.NewEncoder(w).Encode(checks)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

const scalarHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Hosting Platform API</title>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
</head>
<body>
  <script id="api-reference" data-url="/docs/openapi.json"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`
