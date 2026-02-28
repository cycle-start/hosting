package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/agent"
	hostingworkflow "github.com/edvin/hosting/internal/workflow"
	"github.com/edvin/hosting/internal/config"
	"github.com/edvin/hosting/internal/logging"
	"github.com/edvin/hosting/internal/metrics"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.Validate("node-agent"); err != nil {
		fmt.Fprintf(os.Stderr, "invalid config: %v\n", err)
		os.Exit(1)
	}

	logger := logging.NewLogger(cfg)

	agentCfg := agent.Config{
		MySQLDSN:          cfg.MySQLDSN,
		MySQLReplPassword: getEnv("MYSQL_REPL_PASSWORD", ""),
		NginxConfigDir:    getEnv("NGINX_CONFIG_DIR", "/etc/nginx"),
		NginxListenPort: getEnv("NGINX_LISTEN_PORT", "80"),
		WebStorageDir:   getEnv("WEB_STORAGE_DIR", "/var/www/storage"),
		CertDir:         getEnv("CERT_DIR", "/etc/ssl/hosting"),
		ValkeyConfigDir: getEnv("VALKEY_CONFIG_DIR", "/etc/valkey"),
		ValkeyDataDir:   getEnv("VALKEY_DATA_DIR", "/var/lib/valkey"),
		InitSystem:      getEnv("INIT_SYSTEM", "direct"),
		ShardName:       cfg.ShardName,
	}

	srv := agent.NewServer(logger, agentCfg)

	tlsConfig, err := cfg.TemporalTLS()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to configure temporal TLS")
	}
	dialOpts := temporalclient.Options{HostPort: cfg.TemporalAddress}
	if tlsConfig != nil {
		dialOpts.ConnectionOptions = temporalclient.ConnectionOptions{TLS: tlsConfig}
		logger.Info().Msg("temporal mTLS enabled")
	}
	tc, err := temporalclient.Dial(dialOpts)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to temporal")
	}
	defer tc.Close()

	taskQueue := "node-" + cfg.NodeID
	w := worker.New(tc, taskQueue, worker.Options{
		Interceptors: []interceptor.WorkerInterceptor{&hostingworkflow.ErrorTypingInterceptor{}},
	})

	s3Mgr := agent.NewS3Manager(
		logger,
		getEnv("RGW_ENDPOINT", "http://localhost:7480"),
		getEnv("RGW_ADMIN_ACCESS_KEY", ""),
		getEnv("RGW_ADMIN_SECRET_KEY", ""),
	)

	nodeActs := activity.NewNodeLocal(
		logger,
		srv.TenantManager(),
		srv.WebrootManager(),
		srv.NginxManager(),
		srv.DatabaseManager(),
		srv.ValkeyManager(),
		s3Mgr,
		srv.SSHManager(),
		srv.CronManager(),
		srv.DaemonManager(),
		srv.TenantULAManager(),
		agent.NewWireGuardManager(logger),
		srv.Runtimes(),
	)
	w.RegisterActivity(nodeActs)

	nodeACMEActs := activity.NewNodeACMEActivity()
	w.RegisterActivity(nodeACMEActs)

	nodeLBActs := activity.NewNodeLB(logger)
	w.RegisterActivity(nodeLBActs)

	if cfg.MetricsAddr != "" {
		infoGauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "node_agent_info",
			Help: "Node agent instance info (always 1)",
		}, []string{"node_id", "node_role", "shard", "region", "cluster"})
		prometheus.MustRegister(infoGauge)
		infoGauge.WithLabelValues(cfg.NodeID, cfg.NodeRole, cfg.ShardName, cfg.RegionID, cfg.ClusterID).Set(1)

		metricsSrv := metrics.NewServer(cfg.MetricsAddr)
		go func() {
			logger.Info().Str("addr", cfg.MetricsAddr).Msg("starting metrics server")
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error().Err(err).Msg("metrics server failed")
			}
		}()
	}

	// Block startup until CephFS is mounted on web nodes. This prevents the
	// Temporal worker from accepting tasks that will immediately fail because
	// the storage filesystem isn't ready. Retries every 5s for up to 2 minutes;
	// if CephFS still isn't available, fatal out and let systemd restart us.
	if cfg.NodeRole == "web" && os.Getenv("CEPHFS_ENABLED") != "false" {
		webStorageDir := agentCfg.WebStorageDir
		const (
			retryInterval = 5 * time.Second
			maxWait       = 2 * time.Minute
		)
		deadline := time.Now().Add(maxWait)
		for {
			if err := agent.CheckMount(webStorageDir); err == nil {
				logger.Info().Str("path", webStorageDir).Msg("CephFS mount verified")
				break
			}
			if time.Now().After(deadline) {
				logger.Fatal().
					Str("path", webStorageDir).
					Dur("waited", maxWait).
					Msg("CephFS not mounted after timeout, exiting")
			}
			logger.Warn().
				Str("path", webStorageDir).
				Msg("CephFS not mounted, retrying...")
			time.Sleep(retryInterval)
		}
	}

	logger.Info().
		Str("nodeID", cfg.NodeID).
		Str("taskQueue", taskQueue).
		Msg("starting node agent temporal worker")

	if err := w.Run(worker.InterruptCh()); err != nil {
		logger.Fatal().Err(err).Msg("worker failed")
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
