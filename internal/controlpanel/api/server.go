package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"

	"github.com/edvin/hosting/internal/controlpanel/docs"
	"github.com/edvin/hosting/internal/controlpanel/api/handler"
	mw "github.com/edvin/hosting/internal/controlpanel/api/middleware"
	"github.com/edvin/hosting/internal/controlpanel/config"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/hosting"
)

type Server struct {
	router   chi.Router
	logger   zerolog.Logger
	services *core.Services
	pool     *pgxpool.Pool
	cfg      *config.Config
}

func NewServer(logger zerolog.Logger, pool *pgxpool.Pool, cfg *config.Config) *Server {
	services := core.NewServices(pool, cfg.JWTSecret, cfg.JWTIssuer)
	hostingClient := hosting.NewClient(cfg.HostingAPIURL, cfg.HostingAPIKey)

	s := &Server{
		router:   chi.NewRouter(),
		logger:   logger,
		services: services,
		pool:     pool,
		cfg:      cfg,
	}

	s.setupMiddleware()
	s.setupRoutes(hostingClient)

	return s
}

func (s *Server) setupMiddleware() {
	s.router.Use(chimw.RequestID)
	s.router.Use(chimw.RealIP)
	s.router.Use(mw.RequestLogger(s.logger))
	s.router.Use(chimw.Recoverer)
	s.router.Use(mw.Metrics)
	s.router.Use(mw.CORS(s.cfg.CORSOrigins))
}

