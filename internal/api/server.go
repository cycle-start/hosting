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
	"github.com/edvin/hosting/internal/sshca"
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
	services := core.NewServices(coreDB, temporalClient, cfg.OIDCIssuerURL, cfg.SecretEncryptionKey)
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

	// OIDC endpoints (no auth required — public)
	oidc := handler.NewOIDC(s.services.OIDC)
	s.router.Get("/.well-known/openid-configuration", oidc.Discovery)
	s.router.Get("/oidc/jwks", oidc.JWKS)
	s.router.Get("/oidc/authorize", oidc.Authorize)
	s.router.Post("/oidc/token", oidc.Token)

	// Terminal WebSocket endpoint — uses query param auth (WebSocket API can't send headers).
	// Registered outside the standard auth middleware group.
	if s.cfg.SSHCAPrivateKey != "" {
		ca, err := sshca.New([]byte(s.cfg.SSHCAPrivateKey))
		if err != nil {
			s.logger.Fatal().Err(err).Msg("failed to parse SSH CA private key")
		}
		terminal := handler.NewTerminal(ca, s.corePool)
		s.router.Get("/api/v1/tenants/{tenantID}/terminal", terminal.Connect)
	}

	s.router.Route("/api/v1", func(r chi.Router) {
		r.Use(mw.Auth(s.corePool))
		r.Use(mw.CallbackURL)
		r.Use(s.auditLogger.Middleware)

		// Initialize handlers
		dashboard := handler.NewDashboard(s.services.Dashboard)
		audit := handler.NewAudit(s.corePool)
		logs := handler.NewLogs(s.cfg.LokiURL, s.cfg.TenantLokiURL)
		platformCfg := handler.NewPlatformConfig(s.services.PlatformConfig)
		brand := handler.NewBrand(s.services.Brand)
		region := handler.NewRegion(s.services.Region)
		cluster := handler.NewCluster(s.services.Cluster)
		clusterLBAddress := handler.NewClusterLBAddressHandler(s.services.ClusterLBAddress)
		shard := handler.NewShard(s.services.Shard)
		node := handler.NewNode(s.services.Node)
		tenant := handler.NewTenant(s.services)
		oidcLogin := handler.NewOIDCLogin(s.services.OIDC)
		oidcClient := handler.NewOIDCClient(s.services.OIDC)
		webroot := handler.NewWebroot(s.services)
		fqdn := handler.NewFQDN(s.services)
		cert := handler.NewCertificate(s.services.Certificate)
		zone := handler.NewZone(s.services)
		zoneRecord := handler.NewZoneRecord(s.services.ZoneRecord)
		database := handler.NewDatabase(s.services.Database, s.services.DatabaseUser, s.services.Tenant)
		dbUser := handler.NewDatabaseUser(s.services.DatabaseUser, s.services.Database)
		valkeyInstance := handler.NewValkeyInstance(s.services.ValkeyInstance, s.services.ValkeyUser, s.services.Tenant)
		valkeyUser := handler.NewValkeyUser(s.services.ValkeyUser, s.services.ValkeyInstance)
		s3Bucket := handler.NewS3Bucket(s.services.S3Bucket, s.services.S3AccessKey, s.services.Tenant)
		s3AccessKey := handler.NewS3AccessKey(s.services.S3AccessKey)
		sshKey := handler.NewSSHKey(s.services.SSHKey, s.services.Tenant)
		egressRule := handler.NewTenantEgressRule(s.services.TenantEgressRule, s.services.Tenant)
		dbAccessRule := handler.NewDatabaseAccessRule(s.services.DatabaseAccessRule, s.services.Database)
		subscription := handler.NewSubscription(s.services)
		emailAccount := handler.NewEmailAccount(s.services)
		emailAlias := handler.NewEmailAlias(s.services.EmailAlias)
		emailForward := handler.NewEmailForward(s.services.EmailForward)
		emailAutoReply := handler.NewEmailAutoReply(s.services.EmailAutoReply)
		backup := handler.NewBackup(s.services.Backup, s.services.Webroot, s.services.Database, s.services.Tenant)
		search := handler.NewSearch(s.services.Search)
		apiKey := handler.NewAPIKey(s.services.APIKey)
		internalNode := handler.NewInternalNode(s.services.DesiredState, s.services.NodeHealth, s.services.CronJob)
		incident := handler.NewIncident(s.services.Incident)
		capabilityGap := handler.NewCapabilityGap(s.services.CapabilityGap)

		// Workflow await (admin-only, blocks until workflow completes)
		workflow := handler.NewWorkflow(s.temporalClient)
		r.Group(func(r chi.Router) {
			r.Use(mw.RequirePlatformAdmin())
			r.Get("/workflows/{workflowID}/await", workflow.Await)
		})

		// Platform-admin-only endpoints (require brands: ["*"])
		r.Group(func(r chi.Router) {
			r.Use(mw.RequirePlatformAdmin())

			// Dashboard
			r.Get("/dashboard/stats", dashboard.Stats)

			// Audit logs
			r.Get("/audit-logs", audit.List)

			// Logs (Loki proxy)
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("audit_logs", "read"))
				r.Get("/logs", logs.Query)
			})

			// Platform config
			r.Get("/platform/config", platformCfg.Get)
			r.Put("/platform/config", platformCfg.Update)

			// Search
			r.Get("/search", search.Search)

			// OIDC clients (admin)
			r.Post("/oidc/clients", oidcClient.Create)

			// API keys
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("api_keys", "read"))
				r.Get("/api-keys", apiKey.List)
				r.Get("/api-keys/{id}", apiKey.Get)
			})
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("api_keys", "write"))
				r.Post("/api-keys", apiKey.Create)
				r.Put("/api-keys/{id}", apiKey.Update)
			})
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("api_keys", "delete"))
				r.Delete("/api-keys/{id}", apiKey.Revoke)
			})

			// Regions
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("regions", "read"))
				r.Get("/regions", region.List)
				r.Get("/regions/{id}", region.Get)
				r.Get("/regions/{id}/runtimes", region.ListRuntimes)
			})
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("regions", "write"))
				r.Post("/regions", region.Create)
				r.Put("/regions/{id}", region.Update)
				r.Post("/regions/{id}/runtimes", region.AddRuntime)
			})
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("regions", "delete"))
				r.Delete("/regions/{id}", region.Delete)
				r.Delete("/regions/{id}/runtimes", region.RemoveRuntime)
			})

			// Clusters
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("clusters", "read"))
				r.Get("/regions/{regionID}/clusters", cluster.ListByRegion)
				r.Get("/clusters/{id}", cluster.Get)
			})
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("clusters", "write"))
				r.Post("/regions/{regionID}/clusters", cluster.Create)
				r.Put("/clusters/{id}", cluster.Update)
			})
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("clusters", "delete"))
				r.Delete("/clusters/{id}", cluster.Delete)
			})

			// Cluster LB addresses
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("clusters", "read"))
				r.Get("/clusters/{clusterID}/lb-addresses", clusterLBAddress.List)
				r.Get("/lb-addresses/{id}", clusterLBAddress.Get)
			})
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("clusters", "write"))
				r.Post("/clusters/{clusterID}/lb-addresses", clusterLBAddress.Create)
			})
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("clusters", "delete"))
				r.Delete("/lb-addresses/{id}", clusterLBAddress.Delete)
			})

			// Shards
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("shards", "read"))
				r.Get("/clusters/{clusterID}/shards", shard.ListByCluster)
				r.Get("/shards/{id}", shard.Get)
			})
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("shards", "write"))
				r.Post("/clusters/{clusterID}/shards", shard.Create)
				r.Put("/shards/{id}", shard.Update)
				r.Post("/shards/{id}/converge", shard.Converge)
				r.Post("/shards/{id}/retry", shard.Retry)
			})
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("shards", "delete"))
				r.Delete("/shards/{id}", shard.Delete)
			})

			// Nodes
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("nodes", "read"))
				r.Get("/clusters/{clusterID}/nodes", node.ListByCluster)
				r.Get("/nodes/{id}", node.Get)
			})
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("nodes", "write"))
				r.Post("/clusters/{clusterID}/nodes", node.Create)
				r.Put("/nodes/{id}", node.Update)
			})
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("nodes", "delete"))
				r.Delete("/nodes/{id}", node.Delete)
			})

			// Internal API (node agent, cron outcome reporting)
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("nodes", "read"))
				r.Get("/internal/v1/nodes/{nodeID}/desired-state", internalNode.GetDesiredState)
				r.Get("/internal/v1/nodes/{nodeID}/health", internalNode.GetHealth)
				r.Get("/internal/v1/nodes/{nodeID}/drift-events", internalNode.ListDriftEvents)
			})
			r.Group(func(r chi.Router) {
				r.Use(mw.RequireScope("nodes", "write"))
				r.Post("/internal/v1/nodes/{nodeID}/health", internalNode.ReportHealth)
				r.Post("/internal/v1/nodes/{nodeID}/drift-events", internalNode.ReportDriftEvents)
				r.Post("/internal/v1/cron-jobs/{cronJobID}/outcome", internalNode.ReportCronOutcome)
				r.Post("/internal/v1/login-sessions/validate", oidcLogin.ValidateLoginSession)
			})
		})

		// Brand-scoped endpoints — scope middleware per resource group
		// Brands
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("brands", "read"))
			r.Get("/brands", brand.List)
			r.Get("/brands/{id}", brand.Get)
			r.Get("/brands/{id}/clusters", brand.ListClusters)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("brands", "write"))
			r.Post("/brands", brand.Create)
			r.Put("/brands/{id}", brand.Update)
			r.Put("/brands/{id}/clusters", brand.SetClusters)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("brands", "delete"))
			r.Delete("/brands/{id}", brand.Delete)
		})

		// Tenants
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("tenants", "read"))
			r.Get("/tenants", tenant.List)
			r.Get("/tenants/{id}", tenant.Get)
			r.Get("/tenants/{id}/resource-summary", tenant.ResourceSummary)
			r.Get("/tenants/{id}/resource-usage", tenant.ResourceUsage)
			r.Get("/tenants/{tenantID}/logs", logs.TenantLogs)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("tenants", "write"))
			r.Post("/tenants", tenant.Create)
			r.Put("/tenants/{id}", tenant.Update)
			r.Post("/tenants/{id}/suspend", tenant.Suspend)
			r.Post("/tenants/{id}/unsuspend", tenant.Unsuspend)
			r.Post("/tenants/{id}/migrate", tenant.Migrate)
			r.Post("/tenants/{id}/retry", tenant.Retry)
			r.Post("/tenants/{id}/retry-failed", tenant.RetryFailed)
			r.Post("/tenants/{id}/login-sessions", oidcLogin.CreateLoginSession)
			r.Delete("/tenants/{tenantID}/logs", logs.DeleteTenantLogs)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("tenants", "delete"))
			r.Delete("/tenants/{id}", tenant.Delete)
		})

		// Subscriptions
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("subscriptions", "read"))
			r.Get("/tenants/{tenantID}/subscriptions", subscription.ListByTenant)
			r.Get("/subscriptions/{id}", subscription.Get)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("subscriptions", "write"))
			r.Post("/tenants/{tenantID}/subscriptions", subscription.Create)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("subscriptions", "delete"))
			r.Delete("/subscriptions/{id}", subscription.Delete)
		})

		// Webroots
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("webroots", "read"))
			r.Get("/tenants/{tenantID}/webroots", webroot.ListByTenant)
			r.Get("/webroots/{id}", webroot.Get)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("webroots", "write"))
			r.Post("/tenants/{tenantID}/webroots", webroot.Create)
			r.Put("/webroots/{id}", webroot.Update)
			r.Post("/webroots/{id}/retry", webroot.Retry)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("webroots", "delete"))
			r.Delete("/webroots/{id}", webroot.Delete)
		})

		// Webroot env vars
		envVar := handler.NewWebrootEnvVar(s.services)
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("webroots", "read"))
			r.Get("/webroots/{webrootID}/env-vars", envVar.List)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("webroots", "write"))
			r.Put("/webroots/{webrootID}/env-vars", envVar.Set)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("webroots", "delete"))
			r.Delete("/webroots/{webrootID}/env-vars/{name}", envVar.Delete)
		})

		// Vault encrypt/decrypt
		vault := handler.NewVault(s.services)
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("webroots", "write"))
			r.Post("/webroots/{webrootID}/vault/encrypt", vault.Encrypt)
			r.Post("/webroots/{webrootID}/vault/decrypt", vault.Decrypt)
		})

		// Daemons
		daemon := handler.NewDaemon(s.services)
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("daemons", "read"))
			r.Get("/webroots/{webrootID}/daemons", daemon.ListByWebroot)
			r.Get("/daemons/{id}", daemon.Get)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("daemons", "write"))
			r.Post("/webroots/{webrootID}/daemons", daemon.Create)
			r.Put("/daemons/{id}", daemon.Update)
			r.Post("/daemons/{id}/enable", daemon.Enable)
			r.Post("/daemons/{id}/disable", daemon.Disable)
			r.Post("/daemons/{id}/retry", daemon.Retry)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("daemons", "delete"))
			r.Delete("/daemons/{id}", daemon.Delete)
		})

		// Cron jobs
		cronJob := handler.NewCronJob(s.services)
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("cron_jobs", "read"))
			r.Get("/webroots/{webrootID}/cron-jobs", cronJob.ListByWebroot)
			r.Get("/cron-jobs/{id}", cronJob.Get)
})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("cron_jobs", "write"))
			r.Post("/webroots/{webrootID}/cron-jobs", cronJob.Create)
			r.Put("/cron-jobs/{id}", cronJob.Update)
			r.Post("/cron-jobs/{id}/enable", cronJob.Enable)
			r.Post("/cron-jobs/{id}/disable", cronJob.Disable)
			r.Post("/cron-jobs/{id}/retry", cronJob.Retry)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("cron_jobs", "delete"))
			r.Delete("/cron-jobs/{id}", cronJob.Delete)
		})

		// FQDNs
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("fqdns", "read"))
			r.Get("/tenants/{tenantID}/fqdns", fqdn.ListByTenant)
			r.Get("/webroots/{webrootID}/fqdns", fqdn.ListByWebroot)
			r.Get("/fqdns/{id}", fqdn.Get)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("fqdns", "write"))
			r.Post("/tenants/{tenantID}/fqdns", fqdn.Create)
			r.Put("/fqdns/{id}", fqdn.Update)
			r.Post("/fqdns/{id}/retry", fqdn.Retry)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("fqdns", "delete"))
			r.Delete("/fqdns/{id}", fqdn.Delete)
		})

		// Certificates
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("certificates", "read"))
			r.Get("/fqdns/{fqdnID}/certificates", cert.ListByFQDN)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("certificates", "write"))
			r.Post("/fqdns/{fqdnID}/certificates", cert.Upload)
			r.Post("/certificates/{id}/retry", cert.Retry)
		})

		// Zones
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("zones", "read"))
			r.Get("/zones", zone.List)
			r.Get("/zones/{id}", zone.Get)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("zones", "write"))
			r.Post("/zones", zone.Create)
			r.Put("/zones/{id}", zone.Update)
			r.Post("/zones/{id}/retry", zone.Retry)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("zones", "delete"))
			r.Delete("/zones/{id}", zone.Delete)
		})

		// Zone records
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("zone_records", "read"))
			r.Get("/zones/{zoneID}/records", zoneRecord.ListByZone)
			r.Get("/zone-records/{id}", zoneRecord.Get)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("zone_records", "write"))
			r.Post("/zones/{zoneID}/records", zoneRecord.Create)
			r.Put("/zone-records/{id}", zoneRecord.Update)
			r.Post("/zone-records/{id}/retry", zoneRecord.Retry)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("zone_records", "delete"))
			r.Delete("/zone-records/{id}", zoneRecord.Delete)
		})

		// Databases
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("databases", "read"))
			r.Get("/tenants/{tenantID}/databases", database.ListByTenant)
			r.Get("/databases/{id}", database.Get)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("databases", "write"))
			r.Post("/tenants/{tenantID}/databases", database.Create)
			r.Post("/databases/{id}/migrate", database.Migrate)
			r.Post("/databases/{id}/retry", database.Retry)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("databases", "delete"))
			r.Delete("/databases/{id}", database.Delete)
		})

		// Database users
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("database_users", "read"))
			r.Get("/databases/{databaseID}/users", dbUser.ListByDatabase)
			r.Get("/database-users/{id}", dbUser.Get)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("database_users", "write"))
			r.Post("/databases/{databaseID}/users", dbUser.Create)
			r.Put("/database-users/{id}", dbUser.Update)
			r.Post("/database-users/{id}/retry", dbUser.Retry)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("database_users", "delete"))
			r.Delete("/database-users/{id}", dbUser.Delete)
		})

		// Valkey instances
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("valkey", "read"))
			r.Get("/tenants/{tenantID}/valkey-instances", valkeyInstance.ListByTenant)
			r.Get("/valkey-instances/{id}", valkeyInstance.Get)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("valkey", "write"))
			r.Post("/tenants/{tenantID}/valkey-instances", valkeyInstance.Create)
			r.Post("/valkey-instances/{id}/migrate", valkeyInstance.Migrate)
			r.Post("/valkey-instances/{id}/retry", valkeyInstance.Retry)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("valkey", "delete"))
			r.Delete("/valkey-instances/{id}", valkeyInstance.Delete)
		})

		// Valkey users
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("valkey", "read"))
			r.Get("/valkey-instances/{instanceID}/users", valkeyUser.ListByInstance)
			r.Get("/valkey-users/{id}", valkeyUser.Get)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("valkey", "write"))
			r.Post("/valkey-instances/{instanceID}/users", valkeyUser.Create)
			r.Put("/valkey-users/{id}", valkeyUser.Update)
			r.Post("/valkey-users/{id}/retry", valkeyUser.Retry)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("valkey", "delete"))
			r.Delete("/valkey-users/{id}", valkeyUser.Delete)
		})

		// S3 buckets
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("s3", "read"))
			r.Get("/tenants/{tenantID}/s3-buckets", s3Bucket.ListByTenant)
			r.Get("/s3-buckets/{id}", s3Bucket.Get)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("s3", "write"))
			r.Post("/tenants/{tenantID}/s3-buckets", s3Bucket.Create)
			r.Put("/s3-buckets/{id}", s3Bucket.Update)
			r.Post("/s3-buckets/{id}/retry", s3Bucket.Retry)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("s3", "delete"))
			r.Delete("/s3-buckets/{id}", s3Bucket.Delete)
		})

		// S3 access keys
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("s3", "read"))
			r.Get("/s3-buckets/{bucketID}/access-keys", s3AccessKey.ListByBucket)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("s3", "write"))
			r.Post("/s3-buckets/{bucketID}/access-keys", s3AccessKey.Create)
			r.Post("/s3-access-keys/{id}/retry", s3AccessKey.Retry)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("s3", "delete"))
			r.Delete("/s3-access-keys/{id}", s3AccessKey.Delete)
		})

		// SSH keys
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("ssh_keys", "read"))
			r.Get("/tenants/{tenantID}/ssh-keys", sshKey.ListByTenant)
			r.Get("/ssh-keys/{id}", sshKey.Get)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("ssh_keys", "write"))
			r.Post("/tenants/{tenantID}/ssh-keys", sshKey.Create)
			r.Post("/ssh-keys/{id}/retry", sshKey.Retry)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("ssh_keys", "delete"))
			r.Delete("/ssh-keys/{id}", sshKey.Delete)
		})

		// Tenant egress rules
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("network", "read"))
			r.Get("/tenants/{tenantID}/egress-rules", egressRule.ListByTenant)
			r.Get("/egress-rules/{id}", egressRule.Get)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("network", "write"))
			r.Post("/tenants/{tenantID}/egress-rules", egressRule.Create)
			r.Post("/egress-rules/{id}/retry", egressRule.Retry)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("network", "delete"))
			r.Delete("/egress-rules/{id}", egressRule.Delete)
		})

		// Database access rules
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("databases", "read"))
			r.Get("/databases/{databaseID}/access-rules", dbAccessRule.ListByDatabase)
			r.Get("/database-access-rules/{id}", dbAccessRule.Get)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("databases", "write"))
			r.Post("/databases/{databaseID}/access-rules", dbAccessRule.Create)
			r.Post("/database-access-rules/{id}/retry", dbAccessRule.Retry)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("databases", "delete"))
			r.Delete("/database-access-rules/{id}", dbAccessRule.Delete)
		})

		// Email accounts
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("email", "read"))
			r.Get("/tenants/{tenantID}/email-accounts", emailAccount.ListByTenant)
			r.Get("/fqdns/{fqdnID}/email-accounts", emailAccount.ListByFQDN)
			r.Get("/email-accounts/{id}", emailAccount.Get)
			r.Get("/email-accounts/{id}/aliases", emailAlias.ListByAccount)
			r.Get("/email-aliases/{aliasID}", emailAlias.Get)
			r.Get("/email-accounts/{id}/forwards", emailForward.ListByAccount)
			r.Get("/email-forwards/{forwardID}", emailForward.Get)
			r.Get("/email-accounts/{id}/autoreply", emailAutoReply.Get)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("email", "write"))
			r.Post("/fqdns/{fqdnID}/email-accounts", emailAccount.Create)
			r.Post("/email-accounts/{id}/retry", emailAccount.Retry)
			r.Post("/email-accounts/{id}/aliases", emailAlias.Create)
			r.Post("/email-aliases/{aliasID}/retry", emailAlias.Retry)
			r.Post("/email-accounts/{id}/forwards", emailForward.Create)
			r.Post("/email-forwards/{forwardID}/retry", emailForward.Retry)
			r.Put("/email-accounts/{id}/autoreply", emailAutoReply.Put)
			r.Post("/email-autoreplies/{id}/retry", emailAutoReply.Retry)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("email", "delete"))
			r.Delete("/email-accounts/{id}", emailAccount.Delete)
			r.Delete("/email-aliases/{aliasID}", emailAlias.Delete)
			r.Delete("/email-forwards/{forwardID}", emailForward.Delete)
			r.Delete("/email-accounts/{id}/autoreply", emailAutoReply.Delete)
		})

		// Backups
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("backups", "read"))
			r.Get("/tenants/{tenantID}/backups", backup.ListByTenant)
			r.Get("/backups/{id}", backup.Get)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("backups", "write"))
			r.Post("/tenants/{tenantID}/backups", backup.Create)
			r.Post("/backups/{id}/restore", backup.Restore)
			r.Post("/backups/{id}/retry", backup.Retry)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("backups", "delete"))
			r.Delete("/backups/{id}", backup.Delete)
		})

		// Incidents
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("incidents", "read"))
			r.Get("/incidents", incident.List)
			r.Get("/incidents/{id}", incident.Get)
			r.Get("/incidents/{id}/events", incident.ListEvents)
			r.Get("/incidents/{id}/gaps", incident.ListIncidentGaps)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("incidents", "write"))
			r.Post("/incidents", incident.Create)
			r.Patch("/incidents/{id}", incident.Update)
			r.Post("/incidents/{id}/resolve", incident.Resolve)
			r.Post("/incidents/{id}/escalate", incident.Escalate)
			r.Post("/incidents/{id}/cancel", incident.Cancel)
			r.Post("/incidents/{id}/events", incident.AddEvent)
		})

		// Capability Gaps
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("incidents", "read"))
			r.Get("/capability-gaps", capabilityGap.List)
			r.Get("/capability-gaps/{id}/incidents", capabilityGap.ListGapIncidents)
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireScope("incidents", "write"))
			r.Post("/capability-gaps", capabilityGap.Report)
			r.Patch("/capability-gaps/{id}", capabilityGap.Update)
		})
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