func (s *Server) setupRoutes(hostingClient *hosting.Client) {
	// Prometheus metrics
	s.router.Handle("/metrics", promhttp.Handler())

	// Health checks
	s.router.Get("/healthz", s.handleHealthz)
	s.router.Get("/readyz", s.handleReadyz)

	// OpenAPI spec and docs (public, no auth)
	s.router.Route("/docs", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(scalarHTML))
		})
		r.Get("/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(docs.SwaggerJSON)
		})
	})

	// Partner-aware routes (partner resolved from hostname)
	s.router.Group(func(r chi.Router) {
		r.Use(mw.Partner(s.services.Partner, s.cfg.DevMode))

		// Public partner info (branding, name)
		partner := handler.NewPartner()
		r.Get("/partner", partner.Get)

		// Auth (no JWT required, but partner required for brand resolution)
		auth := handler.NewAuth(s.services.Auth)
		r.Post("/auth/login", auth.Login)

		// Terminal WebSocket (query-param auth, outside JWT middleware)
		terminal := handler.NewTerminal(s.services.Auth, s.services.Customer, hostingClient, s.cfg.HostingWSURL(), s.cfg.HostingAPIKey)
		r.Get("/api/v1/webroots/{id}/terminal", terminal.Connect)

		// Authenticated API routes
		r.Route("/api/v1", func(r chi.Router) {
			r.Use(mw.Auth(s.services.Auth))

			me := handler.NewMe(s.services.User, s.services.Customer)
			r.Get("/me", me.Get)
			r.Patch("/me", me.Update)

			customer := handler.NewCustomer(s.services.Customer)
			r.Get("/customers", customer.List)

			dashboard := handler.NewDashboard(s.services.Customer, s.services.Subscription, s.services.Module)
			r.Get("/customers/{id}/dashboard", dashboard.Get)

			webroot := handler.NewWebroot(s.services.Customer, s.services.Subscription, hostingClient)
			r.Get("/customers/{cid}/webroots", webroot.ListByCustomer)
			r.Get("/webroots/{id}", webroot.Get)
			r.Put("/webroots/{id}", webroot.Update)
			r.Get("/webroots/{id}/runtimes", webroot.ListRuntimes)
			r.Get("/webroots/{id}/fqdns", webroot.ListFQDNs)
			r.Get("/webroots/{id}/available-fqdns", webroot.ListAvailableFQDNs)
			r.Post("/webroots/{id}/fqdns/{fqdnId}/attach", webroot.AttachFQDN)
			r.Post("/webroots/{id}/fqdns/{fqdnId}/detach", webroot.DetachFQDN)

			envvar := handler.NewEnvVar(s.services.Customer, hostingClient)
			r.Get("/webroots/{id}/env-vars", envvar.List)
			r.Put("/webroots/{id}/env-vars", envvar.Set)
			r.Delete("/webroots/{id}/env-vars/{name}", envvar.Delete)
			r.Post("/webroots/{id}/vault/encrypt", envvar.VaultEncrypt)
			r.Post("/webroots/{id}/vault/decrypt", envvar.VaultDecrypt)

			// Daemons (webroot sub-resource)
			daemon := handler.NewDaemonHandler(s.services.Customer, hostingClient)
			r.Get("/webroots/{id}/daemons", daemon.List)
			r.Post("/webroots/{id}/daemons", daemon.Create)
			r.Put("/daemons/{id}", daemon.Update)
			r.Post("/daemons/{id}/enable", daemon.Enable)
			r.Post("/daemons/{id}/disable", daemon.Disable)
			r.Delete("/daemons/{id}", daemon.Delete)

			// Cron Jobs (webroot sub-resource)
			cronjob := handler.NewCronJobHandler(s.services.Customer, hostingClient)
			r.Get("/webroots/{id}/cron-jobs", cronjob.List)
			r.Post("/webroots/{id}/cron-jobs", cronjob.Create)
			r.Put("/cron-jobs/{id}", cronjob.Update)
			r.Post("/cron-jobs/{id}/enable", cronjob.Enable)
			r.Post("/cron-jobs/{id}/disable", cronjob.Disable)
			r.Delete("/cron-jobs/{id}", cronjob.Delete)

			// Databases
			database := handler.NewDatabaseHandler(s.services.Customer, s.services.Subscription, hostingClient)
			r.Get("/customers/{cid}/databases", database.ListByCustomer)
			r.Get("/databases/{id}", database.Get)
			r.Delete("/databases/{id}", database.Delete)
			r.Get("/databases/{id}/users", database.ListUsers)
			r.Post("/databases/{id}/users", database.CreateUser)
			r.Put("/databases/{id}/users/{userId}", database.UpdateUser)
			r.Delete("/databases/{id}/users/{userId}", database.DeleteUser)
			r.Post("/databases/{id}/login-session", database.CreateLoginSession)

			// Valkey
			valkey := handler.NewValkeyHandler(s.services.Customer, s.services.Subscription, hostingClient)
			r.Get("/customers/{cid}/valkey", valkey.ListByCustomer)
			r.Get("/valkey/{id}", valkey.Get)
			r.Delete("/valkey/{id}", valkey.Delete)
			r.Get("/valkey/{id}/users", valkey.ListUsers)
			r.Post("/valkey/{id}/users", valkey.CreateUser)
			r.Put("/valkey/{id}/users/{userId}", valkey.UpdateUser)
			r.Delete("/valkey/{id}/users/{userId}", valkey.DeleteUser)

			// S3 Buckets
			s3bucket := handler.NewS3BucketHandler(s.services.Customer, s.services.Subscription, hostingClient)
			r.Get("/customers/{cid}/s3-buckets", s3bucket.ListByCustomer)
			r.Get("/s3-buckets/{id}", s3bucket.Get)
			r.Put("/s3-buckets/{id}", s3bucket.Update)
			r.Delete("/s3-buckets/{id}", s3bucket.Delete)
			r.Get("/s3-buckets/{id}/access-keys", s3bucket.ListAccessKeys)
			r.Post("/s3-buckets/{id}/access-keys", s3bucket.CreateAccessKey)
			r.Delete("/s3-buckets/{id}/access-keys/{keyId}", s3bucket.DeleteAccessKey)

			// Email
			email := handler.NewEmailHandler(s.services.Customer, s.services.Subscription, hostingClient)
			r.Get("/customers/{cid}/email", email.ListByCustomer)
			r.Get("/email/{id}", email.Get)
			r.Delete("/email/{id}", email.Delete)
			r.Get("/email/{id}/aliases", email.ListAliases)
			r.Post("/email/{id}/aliases", email.CreateAlias)
			r.Delete("/email/{id}/aliases/{aliasId}", email.DeleteAlias)
			r.Get("/email/{id}/forwards", email.ListForwards)
			r.Post("/email/{id}/forwards", email.CreateForward)
			r.Delete("/email/{id}/forwards/{forwardId}", email.DeleteForward)
			r.Get("/email/{id}/autoreply", email.GetAutoreply)
			r.Put("/email/{id}/autoreply", email.SetAutoreply)
			r.Delete("/email/{id}/autoreply", email.DeleteAutoreply)

			// DNS Zones
			dns := handler.NewDNSHandler(s.services.Customer, s.services.Subscription, hostingClient)
			r.Get("/customers/{cid}/dns-zones", dns.ListByCustomer)
			r.Get("/dns-zones/{id}", dns.Get)
			r.Put("/dns-zones/{id}", dns.Update)
			r.Delete("/dns-zones/{id}", dns.Delete)
			r.Get("/dns-zones/{id}/records", dns.ListRecords)
			r.Post("/dns-zones/{id}/records", dns.CreateRecord)
			r.Put("/dns-zones/{id}/records/{recordId}", dns.UpdateRecord)
			r.Delete("/dns-zones/{id}/records/{recordId}", dns.DeleteRecord)

			// WireGuard
			wg := handler.NewWireGuardHandler(s.services.Customer, s.services.Subscription, hostingClient)
			r.Get("/customers/{cid}/wireguard", wg.ListByCustomer)
			r.Get("/wireguard/{id}", wg.Get)
			r.Post("/customers/{cid}/wireguard", wg.Create)
			r.Delete("/wireguard/{id}", wg.Delete)

			// SSH Keys
			sshkey := handler.NewSSHKeyHandler(s.services.Customer, s.services.Subscription, hostingClient)
			r.Get("/customers/{cid}/ssh-keys", sshkey.ListByCustomer)
			r.Post("/customers/{cid}/ssh-keys", sshkey.Create)
			r.Delete("/ssh-keys/{id}", sshkey.Delete)

			// Backups
			backup := handler.NewBackupHandler(s.services.Customer, s.services.Subscription, hostingClient)
			r.Get("/customers/{cid}/backups", backup.ListByCustomer)
			r.Post("/customers/{cid}/backups", backup.Create)
			r.Post("/backups/{id}/restore", backup.Restore)
			r.Delete("/backups/{id}", backup.Delete)
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

	if err := s.pool.Ping(ctx); err != nil {
		checks["db"] = err.Error()
		healthy = false
	} else {
		checks["db"] = "ok"
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
  <title>Control Panel API</title>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
</head>
<body>
  <script id="api-reference" data-url="/docs/openapi.json"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`
